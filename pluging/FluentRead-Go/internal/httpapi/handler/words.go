// words.go 实现 GET/POST/DELETE /api/words（P5，FR-702/703/1105/1201）。
//
// @file internal/httpapi/handler/words.go
// 文件职责：词表 CRUD——收藏（幂等，带原句上下文）、列表（最近收藏/最少使用排序）、
// 删除；随访客 cookie 存服务端（FR-1105）。
//
// 主要内容：Words、WordsCreate、WordsDelete。
//
// 模块边界：存取在 store.WordRepo；"再使用计数"在 service（翻译结算时）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/store"
)

// wordsBody 是收藏请求体（FR-1201：收藏时自动带所在句）。
type wordsBody struct {
	Word            string `json:"word"`
	Lang            string `json:"lang"`
	ContextSentence string `json:"contextSentence"`
	SourceURL       string `json:"sourceUrl"`
}

// Words 返回词表（sort=recent|least_used，FR-1202）。
func (a *App) Words(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	sort := c.DefaultQuery("sort", "recent")
	if sort != "recent" && sort != "least_used" {
		sort = "recent"
	}
	list, err := a.WordRepo.List(visitor, sort)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	if list == nil {
		list = []store.Word{}
	}
	OK(c, list)
}

// WordsCreate 收藏一个词（幂等，§八 8.5）。
func (a *App) WordsCreate(c *gin.Context) {
	var body wordsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}
	if body.Word == "" {
		Fail(c, errs.InvalidRequest("缺少要收藏的词"))
		return
	}
	if body.Lang == "" {
		body.Lang = "en"
	}
	visitor := middleware.VisitorID(c)
	w, created, err := a.WordRepo.Collect(visitor, body.Word, body.Lang, body.ContextSentence, body.SourceURL)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, gin.H{"word": w, "created": created})
}

// WordsDelete 删除一个收藏词。
func (a *App) WordsDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		Fail(c, errs.InvalidRequest("缺少 id"))
		return
	}
	if err := a.WordRepo.Delete(middleware.VisitorID(c), id); err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, gin.H{"deleted": true})
}
