// config_test.go 验证环境变量装载与 models.json 校验（附录 A / FR-409）。
//
// @file internal/config/config_test.go
// 文件职责：默认值、校验失败路径、清单 primary 解析与占位模板拒绝。
//
// 模块边界：纯单元测试；用 t.Setenv 隔离环境变量。
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"FR_LISTEN", "FR_DB_PATH", "FR_BASE_URL", "FR_API_KEY", "FR_MODEL",
		"FR_FALLBACK_BASE_URL", "FR_FALLBACK_TYPE", "FR_MODELS_FILE",
		"FR_UPSTREAM_TIMEOUT_SECONDS", "FR_VISITOR_DAILY_SEGMENTS",
		"FR_SITE_DAILY_PAID_CHARS", "FR_RATE_PER_MINUTE", "FR_CACHE_ENABLED",
		"FR_SDK_ORIGINS",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

// TestLoadDefaults 默认值必须与附录 A 一致。
func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Listen != ":8787" {
		t.Errorf("Listen = %q, want :8787", cfg.Listen)
	}
	if cfg.DBPath != "data/fluentread.db" {
		t.Errorf("DBPath = %q", cfg.DBPath)
	}
	if cfg.UpstreamTimeout.Seconds() != 30 {
		t.Errorf("UpstreamTimeout = %v", cfg.UpstreamTimeout)
	}
	if cfg.VisitorDailySegments != 200 {
		t.Errorf("VisitorDailySegments = %d", cfg.VisitorDailySegments)
	}
	if cfg.SiteDailyPaidChars != 200000 {
		t.Errorf("SiteDailyPaidChars = %d", cfg.SiteDailyPaidChars)
	}
	if cfg.RatePerMinute != 20 {
		t.Errorf("RatePerMinute = %d", cfg.RatePerMinute)
	}
	if !cfg.CacheEnabled {
		t.Error("缓存默认应开启（FR-1101）")
	}
}

// TestLoadValidation 非法配置要报错。
func TestLoadValidation(t *testing.T) {
	clearEnv(t)
	t.Setenv("FR_BASE_URL", "ftp://x")
	if _, err := Load(); err == nil {
		t.Fatal("非 http(s) 的 FR_BASE_URL 应报错")
	}

	clearEnv(t)
	t.Setenv("FR_FALLBACK_TYPE", "deepl")
	if _, err := Load(); err == nil {
		t.Fatal("未知 FR_FALLBACK_TYPE 应报错")
	}

	clearEnv(t)
	t.Setenv("FR_UPSTREAM_TIMEOUT_SECONDS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("超时必须大于 0")
	}
}

// TestLoadFallbackDefault 启用 mymemory 但没给地址时回落官方端点。
func TestLoadFallbackDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("FR_FALLBACK_TYPE", "mymemory")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.FallbackBaseURL == "" {
		t.Fatal("启用降级但未给地址时应回落官方端点")
	}
}

// TestSDKOrigins 白名单按逗号拆分并去空白（FR-906）。
func TestSDKOrigins(t *testing.T) {
	clearEnv(t)
	t.Setenv("FR_SDK_ORIGINS", " https://a.com , ,https://b.com ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.SDKOrigins) != 2 || cfg.SDKOrigins[0] != "https://a.com" {
		t.Fatalf("SDKOrigins = %v", cfg.SDKOrigins)
	}
}

func writeModels(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCatalogPrimary L1 默认取 role=primary 且 enabled 的第一项（FR-402）。
func TestCatalogPrimary(t *testing.T) {
	p := writeModels(t, `[
		{"id":"a","role":"primary","enabled":true},
		{"id":"b","role":"pool","enabled":true},
		{"id":"c","role":"primary","enabled":false}
	]`)
	cat, err := LoadCatalog(p)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	m, ok := cat.Primary()
	if !ok || m.ID != "a" {
		t.Fatalf("Primary = %+v ok=%v", m, ok)
	}
	if got := len(cat.Enabled()); got != 2 {
		t.Fatalf("Enabled = %d, want 2", got)
	}
	if !cat.Allowed("b") || cat.Allowed("c") {
		t.Fatal("Allowed 应只认 enabled 的模型")
	}
}

// TestCatalogPlaceholderRejected 占位模板启用时启动失败（FR-409）。
func TestCatalogPlaceholderRejected(t *testing.T) {
	p := writeModels(t, `[{"id":"my-custom","label":"自定义","enabled":true}]`)
	if _, err := LoadCatalog(p); err == nil {
		t.Fatal("my-custom 启用应被启动校验拒绝")
	}

	p2 := writeModels(t, `[{"id":"my-custom","label":"自定义","enabled":false}]`)
	if _, err := LoadCatalog(p2); err != nil {
		t.Fatalf("禁用的占位模板应放行: %v", err)
	}
}

// TestCatalogMissing 没有清单文件不报错——站点处于未配置态（§5.2）。
func TestCatalogMissing(t *testing.T) {
	cat, err := LoadCatalog(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if cat.Count() != 0 {
		t.Fatal("空清单条数应为 0")
	}
}

// TestCatalogBadFormat 格式错误要报错（避免静默变成空清单）。
func TestCatalogBadFormat(t *testing.T) {
	p := writeModels(t, `{not json`)
	if _, err := LoadCatalog(p); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
}
