// pool.go 是模型池与降级链（§7.6）：L1 默认线路、L2 站点池、L3 访客自定义、
// MyMemory 免费后备，以及三者之间的编排。
//
// @file internal/gateway/pool.go
// 文件职责：把"一段文本 + 模型选择"解析成一条调用链并执行：
//
//	L3 自定义（有则先试）→ L1/L2 站点线路 →（失败或站点字符触顶）→ MyMemory 免费后备
//
// 全部失败才把该段判为 failed；降级成功时带 degraded:true + 原因（FR-306）。
// 访客自定义端点由本层直接转发（FR-403/404），不经过站长网关，密钥即用即弃。
//
// 主要内容：Pool、OneRequest/OneResult、Pool.TranslateOne、SitePaidCharsToday。
//
// 模块边界：依赖 errs / config（只读 Catalog 与上限值）/ store（站点用量只读）。
// 访客配额与计费在 service 层，本包不碰 usage_daily。
package gateway

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"fluentread/internal/config"
	"fluentread/internal/errs"
)

// 线路名。进日志与内部统计；用户文案由 errs 统一给出（NFR-11）。
const (
	LineSite   = "site"   // L1 默认 / L2 站点池（同一条 OpenAI 兼容线路）
	LineCustom = "custom" // L3 访客自定义端点
	LineFree   = "free"   // MyMemory 后备
)

// CustomUpstream 是 L3 访客自定义端点（FR-403）。
type CustomUpstream struct {
	BaseURL string
	APIKey  string
	Model   string
}

// OneRequest 是单段翻译请求。
type OneRequest struct {
	Text     string
	From     string
	To       string
	Style    string
	StyleHint string
	Glossary [][2]string
	// Model 是访客选的 L2 模型；空 = L1 默认。
	Model string
	// ToLabel 是目标语言中文名，用于提示词。
	ToLabel string
	// Custom 非空时先走访客自定义端点（L3）。
	Custom *CustomUpstream
	// SkipPaid 为 true 表示站点付费字符触顶，直接走免费后备（FR-603）。
	SkipPaid bool
}

// OneResult 是单段翻译结果。
type OneResult struct {
	Text  string
	Model string
	Line  string
	// Degraded 为 true 表示没有走"应走的"线路（触顶或失败后切了后备）。
	Degraded       bool
	DegradedReason string
	MS             int64
	// CustomFailed 为 true 表示自定义模型失败已回落默认模型（FR-405 提示用）。
	CustomFailed bool
}

// Pool 是模型池：一条站点线路 + 可选免费后备 + 缓存。
type Pool struct {
	client *Client
	free   *MyMemory
	cfg    *config.Config
	// catalog 是模型展示清单，用于取 L1 primary（FR-402）。
	catalog *config.Catalog
	cache   *Cache
	// timeout 是单次上游调用的预算（附录 A FR_UPSTREAM_TIMEOUT_SECONDS）。
	timeout time.Duration
	// degraded 是巡检降级标记（FR-1103）：有 enabled 模型连续对账失败即置位。
	degraded atomic.Bool
}

// NewPool 构造池。free 传 nil 表示禁用降级（FR_FALLBACK_TYPE 为空）。
func NewPool(client *Client, free *MyMemory, cfg *config.Config, catalog *config.Catalog, cache *Cache, timeout time.Duration) *Pool {
	return &Pool{client: client, free: free, cfg: cfg, catalog: catalog, cache: cache, timeout: timeout}
}

// Cache 暴露缓存（service 层构造缓存键用）。
func (p *Pool) Cache() *Cache { return p.cache }

// DefaultModel 返回 L1 默认模型名（FR-401）：FR_MODEL 优先，否则清单 primary。
// 未配置时返回空串——上层据此走六态之"未配置"。
func (p *Pool) DefaultModel() string { return p.primaryModel() }

// SiteClient 暴露站点线路客户端（健康检查与巡检用）。
func (p *Pool) SiteClient() *Client { return p.client }

// SetDegraded 供巡检（P3.5，FR-1103）置位：有模型连续对账失败即降级。
func (p *Pool) SetDegraded(v bool) { p.degraded.Store(v) }

// Degraded 返回巡检降级标记，健康检查读取。
func (p *Pool) Degraded() bool { return p.degraded.Load() }

