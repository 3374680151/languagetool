# 接口契约

统一前缀 `/api`。请求与响应均为 `application/json; charset=utf-8`。
错误统一走 `{"error":{"code","message","hint"}}`（见 `01-architecture.md` 第四节）。

---

## 一、总表

| 方法 | 路径 | 用途 | 鉴权 |
| --- | --- | --- | --- |
| GET | `/api/config` | 站点能力与可选模型 | 无 |
| POST | `/api/translate` | 批量翻译（段落级） | 访客 |
| POST | `/api/proofread` | 英文纠错 | 访客 |
| GET | `/api/word` | 词典查询 | 访客 |
| GET | `/api/vocab` | 词表列表 | 访客 |
| POST | `/api/vocab` | 收藏 | 访客 |
| DELETE | `/api/vocab/{id}` | 删除 | 访客 |
| GET | `/api/vocab/export` | 导出 JSON / TSV | 访客 |
| GET | `/api/glossary` | 生效术语 | 访客 |
| POST | `/api/glossary` | 新增术语 | 管理员 |
| DELETE | `/api/glossary/{id}` | 删除术语 | 管理员 |
| POST | `/api/auth/login` | 后台登录 | 无 |
| POST | `/api/auth/logout` | 登出 | 管理员 |
| GET | `/api/auth/me` | 当前会话 | 管理员 |
| POST | `/api/sdk/translate` | SDK 翻译 | 站点 token |
| GET | `/api/sdk/word` | SDK 词典 | 站点 token |
| GET | `/api/health` | 健康检查 | 无 |

**访客身份**：首次请求由服务端下发 `fr_vid`（HttpOnly cookie，随机 UUID），后续按它关联词表。
不要求注册，不要求邮箱。

---

## 二、GET /api/config

前端启动时拉一次，决定界面能显示什么。**绝不下发任何密钥。**

```json
{
  "siteName": "FluentRead",
  "languages": [
    { "id": "auto", "label": "自动检测" },
    { "id": "zh-Hans", "label": "简体中文" },
    { "id": "en", "label": "English" }
  ],
  "defaults": { "from": "zh-Hans", "to": "en", "style": "academic" },
  "models": [
    { "id": "deepseek-chat", "label": "DeepSeek",    "quality": "高", "speed": "快", "cost": "低", "enabled": true },
    { "id": "qwen-plus",     "label": "通义千问",    "quality": "中", "speed": "快", "cost": "低", "enabled": true }
  ],
  "defaultModel": "deepseek-chat",
  "features": { "proofread": false, "dictionary": false, "visitorModel": false },
  "quota": { "dailyLimit": 200, "used": 12, "remaining": 188, "resetAt": "2026-10-31T00:00:00+08:00", "unlimited": false },
  "upstream": { "cap": 200000, "used": 4120, "degraded": false }
}
```

前端据此：

- `models` 为空 → 隐藏模型下拉，用默认。
- `features.proofread=false` → 纠错按钮置灰，提示"本站未开启纠错"。
- `features.visitorModel=false` → 设置页隐藏"自定义模型"。

> **当前实现状态**（2026-10-30）：`proofread` 跟随网关是否就绪；
> `dictionary` 与 `visitorModel` **恒为 false**——`/api/word` 与 `/api/ai` 都还没实现。
> 这三个字段必须是"端点在不在"的真实反映，不能写成设计目标：
> 报 true 而端点不存在，前端会摆出一个点了就报错的入口。
- `quota.used` / `quota.remaining` 是**当前访客**的用量，不是全站总量。
  `quota.unlimited=true` 时不显示剩余。
- `upstream.degraded=true` 表示本站付费额度已用完，页面应直接显示
  "本站额度已用完，已切到免费线路"，不要等用户点了翻译才发现。

---

## 三、POST /api/translate

**请求**

```json
{
  "texts": ["本系统采用检索增强生成架构。", "在回答前先召回相关文档片段。"],
  "from": "zh-Hans",
  "to": "en",
  "style": "academic",
  "model": "deepseek-chat",
  "glossary": [{ "term": "检索增强生成", "translation": "retrieval-augmented generation" }]
}
```

| 字段 | 约束 |
| --- | --- |
| `texts` | 1–20 条，单条 ≤ 5000 字，单次合计 ≤ 20000 字 |
| `from` / `to` | 必须在 `languages` 枚举内；两者相同且非 auto → 400 |
| `style` | `academic` `formal` `concise` `casual`，缺省 `academic`；**写错回落默认值并记 WARN，不返回 400** |
| `model` | 必须在 `config.models` 且 `enabled`；缺省用 `defaultModel` |
| `glossary` | 可选，服务端会与站点术语合并去重 |

**空段落不算错误**：`texts` 里的空白行原样占位，返回 `ok:true` + `skipped:true` + 空 `text`，
不打上游、不计用量。文档里空行很常见，若判成失败会让整篇标红。
只有全部段落都是空白时，才整体返回 `BAD_REQUEST`。

**`style` 为什么宽松**：风格是排版偏好，不是功能开关。SDK 的 `data-fr-style` 由站长手写，
拼错一个词不该让整站翻译全部失效，因此回落默认值。语言与模型不享受这个宽松度——
它们错了就是真的翻不了，必须明确报错。

**响应**

```json
{
  "results": [
    { "index": 0, "ok": true,  "text": "This system adopts a retrieval-augmented generation architecture.", "ms": 820 },
    { "index": 1, "ok": true,  "skipped": true, "ms": 0 },
    { "index": 2, "ok": false, "error": "上游超时", "ms": 20000 }
  ],
  "provider": "llm",
  "model": "deepseek-chat",
  "glossaryApplied": 1,
  "degraded": false
}
```

