// Package middleware 存放 Gin 中间件：访客身份、访问日志、panic 恢复、限流。
//
// @file internal/httpapi/middleware/visitor.go
// 文件职责：匿名访客身份（FR-1104）——首访下发 HttpOnly cookie（UUID，v4），
// 60 天滑动续期；身份注入 gin.Context 供 handler 与 service 使用；
// 续访客表（visitors.last_seen_at）异步进行，不阻塞请求。
//
// 主要内容：Visitor、VisitorID。
//
// 模块边界：只依赖 Gin、标准库与 store.VisitorRepo；不解析业务参数；
// 不采集邮箱、不落库 IP，只用一个随机 id 区分浏览器。
package middleware

import (
	"crypto/rand"
	"log/slog"

	"github.com/gin-gonic/gin"

	"fluentread/internal/store"
)

// CookieName 是访客身份 cookie 名。
const CookieName = "fr_vid"

// cookieMaxAge 是 60 天滑动有效期（FR-1104）。
const cookieMaxAge = 60 * 24 * 60 * 60

// ctxVisitorKey 是 gin.Context 里的访客 id 键。
const ctxVisitorKey = "fr_visitor_id"

// Visitor 返回访客身份中间件。repo 可为 nil（无库也能起，只发 cookie）。
func Visitor(repo *store.VisitorRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := c.Cookie(CookieName)
		if err != nil || !validID(id) {
			id = newID()
			c.SetCookie(CookieName, id, cookieMaxAge, "/", "", false, true)
		}
		c.Set(ctxVisitorKey, id)

		// 滑动续期：每次请求都刷新 last_seen_at（FR-1104）。
		// 失败不阻断请求——身份已随 cookie 成立；无库时跳过。
		if repo != nil {
			go func(id string) {
				if err := repo.Touch(id, timeNow()); err != nil {
					slog.Warn("访客续期失败", "err", err)
				}
			}(id)
		}
		c.Next()
	}
}

// VisitorID 从 gin.Context 取访客 id；中间件未挂载时返回空串。
func VisitorID(c *gin.Context) string {
	if v, ok := c.Get(ctxVisitorKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// validID 只接受标准 UUID 字符串（36 字符 8-4-4-4-12），防止伪造超长 cookie。
func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, ch := range id {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			isHex := (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')
			if !isHex {
				return false
			}
		}
	}
	// UUID v4 的版本位固定为 4（第 14 个十六进制位）。
	if id[14] != '4' {
		return false
	}
	return true
}

// newID 生成 UUID v4（RFC 4122）。随机源不可用时 panic：宁可拒绝服务，
// 也不生成可预测的 id。
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand 不可用: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[v>>4], hex[v&0x0f])
	}
	return string(out)
}
