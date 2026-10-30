// Package httpx 提供 HTTP 层的统一响应包写入（NFR-12）。
//
// @file internal/httpx/respond.go
// 文件职责：定义 {ok, data} / {ok, error} 统一响应信封（规格 §7.4），handler 与
// 中间件都通过这里写 JSON；错误一律先经 internal/errs 归一，429 额外写 Retry-After 头。
//
// 主要内容：OK / Fail / DecodeJSON 三个入口、信封结构体。
//
// 模块边界：只依赖 internal/errs 与标准库；不认识 Gin 上下文（handler 负责取*w.ResponseWriter）
// 与任何业务类型；日志不记请求与响应正文（NFR-7）。
package httpx

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"fluentread/internal/errs"
)

// errorPayload 是错误信封的 error 字段（§7.4）。
type errorPayload struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

// failEnvelope 是失败响应：{"ok":false,"error":{...}}。
type failEnvelope struct {
	OK    bool         `json:"ok"`
	Error errorPayload `json:"error"`
}

// okEnvelope 是成功响应：{"ok":true,"data":...}。
type okEnvelope struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

// writeRaw 原样写出 JSON。编码失败时降级为固定错误体，避免半截响应。
func writeRaw(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("响应编码失败", "err", err)
		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.Header().Set("cache-control", "no-store")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"INTERNAL","message":"服务出错了，请稍后重试","retryable":true}}`))
		return
	}
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// OK 写出成功响应 {"ok":true,"data":...}。
func OK(w http.ResponseWriter, data any) {
	writeRaw(w, http.StatusOK, okEnvelope{OK: true, Data: data})
}

// Fail 把错误归一为 *Error 后写出统一失败信封。
// 429 时同步写 Retry-After 头（FR-605：给程序看的头 + 给人看的文案）。
// 5xx 记 Error 日志并保留内部原因，4xx 记 Warn（NFR-13）。
func Fail(w http.ResponseWriter, err error) {
	e := errs.As(err)
	if e.RetryAfter > 0 {
		w.Header().Set("retry-after", strconv.Itoa(e.RetryAfter))
	}
	if e.Status >= http.StatusInternalServerError {
		slog.Error("请求失败", "code", e.Code, "cause", e.Err)
	} else {
		slog.Warn("请求被拒绝", "code", e.Code)
	}
	writeRaw(w, e.Status, failEnvelope{OK: false, Error: errorPayload{
		Code:       e.Code,
		Message:    e.Message,
		Retryable:  e.Retryable(),
		RetryAfter: e.RetryAfter,
	}})
}

// DecodeJSON 读取并解析请求体，限制 1MB，非法时返回 INVALID_REQUEST。
// 不开启 DisallowUnknownFields：前端多传字段不应导致请求失败。
func DecodeJSON(r io.Reader, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r, 1<<20))
	if err := dec.Decode(dst); err != nil {
		return errs.InvalidRequest("请求体不是有效 JSON")
	}
	return nil
}