// TranslateOne 执行单段翻译的完整降级链。
// 任何一步成功且结果校验通过即返回；失败尝试下一步；全部失败返回最后错误。
func (p *Pool) TranslateOne(ctx context.Context, req OneRequest) (OneResult, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	var lastErr error

	// ① L3 访客自定义端点：不经过站长网关（§7.6），失败回落默认线路并提示（FR-405）。
	if req.Custom != nil && req.Custom.BaseURL != "" && req.Custom.Model != "" {
		text, err := p.callCustom(ctx, req)
		if err == nil {
			return OneResult{Text: text, Model: req.Custom.Model, Line: LineCustom}, nil
		}
		lastErr = err
		// CustomFailed 标记在 ② 成功时设置。
	}

	// ② 站点线路（L1 默认 / L2 选定模型）。触顶时跳过。
	if !req.SkipPaid && p.client.Configured() {
		model := req.Model
		if model == "" {
			model = p.primaryModel()
		}
		system := TranslateSystemPrompt(p.cfg.TranslatePrompt, PromptOptions{
			To: req.To, ToLabel: req.ToLabel, Style: req.Style,
			StyleHint: req.StyleHint, Glossary: req.Glossary,
		})
		started := time.Now()
		resp, err := p.client.Chat(ctx, ChatRequest{
			Model:       model,
			Messages:    [][2]string{{"system", system}, {"user", req.Text}},
			Temperature: 0.2,
			MaxTokens:   2048,
		})
		if err == nil {
			cleaned := CleanTranslation(resp.Text, req.Text)
			if ValidateTranslation(cleaned, req.Text, req.To) {
				res := OneResult{Text: cleaned, Model: model, Line: LineSite, MS: time.Since(started).Milliseconds()}
				if req.Custom != nil {
					// 走到这里说明自定义失败、默认线路成功（FR-405）。
					res.CustomFailed = true
				}
				return res, nil
			}
			err = errs.UpstreamUnavailable(errInvalidTranslation{kind: "bad_quality"})
		}
		lastErr = err
	} else if req.SkipPaid {
		// 站点触顶跳过了付费线路；若连免费后备都没有，这段注定失败。
		lastErr = errs.UpstreamUnavailable(errSkipPaid{})
	}

	// ③ MyMemory 免费后备（DR-10）：仍失败才向上报错。
	if p.free != nil {
		started := time.Now()
		text, err := p.free.Translate(ctx, req.Text, req.From, req.To)
		if err == nil {
			reason := ""
			if req.SkipPaid {
				// FR-306：触顶是"切换"不是"失败"，原因要说清（内部日志用；
				// 用户文案由 service 层按 NFR-11 另行生成）。
				reason = "site_paid_cap"
			} else if req.Custom != nil {
				reason = "custom_failed"
			} else {
				reason = "model_failed"
			}
			return OneResult{
				Text: text, Line: LineFree, Degraded: true, DegradedReason: reason,
				MS: time.Since(started).Milliseconds(), CustomFailed: req.Custom != nil,
			}, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errs.ModelNotConfigured()
	}
	return OneResult{}, lastErr
}

// callCustom 调访客自定义端点。临时构造一个一次性客户端：密钥只在内存，
// 请求结束即弃（FR-404），不落盘不写日志。
func (p *Pool) callCustom(ctx context.Context, req OneRequest) (string, error) {
	one := New(req.Custom.BaseURL, []string{req.Custom.APIKey}, p.timeout)
	system := TranslateSystemPrompt(p.cfg.TranslatePrompt, PromptOptions{
		To: req.To, ToLabel: req.ToLabel, Style: req.Style,
		StyleHint: req.StyleHint, Glossary: req.Glossary,
	})
	resp, err := one.Chat(ctx, ChatRequest{
		Model:       req.Custom.Model,
		Messages:    [][2]string{{"system", system}, {"user", req.Text}},
		Temperature: 0.2,
		MaxTokens:   2048,
	})
	if err != nil {
		slog.Info("自定义模型调用失败，将回落默认线路", "err", err)
		return "", err
	}
	cleaned := CleanTranslation(resp.Text, req.Text)
	if !ValidateTranslation(cleaned, req.Text, req.To) {
		return "", errs.UpstreamUnavailable(errInvalidTranslation{kind: "bad_quality"})
	}
	return cleaned, nil
}

// primaryModel 返回 L1 默认模型名：FR_MODEL 优先，否则清单 primary（FR-402）。
func (p *Pool) primaryModel() string {
	if p.cfg.Model != "" {
		return p.cfg.Model
	}
	if p.catalog != nil {
		if m, ok := p.catalog.Primary(); ok {
			return m.ID
		}
	}
	return ""
}

// errInvalidTranslation 标记"结果校验不过"类失败。
type errInvalidTranslation struct{ kind string }

func (e errInvalidTranslation) Error() string { return "translation failed validation: " + e.kind }

// errSkipPaid 标记"站点触顶跳过付费线路"。
type errSkipPaid struct{}

func (e errSkipPaid) Error() string { return "site paid cap reached, paid line skipped" }
