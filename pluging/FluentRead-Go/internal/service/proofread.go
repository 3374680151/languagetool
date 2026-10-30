// proofread.go 是纠错编排（P3，FR-2xx）。
//
// @file internal/service/proofread.go
// 文件职责：把一段英文交给模型逐条挑错，并做服务端兜底校验——
// original 必须是原文子串（FR-204，模型不许算偏移量）、类别必须在枚举内（FR-203）、
// 理由必须非空（FR-203 教学解释）。全部被丢弃时返回"没有发现问题"（FR-209）。
// 只服务英文段落：CJK 文本直接跳过（FR-206）。
//
// 主要内容：Proofreader、ProofreadRequest、Suggestion、Proofread。
//
// 模块边界：依赖 gateway（协议）、errs、store（站点用量只记数）；不认识 Gin。
// 纠错不做整段重写（FR-208）：模型的职责是逐条建议，不是改写。
package service

import (
	"context"
	"encoding/json"
	"strings"

	"fluentread/internal/config"
	"fluentread/internal/errs"
	"fluentread/internal/gateway"
	"fluentread/internal/store"
)

// Proofreader 是纠错编排器。
type Proofreader struct {
	pool  *gateway.Pool
	usage *store.UsageRepo
	cfg   *config.Config
}

// NewProofreader 构造纠错编排器。
func NewProofreader(pool *gateway.Pool, usage *store.UsageRepo, cfg *config.Config) *Proofreader {
	return &Proofreader{pool: pool, usage: usage, cfg: cfg}
}

// Suggestion 是一条纠错建议（FR-203 四字段）。
type Suggestion struct {
	Original   string `json:"original"`
	Suggestion string `json:"suggestion"`
	Category   string `json:"category"`
	Reason     string `json:"reason"`
}

// ProofreadRequest 是一次纠错请求。
type ProofreadRequest struct {
	Text string
	Lang string
}

// ProofreadResponse 是纠错结果。
type ProofreadResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

// Proofread 执行纠错。
func (p *Proofreader) Proofread(ctx context.Context, req *ProofreadRequest) (*ProofreadResponse, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, errs.InvalidRequest("没有要检查的文本")
	}
	// FR-206：只检查英文段落。CJK 文本直接给"没有发现问题"——不是错误，是不适用。
	if gateway.DetectScript(text) == gateway.ScriptCJK {
		return &ProofreadResponse{Suggestions: []Suggestion{}}, nil
	}

	model := p.pool.DefaultModel()
	if model == "" {
		return nil, errs.ModelNotConfigured()
	}

	system := gateway.ProofreadSystemPrompt(p.cfg.ProofreadPrompt)
	resp, err := p.pool.SiteClient().Chat(ctx, gateway.ChatRequest{
		Model:       model,
		Messages:    [][2]string{{"system", system}, {"user", text}},
		Temperature: 0.1,
		MaxTokens:   2048,
	})
	if err != nil {
		return nil, err
	}

	suggestions := p.parseAndValidate(resp.Text, text)

	// 站点用量：只记请求次数（proofread_req），不计付费字符——
	// 站点字符封顶针对翻译体量（FR-603/604），纠错次数另计。
	_ = p.usage.AddSiteUsage(0, 0, 0, 1, 0)

	return &ProofreadResponse{Suggestions: suggestions}, nil
}

// parseAndValidate 解析模型输出并逐条校验（FR-203/204）。
// 模型输出不是合法 JSON、或没有一条通过校验时，返回空列表（FR-209 由 handler 呈现）。
func (p *Proofreader) parseAndValidate(raw, source string) []Suggestion {
	var candidates []Suggestion
	if err := json.Unmarshal([]byte(raw), &candidates); err != nil {
		return nil
	}

	out := make([]Suggestion, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		// FR-204：original 必须是原文子串，否则丢弃且不报错。
		if c.Original == "" || !strings.Contains(source, c.Original) {
			continue
		}
		// FR-203：类别枚举 + 理由非空（教学解释由提示词约束，服务端兜底非空）。
		if !ValidCategory(c.Category) || strings.TrimSpace(c.Reason) == "" {
			continue
		}
		if strings.TrimSpace(c.Suggestion) == "" {
			continue
		}
		// 去重：同一段内相同的 original+suggestion 只保留一条。
		key := c.Original + "→" + c.Suggestion
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Suggestion{
			Original:   c.Original,
			Suggestion: c.Suggestion,
			Category:   c.Category,
			Reason:     c.Reason,
		})
	}
	return out
}
