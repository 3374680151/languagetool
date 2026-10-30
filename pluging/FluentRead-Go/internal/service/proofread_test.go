// proofread_test.go 验证纠错的服务端兜底校验（FR-203/204/206/209）。
//
// @file internal/service/proofread_test.go
// 文件职责：original 非子串丢弃、类别枚举校验、去重、CJK 跳过、空文本拒绝。
//
// 模块边界：纯单元测试（模型调用路径由 e2e 覆盖）。
package service

import (
	"testing"
)

func newProoftester() *Proofreader {
	return &Proofreader{}
}

// TestParseAndValidateSubstring 模型返回不存在的片段时丢弃且不报错（FR-204）。
func TestParseAndValidateSubstring(t *testing.T) {
	source := "The system recieves data from a RAG pipeline."
	raw := `[
		{"original":"recieves","suggestion":"receives","category":"spelling","reason":"i before e except after c。"},
		{"original":"not-in-source","suggestion":"x","category":"grammar","reason":"子串不存在"},
		{"original":"a RAG","suggestion":"an RAG","category":"grammar","reason":"元音开头用 an。"}
	]`
	got := newProoftester().parseAndValidate(raw, source)
	if len(got) != 2 {
		t.Fatalf("应保留 2 条（不存在的片段被丢弃）: %+v", got)
	}
	if got[0].Original != "recieves" || got[1].Original != "a RAG" {
		t.Fatalf("保留的条目不对: %+v", got)
	}
}

// TestParseAndValidateEnum 类别不在枚举内丢弃（FR-203）。
func TestParseAndValidateEnum(t *testing.T) {
	source := "The system recieves data."
	raw := `[
		{"original":"recieves","suggestion":"receives","category":"语气","reason":"类别非法"},
		{"original":"recieves","suggestion":"receives","category":"spelling","reason":"合法"}
	]`
	got := newProoftester().parseAndValidate(raw, source)
	if len(got) != 1 || got[0].Category != "spelling" {
		t.Fatalf("非法类别应被丢弃: %+v", got)
	}
}

// TestParseAndValidateReason 理由为空丢弃（FR-203 教学解释）。
func TestParseAndValidateReason(t *testing.T) {
	source := "The system recieves data."
	raw := `[
		{"original":"recieves","suggestion":"receives","category":"spelling","reason":"  "},
		{"original":"recieves","suggestion":"receives","category":"spelling","reason":"有理由"}
	]`
	got := newProoftester().parseAndValidate(raw, source)
	if len(got) != 1 || got[0].Reason != "有理由" {
		t.Fatalf("空理由应被丢弃: %+v", got)
	}
}

// TestParseAndValidateDedup 相同 original→suggestion 只保留一条。
func TestParseAndValidateDedup(t *testing.T) {
	source := "The system recieves data."
	raw := `[
		{"original":"recieves","suggestion":"receives","category":"spelling","reason":"a"},
		{"original":"recieves","suggestion":"receives","category":"spelling","reason":"b"}
	]`
	got := newProoftester().parseAndValidate(raw, source)
	if len(got) != 1 {
		t.Fatalf("重复建议应去重: %+v", got)
	}
}

// TestParseAndValidateBadJSON 模型输出不是 JSON 时返回空（不报 500，FR-209）。
func TestParseAndValidateBadJSON(t *testing.T) {
	got := newProoftester().parseAndValidate("这不是 JSON", "any text")
	if len(got) != 0 {
		t.Fatalf("坏输出应得空列表: %+v", got)
	}
}

// TestProofreadCJKSkip 中文段直接跳过（FR-206）。
func TestProofreadCJKSkip(t *testing.T) {
	// 无池的 Proofreader 遇到 CJK 应在打模型之前返回空建议。
	resp, err := newProoftester().Proofread(testCtx(), &ProofreadRequest{Text: "这是一段中文", Lang: "zh-Hans"})
	if err != nil {
		t.Fatalf("中文段应跳过而不是报错: %v", err)
	}
	if len(resp.Suggestions) != 0 {
		t.Fatalf("中文段应无建议: %+v", resp.Suggestions)
	}
}

// TestProofreadEmpty 空文本拒绝。
func TestProofreadEmpty(t *testing.T) {
	if _, err := newProoftester().Proofread(testCtx(), &ProofreadRequest{Text: "   ", Lang: "en"}); err == nil {
		t.Fatal("空文本应报 INVALID_REQUEST")
	}
}
