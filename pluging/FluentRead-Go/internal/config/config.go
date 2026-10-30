// Package config 负责从环境变量解析并校验运行时配置（附录 A）。
//
// @file internal/config/config.go
// 文件职责：启动时一次性读取全部环境变量并校验；密钥类只存在于环境变量，
// 不入库、不写日志、不下发前端（FR-1002）。配置缺失大多不阻断启动——
// 未配置模型走六态之"未配置"，而不是拒绝服务。
//
// 主要内容：Config 结构体（与附录 A 一一对应）、Load、Warnings。
//
// 模块边界：只依赖标准库；被 cmd/server 与各 service 读取；不认识 HTTP 与数据库。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 是服务运行所需的全部配置。字段与附录 A 一一对应。
type Config struct {
	// Listen 监听地址，默认 :8787。
	Listen string
	// DBPath 是 SQLite 文件路径。
	DBPath string
	// BaseURL 是 L1 默认线路的 OpenAI 兼容端点（推荐 LiteLLM，DR-9），不含 /v1。
	BaseURL string
	// APIKey 是 L1 线路密钥；支持逗号分隔多把（FR-1102），失败自动切换。
	APIKey string
	// Model 是 L1 默认模型名（FR-407：不写死在业务代码里）。空 = 从 models.json 取 primary。
	Model string
	// FallbackBaseURL 是免费降级线路地址（DR-10：Go 直连，不经 LiteLLM）。
	FallbackBaseURL string
	// FallbackType 取值 mymemory；空 = 禁用降级。
	FallbackType string
	// ModelsFile 是 L2 模型池展示清单路径。
	ModelsFile string
	// UpstreamTimeout 是上游单次调用超时。
	UpstreamTimeout time.Duration
	// VisitorDailySegments 是访客日配额（段）；0 = 不限。
	VisitorDailySegments int
	// SiteDailyPaidChars 是站点付费日字符封顶；0 = 不限（FR-603）。
	SiteDailyPaidChars int
	// RatePerMinute 是访客每分钟桶；IP 桶为 4×（FR-602）。
	RatePerMinute int
	// DictRatePerMinute 是词典模型兜底的每分钟限频（§八 8.5，默认 10）。
	DictRatePerMinute int
	// CacheEnabled 是服务端翻译缓存开关（FR-1101）。
	CacheEnabled bool
	// SDKOrigins 是 SDK 域名白名单，逗号分隔；空 = SDK 关闭（FR-906 安全默认）。
	SDKOrigins []string
	// TranslatePrompt / ProofreadPrompt 是可选的提示词模板覆盖（§7.7 站长调优不改代码）。
	TranslatePrompt string
	ProofreadPrompt string
}

// Load 读取环境变量并返回校验后的配置。
// 必填项缺失返回错误；密钥类缺失只记警告（允许先启动再配置模型）。
func Load() (*Config, error) {
	cfg := &Config{
		Listen:          env("FR_LISTEN", ":8787"),
		DBPath:          env("FR_DB_PATH", "data/fluentread.db"),
		BaseURL:         strings.TrimRight(env("FR_BASE_URL", ""), "/"),
		APIKey:          env("FR_API_KEY", ""),
		Model:           env("FR_MODEL", ""),
		FallbackBaseURL: strings.TrimRight(env("FR_FALLBACK_BASE_URL", ""), "/"),
		FallbackType:    strings.ToLower(env("FR_FALLBACK_TYPE", "")),
		ModelsFile:      env("FR_MODELS_FILE", "deploy/models.json"),
		SDKOrigins:      splitList(env("FR_SDK_ORIGINS", "")),
		TranslatePrompt: os.Getenv("FR_TRANSLATE_PROMPT"),
		ProofreadPrompt: os.Getenv("FR_PROOFREAD_PROMPT"),
	}

	var err error
	if cfg.UpstreamTimeout, err = envSeconds("FR_UPSTREAM_TIMEOUT_SECONDS", 30); err != nil {
		return nil, err
	}
	if cfg.VisitorDailySegments, err = envInt("FR_VISITOR_DAILY_SEGMENTS", 200); err != nil {
		return nil, err
	}
	if cfg.SiteDailyPaidChars, err = envInt("FR_SITE_DAILY_PAID_CHARS", 200000); err != nil {
		return nil, err
	}
	if cfg.RatePerMinute, err = envInt("FR_RATE_PER_MINUTE", 20); err != nil {
		return nil, err
	}
	if cfg.DictRatePerMinute, err = envInt("FR_DICT_RATE_PER_MINUTE", 10); err != nil {
		return nil, err
	}
	if cfg.CacheEnabled, err = envBool("FR_CACHE_ENABLED", true); err != nil {
		return nil, err
	}

	if cfg.Listen == "" {
		return nil, fmt.Errorf("FR_LISTEN 不能为空")
	}
	if cfg.DBPath == "" {
		return nil, fmt.Errorf("FR_DB_PATH 不能为空")
	}
	if cfg.BaseURL != "" && !strings.HasPrefix(cfg.BaseURL, "http://") && !strings.HasPrefix(cfg.BaseURL, "https://") {
		return nil, fmt.Errorf("FR_BASE_URL 必须以 http:// 或 https:// 开头，当前为 %q", cfg.BaseURL)
	}
	if cfg.FallbackType != "" && cfg.FallbackType != "mymemory" {
		return nil, fmt.Errorf("FR_FALLBACK_TYPE 只支持 mymemory，当前为 %q", cfg.FallbackType)
	}
	if cfg.FallbackType == "mymemory" && cfg.FallbackBaseURL == "" {
		// 免费线路启用但没给地址：按官方端点处理，不阻断启动。
		cfg.FallbackBaseURL = "https://api.mymemory.translated.net/get"
	}
	if cfg.VisitorDailySegments < 0 || cfg.SiteDailyPaidChars < 0 || cfg.RatePerMinute < 0 {
		return nil, fmt.Errorf("配额与限流不能为负数")
	}
	return cfg, nil
}

// Warnings 返回不阻断启动但需要提示的问题，由 main 逐条打印。
func (c *Config) Warnings() []string {
	var out []string
	if c.BaseURL == "" || c.APIKey == "" {
		out = append(out, "未配置 FR_BASE_URL / FR_API_KEY：模型翻译与纠错不可用，站点处于未配置态")
	}
	if c.Model == "" {
		out = append(out, "未设置 FR_MODEL：将取模型清单 role=primary 且 enabled 的第一项")
	}
	if c.SiteDailyPaidChars == 0 {
		out = append(out, "FR_SITE_DAILY_PAID_CHARS=0：站点付费字符不设上限，额度被刷穿时不会自动切换")
	}
	if len(c.SDKOrigins) == 0 {
		out = append(out, "未设置 FR_SDK_ORIGINS：SDK 处于关闭状态")
	}
	return out
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数，当前为 %q", key, raw)
	}
	return n, nil
}

// envSeconds 解析"秒数"类变量为 Duration。
func envSeconds(key string, fallback int) (time.Duration, error) {
	n, err := envInt(key, fallback)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s 必须大于 0", key)
	}
	return time.Duration(n) * time.Second, nil
}

func envBool(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s 必须是布尔值（true/false），当前为 %q", key, raw)
	}
	return v, nil
}

// splitList 解析逗号分隔列表，去空白、去空项。
func splitList(raw string) []string {
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
