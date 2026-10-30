// prompt.go 集中管理提示词模板（§7.7）：回译 / 纠错 / 词典兜底。
//
// @file internal/gateway/prompt.go
// 文件职责：把语言对、风格、术语表编译成系统提示词；术语注入保证"术语真的进提示词"
//（FR-502）；全部模板可被环境变量覆盖（FR_TRANSLATE_PROMPT / FR_PROOFREAD_PROMPT，
// 站长调优不改代码）。纠错提示词要求模型输出 JSON 数组且禁止输出偏移量
//（FR-204 由服务端子串校验兜底）。
//
// 主要内容：TranslateSystemPrompt、ProofreadSystemPrompt、DictSystemPrompt、
// CacheFingerprint。
//
// 模块边界：只做字符串拼装；不调用模型、不认识数据库。语言标签表在 service 包，
// 通过参数传入，避免反向依赖。
package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// PromptOptions 是编译提示词所需的输入。
type PromptOptions struct {
	From     string
	To       string
	ToLabel  string // 目标语言中文名（FR-301）
	Style    string
	StyleHint string
	Glossary [][2]string // [原文术语, 指定译法]
}

// TranslateSystemPrompt 生成回译用系统提示词（§7.7 表一）。
// 内置模板可被 FR_TRANSLATE_PROMPT 覆盖；覆盖时仍会追加术语行——
// 术语是数据不是措辞，站长换模板不该把术语一起换掉。
func TranslateSystemPrompt(custom string, opt PromptOptions) string {
	var b strings.Builder
	if custom != "" {
		b.WriteString(custom)
		b.WriteString("\n\n")
	} else {
		fmt.Fprintf(&b, "你是专业译者。把用户给出的文本翻译成%s。\n\n", opt.ToLabel)
		b.WriteString("要求：\n")
		b.WriteString("1. 只输出译文本身，不要解释、不要加引号、不要重复原文、不要加\"译文：\"这类前缀。\n")
		b.WriteString("2. 保持原文的段落结构与标点层级。\n")
		fmt.Fprintf(&b, "3. 风格：%s。\n", opt.StyleHint)
	}
	if len(opt.Glossary) > 0 {
		b.WriteString("以下术语必须严格使用给定译法，不得替换为同义词：\n")
		for _, term := range opt.Glossary {
			fmt.Fprintf(&b, "- %s → %s\n", term[0], term[1])
		}
	}
	return strings.TrimSpace(b.String())
}

// ProofreadSystemPrompt 生成纠错用系统提示词（§7.7 表二）。
// 输出契约：JSON 数组，original 必须逐字来自原文；理由必须是教学解释
//（错在哪、规则是什么、为什么这样改，FR-203）。
func ProofreadSystemPrompt(custom string) string {
	if custom != "" {
		return custom
	}
	return strings.TrimSpace(`
你是英文写作老师，做纠错检查：逐条指出这段英文里的错误并讲清规则。只输出 JSON 数组，不要输出其他任何内容。
每个元素形如：
{"original":"逐字来自原文的片段","suggestion":"改法","category":"类别","reason":"一句话理由"}
规则：
1. original 必须是原文里逐字存在的片段（一个词或短语），禁止输出偏移量，禁止改写原文。
2. category 只能取：spelling、grammar、punctuation、style、word_choice。
3. reason 必须是教学解释：错在哪、规则是什么、为什么这样改，让作者能学到一条可复用的规则；禁止"表达欠佳"这类笼统评价。
4. 只报确定的错误，不报可选的润色；没有错误就输出 []。
5. 不做整段重写。
`)
}

// DictSystemPrompt 生成词典兜底用系统提示词（§7.7 表三 + §八 8.3 在线态）。
// 模型解释要求：先指出疑似拼写错误（AD-5），再给词性、简短中文释义与一个英文例句。
func DictSystemPrompt() string {
	return strings.TrimSpace(`
你是一本英汉词典。用户给出一个英文单词或短语（2-4 个词）。
先判断它是否像拼写错误：如果是，在解释的第一行给出"可能是 XXX 的拼写错误"。
然后输出 JSON（不要输出其他内容）：
{"pos":"词性缩写，如 v. n.","definition":"简短中文释义（多个词性用分号隔开）","example":"一个英文例句"}
例句必须展示该词的典型用法，不超过 25 个词。
`)
}

// CacheFingerprint 计算缓存键的业务部分（FR-1101）：
// 模型 + 语言对 + 风格 + 术语指纹 + 段落文本 的 SHA-256。
// 术语变更 → 指纹变化 → 同段文本不再命中旧缓存（FR-503 前端横幅依赖同一指纹）。
func CacheFingerprint(model, from, to, style string, glossary [][2]string, text string) string {
	h := sha256.New()
	var gb strings.Builder
	for _, t := range glossary {
		gb.WriteString(t[0])
		gb.WriteString("→")
		gb.WriteString(t[1])
		gb.WriteString(";")
	}
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s", model, from, to, style, gb.String(), text)
	return hex.EncodeToString(h.Sum(nil))
}
