// usage_repo.go 是配额与用量的存取层（FR-601/604，P1.5）。
//
// @file internal/store/usage_repo.go
// 文件职责：访客日配额的"预扣"（Consume：检查与累加同一临界区，超限不写库）、
// 失败数补记、站点级付费字符累计（FR-603 封顶依据）。两张 usage 表都只有数字（NFR-6）。
//
// 主要内容：QuotaDecision、UsageRepo、Consume、AddFailures、TodayUsed、
// AddSiteUsage、PaidCharsToday。
//
// 模块边界：只依赖 GORM；被 service.translate 在"调用上游之前"与"结算时"调用。
// 并发正确性：单机单实例 + 进程内互斥；配额判定必须发生在打上游之前（先花钱再拒绝等于没拦）。
package store

import (
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QuotaDecision 是一次配额判定的结果。
type QuotaDecision struct {
	// Allowed 为 false 表示本次请求会推过上限，调用方必须直接拒绝。
	Allowed bool
	// Used 是判定前的已用段数。
	Used int64
	// Limit 是日上限段数；0 表示不限。
	Limit int64
	// Remaining 是判定后的剩余段数；不限时为 -1。
	Remaining int64
	// ResetAt 是额度恢复时刻（本地时区次日零点）。
	ResetAt time.Time
}

// UsageRepo 读写 usage_daily 与 site_usage_daily。
type UsageRepo struct {
	db *gorm.DB
	// mu 串行化配额判定。SQLite 交错的读-改-写会产生超发，锁是最朴素正确的做法。
	mu sync.Mutex
	// now 可被测试替换。
	now func() time.Time
}

// NewUsageRepo 构造仓储。
func NewUsageRepo(db *gorm.DB) *UsageRepo {
	return &UsageRepo{db: db, now: time.Now}
}

// Now 供测试替换时钟。
func (r *UsageRepo) Now() func() time.Time { return r.now }

// SetNow 供测试替换时钟。
func (r *UsageRepo) SetNow(f func() time.Time) { r.now = f }

// Day 返回用量所属的日期字符串（本地时区）。
func Day(t time.Time) string { return t.Format("2006-01-02") }

// ResetAt 返回 t 所在自然日的次日零点，即配额恢复时刻。
func ResetAt(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
}

// Consume 尝试为本次请求预扣 segments 段、chars 字符（FR-604 计费口径：
// 按段计，空白段由调用方先过滤掉，不会传进来）。
// limit<=0 视为不限（仍累加用量，便于站方观察）。
// 超过上限时不写库，返回 Allowed=false 与当前用量。
func (r *UsageRepo) Consume(visitorID string, segments, chars, limit int) (QuotaDecision, error) {
	now := r.now()
	day := Day(now)
	decision := QuotaDecision{Limit: int64(limit), ResetAt: ResetAt(now)}

	if visitorID == "" {
		// 没有访客 id 就无法归属：放行但不记账，避免把用量算到空 id 上。
		decision.Allowed = true
		decision.Remaining = -1
		return decision, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var row UsageDaily
	err := r.db.Where("visitor_id = ? AND date = ?", visitorID, day).First(&row).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return QuotaDecision{}, err
	}

	decision.Used = row.Segments
	if limit > 0 {
		decision.Remaining = int64(limit) - row.Segments
		if decision.Remaining < 0 {
			decision.Remaining = 0
		}
		// "已用 + 本次 > 上限"才拒绝：恰好用完的那次允许通过。
		if row.Segments+int64(segments) > int64(limit) {
			return decision, nil
		}
	} else {
		decision.Remaining = -1
	}

	if err := r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "visitor_id"}, {Name: "date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"segments":   gorm.Expr("segments + ?", segments),
			"chars":      gorm.Expr("chars + ?", chars),
			"updated_at": now,
		}),
	}).Create(&UsageDaily{
		VisitorID: visitorID, Date: day,
		Segments: int64(segments), Chars: int64(chars),
	}).Error; err != nil {
		return QuotaDecision{}, err
	}

	decision.Allowed = true
	if limit > 0 {
		decision.Remaining = int64(limit) - row.Segments - int64(segments)
		if decision.Remaining < 0 {
			decision.Remaining = 0
		}
	}
	return decision, nil
}

// AddFailures 累加失败段数。失败不退款：段已经送去上游，退款会让
// "故意制造失败"变成绕过配额的手段。
func (r *UsageRepo) AddFailures(visitorID string, failures int) error {
	if visitorID == "" || failures <= 0 {
		return nil
	}
	return r.db.Model(&UsageDaily{}).
		Where("visitor_id = ? AND date = ?", visitorID, Day(r.now())).
		Update("failures", gorm.Expr("failures + ?", failures)).Error
}

// TodayUsed 返回某访客今天的已用段数；没有记录时返回 0。
func (r *UsageRepo) TodayUsed(visitorID string) (int64, error) {
	if visitorID == "" {
		return 0, nil
	}
	var row UsageDaily
	err := r.db.Select("segments").Where("visitor_id = ? AND date = ?", visitorID, Day(r.now())).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.Segments, nil
}

// SiteUsage 是站点级日用量聚合。
type SiteUsage struct {
	PaidChars int64
	FreeChars int64
	Translate int64
	Proofread int64
	Degraded  int64
}

// AddSiteUsage 累加站点级用量（只在结算时调用；delta 为 0 的项跳过写库）。
func (r *UsageRepo) AddSiteUsage(paidChars, freeChars, translateReq, proofreadReq, degradedReq int) error {
	now := r.now()
	if paidChars == 0 && freeChars == 0 && translateReq == 0 && proofreadReq == 0 && degradedReq == 0 {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"paid_chars":    gorm.Expr("paid_chars + ?", paidChars),
			"free_chars":    gorm.Expr("free_chars + ?", freeChars),
			"translate_req": gorm.Expr("translate_req + ?", translateReq),
			"proofread_req": gorm.Expr("proofread_req + ?", proofreadReq),
			"degraded_req":  gorm.Expr("degraded_req + ?", degradedReq),
		}),
	}).Create(&SiteUsageDaily{
		Date: Day(now),
		PaidChars: int64(paidChars), FreeChars: int64(freeChars),
		TranslateReq: int64(translateReq), ProofreadReq: int64(proofreadReq),
		DegradedReq: int64(degradedReq),
	}).Error
}

// PaidCharsToday 返回今天站点付费线路已消耗的字符数；无记录为 0（FR-603 判定依据）。
func (r *UsageRepo) PaidCharsToday() (int64, error) {
	var row SiteUsageDaily
	err := r.db.Select("paid_chars").Where("date = ?", Day(r.now())).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.PaidChars, nil
}
