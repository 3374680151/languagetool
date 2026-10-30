// visitor_repo.go 是匿名访客身份的仓储（FR-1104）。
//
// @file internal/store/visitor_repo.go
// 文件职责：visitors 表的写入与续期——首访插入 UUID，之后每次请求滑动续期
// last_seen_at（60 天滑动）；无注册、无账号。
//
// 主要内容：VisitorRepo、Upsert。
//
// 模块边界：只依赖 GORM；被 httpapi 的 visitor 中间件调用（异步续期，不阻塞请求）。
package store

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VisitorRepo 读写 visitors 表。
type VisitorRepo struct {
	db *gorm.DB
}

// NewVisitorRepo 构造仓储。
func NewVisitorRepo(db *gorm.DB) *VisitorRepo {
	return &VisitorRepo{db: db}
}

// Touch 插入或续期一个访客。幂等：同一 UUID 重复调用只更新 last_seen_at。
// 续期失败不阻断请求（身份已随 cookie 成立），由调用方记日志。
func (r *VisitorRepo) Touch(id string, now time.Time) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{"last_seen_at": now}),
	}).Create(&Visitor{ID: id, CreatedAt: now, LastSeenAt: now}).Error
}
