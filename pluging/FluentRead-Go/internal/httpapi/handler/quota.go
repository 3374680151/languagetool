// quota.go 实现 GET /api/quota（P1.5，FR-606）。
//
// @file internal/httpapi/handler/quota.go
// 文件职责：返回当前访客的日额度用量；前端只在"快用完"时展示，
// 不做常驻顶栏（FR-606）。数据口径与配额预扣一致（usage_daily）。
//
// 主要内容：Quota。
//
// 模块边界：只读 UsageRepo 与配置；额度判定在 service（先扣后拒）。
package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/store"
)

// Quota 返回访客额度：{used, limit, remaining, resetsAt, unlimited}。
func (a *App) Quota(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	used, err := a.Usage.TodayUsed(visitor)
	if err != nil {
		// 读不到用量按 0 处理并记日志：额度展示是辅助信息，不该让接口 500。
		slog.Warn("读取访客用量失败", "err", err)
	}

	limit := a.Cfg.VisitorDailySegments
	resp := gin.H{
		"used":     used,
		"limit":    limit,
		"resetsAt": store.ResetAt(time.Now()).Format(time.RFC3339),
	}
	if limit > 0 {
		remaining := int64(limit) - used
		if remaining < 0 {
			remaining = 0
		}
		resp["remaining"] = remaining
		resp["unlimited"] = false
	} else {
		resp["remaining"] = -1
		resp["unlimited"] = true
	}
	OK(c, resp)
}
