// spa.go 负责托管内嵌的前端单页应用。
//
// @file internal/httpapi/handler/spa.go
// 文件职责：非 /api 路径一律回落 index.html，让前端路由（/、/words、/review、/settings）
// 刷新也能打开；带内容哈希的 /assets/ 长期缓存，index.html 不缓存
//（否则发版后用户拿到旧壳）；前端还没构建时给一段可读的说明而不是 404 或空白。
//
// 主要内容：SPA、writeNotBuilt。
//
// 模块边界：只做静态文件与回落，不认识任何业务接口。
package handler

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// notBuiltPage 是"前端还没构建"时返回的说明页。
// 直接给 404 会让人以为服务坏了，实际只是少跑了一条构建。
const notBuiltPage = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>FluentRead</title>
<style>body{font-family:-apple-system,"PingFang SC",sans-serif;background:#f3f4f6;color:#1b1f23;
margin:0;display:flex;min-height:100vh;align-items:center;justify-content:center}
.card{background:#fff;border:1px solid #e4e6e9;border-radius:10px;padding:24px 28px;max-width:560px}
h1{font-size:16px;margin:0 0 8px}p{font-size:13px;line-height:1.8;color:#4c545b;margin:4px 0}
code{font-family:ui-monospace,Menlo,monospace;background:#f7f8f9;padding:1px 5px;border-radius:4px}</style>
</head><body><div class="card">
<h1>前端还没有构建</h1>
<p>后端已经跑起来了，但二进制里没有前端产物。</p>
<p>执行 <code>make web</code> 生成 <code>web/dist</code>，再 <code>make build</code> 重新编译即可。</p>
<p>接口本身可用：<code>/api/health</code>。</p>
</div></body></html>`

// SPA 返回单页应用处理器。assets 为 nil 时所有页面都返回"未构建"说明。
func (a *App) SPA() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.Assets == nil {
			writeNotBuilt(c)
			return
		}

		// 只接受 GET/HEAD；其余方法交给 405，避免把 POST 也回落成首页。
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(c.Request.URL.Path, "/")
		if name == "" {
			serveIndex(c, a.Assets)
			return
		}

		// 目录请求（如 /assets/）不列目录，直接回首页。
		info, err := fs.Stat(a.Assets, name)
		if err == nil && !info.IsDir() {
			// 带哈希的资源可以长期缓存；其余（favicon 等）给一天。
			if strings.HasPrefix(name, "assets/") {
				c.Header("cache-control", "public, max-age=31536000, immutable")
			} else {
				c.Header("cache-control", "public, max-age=86400")
			}
			c.FileFromFS(name, http.FS(a.Assets))
			return
		}

		// 前端路由（/words、/review/xxx）与未知路径都回落到 index.html。
		serveIndex(c, a.Assets)
	}
}

// serveIndex 输出 index.html。缺失时降级为说明页。
// 注意不能用 http.ServeFile：URL 路径以 /index.html 结尾时会触发它内建的 301
// 重定向（浏览器地址栏会变成 /index.html，前端路由 /、/words 全被破坏），
// 这里直接读字节写出。
func serveIndex(c *gin.Context, assets fs.FS) {
	data, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		writeNotBuilt(c)
		return
	}
	// index.html 绝不能被缓存：它引用的资源名带哈希，缓存了旧壳就会一直 404。
	c.Header("cache-control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// writeNotBuilt 输出"前端未构建"说明页。
func writeNotBuilt(c *gin.Context) {
	c.Header("content-type", "text/html; charset=utf-8")
	c.Header("cache-control", "no-store")
	c.String(http.StatusOK, notBuiltPage)
}
