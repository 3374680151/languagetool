// models.go 实现 POST /api/models/test 与模型池查询（P3.5，FR-403/404/408）。
//
// @file internal/httpapi/handler/models.go
// 文件职责：自定义模型"测试连接"——服务端转发一次最小 chat 调用，结果只有
// ok/message/latency；访客密钥即用即弃（不落盘、不写日志、响应 no-store，FR-404）；
// GET /api/models 返回当前下拉清单（巡检剔除后的，FR-408/1103）。
//
// 主要内容：ModelTest、Models。
//
// 模块边界：转发细节在 gateway.Client；本 handler 不持久化任何密钥。
package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/gateway"
)

// modelTestBody 是测试连接请求体（FR-403：baseURL + Key + 模型名）。
type modelTestBody struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
}

// ModelTest 用一次最小补全验证自定义端点可用性。
func (a *App) ModelTest(c *gin.Context) {
	var body modelTestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}
	if body.BaseURL == "" || body.APIKey == "" || body.Model == "" {
		Fail(c, errs.InvalidRequest("baseURL、apiKey、model 都必填"))
		return
	}

	// 一次性客户端：密钥只在内存，函数返回即弃（FR-404）。
	client := gateway.New(body.BaseURL, []string{body.APIKey}, 10*time.Second)
	started := time.Now()
	resp, err := client.Chat(c.Request.Context(), gateway.ChatRequest{
		Model:       body.Model,
		Messages:    [][2]string{{"user", "ping"}},
		Temperature: 0,
		MaxTokens:   16,
	})
	latency := time.Since(started).Milliseconds()

	// 响应绝不缓存（no-store 由 httpx 统一写）；密钥不进日志。
	if err != nil {
		OK(c, gin.H{"ok": false, "message": errs.As(err).Message, "latencyMs": latency})
		return
	}
	if resp.Text == "" {
		OK(c, gin.H{"ok": false, "message": "端点返回了空内容", "latencyMs": latency})
		return
	}
	OK(c, gin.H{"ok": true, "message": "连接成功", "latencyMs": latency})
}

// Models 返回当前下拉清单（巡检剔除后）与健康态（FR-408/1103）。
func (a *App) Models(c *gin.Context) {
	type item struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Role    string `json:"role"`
		Quality string `json:"quality"`
		Speed   string `json:"speed"`
		Cost    string `json:"cost"`
		Health  string `json:"health"`
	}
	out := make([]item, 0, 8)
	for _, m := range a.Catalog.Enabled() {
		health := a.Pool.ModelHealth(m.ID)
		if health == "removed" {
			// 连续对账失败的模型不出现在下拉（FR-408：不许出现调不通的模型）。
			continue
		}
		out = append(out, item{
			ID: m.ID, Label: m.Label, Role: m.Role,
			Quality: m.Quality, Speed: m.Speed, Cost: m.Cost, Health: health,
		})
	}
	OK(c, gin.H{"models": out, "degraded": a.Pool.Degraded()})
}
