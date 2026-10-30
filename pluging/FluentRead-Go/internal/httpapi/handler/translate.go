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
	"fluentread/internal/gateway"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/service"
	"fluentread/internal/store"
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
	// Custom 是访客自定义端点（L3，FR-403）：服务端即用即弃转发（FR-404）。
	Custom *struct {
		BaseURL string `json:"baseURL"`
		APIKey  string `json:"apiKey"`
		Model   string `json:"model"`
	} `json:"custom"`
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
	if body.Custom != nil && body.Custom.BaseURL != "" && body.Custom.APIKey != "" && body.Custom.Model != "" {
		req.Custom = &gateway.CustomUpstream{
			BaseURL: body.Custom.BaseURL,
			APIKey:  body.Custom.APIKey,
			Model:   body.Custom.Model,
		}
	}

	// ③ 生效术语（站点 > 用户合并）注入请求：术语必须真的进提示词（FR-502），
	// 并随响应回传指纹，前端凭它判断"译文已过期"（FR-503）。
	merged, err := a.GlossaryRepo.Merged(middleware.VisitorID(c))
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	glossary := make([][2]string, 0, len(merged))
	for _, g := range merged {
		glossary = append(glossary, [2]string{g.SourceTerm, g.TargetTerm})
	}
	req.Glossary = glossary

	// ④ 编排（校验/配额/并发/降级/计费都在 service）。
	// Validate 类错误与配额错误都是 errs 语义，Fail 统一渲染。
	resp, err := a.Translator.Translate(c.Request.Context(), middleware.VisitorID(c), req)
	if err != nil {
		Fail(c, err)
		return
	}
	resp.GlossaryFingerprint = store.Fingerprint(merged)
	OK(c, resp)
}
