// dict.go 实现 GET /api/dict/:word（P4，FR-701 + §八 详细设计）。
//
// @file internal/httpapi/handler/dict.go
// 文件职责：查词链路——服务端内存缓存 → ECDICT embed（命中即返回，source=ecdict）
// → 模型兜底（source=model，单独限频默认 10 次/分钟/访客，结果写回缓存）；
// 词形还原（与 FR-1202 同一实现，AD-7）；已收藏状态按访客查 words 表。
// 查词不计翻译配额（FR-604 不受影响）。
//
// 主要内容：Dict、dictLookup、ecdictToEntry、modelDictFallback。
//
// 模块边界：ECDICT 查询在 internal/dict；模型协议在 gateway；words 状态在 store。
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/dict"
	"fluentread/internal/errs"
	"fluentread/internal/gateway"
	"fluentread/internal/httpapi/middleware"
)

// dictEntry 是查词卡响应（§八 8.5 响应结构）。
type dictEntry struct {
	Word      string        `json:"word"`
	Query     string        `json:"query"`
	Phonetic  string        `json:"phonetic,omitempty"`
	Pos       string        `json:"pos,omitempty"`
	Senses    []dictSense   `json:"senses"`
	Tags      []string      `json:"tags"`
	Source    string        `json:"source"`
	Example   string        `json:"example,omitempty"`
	TypoHint  *dictTypo     `json:"typoHint,omitempty"`
	Collected dictCollected `json:"collected"`
}

type dictSense struct {
	Pos string `json:"pos"`
	Def string `json:"def"`
}

type dictTypo struct {
	CorrectTo string `json:"correctTo"`
}

type dictCollected struct {
	Yes      bool  `json:"yes"`
	UseCount int64 `json:"useCount"`
}

// Dict 查词入口。允许 1-4 个词的短语（§八 8.1；≥5 词视为句子不查）。
func (a *App) Dict(c *gin.Context) {
	query := normalizeWordParam(c.Param("word"))
	if query == "" || len(query) > 60 {
		Fail(c, errs.InvalidRequest("要查询的词不合法"))
		return
	}
	if len(strings.Fields(query)) > 4 {
		Fail(c, errs.InvalidRequest("一次最多查 4 个词的短语"))
		return
	}

	// Step 1: 服务端内存缓存（§八 8.5 查询链；模型兜底结果也写回这里）。
	if a.DictCache != nil {
		if raw, ok := a.DictCache.Get("dict:" + query); ok {
			var e dictEntry
			if json.Unmarshal([]byte(raw), &e) == nil {
				e.Query = query
				e.Collected = a.collectedStatus(middleware.VisitorID(c), e.Word)
				OK(c, e)
				return
			}
		}
	}

	// Step 2: ECDICT 精简库（目标 <100ms，DR-6）。词形还原与 FR-1202 同一实现。
	if a.Dictionary != nil {
		if lemma := a.Dictionary.Lemma(query); lemma != "" {
			if e, ok := a.Dictionary.Lookup(lemma); ok {
				entry := ecdictToEntry(query, lemma, e)
				entry.Collected = a.collectedStatus(middleware.VisitorID(c), lemma)
				a.dictCachePut("dict:"+query, entry)
				OK(c, entry)
				return
			}
		}
	}

	// Step 3: 模型兜底。限频 10 次/分钟/访客（超限 429 + 卡内"稍后再试"，AD-9）。
	if ok, retry := a.DictLimiter.Allow("v:" + middleware.VisitorID(c)); !ok {
		Fail(c, errs.RateLimited(retry))
		return
	}

	entry := a.modelDictFallback(c.Request.Context(), query)
	entry.Query = query
	entry.Collected = a.collectedStatus(middleware.VisitorID(c), entry.Word)
	a.dictCachePut("dict:"+query, entry)
	OK(c, entry)
}

// normalizeWordParam 清洗查询词：小写、去首尾标点（§八 8.2 预处理）。
func normalizeWordParam(w string) string {
	w = strings.ToLower(strings.TrimSpace(w))
	w = strings.Trim(w, ",.!?;:\"'`()[]{}")
	return w
}

