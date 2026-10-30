// Package web 把前端构建产物与 SDK 一起打进 Go 二进制。
//
// @file web/embed.go
// 文件职责：go:embed web/dist（SPA）与 web/sdk（SDK 脚本 + 演示页，P6）。
// 一个进程、一个 SQLite 文件即可跑（NFR-4）；没跑过前端构建时 SPA 目录只有
// .gitkeep，此时页面会显示"前端还没构建"说明，SDK 单独可用。
//
// 主要内容：FS（dist）、SDKAsset/SDKAssetDemo（SDK 文件字节）。
//
// 模块边界：只做 embed 与子文件系统；不含业务逻辑。
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

//go:embed sdk/fluentread-sdk.js sdk/demo.html
var sdkFS embed.FS

// FS 返回以 dist 为根的只读文件系统。
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}

// SDKAsset 返回 SDK 脚本字节。
func SDKAsset() ([]byte, error) {
	return sdkFS.ReadFile("sdk/fluentread-sdk.js")
}

// SDKAssetDemo 返回演示页字节。
func SDKAssetDemo() ([]byte, error) {
	return sdkFS.ReadFile("sdk/demo.html")
}
