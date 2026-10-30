// Package dict 是 ECDICT 精简词典的查询层（DR-6：约 3000 高频词随二进制 embed）。
//
// @file internal/dict/dict.go
// 文件职责：加载 embed 的 ecdict-slim.json；提供 Lookup（精确查询，目标 <100ms）、
// Lemma（词形还原，FR-1202 再使用计数与查词卡共用同一实现，AD-7）、
// Senses 解析（translation 行 → 词性+释义）。
//
// 主要内容：Entry、Dictionary、Load、Lookup、Lemma、buildLemmaIndex。
//
// 模块边界：只读 embed 数据，无网络无数据库；模型兜底在 handler 层。
package dict

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/ecdict-slim.json
var dataFS embed.FS

// Entry 是词典里的一条记录（来自 ECDICT 字段裁剪）。
type Entry struct {
	Word        string `json:"word"`
	Phonetic    string `json:"phonetic"`
	Translation string `json:"translation"`
	Pos         string `json:"pos"`
	Tag         string `json:"tag"`
	Exchange    string `json:"exchange"`
}

// Sense 是一条"词性 + 释义"。
type Sense struct {
	Pos string `json:"pos"`
	Def string `json:"def"`
}

// Dictionary 是加载后的词典索引。
type Dictionary struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	// lemmaOf 是"曲折形式 → 原形"的反查表（由 exchange 生成）。
	lemmaOf map[string]string
}

// Load 读取 embed 数据并建索引。进程内单例（sync.OnceValue 由调用方持有）。
func Load() (*Dictionary, error) {
	raw, err := dataFS.ReadFile("data/ecdict-slim.json")
	if err != nil {
		return nil, err
	}
	var entries []*Entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	d := &Dictionary{
		entries: make(map[string]*Entry, len(entries)),
		lemmaOf: make(map[string]string, len(entries)*4),
	}
	for _, e := range entries {
		w := strings.ToLower(e.Word)
		d.entries[w] = e
		d.buildLemmaIndex(w, e.Exchange)
	}
	return d, nil
}

// buildLemmaIndex 解析 exchange 字段（ECDICT 约定：p 过去式/d 过去分词/i 现在分词/
// 3 三单/r 比较级/t 最高级/s 复数/0 原形），把每个曲折形式映射回原形。
func (d *Dictionary) buildLemmaIndex(word, exchange string) {
	if exchange == "" {
		return
	}
	for _, part := range strings.Split(exchange, "/") {
		code, form, ok := strings.Cut(part, ":")
		if !ok || form == "" {
			continue
		}
		form = strings.ToLower(strings.TrimSpace(form))
		switch code {
		case "p", "d", "i", "3", "r", "t", "s":
			// 曲折形式 → 原形。
			if _, exists := d.lemmaOf[form]; !exists {
				d.lemmaOf[form] = word
			}
		case "0":
			// 原形标注：把变体（如复数原形）也归到这里。
			if _, exists := d.lemmaOf[form]; !exists {
				d.lemmaOf[form] = word
			}
		}
	}
}

// Lookup 精确查询一个词（小写归一）。未命中返回 false。
func (d *Dictionary) Lookup(word string) (*Entry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	e, ok := d.entries[strings.ToLower(strings.TrimSpace(word))]
	return e, ok
}

// Lemma 返回词的原形：先查 exchange 反查表，再用规则后缀还原。
// 查词卡与 FR-1202 再使用计数共用本实现（AD-7）。
func (d *Dictionary) Lemma(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	d.mu.RLock()
	if lemma, ok := d.lemmaOf[w]; ok {
		d.mu.RUnlock()
		return lemma
	}
	d.mu.RUnlock()

	// 规则还原（只处理规则变化，不规则变化靠 exchange 反查）。
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "es") && len(w) > 3 &&
		(strings.HasSuffix(w, "ses") || strings.HasSuffix(w, "xes") || strings.HasSuffix(w, "zes") ||
			strings.HasSuffix(w, "ches") || strings.HasSuffix(w, "shes")):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "ed") && len(w) > 3:
		// doubled → double；stopped → stop（双写）。
		base := w[:len(w)-2]
		if d.hasWord(base + "e") {
			return base + "e"
		}
		if len(base) >= 3 && base[len(base)-1] == base[len(base)-2] && d.hasWord(base[:len(base)-1]) {
			return base[:len(base)-1]
		}
		return base
	case strings.HasSuffix(w, "ing") && len(w) > 4:
		base := w[:len(w)-3]
		if d.hasWord(base + "e") {
			return base + "e"
		}
		if len(base) >= 3 && base[len(base)-1] == base[len(base)-2] && d.hasWord(base[:len(base)-1]) {
			return base[:len(base)-1]
		}
		return base
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") && len(w) > 2:
		return w[:len(w)-1]
	}
	return w
}

// hasWord 判断词是否在词典里（规则还原的校验）。
func (d *Dictionary) hasWord(w string) bool {
	_, ok := d.entries[w]
	return ok
}

// Senses 把 translation 字段按行解析成 [{pos, def}]。
// ECDICT 的 translation 形如 "v. 回忆起；回想起\nn. 记忆力；召回"。
func Senses(translation string) []Sense {
	out := make([]Sense, 0, 4)
	for _, line := range strings.Split(translation, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 网络释义等非标准行跳过。
		if strings.HasPrefix(line, "[") {
			continue
		}
		pos, def, ok := cutPos(line)
		if !ok {
			// 没有词性前缀的续行：并入上一条。
			if len(out) > 0 {
				out[len(out)-1].Def += "；" + line
			}
			continue
		}
		out = append(out, Sense{Pos: pos, Def: def})
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// cutPos 从一行里切出词性缩写与释义，如 "v. 回忆起" → ("v.", "回忆起")。
func cutPos(line string) (string, string, bool) {
	for _, prefix := range []string{"art.", "abbr.", "adj.", "adv.", "conj.", "int.", "n.", "pron.", "prep.", "v.", "vi.", "vt.", "num.", "aux."} {
		if strings.HasPrefix(line, prefix) {
			return prefix, strings.TrimSpace(line[len(prefix):]), true
		}
	}
	return "", "", false
}

// Tags 把 tag 字段拆成标签数组（考纲标签，FR-701 卡片展示）。
func Tags(tag string) []string {
	if strings.TrimSpace(tag) == "" {
		return []string{}
	}
	parts := strings.Fields(tag)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p)
	}
	return out
}
