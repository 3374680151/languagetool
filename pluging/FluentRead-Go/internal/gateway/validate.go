// validate.go 是译文结果校验（§7.6 结果校验，继承扩展 broker 经验）。
//
// @file internal/gateway/validate.go
// 文件职责：拒绝空译文、拒绝原文回显、拒绝目标语种不符——三者都判为该段失败，
// 不把坏结果交给用户；另含 CleanOutput（剥模型输出包裹物）与轻量脚本检测
//（CJK/拉丁，服务 FR-301 auto 档与语种校验）。
//
// 主要内容：CleanTranslation、ValidateTranslation、DetectScript、HasCJK。
//
// 模块边界：纯函数，无状态；被 gateway.Pool 与 service 调用。
package gateway

import (
	"strings"
	"unicode"
)

// CleanTranslation 清理模型输出里的常见包裹物，只保留译文本身。
// 模型经常自作主张加"译文："前缀、引号或重复原文，这里统一剥掉。
func CleanTranslation(raw, source string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	// 去掉成对包裹的引号（中英文都处理）。
	for _, pair := range [][2]string{{`"`, `"`}, {"“", "”"}, {"'", "'"}, {"「", "」"}} {
		if strings.HasPrefix(text, pair[0]) && strings.HasSuffix(text, pair[1]) && len(text) > len(pair[0])+len(pair[1]) {
			text = strings.TrimSpace(text[len(pair[0]) : len(text)-len(pair[1])])
		}
	}

	// 去掉常见前缀。
	for _, prefix := range []string{"译文：", "译文:", "翻译：", "翻译:", "Translation:", "translation:"} {
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
		}
	}

	// 模型有时把原文也带回来，形如 "原文\n译文"。
	if source != "" && strings.HasPrefix(text, source) && len(text) > len(source) {
		text = strings.TrimSpace(strings.TrimPrefix(text, source))
	}

	return text
}

// ValidateTranslation 校验一段译文是否可用（§7.6 三条拒绝规则）。
// 返回 false 表示该段应判失败，不把坏结果交给用户。
func ValidateTranslation(translated, source, to string) bool {
	translated = strings.TrimSpace(translated)
	if translated == "" {
		return false
	}
	// 原文回显：与原文完全一致（或仅大小写差异）等于没翻。
	if strings.EqualFold(translated, strings.TrimSpace(source)) {
		return false
	}
	// 目标语种不符：目标期望 CJK（中/日/韩）却没有任何 CJK 字符；
	// 期望拉丁字母（英/法/德/西/俄）却几乎全是 CJK。
	switch to {
	case "zh-Hans", "zh-Hant", "ja", "ko":
		return HasCJK(translated)
	default:
		// 拉丁目标语言：CJK 占比超过三成判为错语种（混入少量符号不算）。
		return CJKRatio(translated) < 0.3
	}
}

// Script 表示轻量脚本检测结果。
type Script int

const (
	ScriptLatin Script = iota
	ScriptCJK
	ScriptOther
)

// DetectScript 判断文本主脚本：CJK/拉丁/其他。只服务 FR-301 的 auto 档
// 与 §7.6 语种校验，不是完整语言识别（规格明确只做轻量判断）。
func DetectScript(text string) Script {
	if HasCJK(text) {
		return ScriptCJK
	}
	for _, r := range text {
		if unicode.IsLetter(r) {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
				return ScriptLatin
			}
			return ScriptOther
		}
	}
	return ScriptLatin
}

// HasCJK 判断文本是否含有 CJK 统一表意文字或日文假名/韩文谚文。
func HasCJK(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
			(r >= 0x3040 && r <= 0x30ff) { // 平假名/片假名
			return true
		}
	}
	return false
}

// CJKRatio 返回 CJK 字符占全部字母类字符的比例。
func CJKRatio(text string) float64 {
	var total, cjk int
	for _, r := range text {
		if unicode.IsLetter(r) {
			total++
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || (r >= 0x3040 && r <= 0x30ff) {
				cjk++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(cjk) / float64(total)
}
