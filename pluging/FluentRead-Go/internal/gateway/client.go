// Package gateway 是模型网关层：OpenAI 兼容协议客户端、多 Key 轮换、
// 模型池与降级链、翻译缓存、提示词模板。
//
// @file internal/gateway/client.go
// 文件职责：对上游只讲一种协议——OpenAI 兼容 chat/completions 与 GET /v1/models
//（DR-7/DR-9）；实现多 Key 轮换（FR-1102：失败自动切下一把并冷却，全部失败才报错）；
// 密钥只在本进程内存，不写日志、不落盘（FR-1002）。
//
// 主要内容：Client、ChatRequest/ChatResponse、Chat、Models、keyState 冷却表。
//
// 模块边界：不认识 Gin 与数据库；错误一律构造为 internal/errs 的错误码；
// 调用方（service/gateway.pool）负责重试与降级编排。
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"fluentread/internal/errs"
)

// Client 是 OpenAI 兼容上游客户端。
type Client struct {
	base string
	// keys 是可用密钥列表（FR_API_KEY 逗号分隔，FR-1102）。
	keys []string
	http *http.Client

	mu       sync.Mutex
	// cooldown 记录每把密钥的冷却截止时刻：命中 401/403/429 的 key 短期内跳过。
	cooldown map[int]time.Time
	// cooldownFor 是单把密钥的冷却时长。
	cooldownFor time.Duration
	// now 可被测试替换。
	now func() time.Time
}

// ChatRequest 是一次补全请求。
type ChatRequest struct {
	Model       string
	Messages    [][2]string // [role, content] 二元组，保持顺序
	Temperature float64
	MaxTokens   int
}

// ChatResponse 是补全结果。
type ChatResponse struct {
	Text string
	MS   int64
}

// New 构造客户端。base 形如 http://litellm:4000（不含 /v1）；keys 按顺序轮换。
func New(base string, keys []string, timeout time.Duration) *Client {
	cleaned := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := strings.TrimSpace(k); v != "" {
			cleaned = append(cleaned, v)
		}
	}
	return &Client{
		base:        strings.TrimRight(base, "/"),
		keys:        cleaned,
		http:        &http.Client{Timeout: timeout},
		cooldown:    make(map[int]time.Time),
		cooldownFor: 60 * time.Second,
		now:         time.Now,
	}
}

// Configured 表示网关地址与密钥都已配置。未配置时上层应走六态之"未配置"。
func (c *Client) Configured() bool {
	return c != nil && c.base != "" && len(c.keys) > 0
}

// Base 返回网关地址（健康检查与巡检用）。
func (c *Client) Base() string { return c.base }

// availableKeys 返回当前未冷却的密钥下标列表。
func (c *Client) availableKeys() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	out := make([]int, 0, len(c.keys))
	for i := range c.keys {
		if until, bad := c.cooldown[i]; bad && now.Before(until) {
			continue
		}
		out = append(out, i)
	}
	// 全部冷却时也放行原顺序：冷却是为了跳过已知坏 key，
	// 而不是把整条线路判死——密钥可能已经恢复正常。
	if len(out) == 0 {
		for i := range c.keys {
			out = append(out, i)
		}
	}
	return out
}

// coolDown 把一把密钥标记为冷却。仅对"换 key 有意义"的错误调用（401/403/429）。
func (c *Client) coolDown(idx int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cooldown[idx] = c.now().Add(c.cooldownFor)
}

// Chat 调用 /v1/chat/completions，按序尝试可用密钥（FR-1102）。
// 换 key 有意义的错误（401/403/429）会冷却该 key 并尝试下一把；
// 网络错误/其余状态对"哪把 key"不敏感，直接按线路失败返回，由上层决定降级。
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if !c.Configured() {
		return ChatResponse{}, errs.ModelNotConfigured()
	}

	body := map[string]any{
		"model":       req.Model,
		"messages":    messagesPayload(req.Messages),
		"temperature": req.Temperature,
		"stream":      false,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, errs.Internal(err)
	}

	var lastErr error
	for _, idx := range c.availableKeys() {
		started := time.Now()
		text, status, err := c.post(ctx, "/v1/chat/completions", raw, c.keys[idx])
		if err == nil {
			return ChatResponse{Text: text, MS: time.Since(started).Milliseconds()}, nil
		}
		lastErr = err
		if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusTooManyRequests {
			// 密钥问题：换下一把有意义。冷却避免后续请求继续撞同一把坏 key。
			c.coolDown(idx)
			continue
		}
		// 线路级错误：换 key 大概率没用，直接返回最后一个错误。
		return ChatResponse{}, err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用密钥")
	}
	return ChatResponse{}, lastErr
}

// Models 拉取上游实际可调用的模型清单，用于巡检对账（FR-408/FR-1103）。
func (c *Client) Models(ctx context.Context) ([]string, error) {
	if !c.Configured() {
		return nil, errs.ModelNotConfigured()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/models", nil)
	if err != nil {
		return nil, errs.Internal(err)
	}
	req.Header.Set("authorization", "Bearer "+c.keys[0])

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errs.UpstreamUnavailable(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.UpstreamUnavailable(fmt.Errorf("模型清单返回 %d", resp.StatusCode))
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&parsed); err != nil {
		return nil, errs.UpstreamUnavailable(err)
	}

	out := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if item.ID != "" {
			out = append(out, item.ID)
		}
	}
	return out, nil
}

// post 发送一次 POST 并解析 OpenAI 兼容的补全响应。
// 返回值 status 是 HTTP 状态码，供 Chat 决定是否换 key。
func (c *Client) post(ctx context.Context, path string, body []byte, key string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
	if err != nil {
		return "", 0, errs.Internal(err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+key)

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			// 上下文取消：区分"调用方主动放弃"与"超过预算"由上层判断，
			// 这里统一包成超时语义（附录 B：UPSTREAM_TIMEOUT 504）。
			return "", 0, errs.UpstreamTimeout(int(c.http.Timeout.Seconds()), err)
		}
		return "", 0, errs.UpstreamUnavailable(err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, errs.UpstreamUnavailable(
			fmt.Errorf("返回 %d: %s", resp.StatusCode, snippet(payload)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", resp.StatusCode, errs.UpstreamUnavailable(fmt.Errorf("响应解析失败: %w", err))
	}
	if len(parsed.Choices) == 0 {
		return "", resp.StatusCode, errs.UpstreamUnavailable(fmt.Errorf("choices 为空"))
	}
	return parsed.Choices[0].Message.Content, resp.StatusCode, nil
}

// messagesPayload 把 [role, content] 二元组转成协议 JSON。
func messagesPayload(msgs [][2]string) []map[string]string {
	out := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, map[string]string{"role": m[0], "content": m[1]})
	}
	return out
}

// snippet 截断上游返回体，避免把整段响应写进日志或错误消息（NFR-7）。
func snippet(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 200 {
		return text[:200] + "…"
	}
	return text
}
