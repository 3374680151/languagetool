// time.go 提供可替换的时钟，供中间件测试使用。
//
// @file internal/httpapi/middleware/time.go
// 文件职责：把 time.Now 收敛到一个包级变量，测试里可以替换；生产代码不用感知。
//
// 主要内容：timeNow 变量。
//
// 模块边界：仅本包内部使用。
package middleware

import "time"

// timeNow 是包内时钟，测试可替换。
var timeNow = time.Now
