// mymemory.go 是免费降级线路 MyMemory 的 Go 直连实现（DR-10：不经 LiteLLM）。
//
// @file internal/gateway/mymemory.go
// 文件职责：把单段文本送 MyMemory 公共接口翻译；仅作为付费线路触顶或失败后的
// 后备（FR-306），默认只当后备不做主线路；测试时可把地址指向本地假上游离线验证。
//
// 主要内容：MyMemory、Translate、myMemoryLang 映射。
//
// 模块边界：只依赖标准库与 internal/errs；被 gateway.Pool 在降级链中调用。
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fluentread/internal/errs"
)

// MyMemory 免费线路客户端。
type MyMemory struct {
	http *http.Client
	// base 是接口地址，默认官方端点；测试指向假上游。
	base string
}

// NewMyMemory 构造免费线路。base 为空时使用官方端点。
func NewMyMemory(timeout time.Duration, base string) *MyMemory {
	if base == "" {
		base = "https://api.mymemory.translated.net/get"
	}
	return &MyMemory{http: &http.Client{Timeout: timeout}, base: base}
}

// Translate 翻译单段文本。MyMemory 不支持批量与源语言自动检测。
// 返回值 err 为 nil 时 Text 是译文；来源线路由调用方标记为 free。
func (m *MyMemory) Translate(ctx context.Context, text, from, to string) (string, error) {
	src := myMemoryLang(from)
	dst := myMemoryLang(to)
	if src == "" || dst == "" {
		// auto 不在支持范围内：该线路帮不上忙，换线语义由上层处理。
		return "", errs.InvalidRequest("该免费方式不支持自动检测源语言")
	}

	query := url.Values{}
	query.Set("q", text)
	query.Set("langpair", src+"|"+dst)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.base+"?"+query.Encode(), nil)
	if err != nil {
		return "", errs.Internal(err)
	}
	req.Header.Set("accept", "application/json")

	resp, err := m.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", errs.UpstreamTimeout(int(m.http.Timeout.Seconds()), err)
		}
		return "", errs.UpstreamUnavailable(err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errs.UpstreamUnavailable(fmt.Errorf("免费接口返回 %d", resp.StatusCode))
	}

	var parsed struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		ResponseStatus any `json:"responseStatus"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", errs.UpstreamUnavailable(fmt.Errorf("免费接口返回格式异常: %w", err))
	}

	out := strings.TrimSpace(parsed.ResponseData.TranslatedText)
	// MyMemory 把限流与错误塞在 translatedText 里，不检查会把警告当译文显示。
	// 两类分开：长度超限是"拆段落"就能解决的确定性错误，报成限流会让前端
	// 给出一个永远重试不成功的重试按钮。
	upper := strings.ToUpper(out)
	if strings.Contains(upper, "QUERY LENGTH LIMIT") {
		return "", errs.TextTooLong(0)
	}
	if out == "" || strings.Contains(upper, "MYMEMORY WARNING") {
		return "", errs.UpstreamUnavailable(fmt.Errorf("免费接口限流或拒绝: %s", snippet([]byte(out))))
	}
	return out, nil
}

// myMemoryLang 把内部语言 id 映射为 MyMemory 的 langpair 取值。
// 返回空串表示该语言不受支持（含 auto），调用方据此决定换线。
func myMemoryLang(id string) string {
	switch id {
	case "zh-Hans":
		return "zh-CN"
	case "zh-Hant":
		return "zh-TW"
	case "en", "ja", "ko", "fr", "de", "es", "ru":
		return id
	default:
		return ""
	}
}
