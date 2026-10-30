// respond_test.go 验证统一响应包（§7.4 / NFR-12）。
//
// @file internal/httpx/respond_test.go
// 文件职责：成功信封 {"ok":true,"data":...}、失败信封 {"ok":false,"error":{...}}、
// 429 时 Retry-After 头（FR-605）、非法 JSON 请求体归一为 INVALID_REQUEST。
//
// 模块边界：纯单元测试，httptest 记录响应。
package httpx

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"fluentread/internal/errs"
)

// TestOKEnvelope 成功响应必须是 {"ok":true,"data":...}。
func TestOKEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	OK(w, map[string]int{"n": 1})
	var body struct {
		OK   bool        `json:"ok"`
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是 JSON: %v", err)
	}
	if !body.OK || body.Data["n"] != 1 {
		t.Fatalf("信封错误: %s", w.Body.String())
	}
	if ct := w.Header().Get("content-type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
}

// TestFailEnvelope 错误信封带 code/message/retryable。
func TestFailEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	Fail(w, errs.TooManySegments())
	var body struct {
		OK    bool `json:"ok"`
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.OK || body.Error.Code != errs.CodeTooManySegments {
		t.Fatalf("信封错误: %s", w.Body.String())
	}
	if body.Error.Retryable {
		t.Fatal("400 不应可重试")
	}
	if w.Code != 400 {
		t.Fatalf("status = %d", w.Code)
	}
}

// TestRetryAfterHeader 429 必须带 Retry-After 头（FR-605）。
func TestRetryAfterHeader(t *testing.T) {
	w := httptest.NewRecorder()
	Fail(w, errs.RateLimited(9))
	if got := w.Header().Get("retry-after"); got != "9" {
		t.Fatalf("retry-after = %q, want 9", got)
	}
	if w.Code != 429 {
		t.Fatalf("status = %d", w.Code)
	}
}

// TestDecodeJSONBad 非法 JSON 归一为 INVALID_REQUEST（400）。
func TestDecodeJSONBad(t *testing.T) {
	err := DecodeJSON(strings.NewReader("{bad"), &struct{}{})
	if errs.As(err).Code != errs.CodeInvalidRequest {
		t.Fatalf("code = %s", errs.As(err).Code)
	}
	if err == nil || !errors.Is(errs.As(err), errs.As(err)) {
		t.Fatal("应返回错误")
	}
}
