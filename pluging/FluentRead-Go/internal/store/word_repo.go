// word_repo.go 是词表（收藏词）的存取层（FR-702/1105/1202/1204，P5 全量）。
//
// @file internal/store/word_repo.go
// 文件职责：words 表 CRUD——收藏（幂等：同访客同词返回已有记录）、列表
//（最近收藏/最少使用排序）、删除、使用次数标记（再使用 FR-1202）、
// 复习调度字段（next_due_at/stage，FR-1204）；查词卡的已收藏状态查询。
//
// 主要内容：WordRepo、Collect、List、Delete、MarkUsed、GetCollected、Due、Schedule。
//
// 模块边界：只依赖 GORM；复习间隔常量 ReviewStages（10min/1/3/7/14/30 天，FR-1204）。
package store

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReviewStages 是复习间隔序列（FR-1204）：10 分钟/1/3/7/14/30 天。
var ReviewStages = []time.Duration{
	10 * time.Minute,
	24 * time.Hour,
	3 * 24 * time.Hour,
	7 * 24 * time.Hour,
	14 * 24 * time.Hour,
	30 * 24 * time.Hour,
}

// normalizeWord 归一化查询词：小写、去首尾空白与常见标点（§八 8.2 预处理规则）。
func normalizeWord(w string) string {
	w = strings.ToLower(strings.TrimSpace(w))
	w = strings.Trim(w, ",.!?;:\"'`()[]{}")
	return w
}

// WordRepo 读写 words。
type WordRepo struct {
	db *gorm.DB
}

// NewWordRepo 构造仓储。
func NewWordRepo(db *gorm.DB) *WordRepo {
	return &WordRepo{db: db}
}

// Collect 收藏一个词（幂等）：同访客同词返回已有记录，不重复建行（§八 8.5）。
// 收藏即进入复习调度：next_due_at = 现在 + 第一档间隔（10 分钟）。
func (r *WordRepo) Collect(visitorID, word, lang, contextSentence, sourceURL string) (*Word, bool, error) {
	word = normalizeWord(word)
	var existing Word
	err := r.db.Where("visitor_id = ? AND word = ?", visitorID, word).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}
	now := time.Now()
	due := now.Add(ReviewStages[0])
	if len(contextSentence) > 300 {
		contextSentence = contextSentence[:300]
	}
	g := &Word{
		VisitorID: visitorID, Word: word, Lang: lang,
		ContextSentence: contextSentence, SourceURL: sourceURL,
		NextDueAt: &due, Stage: 0, CreatedAt: now,
	}
	if err := r.db.Create(g).Error; err != nil {
		return nil, false, err
	}
	return g, true, nil
}

// List 返回词表，按最近收藏或最少使用排序（FR-1202）。
func (r *WordRepo) List(visitorID, sort string) ([]Word, error) {
	q := r.db.Where("visitor_id = ?", visitorID)
	switch sort {
	case "least_used":
		q = q.Order("use_count asc, created_at desc")
	default:
		q = q.Order("created_at desc")
	}
	var out []Word
	if err := q.Limit(500).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Delete 删除一个收藏词（只能删自己的）。
func (r *WordRepo) Delete(visitorID string, id int64) error {
	return r.db.Where("visitor_id = ?", visitorID).Delete(&Word{}, id).Error
}

// MarkUsed 使用次数 +1 并刷新 last_used_at（FR-1202：再使用标记）。
// 幂等由调用方保证（每段每次翻译只标一次）。
func (r *WordRepo) MarkUsed(visitorID string, words []string, now time.Time) error {
	if len(words) == 0 {
		return nil
	}
	return r.db.Model(&Word{}).
		Where("visitor_id = ? AND word IN ?", visitorID, words).
		Updates(map[string]any{
			"use_count":    gorm.Expr("use_count + 1"),
			"last_used_at": now,
		}).Error
}

// WordsOfVisitor 返回该访客的全部收藏词（供再使用匹配）。
func (r *WordRepo) WordsOfVisitor(visitorID string) ([]string, error) {
	var out []string
	if err := r.db.Model(&Word{}).Where("visitor_id = ?", visitorID).Pluck("word", &out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// GetCollected 返回某个词的收藏状态（查词卡"已收藏"态，§八 8.5）。
func (r *WordRepo) GetCollected(visitorID, word string) (*Word, bool) {
	var w Word
	err := r.db.Where("visitor_id = ? AND word = ?", visitorID, normalizeWord(word)).First(&w).Error
	if err != nil {
		return nil, false
	}
	return &w, true
}

// Due 返回到期的复习材料（词表部分，FR-1204）。
func (r *WordRepo) Due(visitorID string, now time.Time) ([]Word, error) {
	var out []Word
	if err := r.db.Where("visitor_id = ? AND next_due_at IS NOT NULL AND next_due_at <= ?", visitorID, now).
		Order("next_due_at asc").Limit(100).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Schedule 复习评分后的调度（FR-1204）：记住进一档，没记住回退一档。
func (r *WordRepo) Schedule(visitorID string, id int64, remembered bool, now time.Time) (*Word, error) {
	var w Word
	if err := r.db.Where("id = ? AND visitor_id = ?", id, visitorID).First(&w).Error; err != nil {
		return nil, err
	}
	stage := w.Stage
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
	w.Stage = stage
	w.NextDueAt = &due
	if err := r.db.Save(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

// 保持 clause 引用（词表唯一索引在 AutoMigrate 声明）。
var _ = clause.OnConflict{}
