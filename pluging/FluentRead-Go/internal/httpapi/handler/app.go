// Package handler 存放全部 HTTP handler（§7.2）。
//
// @file internal/httpapi/handler/app.go
// 文件职责：定义 App 依赖容器——HTTP 层能看到的一切依赖；handler 只做
// 参数解析与响应组装，业务规则在 service，上游协议在 gateway，SQL 在 store（§7.3）。
// 错误一律经 internal/errs 构造、internal/httpx 渲染，禁止裸写 c.JSON(500,...)（NFR-12）。
//
// 主要内容：App 结构体、NewApp。
//
// 模块边界：被 router.go 与各 handler 文件使用；由 cmd/server 装配。
package handler

import (
	"io/fs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"fluentread/internal/config"
	"fluentread/internal/gateway"
	"fluentread/internal/httpx"
	"fluentread/internal/ratelimit"
	"fluentread/internal/service"
	"fluentread/internal/store"
)

// Version 是当前构建版本，健康检查与首页会带上。
const Version = "1.0.0"

// App 是 HTTP 层的依赖集合。
type App struct {
	Cfg     *config.Config
	Catalog *config.Catalog
	DB      *gorm.DB
	// Usage 提供访客配额与站点用量（落库，重启不丢）。
	Usage *store.UsageRepo
	// VisitorRepo 提供匿名访客续期（FR-1104）。
	VisitorRepo *store.VisitorRepo
	// PrefsRepo 提供用户偏好存取（FR-1001/1105）。
	PrefsRepo *store.PrefsRepo
	// Limiter 是按访客与 IP 的内存限流（重启清零是正常语义）。
	Limiter *ratelimit.Limiter
	// Pool 是模型池与降级链。
	Pool *gateway.Pool
	// Translator 是翻译编排。
	Translator *service.Translator
	// Assets 是内嵌的前端产物（web/dist）。为 nil 时返回"前端未构建"说明页。
	Assets fs.FS
}

// NewApp 构造依赖容器。
func NewApp(cfg *config.Config, catalog *config.Catalog, db *gorm.DB, pool *gateway.Pool, translator *service.Translator, assets fs.FS) *App {
	return &App{
		Cfg: cfg, Catalog: catalog, DB: db,
		Usage:       store.NewUsageRepo(db),
		VisitorRepo: store.NewVisitorRepo(db),
		PrefsRepo:   store.NewPrefsRepo(db),
		Limiter:     ratelimit.New(cfg.RatePerMinute),
		Pool:        pool, Translator: translator, Assets: assets,
	}
}

// OK 统一成功出口：{"ok":true,"data":...}。
func OK(c *gin.Context, data any) { httpx.OK(c.Writer, data) }

// Fail 统一错误出口：{"ok":false,"error":{...}}（NFR-12）。
func Fail(c *gin.Context, err error) { httpx.Fail(c.Writer, err) }
