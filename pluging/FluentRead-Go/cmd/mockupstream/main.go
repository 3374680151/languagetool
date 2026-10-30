// mockupstream 是本地假上游：把 OpenAI 兼容路径（翻译、纠错、词典兜底、模型清单）
// 全部变成确定性、可注入故障的本地实现，外加 MyMemory 兼容的 /get。
//
// @file cmd/mockupstream/main.go
// 文件职责：没有真实模型密钥时，验证链路必须靠它（§7.6 测试基座）。支持注入：
//
//	fail=500|timeout|slow|malformed|empty|overlong，ms=延迟毫秒
//
// 另提供 /__stats（调用计数：验证"超限时不打上游"）、/__reset（清零）、
// /__control（粘性故障开关，UI 测试用）。只用于测试，不进生产镜像。
//
// 主要内容：main、故障注入、确定性假译文/假纠错/假词典。
//
// 模块边界：独立进程，零内部依赖；用源文本哈希选模板，保证同输入恒定输出。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// counters 记录各条线路被调用的次数。
// 验证"配额/限流超限时不打上游"必须靠它：只看响应码无法证明上游没被调用。
type counters struct {
	chat  atomic.Int64
	model atomic.Int64
	free  atomic.Int64
}

var (
	counts counters
	// fault 是粘性故障开关，由 /__control 设置。
	faultMu sync.RWMutex
	fault   string
	faultMS int
	// keyStatus 按 Authorization 注入按密钥的失败（验证 FR-1102 多 Key 轮换）。
	keyStatus sync.Map // key -> int
)

// zhTemplates 是中文假译文模板池（规格默认语言对 英→中，FR-101）。
// 必须含 CJK：结果校验（§7.6）会拒绝"目标中文却无 CJK"的译文。
var zhTemplates = []string{
	"检索增强生成把模型锚定在检索到的文档上，让回答更有依据。",
	"回答之前，系统先从知识库里召回最相关的段落。",
	"段落边界被保留下来，每一段都可以独立翻译和修改。",
	"术语在站点级别锁定，长文里的措辞因此保持一致。",
	"失败的段落会在原地标出，可以单独重试，不必重跑整篇。",
	"编辑器不打扰写作：作者还在打字时不会自动翻译。",
	"每位访客都有一个匿名身份，收藏的词跟着浏览器走。",
	"配额按翻译的段落计费，空行不消耗当日额度。",
	"付费额度用完时，服务会自动切到免费方式，而不是直接失败。",
	"错误提示要告诉读者下一步做什么，而不是只说出了问题。",
	"双栏逐行对齐，读者一眼就能找到每段译文对应的原文。",
	"导出的 Markdown 把原文和译文放在一起，方便以后复查。",
}

// jaTemplates 是日文假译文模板池。
var jaTemplates = []string{
	"検索拡張生成は、検索した文書に基づいて回答の質を高めます。",
	"回答の前に、システムは知識ベースから最も関連の高い段落を取得します。",
}

// englishTemplates 是拉丁语系假译文模板池（英/法/德/西/俄目标共用）。
var englishTemplates = []string{
	"Retrieval-augmented generation improves answer quality by grounding the model in retrieved documents.",
	"Before answering, the system first recalls the most relevant passages from the knowledge base.",
	"Paragraph boundaries are preserved so that each segment can be translated and revised independently.",
	"Terminology is locked at the site level, which keeps the wording consistent across a long article.",
	"A failed segment is marked in place and can be retried without rerunning the whole document.",
	"The editor stays out of the way: no automatic translation while the author is still typing.",
	"Every visitor gets an anonymous identity so that saved words follow the browser, not an account.",
	"Quotas are charged per translated segment, so blank lines never consume the daily budget.",
	"When the paid budget runs out, the service switches to a free fallback instead of failing.",
	"Errors must tell the reader what to do next, not merely that something went wrong.",
	"The workbench keeps two columns aligned so a reader can match each line at a glance.",
	"The exported Markdown keeps the source and the translation side by side for later review.",
}

