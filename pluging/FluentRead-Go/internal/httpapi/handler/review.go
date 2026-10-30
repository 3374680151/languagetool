// review.go 实现错误本与复习队列（P5.5，FR-1203~1205，§7.4）。
//
// @file internal/httpapi/handler/review.go
// 文件职责：
//   - GET/POST/DELETE /api/mistakes：错误本（仅"接受"纠错建议时写入片段、
//     类别筛选、单条删除、确认清空）。
//   - GET /api/review：到期队列——材料来自你自己的写作（挖空自己的句子 FR-1204）。
//   - POST /api/review/score：两键评分后的间隔调度（10min/1/3/7/14/30 天）。
//   - GET /api/assets/export · POST /api/assets/clear：学习资产导出与清空（FR-1205）。
//
// 主要内容：Mistakes 系列、Review、ReviewScore、AssetsExport、AssetsClear。
//
// 模块边界：存取在 store（WordRepo/MistakeRepo）；词形还原用 internal/dict（同一实现）。
package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/dict"
	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/store"
)

// mistakeBody 是错误本写入体（接受纠错建议时由前端调用）。
type mistakeBody struct {
	SourceSnippet    string `json:"sourceSnippet"`
	CorrectedSnippet string `json:"correctedSnippet"`
	Category         string `json:"category"`
	Reason           string `json:"reason"`
}

// MistakesList GET /api/mistakes?category=。
func (a *App) MistakesList(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	list, err := a.MistakeRepo.List(visitor, c.Query("category"))
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	if list == nil {
		list = []store.Mistake{}
	}
	OK(c, list)
}

// MistakesCreate POST /api/mistakes（接受建议时写入）。
func (a *App) MistakesCreate(c *gin.Context) {
	var body mistakeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}
	if body.SourceSnippet == "" || body.CorrectedSnippet == "" {
		Fail(c, errs.InvalidRequest("缺少原片段或改法"))
		return
	}
	visitor := middleware.VisitorID(c)
	m, err := a.MistakeRepo.Add(visitor, body.SourceSnippet, body.CorrectedSnippet, body.Category, body.Reason)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, m)
}

// MistakesDelete DELETE /api/mistakes?id=N 或 ?all=1（确认后清空）。
func (a *App) MistakesDelete(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	if c.Query("all") == "1" {
		if err := a.MistakeRepo.Clear(visitor); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
		OK(c, gin.H{"cleared": true})
		return
	}
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		Fail(c, errs.InvalidRequest("缺少 id"))
		return
	}
	if err := a.MistakeRepo.Delete(visitor, id); err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, gin.H{"deleted": true})
}

// reviewItem 是复习卡（FR-1204：挖空你自己的句子）。
type reviewItem struct {
	Kind     string `json:"kind"` // word | mistake
	ID       int64  `json:"id"`
	Prompt   string `json:"prompt"`
	Cloze    string `json:"cloze"`
	Answer   string `json:"answer"`
	Hint     string `json:"hint,omitempty"`
	Category string `json:"category,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Review GET /api/review：词表到期（原句挖空）+ 错误本到期（正确改法挖空）。
func (a *App) Review(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	now := time.Now()
	out := make([]reviewItem, 0, 16)

	// ① 收藏词：挖空你自己写的句子。
	if dueWords, err := a.WordRepo.Due(visitor, now); err == nil {
		for _, w := range dueWords {
			cloze := w.ContextSentence
			if cloze != "" {
				cloze = blankWordIn(cloze, w.Word, a)
			}
			hint := ""
			if a.Dictionary != nil {
				if e, ok := a.Dictionary.Lookup(w.Word); ok {
					// 释义首行做提示（不直接给答案）。
					senses := dict.Senses(e.Translation)
					if len(senses) > 0 {
						hint = senses[0].Pos + " " + senses[0].Def
					}
				}
			}
			// 没有原句（搜索框收藏）：退化为"词 → 释义"卡（FR-1204 兼容）。
			if cloze == "" {
				out = append(out, reviewItem{
					Kind: "word", ID: w.ID, Prompt: w.Word, Cloze: w.Word, Answer: w.Word, Hint: hint,
				})
				continue
			}
			out = append(out, reviewItem{
				Kind: "word", ID: w.ID, Prompt: w.Word, Cloze: cloze, Answer: w.Word, Hint: hint,
			})
		}
	}

	// ② 错误本：挖空正确改法。
	if dueMistakes, err := a.MistakeRepo.Due(visitor, now); err == nil {
		for _, m := range dueMistakes {
			out = append(out, reviewItem{
				Kind: "mistake", ID: m.ID,
				Prompt:   "把错误改对",
				Cloze:    m.SourceSnippet,
				Answer:   m.CorrectedSnippet,
				Category: m.Category,
				Reason:   m.Reason,
			})
		}
	}
	OK(c, gin.H{"items": out, "count": len(out)})
}

// blankWordIn 把句子里与目标词（或其词形变体）匹配的 token 挖成 ____。
func blankWordIn(sentence, word string, a *App) string {
	tokens := strings.Fields(sentence)
	for i, tok := range tokens {
		bare := strings.Trim(strings.ToLower(tok), ",.!?;:\"'`()[]{}")
		if bare == word {
			tokens[i] = strings.Replace(tok, tok, "____", 1)
			return strings.Join(tokens, " ")
		}
		if a.Dictionary != nil && a.Dictionary.Lemma(bare) == word {
			tokens[i] = strings.Replace(tok, bare, "____", 1)
			return strings.Join(tokens, " ")
		}
	}
	return sentence
}

// scoreBody 是评分请求体。
type scoreBody struct {
	Kind       string `json:"kind"`
	ID         int64  `json:"id"`
	Remembered bool   `json:"remembered"`
}

// ReviewScore POST /api/review/score：记住进一档、没记住回退一档（FR-1204）。
func (a *App) ReviewScore(c *gin.Context) {
	var body scoreBody
	if err := c.ShouldBindJSON(&body); err != nil || body.ID == 0 {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON 或缺少 id"))
		return
	}
	visitor := middleware.VisitorID(c)
	now := time.Now()
	switch body.Kind {
	case "word":
		w, err := a.WordRepo.Schedule(visitor, body.ID, body.Remembered, now)
		if err != nil {
			Fail(c, errs.Internal(err))
			return
		}
		OK(c, gin.H{"nextDueAt": w.NextDueAt})
	case "mistake":
		m, err := a.MistakeRepo.Schedule(visitor, body.ID, body.Remembered, now)
		if err != nil {
			Fail(c, errs.Internal(err))
			return
		}
		OK(c, gin.H{"nextDueAt": m.NextDueAt})
	default:
		Fail(c, errs.InvalidRequest("kind 只支持 word/mistake"))
	}
}


// AssetsExport GET /api/assets/export：词表 + 错误本 JSON（FR-1205）。
// 只含用户显式动作产生的学习资产片段，不包含任何正文/译文（NFR-6）。
func (a *App) AssetsExport(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	words, err := a.WordRepo.List(visitor, "recent")
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	mistakes, err := a.MistakeRepo.List(visitor, "")
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, gin.H{
		"words":      words,
		"mistakes":   mistakes,
		"exportedAt": time.Now().Format(time.RFC3339),
	})
}

// AssetsClear POST /api/assets/clear：确认后清空词表与错误本（FR-1205，二次确认由前端完成）。
func (a *App) AssetsClear(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	if err := a.WordRepo.Clear(visitor); err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	if err := a.MistakeRepo.Clear(visitor); err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, gin.H{"cleared": true})
}
