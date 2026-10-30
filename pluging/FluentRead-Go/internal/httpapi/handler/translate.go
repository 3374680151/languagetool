// translate.go 实现 POST /api/translate（P1）。
//
// @file internal/httpapi/handler/translate.go
// 文件职责：只做参数解析与响应组装（§7.3）——限流（双桶 FR-602）、解码请求体、
// 交给 service.Translator 编排、统一响应包写出。业务规则（分段校验/配额/降级/计费）
// 全在 service 层；错误一律是 errs 语义，由 httpx 渲染（NFR-12）。
//
// 主要内容：Translate、translateBody。
//
// 模块边界：只认识 service.TranslateRequest；不认识 gateway 细节。
package handler

import (
	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/service"
)

// translateBody 是请求体（§7.4）。
type translateBody struct {
	Segments []struct {
		Text string `json:"text"`
	} `json:"segments"`
	From  string `json:"from"`
	To    string `json:"to"`
	Style string `json:"style"`
	Model string `json:"model"`
}

// Translate 处理逐段翻译。
func (a *App) Translate(c *gin.Context) {
	// ① 双桶限流：内存判断，最便宜，先挡住洪峰（FR-602）。
	if ok, retry := middleware.AllowRequest(c, a.Limiter); !ok {
		Fail(c, errs.RateLimited(retry))
		return
	}

	// ② 解码请求体。
	var body translateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}

	req := &service.TranslateRequest{
		Segments: make([]string, 0, len(body.Segments)),
		From:     body.From,
		To:       body.To,
		Style:    body.Style,
		Model:    body.Model,
	}
	for _, s := range body.Segments {
		req.Segments = append(req.Segments, s.Text)
	}

	// ③ 编排（校验/配额/并发/降级/计费都在 service）。
	// Validate 类错误与配额错误都是 errs 语义，Fail 统一渲染。
	resp, err := a.Translator.Translate(c.Request.Context(), middleware.VisitorID(c), req)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, resp)
}
