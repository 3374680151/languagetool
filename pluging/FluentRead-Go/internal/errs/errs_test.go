// errs_test.go 验证错误码 ↔ HTTP 状态 ↔ 用户文案的唯一映射（附录 B）。
//
// @file internal/errs/errs_test.go
// 文件职责：保证每个错误码的 HTTP 状态、Retry-After 语义与"说人话"文案不被
// 后续改动破坏（NFR-11 禁用内部词、FR-605 双通道等待时间）。
//
// 模块边界：纯单元测试，不依赖网络与数据库。
package errs

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestAppendixBMapping 逐条核对附录 B 的错误码 → HTTP 状态。
func TestAppendixBMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    *Error
		code   string
		status int
	}{
		{"unsupported", UnsupportedLanguage(), CodeUnsupportedLanguage, 400},
		{"too_long", TextTooLong(3), CodeTextTooLong, 400},
		{"too_many", TooManySegments(), CodeTooManySegments, 400},
		{"invalid", InvalidRequest("x"), CodeInvalidRequest, 400},
		{"quota", QuotaExceeded(200, 18000), CodeQuotaExceeded, 429},
		{"rate", RateLimited(7), CodeRateLimited, 429},
		{"timeout", UpstreamTimeout(30, nil), CodeUpstreamTimeout, 504},
		{"unavailable", UpstreamUnavailable(nil), CodeUpstreamUnavailable, 502},
		{"not_configured", ModelNotConfigured(), CodeModelNotConfigured, 503},
		{"origin", OriginForbidden(), CodeOriginForbidden, 403},
		{"internal", Internal(nil), CodeInternal, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.code {
				t.Fatalf("code = %s, want %s", tc.err.Code, tc.code)
			}
			if tc.err.Status != tc.status {
				t.Fatalf("status = %d, want %d", tc.err.Status, tc.status)
			}
			if tc.err.Message == "" {
				t.Fatal("message 不能为空")
			}
		})
	}
}

// TestBannedWords 用户文案禁用内部词（NFR-11）：上游、线路、配额、跳、回退、provider。
func TestBannedWords(t *testing.T) {
	all := []*Error{
		UnsupportedLanguage(), TextTooLong(1), TextTooLong(0), TooManySegments(),
		InvalidRequest("x"), QuotaExceeded(1, 60), RateLimited(3),
		UpstreamTimeout(5, nil), UpstreamUnavailable(nil), ModelNotConfigured(),
		CustomModelFailed(nil), OriginForbidden(), Internal(nil),
	}
	banned := []string{"上游", "线路", "配额", "跳", "回退", "provider", "Provider"}
	for i, e := range all {
		for _, w := range banned {
			if strings.Contains(e.Message, w) {
				t.Errorf("case %d 文案 %q 含内部词 %q（NFR-11）", i, e.Message, w)
			}
		}
	}
}

// TestRetryAfterQuota 429 必须同时给 Retry-After 秒数（FR-605）。
func TestRetryAfterQuota(t *testing.T) {
	e := QuotaExceeded(200, 18000)
	if e.RetryAfter != 18000 {
		t.Fatalf("RetryAfter = %d, want 18000", e.RetryAfter)
	}
	if !strings.Contains(e.Message, "5 小时") {
		t.Fatalf("文案应含可读等待时间: %q", e.Message)
	}
	if !strings.Contains(e.Message, "已译 200 段") {
		t.Fatalf("文案应含已用量: %q", e.Message)
	}
	// 不足 1 小时时用分钟表述。
	e2 := QuotaExceeded(5, 120)
	if !strings.Contains(e2.Message, "2 分钟") {
		t.Fatalf("短等待应用分钟表述: %q", e2.Message)
	}
}

// TestRetryable 429/5xx 可重试，4xx 参数类不可重试。
func TestRetryable(t *testing.T) {
	if !QuotaExceeded(1, 60).Retryable() {
		t.Fatal("429 应可重试")
	}
	if !UpstreamUnavailable(nil).Retryable() {
		t.Fatal("502 应可重试")
	}
	if TextTooLong(1).Retryable() {
		t.Fatal("400 超长不应可重试（拆段才是正解）")
	}
	if ModelNotConfigured().Retryable() {
		t.Fatal("503 未配置重试无意义")
	}
}

// TestAs 归一化：原生 error 一律转 INTERNAL；*Error 原样透传。
func TestAs(t *testing.T) {
	if As(nil) != nil {
		t.Fatal("nil 应返回 nil")
	}
	if e := As(errors.New("boom")); e.Code != CodeInternal {
		t.Fatalf("普通 error 应归一为 INTERNAL，got %s", e.Code)
	}
	orig := RateLimited(5)
	if As(orig) != orig {
		t.Fatal("*Error 应原样透传")
	}
}

// TestTextTooLongSegment 第 N 段要出现在文案里（FR-305 验收）。
func TestTextTooLongSegment(t *testing.T) {
	if !strings.Contains(TextTooLong(4).Message, "第 4 段") {
		t.Fatalf("应指明段号: %q", TextTooLong(4).Message)
	}
	if TextTooLong(0).Message == "" || TextTooLong(0).Status != http.StatusBadRequest {
		t.Fatal("无段号文案也要可读且 400")
	}
}
