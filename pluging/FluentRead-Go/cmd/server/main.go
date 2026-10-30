// main 是程序入口：解析配置、打开数据库、AutoMigrate、装配网关与服务、启动并等待信号。
//
// @file cmd/server/main.go
// 文件职责：装配顺序即依赖顺序（§7.2）——config → store → gateway(pool/cache/free)
// → service(translator) → httpapi(router)。任一环节失败立刻退出并打印原因，
// 不带半截配置运行。退出时优雅排空连接。
//
// 主要内容：main。
//
// 模块边界：只做装配，不写业务；密钥只经环境变量进入本进程（FR-1002）。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fluentread/internal/config"
	"fluentread/internal/gateway"
	"fluentread/internal/httpapi"
	"fluentread/internal/httpapi/handler"
	"fluentread/internal/service"
	"fluentread/internal/store"
	"fluentread/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置校验失败，已退出", "err", err)
		os.Exit(1)
	}
	for _, warning := range cfg.Warnings() {
		slog.Warn("配置提醒", "detail", warning)
	}

	// Step 1: 数据库（GORM + glebarez 纯 Go 驱动，DR-3）。
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("数据库初始化失败，已退出", "err", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close(db) }()

	if err := store.Migrate(db); err != nil {
		slog.Error("结构迁移失败，已退出", "err", err)
		os.Exit(1)
	}

	// Step 2: 模型清单（FR-402/409 启动校验）。
	catalog, err := config.LoadCatalog(cfg.ModelsFile)
	if err != nil {
		slog.Error("模型清单加载失败，已退出", "err", err)
		os.Exit(1)
	}
	if catalog.Count() == 0 {
		slog.Warn("模型清单为空", "file", cfg.ModelsFile, "detail", "前端不会出现模型下拉，站点处于未配置态")
	}

	// FR_MODEL 缺省时取清单 primary（FR-401/402）：不能把空模型名发给网关，
	// 那会让每次请求都先打一次必然失败的调用。
	model := cfg.Model
	if model == "" {
		if m, ok := catalog.Primary(); ok {
			model = m.ID
			slog.Info("未设置 FR_MODEL，已取清单 primary", "model", model)
		}
	}

	// Step 3: 网关层：站点线路客户端（多 Key 轮换 FR-1102）、免费后备（DR-10）、缓存（FR-1101）。
	keys := splitKeys(cfg.APIKey)
	client := gateway.New(cfg.BaseURL, keys, cfg.UpstreamTimeout)
	var free *gateway.MyMemory
	if cfg.FallbackType == "mymemory" {
		free = gateway.NewMyMemory(cfg.UpstreamTimeout, cfg.FallbackBaseURL)
		slog.Info("免费后备线路已启用", "type", cfg.FallbackType)
	}
	var cache *gateway.Cache
	if cfg.CacheEnabled {
		cache = gateway.NewCache(0, 0, 0)
	}
	pool := gateway.NewPool(client, free, cfg, catalog, cache, cfg.UpstreamTimeout)

	// Step 3.5: 模型池巡检（FR-1103）：启动时 + 每小时对账；
	// 间隔可经 FR_INSPECT_INTERVAL_SECONDS 覆盖（测试用），<=0 时禁用定时只跑启动那轮。
	inspectInterval := 1 * time.Hour
	if v := os.Getenv("FR_INSPECT_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			inspectInterval = time.Duration(n) * time.Second
		}
	}
	go func() {
		runInspect := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			live := map[string]bool{}
			if ids, err := client.Models(ctx); err != nil {
				slog.Warn("模型清单对账失败，本轮保留现状", "err", err)
				pool.Inspect(nil)
				return
			} else {
				for _, id := range ids {
					live[id] = true
				}
			}
			pool.Inspect(live)
		}
		runInspect()
		if inspectInterval <= 0 {
			return
		}
		ticker := time.NewTicker(inspectInterval)
		defer ticker.Stop()
		for range ticker.C {
			runInspect()
		}
	}()

	// Step 4: 服务层与 HTTP 层。
	translator := service.NewTranslator(pool, store.NewUsageRepo(db), cfg)
	proofreader := service.NewProofreader(pool, store.NewUsageRepo(db), cfg)

	assets, err := web.FS()
	if err != nil {
		// 前端产物缺失不该阻断 API：接口照常可用，页面会显示"未构建"说明。
		slog.Warn("前端产物加载失败，页面将不可用", "err", err)
		assets = nil
	}

	app := handler.NewApp(cfg, catalog, db, pool, translator, proofreader, assets)
	router := httpapi.NewRouter(app)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      cfg.UpstreamTimeout + 30*time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("服务启动",
			"addr", cfg.Listen,
			"db", cfg.DBPath,
			"base", cfg.BaseURL,
			"model", model,
			"version", handler.Version,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("监听失败", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("收到退出信号，开始排空连接")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("优雅关闭未完成", "err", err)
	}
	slog.Info("已退出")
}

// splitKeys 把逗号分隔的密钥串拆成多把（FR-1102）。
func splitKeys(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