// freeTemplatesZH 是免费线路的中文假译文，故意与付费线路不同：
// "降级后质量变化"在测试里可观测，而不是只靠一个布尔值。
var freeTemplatesZH = []string{
	"[免费] 这是免费方式的机翻结果。",
	"[免费] 免费方式给出的句子更短、更朴素。",
	"[免费] 免费方式的结果能读，但不够发表。",
}

// freeTemplatesEN 是免费线路的拉丁语系假译文。
var freeTemplatesEN = []string{
	"[free] This is a machine translation from the free line.",
	"[free] The free line returns a shorter, plainer sentence.",
	"[free] Free line output, good enough to read but not to publish.",
}

// pickPoolByTarget 按系统提示词里的目标语言选模板池。
// 假译文必须与目标语言一致，否则 §7.6 的语种校验会正确地拒绝它。
func pickPoolByTarget(system string) []string {
	switch {
	case strings.Contains(system, "简体中文") || strings.Contains(system, "繁體中文") || strings.Contains(system, "中文"):
		return zhTemplates
	case strings.Contains(system, "日本語"):
		return jaTemplates
	default:
		return englishTemplates
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:18081", "监听地址")
	modelsFile := flag.String("models", "deploy/models.json", "模型清单文件")
	flag.Parse()

	models := loadModels(*modelsFile)
	mux := http.NewServeMux()

	// OpenAI 兼容模型清单：巡检对账（FR-1103）的对账基准。
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		counts.model.Add(1)
		if !applyFault2(w, r) {
			return
		}
		data := make([]map[string]any, 0, len(models))
		for _, id := range models {
			data = append(data, map[string]any{"id": id, "object": "model"})
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
	})

	// OpenAI 兼容补全：确定性假译文 / 假纠错 / 假词典解释。
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		counts.chat.Add(1)
		mode, ms := resolveFault(r)
		if mode == "" {
			// 按密钥注入的失败优先于粘性故障（模拟"第 1 把 key 401"）。
			if st := keyStatusFor(r.Header.Get("authorization")); st != 0 {
				writeJSON(w, st, map[string]any{"error": map[string]any{"message": "injected key failure"}})
				return
			}
		}
		if !applyFault(w, r, mode, ms) {
			return
		}

		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad json"})
			return
		}

		system, user := splitMessages(body.Messages)
		// 内容标记的段级故障：让 e2e 能在同一次请求里制造"1 段失败、2 段成功"。
		// 标记写在段落文本里：[[fail]] 500 / [[timeout]] 超时 / [[empty]] 空译文 / [[echo]] 回显。
		if marker := contentFault(user); marker != "" {
			if !applyFault(w, r, marker, 0) {
				return
			}
		}
		content := fakeContent(system, user, body.Model)
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      "chatcmpl-fake",
			"object":  "chat.completion",
			"model":   body.Model,
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": content}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": len(user) / 4, "completion_tokens": len(content) / 4},
		})
	})

	// MyMemory 兼容端点：免费线路的兜底实现，让降级路径也能离线验证。
	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		counts.free.Add(1)
		if !applyFault2(w, r) {
			return
		}
		q := r.URL.Query().Get("q")
		// 免费线路也按目标语言给假译文（langpair=en|zh-CN → 中文）。
		pool := freeTemplatesEN
		if to, _, ok := strings.Cut(r.URL.Query().Get("langpair"), "|"); ok {
			if strings.HasPrefix(to, "zh") {
				pool = freeTemplatesZH
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"responseData":    map[string]any{"translatedText": pick(pool, q)},
			"responseStatus":  200,
			"responseDetails": "",
		})
	})

	// 调用计数与故障开关状态。
	mux.HandleFunc("/__stats", func(w http.ResponseWriter, r *http.Request) {
		faultMu.RLock()
		mode, ms := fault, faultMS
		faultMu.RUnlock()
		keyFails := map[string]int{}
		keyStatus.Range(func(k, v any) bool {
			keyFails[fmt.Sprintf("%v", k)] = v.(int)
			return true
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"chat": counts.chat.Load(), "models": counts.model.Load(), "free": counts.free.Load(),
			"fault": mode, "faultMs": ms, "keyStatus": keyFails,
		})
	})

	mux.HandleFunc("/__reset", func(w http.ResponseWriter, r *http.Request) {
		counts.chat.Store(0)
		counts.model.Store(0)
		counts.free.Store(0)
		faultMu.Lock()
		fault, faultMS = "", 0
		faultMu.Unlock()
		keyStatus = sync.Map{}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.HandleFunc("/__control", func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("fail")
		ms := 0
		if v := r.URL.Query().Get("ms"); v != "" {
			_, _ = fmt.Sscanf(v, "%d", &ms)
		}
		if key := r.URL.Query().Get("key"); key != "" {
			st := 0
			if v := r.URL.Query().Get("status"); v != "" {
				_, _ = fmt.Sscanf(v, "%d", &st)
			}
			if st == 0 {
				keyStatus.Delete("Bearer " + key)
			} else {
				keyStatus.Store("Bearer "+key, st)
			}
		}
		faultMu.Lock()
		fault, faultMS = mode, ms
		faultMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fault": mode, "ms": ms})
	})

	log.Printf("mockupstream listening on %s, models=%v", *addr, models)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// contentFault 从用户文本里解析段级故障标记；无标记返回空串。
func contentFault(user string) string {
	switch {
	case strings.Contains(user, "[[fail]]"):
		return "500"
	case strings.Contains(user, "[[timeout]]"):
		return "timeout"
	case strings.Contains(user, "[[empty]]"):
		return "empty"
	case strings.Contains(user, "[[echo]]"):
		return "echo"
	}
	return ""
}

// resolveFault 决定本次请求的故障模式：query 优先，其次粘性开关。
func resolveFault(r *http.Request) (string, int) {
	if v := r.URL.Query().Get("fail"); v != "" {
		ms := 0
		if s := r.URL.Query().Get("ms"); s != "" {
			_, _ = fmt.Sscanf(s, "%d", &ms)
		}
		return v, ms
	}
	faultMu.RLock()
	defer faultMu.RUnlock()
	return fault, faultMS
}

// keyStatusFor 取该密钥注入的状态码。
func keyStatusFor(auth string) int {
	if v, ok := keyStatus.Load(auth); ok {
		return v.(int)
	}
	return 0
}

// applyFault2 是"/v1/models 与 /get"这类无请求体端点的故障入口：
// 解析故障模式后转给 applyFault。
func applyFault2(w http.ResponseWriter, r *http.Request) bool {
	mode, ms := resolveFault(r)
	return applyFault(w, r, mode, ms)
}

// applyFault 执行故障注入。返回 false 表示已经写过响应，调用方应直接结束。
func applyFault(w http.ResponseWriter, r *http.Request, mode string, ms int) bool {
	switch mode {
	case "":
		return true
	case "timeout":
		// 故意睡过客户端超时，模拟"端点指向黑洞"。睡 90 秒足够覆盖测试里的短超时。
		time.Sleep(90 * time.Second)
		writeJSON(w, http.StatusOK, map[string]any{"choices": []any{}})
		return false
	case "slow":
		if ms <= 0 {
			ms = 3000
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return true
	case "500":
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "upstream exploded"}})
		return false
	case "malformed":
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>not json at all</html>"))
		return false
	case "empty":
		// 200 但内容为空：结果校验必须拒绝（§7.6 拒绝空译文）。
		writeJSON(w, http.StatusOK, map[string]any{
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": ""}}},
		})
		return false
	case "echo":
		// 200 但回显原文：结果校验必须拒绝（§7.6 拒绝原文回显）。
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		last := ""
		for _, m := range body.Messages {
			if m.Role == "user" {
				last = m.Content
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": last}}},
		})
		return false
	case "overlong":
		// 超长响应：单段超限的极端形态，验证长度护栏与前端渲染不被打爆。
		writeJSON(w, http.StatusOK, map[string]any{
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": strings.Repeat("长", 12000)}}},
		})
		return false
	default:
		return true
	}
}

