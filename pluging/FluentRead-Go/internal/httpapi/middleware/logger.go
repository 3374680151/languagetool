// logger.go 与 recover.go：访问日志与 panic 恢复（NFR-13）。
//
// @file internal/httpapi/middleware/logger.go
// 文件职责：Logger 记录每个请求的方法、路径、状态码、耗时、字节数（结构化 JSON 日志），
// 不记录查询串与请求/响应正文（NFR-7）；Recover 捕获 handler panic，转为
// INTERNAL 错误响应并记录堆栈，不把 panic 内容返回给客户端。
//
// 主要内容：Logger、Recover（Gin 中间件形式）。
//
// 模块边界：只依赖 Gin、slog 与 internal/httpx / internal/errs。
package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpx"
)

// Logger 记录每个请求的结果。路径不含查询串，避免把用户输入写进日志。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(),
			"bytes", c.Writer.Size(),
		)
	}
}

// Recover 捕获 handler 内的 panic，统一转为 500 响应（NFR-12：错误码唯一出口）。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("handler panic",
					"value", fmt.Sprintf("%v", v),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"stack", string(debug.Stack()),
				)
				httpx.Fail(c.Writer, errs.Internal(fmt.Errorf("panic: %v", v)))
				c.Abort()
			}
		}()
		c.Next()
	}
}
