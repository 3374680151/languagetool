// prefs.go 实现 GET/PUT /api/user/prefs（P2，FR-1001）。
//
// @file internal/httpapi/handler/prefs.go
// 文件职责：用户偏好 6 项（语言对/风格/模型/主题/查词触发方式/词卡自动朗读）的
// 读取与保存；存服务端 user_prefs 表，刷新后保留、跨浏览器一致（FR-1105）。
// 校验：语言码/风格必须合法；模型必须在站点池内或为空（FR-409 不允许占位模型）。
//
// 主要内容：Prefs、putPrefsBody。
//
// 模块边界：只解析与校验参数；存取在 store.PrefsRepo。
package handler

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/service"
)

// prefsBody 是 PUT 请求体（前端 UserPrefs 类型）。
type prefsBody struct {
	LangPair *struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"langPair"`
	Style       *string `json:"style"`
	Model       *string `json:"model"`
	Theme       *string `json:"theme"`
	WordTrigger *string `json:"wordTrigger"`
	AutoSpeak   *bool   `json:"autoSpeak"`
}

// prefsKey 偏好存储键。
const (
	prefKeyLangPair   = "langPair"
	prefKeyStyle      = "style"
	prefKeyModel      = "model"
	prefKeyTheme      = "theme"
	prefKeyTrigger    = "wordTrigger"
	prefKeyAutoSpeak  = "autoSpeak"
)

// Prefs 返回当前访客的偏好（缺省用默认值补齐）。
func (a *App) Prefs(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	raw, err := a.prefs(visitor)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, raw)
}

// PutPrefs 保存偏好。只更新请求里出现的键（部分更新语义）。
func (a *App) PutPrefs(c *gin.Context) {
	visitor := middleware.VisitorID(c)
	var body prefsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		Fail(c, errs.InvalidRequest("请求体不是有效 JSON"))
		return
	}

	if body.LangPair != nil {
		if !service.ValidLanguage(body.LangPair.From) || !service.ValidLanguage(body.LangPair.To) {
			Fail(c, errs.UnsupportedLanguage())
			return
		}
		if body.LangPair.From == body.LangPair.To && body.LangPair.From != "auto" {
			Fail(c, errs.UnsupportedLanguage())
			return
		}
		if err := a.savePref(visitor, prefKeyLangPair, map[string]string{
			"from": body.LangPair.From, "to": body.LangPair.To,
		}); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
	}
	if body.Style != nil {
		style := service.NormalizeStyle(*body.Style) // 拼错回落默认（FR-302）
		if err := a.savePref(visitor, prefKeyStyle, style); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
	}
	if body.Model != nil {
		model := *body.Model
		if model != "" && a.Catalog != nil && a.Catalog.Count() > 0 && !a.Catalog.Allowed(model) {
			Fail(c, errs.InvalidRequest("模型不在站点可用清单里"))
			return
		}
		if err := a.savePref(visitor, prefKeyModel, model); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
	}
	if body.Theme != nil {
		theme := *body.Theme
		if theme != "light" && theme != "dark" {
			Fail(c, errs.InvalidRequest("主题只支持 light/dark"))
			return
		}
		if err := a.savePref(visitor, prefKeyTheme, theme); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
	}
	if body.WordTrigger != nil {
		trigger := *body.WordTrigger
		switch trigger {
		case "direct", "icon", "modifier":
		default:
			Fail(c, errs.InvalidRequest("查词触发方式只支持 direct/icon/modifier"))
			return
		}
		if err := a.savePref(visitor, prefKeyTrigger, trigger); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
	}
	if body.AutoSpeak != nil {
		if err := a.savePref(visitor, prefKeyAutoSpeak, *body.AutoSpeak); err != nil {
			Fail(c, errs.Internal(err))
			return
		}
	}

	raw, err := a.prefs(visitor)
	if err != nil {
		Fail(c, errs.Internal(err))
		return
	}
	OK(c, raw)
}

// prefs 组装带默认值的完整偏好视图（§7.4）。
func (a *App) prefs(visitor string) (map[string]any, error) {
	saved, err := a.PrefsRepo.GetAll(visitor)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		// 默认语言对永远是英→中（FR-101/112）。
		prefKeyLangPair:  map[string]string{"from": service.DefaultFrom, "to": service.DefaultTo},
		prefKeyStyle:     service.DefaultStyle,
		prefKeyModel:     "",
		prefKeyTheme:     "light",
		prefKeyTrigger:   "direct", // 划选即弹（§八 8.2 默认）
		prefKeyAutoSpeak: false,
	}
	if v, ok := saved[prefKeyLangPair]; ok {
		var pair map[string]string
		if json.Unmarshal([]byte(v), &pair) == nil && pair["from"] != "" {
			out[prefKeyLangPair] = pair
		}
	}
	if v, ok := saved[prefKeyStyle]; ok && service.ValidStyle(v) {
		out[prefKeyStyle] = v
	}
	if v, ok := saved[prefKeyModel]; ok {
		out[prefKeyModel] = v
	}
	if v, ok := saved[prefKeyTheme]; ok && (v == "light" || v == "dark") {
		out[prefKeyTheme] = v
	}
	if v, ok := saved[prefKeyTrigger]; ok {
		switch v {
		case "direct", "icon", "modifier":
			out[prefKeyTrigger] = v
		}
	}
	if v, ok := saved[prefKeyAutoSpeak]; ok {
		var b bool
		if json.Unmarshal([]byte(v), &b) == nil {
			out[prefKeyAutoSpeak] = b
		}
	}
	return out, nil
}

// savePref 序列化并保存一条偏好。字符串原样存（不包 JSON 引号），
// 结构化值（语言对、布尔）才用 JSON——读取侧据此区分。
func (a *App) savePref(visitor, key string, value any) error {
	var raw []byte
	if s, ok := value.(string); ok {
		raw = []byte(s)
	} else {
		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}
	return a.PrefsRepo.Put(visitor, key, string(raw))
}