// splitMessages 取出 system 与最后一条 user 消息。
func splitMessages(msgs []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) (string, string) {
	var system, user string
	for _, m := range msgs {
		switch m.Role {
		case "system":
			system = m.Content
		case "user":
			user = m.Content
		}
	}
	return system, user
}

// fakeContent 按请求类型返回假译文 / 假纠错 / 假词典。
// 判据是系统提示词里的关键字——接入新能力后一般不需要改这里。
func fakeContent(system, user, model string) string {
	low := strings.ToLower(system)
	switch {
	case strings.Contains(system, "纠错") || strings.Contains(system, "校对") || strings.Contains(low, "proofread"):
		return proofreadFake(user)
	case strings.Contains(system, "词典") || strings.Contains(low, "dictionary") || strings.Contains(low, "英汉词典"):
		return dictFake(user)
	}
	out := pick(pickPoolByTarget(system), user)
	if model != "" {
		// 让"换模型"在输出上可观测，否则切换下拉看不出任何差别。
		out = "[" + model + "] " + out
	}
	// 末尾附上原文与系统提示词的短指纹：模板池有限，不同段落容易撞同一句；
	// 指纹还能证明"术语/风格确实被送到了上游"——改术语，指纹就变。
	return out + " §" + fingerprint(user) + "-" + fingerprint(system)
}

