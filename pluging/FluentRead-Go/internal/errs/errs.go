// Package errs 定义统一的业务错误类型与错误码，是全站错误的唯一出口。
//
// @file internal/errs/errs.go
// 文件职责：把"错误码 ↔ HTTP 状态码 ↔ 用户可见文案"收敛到一处（规格附录 B），
// 业务层只构造这里定义的错误，handler 层只负责把 *Error 渲染成统一响应包；
// 任何地方禁止裸写 c.JSON(500, ...)（NFR-12）。
//
// 主要内容：错误码常量（附录 B 全集）、Error 类型（Code/Message/Status/RetryAfter/Err）、
// 按错误码分类的构造函数、As 归一化。
//
// 模块边界：只被 service/handler/gateway 依赖；不认识 Gin、数据库与任何业务类型。
// 内部原因只进 Err 字段（只写日志，不对外输出）。
package errs

import (
	"errors"
	"fmt"
	"net/http"
)

// 错误码全集（附录 B）。新增时同步更新本文件与 docs/acceptance 的 FR-CHECKLIST。
const (
	CodeUnsupportedLanguage = "UNSUPPORTED_LANGUAGE"
	CodeTextTooLong         = "TEXT_TOO_LONG"
	CodeTooManySegments     = "TOO_MANY_SEGMENTS"
	CodeInvalidRequest      = "INVALID_REQUEST"
	CodeQuotaExceeded       = "QUOTA_EXCEEDED"
	CodeRateLimited         = "RATE_LIMITED"
	CodeUpstreamTimeout     = "UPSTREAM_TIMEOUT"
	CodeUpstreamUnavailable = "UPSTREAM_UNAVAILABLE"
	CodeModelNotConfigured  = "MODEL_NOT_CONFIGURED"
	CodeCustomModelFailed   = "CUSTOM_MODEL_FAILED"
	CodeOriginForbidden     = "ORIGIN_FORBIDDEN"
	CodeInternal            = "INTERNAL"
)

// Error 是对外错误。Message 必须是给人看的中文（NFR-11：禁用"上游/线路/配额/跳/回退/provider"等内部词）。
type Error struct {
	Code    string
	Message string
	Status  int
	// RetryAfter 仅 429 有意义：建议等待秒数，由 httpx 写成 Retry-After 头（FR-605）。
	RetryAfter int
	// Err 是内部原因，只进日志，不进响应。
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// Retryable 判断该错误是否值得让用户重试：限流、超时、服务不可用可以重试；
// 参数错误、来源被拒、未配置模型重试也不会成功。
func (e *Error) Retryable() bool {
	switch e.Code {
	case CodeQuotaExceeded, CodeRateLimited, CodeUpstreamTimeout, CodeUpstreamUnavailable, CodeInternal:
		return true
	}
	return false
}

// UnsupportedLanguage 400。
func UnsupportedLanguage() *Error {
	return &Error{Code: CodeUnsupportedLanguage, Message: "该语言暂不支持，请更换语言", Status: http.StatusBadRequest}
}

// TextTooLong 400。hint 说明"请拆分第 N 段"（FR-305：不给重试按钮，重试不会成功）。
func TextTooLong(segment int) *Error {
	msg := "有一段超过 5000 字，请把它拆成几段再翻译"
	if segment > 0 {
		msg = fmt.Sprintf("第 %d 段超过 5000 字，请把它拆成几段再翻译", segment)
	}
	return &Error{Code: CodeTextTooLong, Message: msg, Status: http.StatusBadRequest}
}

// TooManySegments 400。
func TooManySegments() *Error {
	return &Error{Code: CodeTooManySegments, Message: "一次最多翻译 20 段，请分批进行", Status: http.StatusBadRequest}
}

// InvalidRequest 400。参数缺失或格式非法。
func InvalidRequest(detail string) *Error {
	return &Error{Code: CodeInvalidRequest, Message: "请求格式有误，请刷新页面重试", Status: http.StatusBadRequest, Err: errors.New(detail)}
}

// QuotaExceeded 429。used/limit/resetAfter 用于生成附录 B 文案
// "今日额度已用完（已译 N 段），约 X 小时后恢复"（FR-308：说清等多久、该做什么）。
func QuotaExceeded(used int, resetAfterSeconds int) *Error {
	if resetAfterSeconds < 1 {
		resetAfterSeconds = 1
	}
	hours := resetAfterSeconds / 3600
	wait := fmt.Sprintf("约 %d 小时后恢复", hours)
	if hours < 1 {
		minutes := resetAfterSeconds / 60
		if minutes < 1 {
			minutes = 1
		}
		wait = fmt.Sprintf("约 %d 分钟后恢复", minutes)
	}
	return &Error{
		Code:       CodeQuotaExceeded,
		Message:    fmt.Sprintf("今日额度已用完（已译 %d 段），%s", used, wait),
		Status:     http.StatusTooManyRequests,
		RetryAfter: resetAfterSeconds,
	}
}

// RateLimited 429。文案 "操作太频繁，请 X 秒后再试"（附录 B）。
func RateLimited(retryAfter int) *Error {
	if retryAfter < 1 {
		retryAfter = 1
	}
	return &Error{
		Code:       CodeRateLimited,
		Message:    fmt.Sprintf("操作太频繁，请 %d 秒后再试", retryAfter),
		Status:     http.StatusTooManyRequests,
		RetryAfter: retryAfter,
	}
}

// UpstreamTimeout 504。waitedSeconds 用于 "翻译超时（已等待 N 秒），可点击重试"。
func UpstreamTimeout(waitedSeconds int, cause error) *Error {
	if waitedSeconds < 1 {
		waitedSeconds = 1
	}
	return &Error{
		Code:    CodeUpstreamTimeout,
		Message: fmt.Sprintf("翻译超时（已等待 %d 秒），可点击重试", waitedSeconds),
		Status:  http.StatusGatewayTimeout,
		Err:     cause,
	}
}

// UpstreamUnavailable 502。所有模型调用都失败时的对外文案（FR-307：不暴露内部结构）。
func UpstreamUnavailable(cause error) *Error {
	return &Error{Code: CodeUpstreamUnavailable, Message: "翻译服务暂时不可用，请稍后重试", Status: http.StatusBadGateway, Err: cause}
}

// ModelNotConfigured 503。六态之"未配置"：页面其余功能照常（§5.2）。
func ModelNotConfigured() *Error {
	return &Error{Code: CodeModelNotConfigured, Message: "站点还没有配置翻译模型，请联系站长", Status: http.StatusServiceUnavailable}
}

// CustomModelFailed 是"200 内降级"信号（FR-405）：自定义模型失败已回落默认模型。
// 调用方不应把它当失败，而是把它放进成功响应的 message 里提示。
func CustomModelFailed(cause error) *Error {
	return &Error{Code: CodeCustomModelFailed, Message: "自定义模型不可用，已改用默认模型", Status: http.StatusOK, Err: cause}
}

// OriginForbidden 403。SDK 接入站不在白名单（FR-906）。
func OriginForbidden() *Error {
	return &Error{Code: CodeOriginForbidden, Message: "本站点未开通翻译服务，请联系站长添加域名", Status: http.StatusForbidden}
}

// Internal 500。对外只给通用文案（FR-307）。
func Internal(cause error) *Error {
	return &Error{Code: CodeInternal, Message: "服务出错了，请稍后重试", Status: http.StatusInternalServerError, Err: cause}
}

// As 把任意 error 归一为 *Error；非 Error 一律视为 Internal（NFR-12：错误码唯一出口）。
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}
