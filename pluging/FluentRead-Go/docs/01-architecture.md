# 架构设计

---

## 一、分层与依赖方向

依赖只能向下，不能反向。`httpapi` 不认识 SQL，`store` 不认识 HTTP。

```
cmd/server          装配：读配置 → 开库 → 建服务 → 挂路由 → 监听
     │
httpapi             只做 HTTP：解析、校验、调服务、写响应
     │
translate / ai / proofread / dict / glossary / vocab     业务编排
     │
store               只做数据存取（sqlc 生成）
     │
SQLite
```

**硬规则**

- handler 里不允许出现 SQL、不允许直接 `http.Client` 打上游。
- 业务包里不允许出现 `http.Request` / `http.ResponseWriter`。
- 任何跨层传递的错误都用 `errs.Error`，不用裸 `error` 直接冒到 handler。

---

## 二、一次翻译请求怎么走

```
浏览器
  │ POST /api/translate {texts, from, to, style, model}
  ▼
middleware   限流 → 解析访客 id → 记录开始时间
  ▼
httpapi      校验：texts 非空、长度 ≤ 5000、语言在枚举内
  ▼
translate    ① 拼装术语约束  ② 选 provider  ③ 并发调用  ④ 失败换下一个
  ▼
ai / provider
  │ OpenAI 兼容请求
  ▼
LiteLLM 网关 ──► 目标模型
  ▼
回写：每段独立结果 + 真实耗时 + 失败原因
```

**关键点：段落级独立。** 一批里某段失败，其他段照常返回，前端只重试失败那段。
不要整批失败——那会让用户重跑一整篇。

---

## 三、核心接口

```go
// internal/translate

type Request struct {
    Texts    []string
    From     string
    To       string
    Style    string        // academic | formal | concise | casual
    Glossary []glossary.Entry
    Model    string        // 空 = 用站点默认
}

type Result struct {
    Texts     []string
    Provider  string
    Model     string
    LatencyMS int64
}

type Translator interface {
    Name() string
    Translate(ctx context.Context, req Request) (Result, error)
}
```

注册表按优先级排列，逐个尝试，全部失败才返回错误：

```go
type Registry struct {
    providers []Translator   // 顺序即换线顺序
}

func (r *Registry) Translate(ctx context.Context, req Request) (Result, error)
```

```go
// internal/proofread

type Issue struct {
    Original   string `json:"original"`   // 原文片段，逐字复制
    Suggestion string `json:"suggestion"`
    Category   string `json:"category"`   // spelling|grammar|punctuation|style|wording
    Reason     string `json:"reason"`
    Severity   int    `json:"severity"`   // 1 提示 2 建议 3 必改
}

type Proofreader interface {
    Check(ctx context.Context, text, lang string) ([]Issue, error)
}
```

**`Issue.Original` 必须是原文的子串。** 校验方式：`strings.Contains(text, issue.Original)`，
不满足的条目直接丢弃——这是防止前端替换错位置的第一道闸。

```go
// internal/dict

type Entry struct {
    Term       string
    Phonetic   string
    PartOfSpeech string
    Senses     []string
    Examples   []Example
    Source     string   // ecdict | llm
}

type Dictionary interface {
    Lookup(ctx context.Context, term, from, to string) (Entry, error)
}
```

```go
// internal/glossary

type Scope string   // global | site | visitor

type Entry struct {
    Term        string
    Translation string
    LangPair    string   // "zh-Hans>en"
    Scope       Scope
    Owner       string
}

// 冲突消解：visitor > site > global，同层取最近更新
func Resolve(entries []Entry) []Entry
```

---

## 四、错误模型

所有对外错误走同一个信封：

```json
{
  "error": {
    "code": "UPSTREAM_TIMEOUT",
    "message": "翻译服务 20 秒未响应",
    "hint": "已自动换到备用线路，可重试"
  }
}
```

```go
// internal/errs

type Error struct {
    Code    string
    Message string
    Hint    string
    Status  int
    Err     error   // 内部原因，不对外输出
}
```

| Code | Status | 场景 |
| --- | --- | --- |
| `BAD_REQUEST` | 400 | 参数缺失或格式错 |
| `TEXT_TOO_LONG` | 400 | 单段超 5000 字 |
| `UNAUTHORIZED` | 401 | 后台未登录 |
| `FORBIDDEN_ORIGIN` | 403 | 嵌入域名不在白名单 |
| `NOT_FOUND` | 404 | 词条不存在 |
| `RATE_LIMITED` | 429 | 触发限流，`hint` 带等待秒数，响应头带 `Retry-After` |
| `QUOTA_EXCEEDED` | 429 | 当日额度用尽，`hint` 必须含已用/剩余/恢复时间 |
| `UPSTREAM_FAILED` | 502 | 上游返回错误 |
| `UPSTREAM_TIMEOUT` | 504 | 上游超时 |
| `INTERNAL` | 500 | 兜底，**不暴露内部细节** |