// fingerprint 返回原文的 6 位十六进制摘要。
func fingerprint(src string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(src))
	return fmt.Sprintf("%06x", h.Sum32()&0xffffff)
}

// proofreadFake 返回可被 strings.Contains 命中的纠错建议。
// original 必须是原文子串，否则服务端会按契约丢弃——这里故意全部命中。
func proofreadFake(text string) string {
	type issue struct {
		Original   string `json:"original"`
		Suggestion string `json:"suggestion"`
		Category   string `json:"category"`
		Reason     string `json:"reason"`
	}
	out := make([]issue, 0, 4)
	if strings.Contains(text, "recieves") {
		out = append(out, issue{"recieves", "receives", "spelling", "拼写规则“i before e except after c”：c 后面接 ei 时要写成 receive，ie 顺序写反就是 recieves。"})
	}
	if strings.Contains(text, "a RAG") {
		out = append(out, issue{"a RAG", "an RAG", "grammar", "RAG 按字母读音 /ɑːr-eɪ-dʒiː/，以元音开头，冠词用 an。"})
	}
	if strings.Contains(text, "it recall") {
		out = append(out, issue{"it recall", "and recalls", "grammar", "主语 it 是第三人称单数，动词要加 s；两个分句需要连词连接，逗号不能直接粘连两个完整句。"})
	}
	if strings.Contains(text, "make a conclusion") {
		out = append(out, issue{"make a conclusion", "draw a conclusion", "word_choice", "conclusion 的固定搭配是 draw/reach a conclusion，make 不与 conclusion 搭配。"})
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}

// dictFake 返回词典兜底结构的假解释（§8.3 在线态）。
func dictFake(word string) string {
	w := strings.TrimSpace(word)
	type def struct {
		Pos        string `json:"pos"`
		Definition string `json:"definition"`
		Example    string `json:"example"`
		IsTypo     bool   `json:"isTypo"`
		CorrectTo  string `json:"correctTo,omitempty"`
	}
	d := def{Pos: "v. n.", Definition: "回忆起；召回；记忆力（模拟在线解释）", Example: "She tried to recall the exact wording of the note."}
	// 拼写错误演示：gerrymander 类生僻词与常见错拼都走"可能是 XXX"提示。
	if strings.HasSuffix(w, "ing") && len(w) > 5 {
		d.CorrectTo = strings.TrimSuffix(w, "ing")
	}
	raw, _ := json.Marshal(d)
	return string(raw)
}

// pick 用源文本哈希选模板，保证同一输入恒定输出。
func pick(pool []string, src string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(src))
	return pool[int(h.Sum32())%len(pool)]
}

func loadModels(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Printf("读取模型清单失败（返回空清单）: %v", err)
		return []string{"fake-model"}
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		log.Printf("模型清单格式错误（返回空清单）: %v", err)
		return []string{"fake-model"}
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it.ID != "" {
			out = append(out, it.ID)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
