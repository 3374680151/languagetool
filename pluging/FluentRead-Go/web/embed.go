// Package web 把前端构建产物打进 Go 二进制。
//
// 为什么用 go:embed 而不是加一个 nginx：整个站点只有一个容器、一个进程，
// 部署时不用维护"静态资源放哪、怎么和反代对齐路径"这类问题。
//
// 构建顺序：先 `make web`（产出 web/dist），再 `make build`。
// 没跑过前端构建时 dist 里只有一个 .gitkeep，此时 Go 仍然能编译通过，
// 访问页面会得到一段"前端还没构建"的说明，而不是编译失败或 404。
package web

import (
	"embed"
	"io/fs"
)

// all: 前缀让 dist 里的点文件（.gitkeep）也被包含，
// 否则空目录不会被 embed，编译期就会因为"找不到匹配文件"失败。
//
//go:embed all:dist
var embedded embed.FS

// FS 返回以 dist 为根的只读文件系统。
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
