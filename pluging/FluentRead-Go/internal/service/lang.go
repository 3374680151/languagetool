// Package service 是业务编排层：分段校验、并发、失败隔离、计费口径、子串校验。
//
// @file internal/service/lang.go
// 文件职责：定义接口允许的语言集合（FR-301）与风格集合（FR-302）、默认值、
// 长度/段数上限常量（FR-305）；风格拼错回落默认值并告警（FR-302）。
// 语言/风格是接口契约的一部分：前端能选的、接口能收的，全部以这里为准。
//
// 主要内容：Languages、Styles、DefaultStyle、ValidLanguage、LanguageLabel、
// ValidStyle、StyleHint、Limits。
//
// 模块边界：纯定义与纯函数；不依赖 gateway/store/HTTP。
package service

import "log/slog"

// Limits 是翻译请求的硬上限（FR-305）。
const (
	// MaxSegments 单次请求最大段数。
	MaxSegments = 20
	// MaxSegmentLength 单段最大字数（按 Unicode 码点）。
	MaxSegmentLength = 5000
	// MaxTotalLength 单次请求合计最大字数。
	MaxTotalLength = 20000
	// MaxConcurrency 段级并发上限（§7.3：默认 5）。
	MaxConcurrency = 5
)

// Language 是一个可翻译的语言。
type Language struct {
	ID    string
	Label string
}

// Languages 是接口允许的语言集合。顺序即前端下拉顺序（FR-301）。
var Languages = []Language{
	{ID: "auto", Label: "自动检测"},
	{ID: "zh-Hans", Label: "简体中文"},
	{ID: "zh-Hant", Label: "繁體中文"},
	{ID: "en", Label: "English"},
	{ID: "ja", Label: "日本語"},
	{ID: "ko", Label: "한국어"},
	{ID: "fr", Label: "Français"},
	{ID: "de", Label: "Deutsch"},
	{ID: "es", Label: "Español"},
	{ID: "ru", Label: "Русский"},
}

// Styles 是可选的写作风格（FR-302）。
var Styles = []string{"academic", "formal", "concise", "casual"}

// DefaultStyle 是缺省风格。写作台默认学术，因为主要场景是技术博客。
const DefaultStyle = "academic"

// DefaultLangPair 是全站默认语言对（FR-101：默认永远是英→中）。
const (
	DefaultFrom = "en"
	DefaultTo   = "zh-Hans"
)

// ValidLanguage 判断语言 id 是否受支持。
func ValidLanguage(id string) bool {
	for _, l := range Languages {
		if l.ID == id {
			return true
		}
	}
	return false
}

// LanguageLabel 返回语言的中文名，未知语言返回 id 本身。
func LanguageLabel(id string) string {
	for _, l := range Languages {
		if l.ID == id {
			return l.Label
		}
	}
	return id
}

// ValidStyle 判断风格是否受支持。
func ValidStyle(style string) bool {
	for _, s := range Styles {
		if s == style {
			return true
		}
	}
	return false
}

// NormalizeStyle 归一化风格：空值回落默认；拼错的回落默认并记 WARN（FR-302 验收）。
// 风格只是偏好不是功能开关：SDK 的 data-fr-style 是站长手写的，拼错很常见，
// 不该让整站翻译不可用。
func NormalizeStyle(style string) string {
	if style == "" {
		return DefaultStyle
	}
	if ValidStyle(style) {
		return style
	}
	slog.Warn("风格不受支持，回落默认值", "given", style, "fallback", DefaultStyle)
	return DefaultStyle
}

// StyleHint 把风格转成给模型的自然语言描述。
func StyleHint(style string) string {
	switch style {
	case "academic":
		return "学术写作风格：正式、精确，避免口语与缩写，术语保持一致"
	case "formal":
		return "正式商务风格：清晰简洁，语气克制"
	case "concise":
		return "简洁风格：去掉冗余修饰，句子短，信息密度高"
	case "casual":
		return "口语化风格：自然、轻松，像母语者日常说话"
	default:
		return "学术写作风格：正式、精确，避免口语与缩写"
	}
}

// ValidCategory 判断纠错类别枚举（FR-203）。
func ValidCategory(c string) bool {
	switch c {
	case "spelling", "grammar", "punctuation", "style", "word_choice":
		return true
	}
	return false
}
