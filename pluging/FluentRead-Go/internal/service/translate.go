// translate.go 是翻译编排（P1）：分段校验 → 配额预扣 → 并发调用 → 结算计费。
//
// @file internal/service/translate.go
// 文件职责：把"一批段落"翻译成"逐段独立的结果"。入口顺序刻意排定，不能调换：
//
//	限流（handler 做，内存，最便宜）→ 参数校验 → 访客日配额（读库）→ 站点字符触顶判定 → 才打上游。
//
// 任何一步失败都在"花钱之前"结束，否则拦截等于没拦（FR-602 验收：被拒请求一次都不打上游）。
// 单段失败只影响该段（FR-303）；空白段 skipped 不计费不打上游（FR-304/604）；
// 缓存命中不调上游、不计配额（FR-1101）。
//
// 主要内容：Translator、TranslateRequest/TranslateResponse、Translate。
//
// 模块边界：依赖 gateway（协议与降级链）、store（配额与站点用量）、errs。
// 不认识 Gin；handler 负责限流与参数解码。
package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"fluentread/internal/config"
	"fluentread/internal/dict"
	"fluentread/internal/errs"
	"fluentread/internal/gateway"
	"fluentread/internal/store"
)

// 线路名（与 gateway 保持同值，内部结算用；不进用户文案，NFR-11）。
const (
	lineSite   = "site"
	lineCustom = "custom"
	lineFree   = "free"
)

// Translator 是翻译编排器。
type Translator struct {
	pool  *gateway.Pool
	usage *store.UsageRepo
	cfg   *config.Config
	// dict 与 words 服务"再用标记"（FR-1202）：收藏词在翻译的源文里再次出现
	//（词形还原后匹配，与查词卡同一实现，AD-7）→ 使用次数 +1；不打断写作。
	dict  *dict.Dictionary
	words *store.WordRepo
}

// NewTranslator 构造编排器。dict/words 可为 nil（再使用标记停用）。
func NewTranslator(pool *gateway.Pool, usage *store.UsageRepo, cfg *config.Config, dictionary *dict.Dictionary, words *store.WordRepo) *Translator {
	return &Translator{pool: pool, usage: usage, cfg: cfg, dict: dictionary, words: words}
}

// TranslateRequest 是一批段落的翻译请求（§7.4）。
type TranslateRequest struct {
	// Segments 与请求里的段落一一对应；空白段在 Validate 阶段归一为 ""。
	Segments []string
	From     string
	To       string
	Style    string
	// Model 是访客选的 L2 模型；空 = L1 默认。
	Model string
	// Custom 是访客自定义端点（L3），可为 nil。
	Custom *gateway.CustomUpstream
	// Glossary 是生效术语（service 层从 store 读取后传入）。
	Glossary [][2]string
}

// Validate 校验并归一化请求参数（FR-301/302/305）。
// 返回错误时直接对外（400 一族）；全部空白段视为无效请求。
func (r *TranslateRequest) Validate() error {
	if len(r.Segments) == 0 {
		return errs.InvalidRequest("缺少要翻译的段落")
	}
	if len(r.Segments) > MaxSegments {
		return errs.TooManySegments()
	}

	total := 0
	nonEmpty := 0
	for i, raw := range r.Segments {
		text := strings.TrimSpace(raw)
		if text == "" {
			// 空白段保留占位，保证结果下标与请求一一对应（FR-104）。
			r.Segments[i] = ""
			continue
		}
		nonEmpty++
		n := utf8.RuneCountInString(text)
		if n > MaxSegmentLength {
			return errs.TextTooLong(i + 1)
		}
		r.Segments[i] = text
		total += n
	}
	if nonEmpty == 0 {
		return errs.InvalidRequest("没有任何非空段落")
	}
	if total > MaxTotalLength {
		return errs.TextTooLong(0)
	}

	if r.From == "" {
		r.From = DefaultFrom
	}
	if r.To == "" {
		r.To = DefaultTo
	}
	if !ValidLanguage(r.From) || !ValidLanguage(r.To) {
		return errs.UnsupportedLanguage()
	}
	if r.From == r.To && r.From != "auto" {
		return errs.UnsupportedLanguage()
	}
	r.Style = NormalizeStyle(r.Style)
	return nil
}

