// proofread.go 实现 POST /api/proofread（P3，§7.4）。
//
// @file internal/httpapi/handler/proofread.go
// 文件职责：参数解析（{text, lang}）、双桶限流（会打模型）、编排调用、
// 统一响应包写出。子串校验与类别枚举在 service 层（FR-203/204）。
//
// 主要内容：Proofread。
//
// 模块边界：只认识 service.Proofreader。
package handler

import (
	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/service"
)

// proofreadBody 是请求体。
type proofreadBody struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

// Proofread 处理段落纠错。
func (a *App) Proofread(c *gin.Context) {
	// 会打模型：与翻译共用双桶限流（FR-602）。
	if ok, retry := middleware.AllowRequest(c, a.Limiter); !ok {
		Fail(c, errs.RateLimited(retry))
		return
	}

	var body proofreadBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}
	if body.Text == "" {
		Fail(c, errs.InvalidRequest("没有要检查的文本"))
		return
	}
	lang := body.Lang
	if lang == "" {
		lang = "en"
	}

	resp, err := a.Proofreader.Proofread(c.Request.Context(), &service.ProofreadRequest{
		Text: body.Text,
		Lang: lang,
	})
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, resp)
}
