// glossary_repo.go 是术语的存取层（FR-501/505）。
//
// @file internal/store/glossary_repo.go
// 文件职责：glossaries 表 CRUD；冲突按「站点 > 用户」消解（FR-505）；
// 返回合并视图 + 指纹（FR-502/503 的"译文已过期"判定依据）。
//
// 主要内容：GlossaryRepo、Merged、Fingerprint、Add/Update/Delete。
//
// 模块边界：只依赖 GORM；指纹计算用 SHA-256（与缓存键的术语部分同源）。
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// GlossaryRepo 读写 glossaries。
type GlossaryRepo struct {
	db *gorm.DB
}

// NewGlossaryRepo 构造仓储。
func NewGlossaryRepo(db *gorm.DB) *GlossaryRepo {
	return &GlossaryRepo{db: db}
}

// Merged 返回合并视图：站点级打底，用户级覆盖同名词（FR-505：站点 > 用户）。
// 只含 enabled 的术语。
func (r *GlossaryRepo) Merged(visitorID string) ([]Glossary, error) {
	var site []Glossary
	if err := r.db.Where("scope = ? AND enabled = ?", "site", true).
		Order("source_term asc").Find(&site).Error; err != nil {
		return nil, err
	}
	var user []Glossary
	if visitorID != "" {
		if err := r.db.Where("scope = ? AND visitor_id = ? AND enabled = ?", "user", visitorID, true).
			Order("source_term asc").Find(&user).Error; err != nil {
			return nil, err
		}
	}
	// 冲突消解（FR-505）：站点 > 用户。用户级打底，站点级覆盖同名词。
	merged := make([]Glossary, 0, len(site)+len(user))
	index := make(map[string]int, len(user))
	for _, g := range user {
		index[strings.ToLower(g.SourceTerm)] = len(merged)
		merged = append(merged, g)
	}
	for _, g := range site {
		if idx, ok := index[strings.ToLower(g.SourceTerm)]; ok {
			merged[idx] = g // 站点级覆盖
			continue
		}
		merged = append(merged, g)
	}
	return merged, nil
}

// Fingerprint 计算合并术语的指纹（FR-502/503）：术语变更 → 指纹变化。
func Fingerprint(terms []Glossary) string {
	pairs := make([]string, 0, len(terms))
	for _, g := range terms {
		pairs = append(pairs, g.SourceTerm+"→"+g.TargetTerm)
	}
	sort.Strings(pairs)
	h := sha256.Sum256([]byte(strings.Join(pairs, ";")))
	return hex.EncodeToString(h[:])[:16]
}

// Add 新增术语。站点级同名词唯一（应用层查重，避免依赖局部索引）。
func (r *GlossaryRepo) Add(scope, visitorID, sourceTerm, targetTerm, note string) (*Glossary, error) {
	sourceTerm = strings.TrimSpace(sourceTerm)
	targetTerm = strings.TrimSpace(targetTerm)
	if sourceTerm == "" || targetTerm == "" {
		return nil, ErrGlossaryEmpty
	}
	var count int64
	q := r.db.Model(&Glossary{}).Where("scope = ? AND source_term = ?", scope, sourceTerm)
	if scope == "user" {
		q = q.Where("visitor_id = ?", visitorID)
	}
	if err := q.Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrGlossaryDuplicate
	}
	g := &Glossary{
		Scope: scope, VisitorID: visitorID,
		SourceTerm: sourceTerm, TargetTerm: targetTerm,
		Note: note, Enabled: true,
	}
	if err := r.db.Create(g).Error; err != nil {
		return nil, err
	}
	return g, nil
}

// Update 更新术语（只改自己作用域里的记录）。
func (r *GlossaryRepo) Update(id int64, visitorID string, mutate func(*Glossary)) (*Glossary, error) {
	var g Glossary
	if err := r.db.First(&g, id).Error; err != nil {
		return nil, err
	}
	// 站点级共享：任何访客都能改（无账号体系，规格如此）；用户级只能改自己的。
	if g.Scope == "user" && g.VisitorID != visitorID {
		return nil, ErrGlossaryForbidden
	}
	mutate(&g)
	if err := r.db.Save(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// Delete 删除术语（作用域规则同 Update）。
func (r *GlossaryRepo) Delete(id int64, visitorID string) error {
	var g Glossary
	if err := r.db.First(&g, id).Error; err != nil {
		return err
	}
	if g.Scope == "user" && g.VisitorID != visitorID {
		return ErrGlossaryForbidden
	}
	return r.db.Delete(&g).Error
}

// 术语仓储的业务错误（handler 映射为 400/403）。
var (
	ErrGlossaryEmpty     = errGlossary("术语原文与译法都不能为空")
	ErrGlossaryDuplicate = errGlossary("同一条术语已存在")
	ErrGlossaryForbidden = errGlossary("不能修改其他访客的术语")
)

type errGlossary string

func (e errGlossary) Error() string { return string(e) }
