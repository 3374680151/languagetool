# 数据模型

SQLite（`modernc.org/sqlite`，纯 Go，无 CGO）。单站点单实例，不做多租户。

---

## 一、表

### visitor — 访客

```sql
CREATE TABLE visitor (
    id           TEXT PRIMARY KEY,          -- UUID，写进 HttpOnly cookie
    created_at   INTEGER NOT NULL,          -- Unix 秒
    last_seen_at INTEGER NOT NULL
);
```

不收集邮箱、不收集 IP（IP 只用于限流计数，内存里，不落库）。

### vocab — 词表

```sql
CREATE TABLE vocab (
    id          TEXT PRIMARY KEY,
    visitor_id  TEXT NOT NULL REFERENCES visitor(id) ON DELETE CASCADE,
    term        TEXT NOT NULL,
    lang        TEXT NOT NULL,              -- 词条所属语言：en / zh-Hans
    translation TEXT NOT NULL DEFAULT '',
    kind        TEXT NOT NULL DEFAULT 'word',   -- word | phrase | sentence
    context     TEXT NOT NULL DEFAULT '',       -- 收藏时的原句，≤ 500 字
    source_url  TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_vocab_dedup ON vocab(visitor_id, term, lang);
CREATE INDEX idx_vocab_recent ON vocab(visitor_id, created_at DESC);
```

**去重语义**：同访客 + 同词 + 同语言 = 同一条。重复收藏只更新 `context` 与 `updated_at`，
不新增行——否则词表会被同一个词刷满。

### glossary — 术语

```sql
CREATE TABLE glossary (
    id          TEXT PRIMARY KEY,
    term        TEXT NOT NULL,
    translation TEXT NOT NULL,
    lang_pair   TEXT NOT NULL,              -- "zh-Hans>en"
    scope       TEXT NOT NULL,              -- global | site | visitor
    owner_id    TEXT NOT NULL DEFAULT '',   -- scope=visitor 时为 visitor_id
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_glossary_dedup ON glossary(term, lang_pair, scope, owner_id);
```

**冲突消解顺序**：`visitor` > `site` > `global`；同层取 `updated_at` 最新。

### site_token — 嵌入站点

```sql
CREATE TABLE site_token (
    id         TEXT PRIMARY KEY,
    origin     TEXT NOT NULL UNIQUE,        -- https://blog.example.com
    token      TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    revoked_at INTEGER
);
```

### usage_daily — 用量聚合

```sql
CREATE TABLE usage_daily (
    day       TEXT PRIMARY KEY,             -- YYYY-MM-DD
    requests  INTEGER NOT NULL DEFAULT 0,   -- 翻译段数，不是 HTTP 请求数
    failures  INTEGER NOT NULL DEFAULT 0,
    ms_total  INTEGER NOT NULL DEFAULT 0,
    tokens    INTEGER NOT NULL DEFAULT 0,
    chars     INTEGER NOT NULL DEFAULT 0,   -- 0002 迁移新增：全站字符数
    upstream_chars INTEGER NOT NULL DEFAULT 0 -- 0004 迁移新增：只算付费上游
);
```

**只存聚合数字，不存原文、不存译文。** 这是隐私红线。

三个口径必须说清，否则配额迟早算错：

- `requests` 记的是**段数**。一次请求带 20 段就是 20，空白段不计。
- `chars` 记的是**全站实际送去翻译的字符数**（含免费线路）。
- `upstream_chars` 记的是**只有付费线路**消耗的字符数。站点级熔断 `FR_UPSTREAM_DAILY_CAP`
  只认这一列——用 `chars` 会让免费线路跑得多的时候误判成"付费额度耗尽"。

### visitor_usage — 按访客的日用量

```sql
CREATE TABLE visitor_usage (
    visitor_id TEXT    NOT NULL,
    day        TEXT    NOT NULL,          -- YYYY-MM-DD，本地时区
    segments   INTEGER NOT NULL DEFAULT 0,
    chars      INTEGER NOT NULL DEFAULT 0,
    failures   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (visitor_id, day)
);

CREATE INDEX idx_visitor_usage_day ON visitor_usage(day);
```

**为什么必须有这张表**：`usage_daily` 只有全站一行，回答不了"这个人今天用了多少"，
于是 `FR_QUOTA_DAILY` 只能解析、只能对外上报，永远拦不住任何人。
一个滥用者可以在几小时内烧掉全站当天的上游额度，而站方连是谁都看不到。

口径与 `usage_daily` 完全一致：`segments` 是实际送去翻译的段数，`chars` 是对应字符数。
配额判定与扣减在同一个临界区里完成（`store.QuotaRepo.Consume`），
先判后扣的写法在并发下会超发。

**失败不退配额**：段已经送去上游、钱已经花了。允许退款等于把"故意制造失败"变成绕过配额的手段。

### config — 可热改配置

```sql
CREATE TABLE config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
```

只放"改了要立刻生效"的项：`allow_origins`、`quota_daily`、`default_model`、`features`。
密钥类（网关 key）**只走环境变量，不入库**，避免备份泄漏。

### admin — 后台账号

```sql
CREATE TABLE admin (
    id            INTEGER PRIMARY KEY CHECK (id = 1),   -- 单行
    password_hash TEXT NOT NULL,                        -- bcrypt
    updated_at    INTEGER NOT NULL
);
```

单管理员。要多人协作得另设计，本期不做。

---

## 二、Go 结构（sqlc 生成，此处示意）

```go
type Vocab struct {
    ID          string
    VisitorID   string
    Term        string
    Lang        string
    Translation string
    Kind        string
    Context     string
    SourceURL   string
    CreatedAt   int64
    UpdatedAt   int64
}
```

`store` 包对外只暴露方法，不暴露 `*sql.DB`：

```go
type Querier interface {
    UpsertVocab(ctx context.Context, arg UpsertVocabParams) (Vocab, error)
    ListVocab(ctx context.Context, arg ListVocabParams) ([]Vocab, error)
    CountVocab(ctx context.Context, visitorID string) (int64, error)
    DeleteVocab(ctx context.Context, id, visitorID string) error
    // glossary / usage / config 同理
}
```

---

## 三、sqlc 约定

- SQL 写在 `internal/store/query/*.sql`，带 `-- name: Xxx :one|:many|:exec` 注释。
- 生成物 `internal/store/*.sql.go` 提交进仓库，避免构建时依赖 sqlc 二进制。
- 命名：`UpsertVocab`、`ListVocab`、`DeleteVocab`，动词在前。
- **禁止**在 SQL 里做业务判断（比如软删除过滤写在 SQL，但"是否可见"的判断留在 Go）。

---

## 四、迁移

- 文件：`internal/store/migrations/0001_init.sql`、`0002_xxx.sql`，只增不改。- 启动时用 `embed.FS` 读取，按版本号顺序执行，记录在 `schema_migrations(version, applied_at)`。
- **禁止**改已发布的迁移文件——线上库已经跑过了，改了不会重跑，只会造成结构漂移。
- 每次迁移必须能在空库上从 0 跑到最新（CI 里验）。

---

## 五、备份

```bash
sqlite3 data/fluentread.db ".backup 'backup-$(date +%F).db'"
```

- 每天一次，保留 7 天。
- 备份文件含访客词表，**不要放进公开的静态目录**。