// mustDictEntry 取词典条目（调用前已确认命中）。
func (a *App) mustDictEntry(lemma string) *dict.Entry {
	e, _ := a.Dictionary.Lookup(lemma)
	return e
}// ecdictToEntry 把 ECDICT 条目转成查词卡响应。query 与 lemma 不同时前端显示还原提示。
func ecdictToEntry(query, lemma string, e *dict.Entry) *dictEntry {
	if e == nil {
		return nil
	}
	return &dictEntry{
		Word:     lemma,
		Query:    query,
		Phonetic: phoneticWrap(e.Phonetic),
		Senses:   dictSenses(e.Translation),
		Tags:     dict.Tags(e.Tag),
		Source:   "ecdict",
	}
}

// dictSenses 把 ECDICT translation 按行解析成 [{pos, def}]。
func dictSenses(translation string) []dictSense {
	out := make([]dictSense, 0, 4)
	for _, line := range strings.Split(translation, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue // 网络释义等非标准行跳过
		}
		pos, def, ok := cutDictPos(line)
		if !ok {
			// 没有词性前缀的续行并入上一条。
			if len(out) > 0 {
				out[len(out)-1].Def += "；" + line
			}
			continue
		}
		out = append(out, dictSense{Pos: pos, Def: def})
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// cutDictPos 从一行切出词性缩写与释义，如 "v. 回忆起" → ("v.", "回忆起")。
func cutDictPos(line string) (string, string, bool) {
	for _, prefix := range []string{"art.", "abbr.", "adj.", "adv.", "conj.", "int.", "n.", "pron.", "prep.", "v.", "vi.", "vt.", "num.", "aux."} {
		if strings.HasPrefix(line, prefix) {
			return prefix, strings.TrimSpace(line[len(prefix):]), true
		}
	}
	return "", "", false
}

// phoneticWrap 给音标加斜杠。
func phoneticWrap(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

// modelDictFallback 模型兜底：词性 + 简短中文释义 + 例句（§八 8.3 在线态）。
// 结果由调用方写回服务端缓存，相同词第二次不再打模型。
func (a *App) modelDictFallback(ctx context.Context, query string) *dictEntry {
	empty := &dictEntry{Word: query, Senses: []dictSense{}, Tags: []string{}, Source: "model"}
	model := a.Pool.DefaultModel()
	if model == "" {
		return empty
	}
	started := time.Now()
	resp, err := a.Pool.SiteClient().Chat(ctx, gateway.ChatRequest{
		Model:       model,
		Messages:    [][2]string{{"system", gateway.DictSystemPrompt()}, {"user", query}},
		Temperature: 0.2,
		MaxTokens:   300,
	})
	if err != nil {
		slog.Warn("词典兜底失败", "err", err)
		return empty
	}
	slog.Info("词典兜底", "ms", time.Since(started).Milliseconds())

	var parsed struct {
		Pos        string `json:"pos"`
		Definition string `json:"definition"`
		Example    string `json:"example"`
		IsTypo     bool   `json:"isTypo"`
		CorrectTo  string `json:"correctTo"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &parsed); err != nil {
		return empty
	}
	entry := &dictEntry{
		Word:    query,
		Pos:     parsed.Pos,
		Senses:  []dictSense{{Pos: parsed.Pos, Def: parsed.Definition}},
		Tags:    []string{},
		Source:  "model",
		Example: parsed.Example,
	}
	// AD-5：拼错的词给"可能是 XXX 的拼写错误"提示。
	if parsed.IsTypo && parsed.CorrectTo != "" {
		lemma := parsed.CorrectTo
		if a.Dictionary != nil {
			lemma = a.Dictionary.Lemma(parsed.CorrectTo)
		}
		entry.TypoHint = &dictTypo{CorrectTo: lemma}
	}
	return entry
}

// collectedStatus 已收藏状态 + 使用次数（§八 8.3 已收藏态）。
func (a *App) collectedStatus(visitor, word string) dictCollected {
	if visitor == "" {
		return dictCollected{}
	}
	if w, ok := a.WordRepo.GetCollected(visitor, word); ok {
		return dictCollected{Yes: true, UseCount: w.UseCount}
	}
	return dictCollected{}
}

// dictCachePut 把条目序列化进服务端内存缓存（FR-1101 同一 LRU 实现）。
func (a *App) dictCachePut(key string, e *dictEntry) {
	if a.DictCache == nil || e == nil {
		return
	}
	if raw, err := json.Marshal(e); err == nil {
		a.DictCache.Put(key, string(raw))
	}
}
