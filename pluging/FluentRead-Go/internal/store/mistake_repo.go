// mistake_repo.go 是错误本的存取层（FR-1203/1204/1205，P5.5）。
//
// @file internal/store/mistake_repo.go
// 文件职责：mistakes 表 CRUD——只在用户"接受"纠错建议时写入片段（NFR-6 例外）、
// 类别筛选、单条删除、确认清空；复习调度（与词表同一间隔序列）。
//
// 主要内容：MistakeRepo、Add、List、Delete、Clear、Due、Schedule。
//
// 模块边界：只依赖 GORM；ReviewStages 复用 word_repo.go 的定义。
package store

import (
	"time"

	"gorm.io/gorm"
)

// MistakeRepo 读写 mistakes。
type MistakeRepo struct {
	db *gorm.DB
}

// NewMistakeRepo 构造仓储。
func NewMistakeRepo(db *gorm.DB) *MistakeRepo {
	return &MistakeRepo{db: db}
}

// Add 在用户"接受"纠错建议时写入一条片段（FR-1203）。忽略不写（由调用方保证）。
func (r *MistakeRepo) Add(visitorID, sourceSnippet, correctedSnippet, category, reason string) (*Mistake, error) {
	now := time.Now()
	due := now.Add(ReviewStages[0])
	if len(sourceSnippet) > 300 {
		sourceSnippet = sourceSnippet[:300]
	}
	if len(correctedSnippet) > 300 {
		correctedSnippet = correctedSnippet[:300]
	}
	m := &Mistake{
		VisitorID: visitorID,
		SourceSnippet: sourceSnippet, CorrectedSnippet: correctedSnippet,
		Category: category, Reason: reason,
		NextDueAt: &due, Stage: 0, CreatedAt: now,
	}
	if err := r.db.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// List 返回错误本，可按类别筛选（FR-1203）。
func (r *MistakeRepo) List(visitorID, category string) ([]Mistake, error) {
	q := r.db.Where("visitor_id = ?", visitorID)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	var out []Mistake
	if err := q.Order("created_at desc").Limit(500).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Delete 单条删除（只能删自己的）。
func (r *MistakeRepo) Delete(visitorID string, id int64) error {
	return r.db.Where("visitor_id = ?", visitorID).Delete(&Mistake{}, id).Error
}

// Clear 确认清空（FR-1205；二次确认由前端完成）。
func (r *MistakeRepo) Clear(visitorID string) error {
	return r.db.Where("visitor_id = ?", visitorID).Delete(&Mistake{}).Error
}

// Due 返回到期的错误条目（FR-1204）。
func (r *MistakeRepo) Due(visitorID string, now time.Time) ([]Mistake, error) {
	var out []Mistake
	if err := r.db.Where("visitor_id = ? AND next_due_at IS NOT NULL AND next_due_at <= ?", visitorID, now).
		Order("next_due_at asc").Limit(100).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Schedule 复习评分后的调度（FR-1204）：记住进一档，没记住回退一档。
func (r *MistakeRepo) Schedule(visitorID string, id int64, remembered bool, now time.Time) (*Mistake, error) {
	var m Mistake
	if err := r.db.Where("id = ? AND visitor_id = ?", id, visitorID).First(&m).Error; err != nil {
		return nil, err
	}
	stage := m.Stage
	if remembered {
		if stage < len(ReviewStages)-1 {
			stage++
		}
	} else {
		if stage > 0 {
			stage--
		}
	}
	due := now.Add(ReviewStages[stage])
	m.Stage = stage
	m.NextDueAt = &due
	if err := r.db.Save(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}
