// gateway_test.go 验证网关层：多 Key 轮换（FR-1102）、缓存（FR-1101）、
// 结果校验（§7.6）、降级链（FR-306）、MyMemory 语义（DR-10）。
//
// @file internal/gateway/gateway_test.go
// 文件职责：用 httptest 起假 OpenAI 兼容上游与假 MyMemory，逐条验证
// 网关层行为；全部离线、确定性。
//
// 模块边界：纯单元测试；不依赖外部网络。
package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fluentread/internal/config"
	"fluentread/internal/errs"
)

// chatPayload 构造一个 OpenAI 兼容的成功响应。
func chatPayload(content string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{
			{"index": 0, "message": map[string]any{"role": "assistant", "content": content}},
		},
	}
}

// TestClientMultiKeyRotation 配 2 把 Key，第 1 把注入 401，翻译仍成功（FR-1102）。
func TestClientMultiKeyRotation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("authorization")
		if auth == "Bearer key-a" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"bad key"}`))
			return
		}
		writeTestJSON(w, chatPayload("你好"))
	}))
	defer srv.Close()

	c := New(srv.URL, []string{"key-a", "key-b"}, 5*time.Second)
	resp, err := c.Chat(context.Background(), ChatRequest{Model: "m", Messages: [][2]string{{"user", "hi"}}})
	if err != nil {
		t.Fatalf("换 Key 后应成功: %v", err)
	}
	if resp.Text != "你好" {
		t.Fatalf("Text = %q", resp.Text)
	}
	// 第 1 把 key 应进入冷却：后续请求直接用第 2 把（不再打 401）。
	if _, until := c.cooldown[0]; !until {
		t.Fatal("401 的 key 应被冷却")
	}
}

// TestClientAllKeysFail 全部密钥失败才向上报错。
func TestClientAllKeysFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, []string{"k1", "k2"}, 5*time.Second)
	if _, err := c.Chat(context.Background(), ChatRequest{Model: "m"}); err == nil {
		t.Fatal("全部 key 失败应报错")
	}
}

// TestClientNotConfigured 未配置时返回 MODEL_NOT_CONFIGURED（六态之"未配置"）。
func TestClientNotConfigured(t *testing.T) {
	c := New("", nil, time.Second)
	_, err := c.Chat(context.Background(), ChatRequest{Model: "m"})
	if errs.As(err).Code != errs.CodeModelNotConfigured {
		t.Fatalf("code = %s", errs.As(err).Code)
	}
}

// TestClientEmptyChoices 空响应判为线路失败。
func TestClientEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"choices": []any{}})
	}))
	defer srv.Close()
	c := New(srv.URL, []string{"k"}, 5*time.Second)
	if _, err := c.Chat(context.Background(), ChatRequest{Model: "m"}); err == nil {
		t.Fatal("空 choices 应报错")
	}
}

// TestCacheLRUAndTTL 缓存淘汰与过期（FR-1101）。
func TestCacheLRUAndTTL(t *testing.T) {
	c := NewCache(2, 1<<20, 50*time.Millisecond)
	c.Put("a", "1")
	c.Put("b", "2")
	c.Put("c", "3") // 淘汰最久未用的 a
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 应被 LRU 淘汰")
	}
	if v, ok := c.Get("b"); !ok || v != "2" {
		t.Fatal("b 应在")
	}
	// TTL 过期。
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("b"); ok {
		t.Fatal("b 应过期")
	}
}

// TestCacheByteCap 单条超上限不缓存；总字节上限触发淘汰。
func TestCacheByteCap(t *testing.T) {
	c := NewCache(100, 32, time.Hour)
	c.Put("big", strings.Repeat("x", 64))
	if _, ok := c.Get("big"); ok {
		t.Fatal("超字节上限的单条不应缓存")
	}
}

// TestValidateTranslation 三条拒绝规则（§7.6）。
func TestValidateTranslation(t *testing.T) {
	if ValidateTranslation("", "hi", "zh-Hans") {
		t.Fatal("空译文应拒绝")
	}
	if ValidateTranslation("hello", "hello", "zh-Hans") {
		t.Fatal("原文回显应拒绝")
	}
	if ValidateTranslation("still english words", "hi", "zh-Hans") {
		t.Fatal("目标中文却无 CJK 应拒绝")
	}
	if !ValidateTranslation("你好，世界", "hello", "zh-Hans") {
		t.Fatal("正常中文应通过")
	}
	if !ValidateTranslation("Hello world", "hi", "en") {
		t.Fatal("英文应通过")
	}
	if ValidateTranslation("全是中文没有外文", "hi", "en") {
		t.Fatal("目标英文却全是 CJK 应拒绝")
	}
}

// TestCleanTranslation 包裹物清理。
func TestCleanTranslation(t *testing.T) {
	if got := CleanTranslation(`"你好"`, "hi"); got != "你好" {
		t.Fatalf("引号应剥掉: %q", got)
	}
	if got := CleanTranslation("译文：你好", "hi"); got != "你好" {
		t.Fatalf("前缀应剥掉: %q", got)
	}
	if got := CleanTranslation("hello\n你好", "hello"); got != "你好" {
		t.Fatalf("原文回显前缀应剥掉: %q", got)
	}
}

// TestDetectScript 轻量脚本检测。
func TestDetectScript(t *testing.T) {
	if DetectScript("hello") != ScriptLatin {
		t.Fatal("英文应判拉丁")
	}
	if DetectScript("你好") != ScriptCJK {
		t.Fatal("中文应判 CJK")
	}
}

// TestPoolDegradeChain 站点线路 500 → 免费后备成功，带 degraded（FR-306）。
func TestPoolDegradeChain(t *testing.T) {
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{
			"responseData": map[string]any{"translatedText": "[free] 你好，世界"},
			"responseStatus": 200,
		})
	}))
	defer free.Close()

	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer site.Close()

	p := NewPool(New(site.URL, []string{"k"}, 5*time.Second), NewMyMemory(5*time.Second, free.URL+"/get"),
		siteCfg(), nil, nil, 5*time.Second)
	res, err := p.TranslateOne(context.Background(), OneRequest{
		Text: "hello", From: "en", To: "zh-Hans", ToLabel: "简体中文",
	})
	if err != nil {
		t.Fatalf("降级后应成功: %v", err)
	}
	if !res.Degraded || res.Line != LineFree || res.DegradedReason != "model_failed" {
		t.Fatalf("应为降级结果: %+v", res)
	}
}

// TestPoolPaidOK 付费线路成功不计降级。
func TestPoolPaidOK(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, chatPayload("你好，世界"))
	}))
	defer site.Close()

	p := NewPool(New(site.URL, []string{"k"}, 5*time.Second), nil, siteCfg(), nil, nil, 5*time.Second)
	res, err := p.TranslateOne(context.Background(), OneRequest{
		Text: "hello", From: "en", To: "zh-Hans", ToLabel: "简体中文",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Degraded || res.Line != LineSite || res.Text != "你好，世界" {
		t.Fatalf("应为正常付费结果: %+v", res)
	}
}

// TestPoolSkipPaid 触顶时直接走免费线路（FR-603）。
func TestPoolSkipPaid(t *testing.T) {
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{
			"responseData":   map[string]any{"translatedText": "免费结果"},
			"responseStatus": 200,
		})
	}))
	defer free.Close()

	paid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("触顶后不应打付费线路")
	}))
	defer paid.Close()

	p := NewPool(New(paid.URL, []string{"k"}, 5*time.Second), NewMyMemory(5*time.Second, free.URL+"/get"),
		siteCfg(), nil, nil, 5*time.Second)
	res, err := p.TranslateOne(context.Background(), OneRequest{
		Text: "hello", From: "en", To: "zh-Hans", ToLabel: "简体中文", SkipPaid: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Degraded || res.DegradedReason != "site_paid_cap" {
		t.Fatalf("触顶降级原因应为 site_paid_cap: %+v", res)
	}
}

// TestPoolEchoRejected 站点线路回显原文 → 判失败 → 无后备时报错。
func TestPoolEchoRejected(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, chatPayload("hello world"))
	}))
	defer site.Close()

	p := NewPool(New(site.URL, []string{"k"}, 5*time.Second), nil, siteCfg(), nil, nil, 5*time.Second)
	if _, err := p.TranslateOne(context.Background(), OneRequest{
		Text: "hello world", From: "en", To: "zh-Hans", ToLabel: "简体中文",
	}); err == nil {
		t.Fatal("回显应判失败")
	}
}

// TestPoolCustomFallback 自定义模型失败回落默认线路并标记（FR-405）。
func TestPoolCustomFallback(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, chatPayload("默认线路译文"))
	}))
	defer site.Close()

	p := NewPool(New(site.URL, []string{"k"}, 5*time.Second), nil, siteCfg(), nil, nil, 5*time.Second)
	res, err := p.TranslateOne(context.Background(), OneRequest{
		Text: "hello", From: "en", To: "zh-Hans", ToLabel: "简体中文",
		Custom: &CustomUpstream{BaseURL: "http://127.0.0.1:1", APIKey: "x", Model: "m"},
	})
	if err != nil {
		t.Fatalf("回落应成功: %v", err)
	}
	if !res.CustomFailed || res.Line != LineSite {
		t.Fatalf("应标记 CustomFailed: %+v", res)
	}
}

// TestMyMemoryLangPair auto 与不支持语言要被拒（换线语义）。
func TestMyMemoryLangPair(t *testing.T) {
	m := NewMyMemory(time.Second, "http://127.0.0.1:1/get")
	if _, err := m.Translate(context.Background(), "hi", "auto", "zh-Hans"); err == nil {
		t.Fatal("auto 应不支持")
	}
}

// TestMyMemoryLengthLimit 长度超限是确定性错误（TEXT_TOO_LONG）。
func TestMyMemoryLengthLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{
			"responseData": map[string]any{"translatedText": "QUERY LENGTH LIMIT EXCEEDED"},
		})
	}))
	defer srv.Close()
	m := NewMyMemory(time.Second, srv.URL)
	_, err := m.Translate(context.Background(), "hi", "en", "zh-Hans")
	if errs.As(err).Code != errs.CodeTextTooLong {
		t.Fatalf("应为 TEXT_TOO_LONG, got %s", errs.As(err).Code)
	}
}

// TestFingerprintChanges 术语变更 → 缓存指纹变化（FR-502/1101）。
func TestFingerprintChanges(t *testing.T) {
	a := CacheFingerprint("m", "en", "zh-Hans", "academic", [][2]string{{"recall", "召回"}}, "hi")
	b := CacheFingerprint("m", "en", "zh-Hans", "academic", [][2]string{{"recall", "回收"}}, "hi")
	if a == b {
		t.Fatal("术语变了指纹必须变")
	}
	if CacheFingerprint("m", "en", "zh-Hans", "academic", nil, "hi") ==
		CacheFingerprint("m2", "en", "zh-Hans", "academic", nil, "hi") {
		t.Fatal("模型变了指纹必须变")
	}
}

// TestTranslatePromptGlossary 术语真的进提示词（FR-502）。
func TestTranslatePromptGlossary(t *testing.T) {
	p := TranslateSystemPrompt("", PromptOptions{
		ToLabel: "简体中文", StyleHint: "学术写作风格", Glossary: [][2]string{{"recall", "召回率"}},
	})
	if !strings.Contains(p, "recall → 召回率") {
		t.Fatalf("术语应出现在提示词里: %s", p)
	}
}

// siteCfg 构造测试用站点配置。
func siteCfg() *config.Config {
	return &config.Config{BaseURL: "http://x", UpstreamTimeout: 5 * time.Second}
}

func writeTestJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
