// prefs_repo.go 是用户偏好的存取层（FR-1001）。
//
// @file internal/store/prefs_repo.go
// 文件职责：user_prefs 表的读写——(visitor_id, key) 复合主键，value 存 JSON 字符串；
// 只存偏好 6 项，不存任何正文（NFR-6）。
//
// 主要内容：PrefsRepo、GetAll、Put。
//
// 模块边界：只依赖 GORM；被 httpapi 的 prefs handler 调用。
package store

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PrefsRepo 读写 user_prefs。
type PrefsRepo struct {
	db *gorm.DB
}

// NewPrefsRepo 构造仓储。
func NewPrefsRepo(db *gorm.DB) *PrefsRepo {
	return &PrefsRepo{db: db}
}

// GetAll 返回该访客的全部偏好键值。
func (r *PrefsRepo) GetAll(visitorID string) (map[string]string, error) {
	if visitorID == "" {
		return map[string]string{}, nil
	}
	var rows []UserPref
	if err := r.db.Where("visitor_id = ?", visitorID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Key] = row.Value
	}
	return out, nil
}

// Put 写入一条偏好，幂等覆盖。
func (r *PrefsRepo) Put(visitorID, key, value string) error {
	if visitorID == "" {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "visitor_id"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": value}),
	}).Create(&UserPref{VisitorID: visitorID, Key: key, Value: value}).Error
}
