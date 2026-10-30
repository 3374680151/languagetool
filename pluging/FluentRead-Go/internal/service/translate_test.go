// translate_test.go 验证翻译编排：分段校验（FR-301/305）与计费口径（FR-604）。
//
// @file internal/service/translate_test.go
// 文件职责：Validate 的全部拒绝路径、billable/dominantModel 纯函数、
// Validate 风格回落（FR-302）。
//
// 模块边界：纯单元测试（Translate 的端到端由 e2e 覆盖）。
package service

import (
	"context"
	"strings"
	"testing"

	"fluentread/internal/errs"
)

func segReq(texts ...string) *TranslateRequest {
	return &TranslateRequest{Segments: texts, From: "en", To: "zh-Hans"}
}

// TestValidateLimits 超段数、超长度、超合计都要 400 一族（FR-305）。
func TestValidateLimits(t *testing.T) {
	if err := segReq().Validate(); err == nil {
		t.Fatal("空段落应拒绝")
	}
	tooMany := &TranslateRequest{Segments: make([]string, MaxSegments+1), From: "en", To: "zh-Hans"}
	if err := tooMany.Validate(); err == nil {
		t.Fatal("超 20 段应拒绝")
	} else if code := errCodeOf(err); code != "TOO_MANY_SEGMENTS" {
		t.Fatalf("code = %s", code)
	}

	long := &TranslateRequest{Segments: []string{strings.Repeat("a", MaxSegmentLength+1)}, From: "en", To: "zh-Hans"}
	if err := long.Validate(); err == nil || errCodeOf(err) != "TEXT_TOO_LONG" {
		t.Fatalf("单段超长应 TEXT_TOO_LONG: %v", err)
	}

	total := &TranslateRequest{
		Segments: []string{strings.Repeat("a", 12000), strings.Repeat("b", 12000)},
		From: "en", To: "zh-Hans",
	}
	if err := total.Validate(); err == nil || errCodeOf(err) != "TEXT_TOO_LONG" {
		t.Fatalf("合计超长应 TEXT_TOO_LONG: %v", err)
	}
}

// TestValidateLanguage 非法语言码返回 UNSUPPORTED_LANGUAGE 而不是 500（FR-301）。
func TestValidateLanguage(t *testing.T) {
	r := segReq("hi")
	r.To = "xx-YY"
	if err := r.Validate(); err == nil || errCodeOf(err) != "UNSUPPORTED_LANGUAGE" {
		t.Fatalf("got %v", err)
	}
	r2 := segReq("hi")
	r2.From = "zh-Hans"
	r2.To = "zh-Hans"
	if err := r2.Validate(); err == nil {
		t.Fatal("同语言应拒绝")
	}
}

// TestValidateBlankSegments 空白段归一为空占位且下标保留（FR-104/304）。
func TestValidateBlankSegments(t *testing.T) {
	r := segReq("first", "  ", "", "second")
	if err := r.Validate(); err != nil {
		t.Fatalf("含空白段应通过: %v", err)
	}
	if r.Segments[0] != "first" || r.Segments[1] != "" || r.Segments[2] != "" || r.Segments[3] != "second" {
		t.Fatalf("归一错误: %q", r.Segments)
	}
	// 计费只算非空段（FR-604）。
	s, c := billable(r.Segments)
	if s != 2 || c != len("first")+len("second") {
		t.Fatalf("billable = %d,%d", s, c)
	}
}

// TestValidateStyleFallback 风格拼错回落默认（FR-302）。
func TestValidateStyleFallback(t *testing.T) {
	r := segReq("hi")
	r.Style = "acadeemic"
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if r.Style != DefaultStyle {
		t.Fatalf("应回落 %s, got %s", DefaultStyle, r.Style)
	}
}

// TestDominantModel 结果展示取多数模型。
func TestDominantModel(t *testing.T) {
	rs := []SegmentResult{{Status: "ok", Model: "a"}, {Status: "ok", Model: "a"}, {Status: "failed"}}
	if dominantModel(rs) != "a" {
		t.Fatal("应取多数")
	}
	if dominantModel(nil) != "" {
		t.Fatal("空结果应为空")
	}
}

// errCodeOf 取错误码。
func errCodeOf(err error) string {
	return errs.As(err).Code
}

// testCtx 供 proofread 测试使用的空 context。
func testCtx() context.Context { return context.Background() }
