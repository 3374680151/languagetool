// sdk.go 实现 SDK 下发与 SDK 专用端点（P6，FR-901~906，§7.9）。
//
// @file internal/httpapi/handler/sdk.go
// 文件职责：
//   - GET /sdk/fluentread-sdk.js：SDK 脚本下发（长缓存；Origin 白名单为空 = SDK 关闭，
//     此时 /api/sdk/* 一律 403，脚本会给出"未开通"提示）。
//   - GET /sdk/demo.html：一行接入演示页（走查 FR-902 用）。
//   - POST /api/sdk/translate · GET/POST /api/sdk/word：SDK 专用端点——
//     Origin ∈ FR_SDK_ORIGINS 否则 403 ORIGIN_FORBIDDEN（FR-906）；
//     与主站同一访客 cookie / 配额 / 限流 / 降级链（FR-903）。
//
// 主要内容：sdkOriginGuard、SDKJs、SDKDemo、SdkTranslate、SdkWordLookup、SdkWordCollect。
//
// 模块边界：翻译走 service.Translator（同主站语义）；查词/收藏复用 dict 与 WordRepo；
// 白名单来自 config.SDKOrigins。
package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/service"
)

// sdkEnabled 白名单非空 = SDK 开启。
func (a *App) sdkEnabled() bool { return len(a.Cfg.SDKOrigins) > 0 }

// SDK whitelisted origin 判定。
func (a *App) sdkOriginAllowed(origin string) bool {
	if len(a.Cfg.SDKOrigins) == 0 {
		return false // 白名单为空 = SDK 关闭（FR-906 安全默认）
	}
	origin = strings.TrimRight(strings.ToLower(origin), "/")
	for _, allowed := range a.Cfg.SDKOrigins {
		if strings.TrimRight(strings.ToLower(allowed), "/") == origin {
			return true
		}
	}
	return false
}

// SDKOriginGuard 是 /api/sdk/* 的来源校验中间件（FR-906）。
// 跨站请求必须带 Origin 且在白名单内；同源部署（如演示页）的请求不带 Origin，
// 在 SDK 开启时放行——白名单本身约束的是"嵌入到别的站点"这一场景。
func (a *App) SDKOriginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && !a.sdkOriginAllowed(origin) {
			Fail(c, errs.OriginForbidden())
			c.Abort()
			return
		}
		if origin == "" && !a.sdkEnabled() {
			// SDK 关闭（白名单为空）：连同源调用也一并拒绝（关闭语义）。
			Fail(c, errs.OriginForbidden())
			c.Abort()
			return
		}
		// CORS：白名单来源允许跨域携带 cookie（同站或子域共享 cookie，§7.9）。
		c.Header("access-control-allow-origin", origin)
		c.Header("access-control-allow-credentials", "true")
		c.Header("vary", "Origin")
		c.Next()
	}
}

// SDKPreflight 处理 SDK 端点的 OPTIONS 预检。
func (a *App) SDKPreflight(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if a.sdkOriginAllowed(origin) {
		c.Header("access-control-allow-origin", origin)
		c.Header("access-control-allow-credentials", "true")
		c.Header("access-control-allow-methods", "GET, POST, OPTIONS")
		c.Header("access-control-allow-headers", "content-type")
		c.Header("vary", "Origin")
		c.Status(http.StatusNoContent)
		return
	}
	Fail(c, errs.OriginForbidden())
}

// sdkJS 是注入的 SDK 脚本内容（web/embed.go 提供）。
func (a *App) SDKJs(c *gin.Context) {
	if a.SDKAsset == nil {
		c.String(http.StatusNotFound, "// SDK 未构建")
		return
	}
	// 脚本内容带版本化路径引用，长缓存（§7.4：长缓存）。
	c.Header("cache-control", "public, max-age=86400")
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", a.SDKAsset)
}

// SDKDemo 是演示页（一行接入示例，FR-902 走查用）。
func (a *App) SDKDemo(c *gin.Context) {
	if a.SDKAssetDemo == nil {
		c.String(http.StatusNotFound, "demo 未构建")
		return
	}
	c.Header("cache-control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", a.SDKAssetDemo)
}

// sdkTranslateBody 是 SDK 翻译请求体。
type sdkTranslateBody struct {
	Segments []struct {
		Text string `json:"text"`
	} `json:"segments"`
	From string `json:"from"`
	To   string `json:"to"`
}

// SdkTranslate POST /api/sdk/translate：与主站同一翻译编排（FR-903）。
func (a *App) SdkTranslate(c *gin.Context) {
	if ok, retry := middleware.AllowRequest(c, a.Limiter); !ok {
		Fail(c, errs.RateLimited(retry))
		return
	}
	var body sdkTranslateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}
	req := &service.TranslateRequest{
		Segments: make([]string, 0, len(body.Segments)),
		From:     body.From,
		To:       body.To,
	}
	for _, s := range body.Segments {
		req.Segments = append(req.Segments, s.Text)
	}
	resp, err := a.Translator.Translate(c.Request.Context(), middleware.VisitorID(c), req)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, resp)
}

// SdkWordLookup GET /api/sdk/word?word=：查词（与主站同一链路，§八 8.5）。
func (a *App) SdkWordLookup(c *gin.Context) {
	// Dict 读路径参数；SDK 用查询串传词，这里转写到 Params 后复用同一条链路。
	if w := c.Query("word"); w != "" {
		c.Params = append(c.Params, gin.Param{Key: "word", Value: w})
	}
	a.Dict(c)
}

// SdkWordCollect POST /api/sdk/word：收藏进同一个词表（FR-902 × FR-1201 联动）。
func (a *App) SdkWordCollect(c *gin.Context) {
	a.WordsCreate(c)
}
