// ratelimit.go 是按访客与来源 IP 的双桶限流（FR-602）。
//
// @file internal/httpapi/middleware/visitor_ratelimit.go（实为 ratelimit.go）
// 文件职责：包装 internal/ratelimit 的令牌桶，对"会打上游"的接口生效：
// 访客桶 FR_RATE_PER_MINUTE 次/分钟，IP 桶 4×——只用 cookie 限流可以被
// "每次清 cookie"绕过；同 NAT 出口共用 IP 桶是个人站点可接受的代价。
//
// 主要内容：AllowRequest。
//
// 模块边界：只依赖 internal/ratelimit 与 Gin；被 translate/proofread/dict 等
// handler 显式调用（不是全局中间件——静态资源与配置读取不需要限流）。
package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"

	"fluentread/internal/ratelimit"
)

// AllowRequest 对一次请求做双桶判定：访客桶 + IP 桶。
// 返回 false 时 retryAfter 是建议等待整秒数（至少 1），用于 Retry-After 头（FR-605）。
func AllowRequest(c *gin.Context, limiter *ratelimit.Limiter) (bool, int) {
	if limiter == nil {
		return true, 0
	}
	visitor := VisitorID(c)
	if visitor != "" {
		if ok, retry := limiter.Allow("v:" + visitor); !ok {
			return false, retry
		}
	}
	if ip := ClientIP(c); ip != "" {
		if ok, retry := limiter.Allow("ip:" + ip); !ok {
			return false, retry
		}
	}
	return true, 0
}

// ClientIP 取来源 IP。优先 X-Forwarded-For 的第一跳：部署在反代后面时
// RemoteAddr 只会是反代自己，按它限流等于全站共用一个桶。
func ClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}
