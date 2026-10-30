// config.go 实现 GET /api/config（P2，§7.4）。
//
// @file internal/httpapi/handler/config.go
// 文件职责：下发公共配置——语言表、风格表、模型池（仅 id/label/标签/health）、
// 默认语言对、能力开关、健康摘要。**无任何密钥**（FR-1002）；未实现的能力
// 如实报 false，前端据此隐藏入口（FR-1004）。
//
// 主要内容：Config。
//
// 模块边界：只读 Catalog/Pool/配置；不下发 FR_API_KEY 等任何密钥。
package handler

import (
	"github.com/gin-gonic/gin"

	"fluentread/internal/service"
)

// Config 返回公共配置。
func (a *App) Config(c *gin.Context) {
	languages := make([]gin.H, 0, len(service.Languages))
	for _, l := range service.Languages {
		languages = append(languages, gin.H{"id": l.ID, "label": l.Label})
	}

	// 模型池：enabled 的才出现（FR-402/409）；health 来自巡检（P3.5），
	// 未巡检前一律 ok——巡检降级时这里会报 degraded 并从下拉剔除。
	models := make([]gin.H, 0, 8)
	for _, m := range a.Catalog.Enabled() {
		health := a.Pool.ModelHealth(m.ID)
		if health == "removed" {
			// 连续对账失败的模型不出现在下拉（FR-408/1103）。
			continue
		}
		models = append(models, gin.H{
			"id": m.ID, "label": m.Label, "role": m.Role,
			"quality": m.Quality, "speed": m.Speed, "cost": m.Cost,
			"enabled": m.Enabled, "health": health,
		})
	}

	primary, hasPrimary := a.Catalog.Primary()
	defaultModel := a.Cfg.Model
	if defaultModel == "" && hasPrimary {
		defaultModel = primary.ID
	}

	// 网关就绪 = 已配置且巡检未把主模型剔除；未就绪时前端显示"未配置"态。
	gatewayReady := a.Pool.SiteClient().Configured()
	health := gin.H{
		"gateway": "ok",
		"db":      "ok",
	}
	if !gatewayReady {
		health["gateway"] = "fail"
	} else if a.Pool.Degraded() {
		health["gateway"] = "degraded"
	}

	// 能力开关（FR-1004）：与分期一致，未实现的如实报 false。
	features := gin.H{
		"translate": gatewayReady,
		"proofread": true,  // P3 已上线
		"visitorModel": true, // P3.5 已上线
		"dictionary": false, // P4 打开
		"glossary":   false, // P4 打开
		"sdk":        false, // P6 打开
	}

	OK(c, gin.H{
		"siteName":        siteName,
		"languages":       languages,
		"styles":          service.Styles,
		"defaultLangPair": gin.H{"from": service.DefaultFrom, "to": service.DefaultTo},
		"defaultStyle":    service.DefaultStyle,
		"models":          models,
		"defaultModel":    defaultModel,
		"features":        features,
		"health":          health,
	})
}
