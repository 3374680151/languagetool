// health.go 实现 GET /api/health（FR-1003 三态健康检查）。
//
// @file internal/httpapi/handler/health.go
// 文件职责：区分"服务活着 / 数据库 / 网关"三处状态——
//
//	{"ok":bool, "db":"ok"|"fail", "gateway":"ok"|"degraded"|"fail", "version":...}
//
// 探测失败时 HTTP 仍返回 200，用 body 表达真实状态：监控能区分
// "服务活着但网关挂了"与"服务挂了"。设置页"服务状态"只读区块读这里（FR-1003）。
//
// 主要内容：Health。
//
// 模块边界：只读 App.DB 与 App.Pool；不写任何状态。
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/store"
)

// 网关状态取值（§7.4）。
const (
	statusOK       = "ok"
	statusDegraded = "degraded"
	statusFail     = "fail"
)

// db 状态取值。
const (
	dbOK   = "ok"
	dbFail = "fail"
)

// Health 返回三态健康检查。
func (a *App) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	dbState := dbOK
	if err := store.Ping(a.DB); err != nil {
		dbState = dbFail
	}

	gwState := a.gatewayState(ctx)

	c.JSON(http.StatusOK, gin.H{
		// ok 是"服务整体健康"：两处探测都过才算。
		"ok":      dbState == dbOK && gwState == statusOK,
		"db":      dbState,
		"gateway": gwState,
		"version": Version,
	})
}

// gatewayState 探测网关：打 /v1/models——那是真正依赖的端点，比 /health 更有意义。
// 未配置 → fail（站点处于六态之"未配置"）；网络失败 → fail；
// degraded 由模型池巡检置位（P3.5 接入），P0 阶段探测通过即 ok。
func (a *App) gatewayState(ctx context.Context) string {
	client := a.Pool.SiteClient()
	if client == nil || !client.Configured() {
		return statusFail
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := client.Models(probeCtx); err != nil {
		return statusFail
	}
	if a.Pool.Degraded() {
		return statusDegraded
	}
	return statusOK
}

// siteName 是站点名（/api/config 与 health 使用）。
const siteName = "FluentRead"