**规矩**：`hint` 必须给用户下一步怎么做，不能只报错。

两个 429 的区别要说清，否则前端会把"等一下"和"明天再来"混成一个提示：

- `RATE_LIMITED` 是**秒级**问题：等 `Retry-After` 秒后同一个请求能成功，前端自动退避即可。
- `QUOTA_EXCEEDED` 是**日级**问题：重试没有任何意义，前端必须提示恢复时间，并隐藏重试按钮。

---

## 四之二、入口护栏的顺序（P1.5）

`/api/translate` 的检查顺序是硬性的，不许调换：

```
限流（内存，最便宜）
  → 参数校验（不花钱）
  → 访客日配额（读库，落盘，重启不丢）
  → 站点级上游熔断（读库，决定要不要降级）
  → 才调用上游
```

任何一步失败都在**花钱之前**结束。这就是"先花钱再拒绝等于没拦"的具体含义。

| 护栏 | 计数口径 | 存储 | 超限行为 |
| --- | --- | --- | --- |
| 访客日配额 `FR_QUOTA_DAILY` | 实际送去翻译的**段数**，空白段不计 | `visitor_usage`（SQLite，重启不丢） | 429 `QUOTA_EXCEEDED` |
| 访客限流 `FR_RATE_LIMIT` | 每分钟请求数，令牌桶，容量 = 上限 | 内存（重启清零是正常语义） | 429 `RATE_LIMITED` + `Retry-After` |
| 上游熔断 `FR_UPSTREAM_DAILY_CAP` | 付费线路实际消耗的**字符数** | `usage_daily.upstream_chars` | **不拒绝**，降级到免费线路，响应带 `degraded:true` |

两条设计取舍：

1. **限流同时挂访客 id 与来源 IP 两个桶。** 只按 cookie 限流可以被"每次清 cookie"绕过。
   代价是同 NAT 出口的用户共用一个 IP 桶——个人站点可接受，多租户场景要重做。
   IP 只进内存计数，不落库、不写日志（见 `03-data-model.md` 的隐私约定）。
2. **熔断判的是"已消耗 + 本次字符数 > 上限"**，不是"已消耗 >= 上限"。
   后者会允许一次超长请求把上限顶穿，也让"上限设成极小值"时第一次请求仍然打付费线路。

---

## 五、日志约定

用 `log/slog`，结构化输出。

```go
slog.Info("translate", "provider", r.Provider, "model", r.Model,
    "chars", n, "ms", r.LatencyMS, "ok", true)
```

**禁止**：

- 不记原文、不记译文、不记访客 key、不记完整请求体。
- 只记长度、耗时、provider、错误码。

理由：这是翻译工具，用户内容敏感。日志泄露一次就是事故。

---

## 六、配置

单一来源：环境变量，启动时一次性解析成结构体并校验。

```go
type Config struct {
    Addr          string        // :8080
    DBPath        string
    GatewayBase   string        // http://litellm:4000
    GatewayKey    string
    DefaultModel  string
    AllowOrigins  []string
    QuotaDaily    int           // 每访客每日段数，0 = 不限
    RatePerMinute int           // 每访客每分钟请求数，0 = 不限
    UpstreamDailyCap int        // 站点付费上游每日字符数，0 = 不限
    AdminPassword string        // bcrypt 哈希，不存明文
    RequestTimeout time.Duration
}
```

启动时校验失败直接退出并打印缺哪项——**不要带着半截配置跑起来**。

---

## 七、并发与超时

| 位置 | 策略 |
| --- | --- |
| 批量翻译 | 每段一个 goroutine，`errgroup` 限并发 3 |
| 上游请求 | 单请求 20s，AI 60s，都用 `context.WithTimeout` |
| 换线 | 单 provider 失败立即换下一个，总预算不超过 3×单请求超时 |
| 优雅关闭 | `signal.NotifyContext` + `srv.Shutdown`，给 10s 排空 |

**不许**在没有 context 的情况下调上游——那会让超时和取消全部失效。

---

## 八、测试策略

| 层 | 怎么测 | 覆盖率目标 |
| --- | --- | --- |
| `proofread` 片段定位 | 纯函数，表驱动 | 100% |
| `glossary.Resolve` | 冲突场景表驱动 | 100% |
| `translate.Registry` 换线 | 假 provider，注入失败 | 100% |
| `httpapi` | `httptest` + 假服务 | 主路径全覆盖 |
| 前端 | Vitest 组件测试 + Playwright 冒烟 | 关键交互全覆盖 |

上游一律打假，**测试不允许真实联网**。