// SegmentResult 是单段结果（§7.4）。Status 取值 ok|skipped|failed。
type SegmentResult struct {
	Status   string `json:"status"`
	Text     string `json:"text,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Degraded bool   `json:"degraded,omitempty"`
	Model    string `json:"model,omitempty"`
	MS       int64  `json:"ms,omitempty"`
	// CustomFailed：自定义模型失败已回落默认线路（FR-405 提示）。
	CustomFailed bool `json:"customFailed,omitempty"`
	// line 是内部结算字段：本段实际走的线路（site/custom/free），不对外序列化。
	line string
}

// TranslateResponse 是整批结果与降级信息。
type TranslateResponse struct {
	Segments []SegmentResult `json:"segments"`
	// Degraded 只要有一段走了免费后备就为 true（FR-306）。
	Degraded       bool   `json:"degraded,omitempty"`
	DegradedReason string `json:"degradedReason,omitempty"`
	Model          string `json:"model,omitempty"`
	// GlossaryFingerprint 是本次翻译实际使用的术语指纹（FR-503 过期判定）。
	GlossaryFingerprint string `json:"glossaryFingerprint,omitempty"`
}

// Translate 执行整批翻译。调用前配额预扣已由本方法完成；
// ctx 由 handler 提供（请求取消会传导到上游调用）。
func (t *Translator) Translate(ctx context.Context, visitorID string, req *TranslateRequest) (*TranslateResponse, error) {
	// Step 0: 分段校验（FR-301/302/305）：空/超长/超段数/语言码，全部发生在打上游之前。
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Step 1: 访客日配额预扣（FR-601/604）：只算非空白段；在打上游之前完成。
	segments, chars := billable(req.Segments)
	decision, err := t.usage.Consume(visitorID, segments, chars, t.cfg.VisitorDailySegments)
	if err != nil {
		return nil, errs.Internal(err)
	}
	if !decision.Allowed {
		return nil, errs.QuotaExceeded(int(decision.Used), secondsUntil(decision.ResetAt))
	}

	// Step 2: 站点付费字符触顶判定（FR-603）：触顶切免费线路而不是拒绝。
	skipPaid := t.siteCapReached(chars)

	// Step 3: 逐段并发（信号量 MaxConcurrency），单段失败只影响该段（FR-303）。
	results := make([]SegmentResult, len(req.Segments))
	sem := make(chan struct{}, MaxConcurrency)
	var wg sync.WaitGroup

	for i, text := range req.Segments {
		if text == "" {
			// 空白段：不打上游、不计配额（FR-304）。
			results[i] = SegmentResult{Status: "skipped"}
			continue
		}
		wg.Add(1)
		go func(idx int, source string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = t.translateSegment(ctx, source, req, skipPaid)
		}(i, text)
	}
	wg.Wait()

	// Step 4: 结算（FR-604）：成功非 skipped 段才计站点请求；
	// 站点付费线路（site）成功才计付费字符；免费/降级记 free_chars。
	var paidChars, freeChars, degraded int
	for i, res := range results {
		switch {
		case res.Status != "ok":
			// 失败段：不退配额（上游钱已花），站点字符不累计。
		case res.line == lineSite:
			paidChars += utf8.RuneCountInString(req.Segments[i])
		case res.line == lineFree:
			freeChars += utf8.RuneCountInString(req.Segments[i])
			degraded++
		case res.line == lineCustom:
			// 访客自己的端点：站点不付费，不计入任何字符口径。
		}
	}
	degradedReason := ""
	if degraded > 0 {
		degradedReason = "model_failed"
		if skipPaid {
			degradedReason = "site_paid_cap"
		}
	}
	// 站点请求计数：只要有非空段就记一次（site_usage_daily.translate_req）。
	if segments > 0 {
		if err := t.usage.AddSiteUsage(paidChars, freeChars, 1, 0, degraded); err != nil {
			slog.Warn("写入站点用量失败", "err", err)
		}
	}

	resp := &TranslateResponse{Segments: results}
	if degraded > 0 {
		resp.Degraded = true
		resp.DegradedReason = degradedReason
	}
	resp.Model = dominantModel(results)

	// Step 5: 再使用标记（FR-1202）：收藏词出现在本次翻译的源文里 → 用过 +1。
	t.markReused(visitorID, req.Segments)

	return resp, nil
}

// markReused 对本次翻译的源文做分词与词形还原，命中收藏词则记一次使用。
// 每词每次请求只 +1；任何失败静默跳过（绝不打断写作，FR-1202）。
func (t *Translator) markReused(visitorID string, segments []string) {
	if t.dict == nil || t.words == nil || visitorID == "" {
		return
	}
	collected, err := t.words.WordsOfVisitor(visitorID)
	if err != nil || len(collected) == 0 {
		return
	}
	seen := make(map[string]bool, len(collected))
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		for _, tok := range tokenizeWords(seg) {
			seen[t.dict.Lemma(tok)] = true
		}
	}
	hits := make([]string, 0, 4)
	for _, c := range collected {
		if seen[c] {
			hits = append(hits, c)
		}
	}
	if len(hits) == 0 {
		return
	}
	if err := t.words.MarkUsed(visitorID, hits, time.Now()); err != nil {
		slog.Warn("再使用标记失败", "err", err)
	}
}

// tokenizeWords 把文本切成小写英文词（字母/连字符/撇号）。
func tokenizeWords(text string) []string {
	out := make([]string, 0, 32)
	start := -1
	for i, r := range text {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '\'' || r == '-'
		if ok && start < 0 {
			start = i
		} else if !ok && start >= 0 {
			out = append(out, strings.ToLower(text[start:i]))
			start = -1
		}
	}
	if start >= 0 {
		out = append(out, strings.ToLower(text[start:]))
	}
	return out
}

// translateSegment 翻译单段：先查缓存（FR-1101），未命中走降级链。
func (t *Translator) translateSegment(ctx context.Context, source string, req *TranslateRequest, skipPaid bool) SegmentResult {
	model := req.Model
	if model == "" {
		model = t.pool.DefaultModel()
	}

	// 缓存键 = hash(模型 + 语言对 + 风格 + 术语指纹 + 段落文本)（FR-1101）。
	if t.cfg.CacheEnabled && t.pool.Cache() != nil {
		key := gateway.CacheFingerprint(model, req.From, req.To, req.Style, req.Glossary, source)
		if text, ok := t.pool.Cache().Get(key); ok {
			return SegmentResult{Status: "ok", Text: text, Model: model, line: lineSite}
		}
	}

	res, err := t.pool.TranslateOne(ctx, gateway.OneRequest{
		Text: source, From: req.From, To: req.To,
		Style: req.Style, StyleHint: StyleHint(req.Style),
		Glossary: req.Glossary, ToLabel: LanguageLabel(req.To),
		Model: req.Model, Custom: req.Custom, SkipPaid: skipPaid,
	})
	if err != nil {
		e := errs.As(err)
		return SegmentResult{Status: "failed", Code: e.Code, Message: e.Message}
	}
	if t.cfg.CacheEnabled && t.pool.Cache() != nil && res.Line == gateway.LineSite {
		key := gateway.CacheFingerprint(model, req.From, req.To, req.Style, req.Glossary, source)
		t.pool.Cache().Put(key, res.Text)
	}

	out := SegmentResult{
		Status: "ok", Text: res.Text, Model: res.Model,
		Degraded: res.Degraded, CustomFailed: res.CustomFailed, MS: res.MS,
	}
	switch res.Line {
	case gateway.LineSite:
		out.line = lineSite
	case gateway.LineCustom:
		out.line = lineCustom
	default:
		out.line = lineFree
	}
	return out
}

// siteCapReached 判定站点付费字符是否触顶（FR-603）。
// 口径用"已消耗 + 本次字符数 > 上限"：一次超长请求不允许把上限顶穿。
func (t *Translator) siteCapReached(chars int) bool {
	capChars := t.cfg.SiteDailyPaidChars
	if capChars <= 0 {
		return false
	}
	used, err := t.usage.PaidCharsToday()
	if err != nil {
		// 读不到按未触顶处理：这是成本护栏不是安全护栏，且配额那步已确认库可用。
		slog.Warn("读取站点用量失败，本次不降级", "err", err)
		return false
	}
	return used+int64(chars) > int64(capChars)
}

// billable 统计真正要翻译的段数与字符数（空白段已被归一为空串）。
func billable(texts []string) (segments, chars int) {
	for _, s := range texts {
		if s == "" {
			continue
		}
		segments++
		chars += utf8.RuneCountInString(s)
	}
	return segments, chars
}

// secondsUntil 返回距 t 的整秒数（至少 1），用于 Retry-After（FR-605）。
func secondsUntil(t time.Time) int {
	n := int(time.Until(t).Seconds())
	if n < 1 {
		n = 1
	}
	return n
}

// dominantModel 返回成功段落里出现最多的模型名。
func dominantModel(results []SegmentResult) string {
	counts := map[string]int{}
	for _, r := range results {
		if r.Status == "ok" && r.Model != "" {
			counts[r.Model]++
		}
	}
	best, n := "", 0
	for k, c := range counts {
		if c > n {
			best, n = k, c
		}
	}
	return best
}
