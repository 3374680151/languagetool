// router.go 负责装配 Gin 路由与中间件（§7.4 端点表）。
//
// @file internal/httpapi/router.go
// 文件职责：只做绑定与依赖注入，不写业务。路由顺序有讲究：
//
//	/api 先注册（保证接口错误永远是 JSON，404/405 也是统一响应包），
//	剩下的路径才交给单页应用处理器回落 index.html。
//
// 端点分批落地：P0 = health + SPA；P1 = /api/translate；P1.5 = /api/quota；
// P2 = /api/config、/api/user/prefs；P3 = /api/proofread；P3.5 = /api/models/test；
// P4 = /api/dict、/api/glossary；P5 = /api/words；P5.5 = /api/mistakes、/api/review；
// P6 = /api/sdk/*、/sdk/fluentread-sdk.js。
//
// 主要内容：NewRouter。
//
// 模块边界：只认识 handler.App；中间件在 middleware 包。
package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"

	"fluentread/internal/errs"
	"fluentread/internal/httpapi/handler"
	"fluentread/internal/httpapi/middleware"
	"fluentread/internal/httpx"
)

// NewRouter 组装完整的 Gin 引擎。
func NewRouter(app *handler.App) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(middleware.Logger())

	// 健康检查不挂访客中间件：监控探针不应在 visitors 表里制造垃圾行。
	r.GET("/api/health", app.Health)

	// 访客身份：首访在任何页面/API 请求上都下发（FR-1104），
	// 词表、偏好、配额随访客 cookie 关联。
	visitor := middleware.Visitor(app.VisitorRepo)

	// 未来的业务端点挂到带 visitor 中间件的 /api 组上。
	api := r.Group("/api", visitor)
	api.POST("/translate", app.Translate)
	api.GET("/quota", app.Quota)
	api.GET("/config", app.Config)
	api.GET("/user/prefs", app.Prefs)
	api.PUT("/user/prefs", app.PutPrefs)
	api.POST("/proofread", app.Proofread)
	api.GET("/models", app.Models)
	api.POST("/models/test", app.ModelTest)
	api.GET("/dict/:word", app.Dict)
	api.GET("/mistakes", app.MistakesList)
	api.POST("/mistakes", app.MistakesCreate)
	api.DELETE("/mistakes", app.MistakesDelete)
	api.GET("/review", app.Review)
	api.POST("/review/score", app.ReviewScore)
	api.GET("/assets/export", app.AssetsExport)
	api.POST("/assets/clear", app.AssetsClear)
	api.GET("/words", app.Words)
	api.POST("/words", app.WordsCreate)
	api.DELETE("/words", app.WordsDelete)
	api.GET("/glossary", app.GlossaryList)
	api.POST("/glossary", app.GlossaryCreate)
	api.PUT("/glossary", app.GlossaryUpdate)
	api.DELETE("/glossary", app.GlossaryDelete)

	// 接口层 404 必须是 JSON 统一响应包，否则前端会把 HTML 当 JSON 解析；
	// 其余路径交给单页应用：静态资源直出，前端路由回落 index.html。
	// SPA 路径也走访客中间件：打开页面即建立匿名身份。
	r.NoRoute(visitor, func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			httpx.Fail(c.Writer, errs.InvalidRequest("接口不存在：请检查请求路径"))
			return
		}
		app.SPA()(c)
	})

	return r
}
