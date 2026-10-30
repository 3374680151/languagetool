// glossary.go 实现术语 CRUD（P4，FR-501~505，§7.4）。
//
// @file internal/httpapi/handler/glossary.go
// 文件职责：GET（合并视图+指纹）/POST/PUT/DELETE /api/glossary；
// scope=site 全站共享、scope=user 随访客；冲突按「站点 > 用户」消解（FR-505）。
// 指纹用于前端"术语表改过了，当前译文还是旧的"横幅（FR-503）。
//
// 主要内容：Glossary、glossaryView。
//
// 模块边界：存取在 store.GlossaryRepo；翻译时的术语注入在 service（提示词，FR-502）。
package handler

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/store"
)

// glossaryView 是合并视图 + 指纹。
type glossaryView struct {
	Items       []store.Glossary `json:"items"`
	Fingerprint string           `json:"fingerprint"`
}

// view 组装合并视图。
func (a *App) view(c *gin.Context) (*glossaryView, error) {
	visitor := middleware.VisitorID(c)
	merged, err := a.GlossaryRepo.Merged(visitor)
	if err != nil {
		return nil, err
	}
	return &glossaryView{Items: merged, Fingerprint: store.Fingerprint(merged)}, nil
}

// GlossaryList GET /api/glossary。
func (a *App) GlossaryList(c *gin.Context) {
	v, err := a.view(c)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, v)
}

// glossaryBody 是 POST/PUT 请求体。
type glossaryBody struct {
	ID          int64  `json:"id"`
	Scope       string `json:"scope"`
	SourceTerm  string `json:"sourceTerm"`
	TargetTerm  string `json:"targetTerm"`
	Note        string `json:"note"`
	Enabled     *bool  `json:"enabled"`
}

// GlossaryCreate POST /api/glossary。
func (a *App) GlossaryCreate(c *gin.Context) {
	var body glossaryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}
	scope := body.Scope
	if scope != "site" && scope != "user" {
		Fail(c, errs.InvalidRequest("scope 只支持 site/user"))
		return
	}
	visitor := middleware.VisitorID(c)
	g, err := a.GlossaryRepo.Add(scope, visitorIDFor(scope, visitor), body.SourceTerm, body.TargetTerm, body.Note)
	if err != nil {
		slog.Warn("术语新增失败", "scope", scope, "source", body.SourceTerm, "err", err)
		Fail(c, glossaryErr(err))
		return
	}
	OK(c, g)
}

// GlossaryUpdate PUT /api/glossary。
func (a *App) GlossaryUpdate(c *gin.Context) {
	var body glossaryBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ID == 0 {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON 或缺少 id"))
		return
	}
	visitor := middleware.VisitorID(c)
	g, err := a.GlossaryRepo.Update(body.ID, visitor, func(x *store.Glossary) {
		if body.SourceTerm != "" {
			x.SourceTerm = body.SourceTerm
		}
		if body.TargetTerm != "" {
			x.TargetTerm = body.TargetTerm
		}
		x.Note = body.Note
		if body.Enabled != nil {
			x.Enabled = *body.Enabled
		}
	})
	if err != nil {
		Fail(c, glossaryErr(err))
		return
	}
	OK(c, g)
}

// GlossaryDelete DELETE /api/glossary?id=N。
func (a *App) GlossaryDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		Fail(c, errs.InvalidRequest("缺少 id"))
		return
	}
	if err := a.GlossaryRepo.Delete(id, middleware.VisitorID(c)); err != nil {
		Fail(c, glossaryErr(err))
		return
	}
	v, err := a.view(c)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, v)
}

// visitorIDFor 站点级术语不挂访客。
func visitorIDFor(scope, visitor string) string {
	if scope == "site" {
		return ""
	}
	return visitor
}

// glossaryErr 把仓储业务错误映射为对外错误。
func glossaryErr(err error) error {
	switch {
	case errors.Is(err, store.ErrGlossaryEmpty), errors.Is(err, store.ErrGlossaryDuplicate):
		return errs.InvalidRequest(err.Error())
	case errors.Is(err, store.ErrGlossaryForbidden):
		return errs.InvalidRequest(err.Error())
	}
	return errs.Internal(err)
}