- **必须逐条给 `ok`**，某条失败不影响其他条。
- `skipped:true` 表示空白段落，前端不渲染译文、不标红、不提供重试。
- `glossaryApplied` 表示实际命中的术语数，前端用它显示"术语 1 条生效"。
- `degraded:true` 表示本次没走付费线路（站点上游额度触顶），此时多一个
  `degradedReason` 字段说明原因。**这不是错误**：译文照常返回，前端显示
  "本站额度已用完，已切到免费线路"的横幅，不要显示成失败。
- 不做整批失败。

**失败响应（入口护栏，都在打上游之前返回）**

| 场景 | HTTP | `code` | `hint` 必含 |
| --- | --- | --- | --- |
| 超过 `FR_RATE_LIMIT` | 429 + `Retry-After` | `RATE_LIMITED` | 等待秒数 |
| 超过 `FR_QUOTA_DAILY` | 429 | `QUOTA_EXCEEDED` | 已用/上限、本次需要、剩余、恢复时间 |

`QUOTA_EXCEEDED` 的 hint 形如：

```
今日已用 200/200 段，本次需要 3 段，剩余 0 段；约 10 小时后（明日 0 点）恢复。可明天再来，或联系站长调高限额。
```

配额按**段**计：一次请求 20 段就是 20，空白段不计。
若"剩余段数 < 本次段数"，整批拒绝——不做部分翻译，避免用户看到半篇译文却不知道原因。

---

## 四、POST /api/proofread

**请求**

```json
{
  "text": "The system adopt a RAG architecture, it recall documents before answering.",
  "lang": "en",
  "model": "deepseek-chat"
}
```

**响应**

```json
{
  "issues": [
    { "original": "adopt",  "suggestion": "adopts",   "category": "grammar",     "reason": "主语 The system 是第三人称单数", "severity": 3 },
    { "original": "it recall", "suggestion": "and recalls", "category": "grammar", "reason": "逗号粘连 + 主谓不一致", "severity": 3 },
    { "original": "a RAG",  "suggestion": "an RAG",   "category": "grammar",     "reason": "RAG 读音以元音开头，用 an", "severity": 2 }
  ],
  "ms": 1240
}
```

**服务端必须做的校验**（不做就会替换错位置）：

1. `original` 必须是 `text` 的子串，否则丢弃该条。
2. `original` 在 `text` 中出现多次 → 保留但标记 `ambiguous: true`，前端提示用户确认。
3. `suggestion` 为空且 `severity=1` → 视为纯提示，不可接受。
4. 模型返回非 JSON → 尝试提取首个 `[...]` 片段再解析；仍失败则返回 `UPSTREAM_FAILED`。

---

## 五、GET /api/word

```
GET /api/word?term=retrieval&from=en&to=zh-Hans
```

```json
{
  "term": "retrieval",
  "phonetic": "/rɪˈtriːvl/",
  "partOfSpeech": "n.",
  "senses": ["检索", "取回"],
  "examples": [{ "src": "information retrieval", "dst": "信息检索" }],
  "source": "ecdict"
}
```

- `source` 为 `llm` 时，前端要明确标注"以下为机器释义"，不伪装成词典。
- 查不到 → `404 NOT_FOUND`，前端提示"未收录"，不要静默。

---

## 六、词表

**GET /api/vocab?q=&limit=50&offset=0**

```json
{
  "total": 128,
  "items": [
    {
      "id": "v_01",
      "term": "retrieval-augmented generation",
      "translation": "检索增强生成",
      "kind": "phrase",
      "context": "本系统采用检索增强生成架构。",
      "sourceUrl": "https://blog.example.com/rag",
      "createdAt": "2026-10-30T10:12:00Z"
    }
  ]
}
```

**POST /api/vocab**

```json
{ "term": "retrieval", "translation": "检索", "kind": "word", "context": "…", "sourceUrl": "…" }
```

去重规则：同 `term` + 同 `lang` 视为同一条，更新 `context` 而不新增。

**GET /api/vocab/export?format=json|tsv** → 文件下载，`Content-Disposition: attachment`。

---

## 七、术语

**GET /api/glossary?from=zh-Hans&to=en** → 合并三层后生效的术语列表。

**POST /api/glossary**（管理员）

```json
{ "term": "检索增强生成", "translation": "retrieval-augmented generation", "scope": "global" }
```

`scope` 为 `global` | `site` | `visitor`。访客只能写 `visitor` 层。

---

## 八、鉴权

**POST /api/auth/login**

```json
{ "password": "…" }
```

成功 → 下发 `fr_admin`（HttpOnly、SameSite=Lax、7 天）。
失败 → 401，**不区分"密码错"和"用户不存在"**，避免探测。

**GET /api/auth/me** → `{ "ok": true }` 或 401。

---

## 九、SDK 接口

与主站接口分离，用站点 token 鉴权，便于按域名统计与限流。

```
POST /api/sdk/translate
X-FR-Site: <站点 token>
```

```json
{ "text": "selected text", "from": "auto", "to": "zh-Hans" }
```

响应比主站精简：`{ "text": "…", "kind": "sentence" | "word" }`。
`kind=word` 时额外带 `entry`（词典结构），让 SDK 一次请求就能决定弹词卡还是译卡。

**校验顺序**：token 存在 → 域名在白名单 → 未超限流 → 才转发上游。
任何一步失败都不打上游，避免被刷额度。

---

## 十、GET /api/health

```json
{ "ok": true, "db": "ok", "gateway": "ok", "version": "0.1.0" }
```

`gateway` 探活失败时返回 `ok:false` 但 **HTTP 200**——让监控能区分"服务活着但上游挂了"。
