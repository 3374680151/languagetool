# FluentRead 写作台（Go 单二进制）

> 左边写英文，右边即时看到中文回译；写错的单词与语法被逐条指出并讲清为什么；
> 查过的词、犯过的错自动变成你的学习材料，复习时挖空的是**你自己写过的句子**。
> 写的多了，英文就会了。

唯一实现依据：[docs/11-rewrite-spec.md](docs/11-rewrite-spec.md)（产品定位、FR 全集、六态、系统设计、分期、附录）。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.25 + **Gin** + **GORM**（glebarez/sqlite，纯 Go 无 CGO，`CGO_ENABLED=0` 可编译） |
| 前端 | Vue 3 + TypeScript + **Element Plus**（唯一 UI 库）+ Pinia + Vue Router + Vite，由 go:embed 打进二进制 |
| 模型接入 | 唯一 OpenAI 兼容协议；推荐 LiteLLM 网关（可选外挂，不部署则 FR_BASE_URL 直指任意兼容端点）；MyMemory 免费降级后备（Go 直连） |
| 部署 | 单二进制 + 单 SQLite 文件；Dockerfile + docker-compose（app 必选，litellm 可选 profile） |

## 一条命令启动

```bash
make build                     # CGO_ENABLED=0 → bin/fluentread（内嵌前端与 SDK）
FR_BASE_URL=http://127.0.0.1:4000 FR_API_KEY=sk-xxx ./bin/fluentread   # 默认 :8787
# 或
docker compose -f deploy/docker-compose.yml up -d --build              # 单容器
docker compose -f deploy/docker-compose.yml --profile litellm up -d    # 可选：加网关
```

环境变量见 [.env.example](.env.example)（与规格附录 A 一一对应）；部署细节见 [deploy/README.md](deploy/README.md)。

## 页面

| 路由 | 用途 |
| --- | --- |
| `/` | 写作台（左英右中逐段回译、纠错、术语、导出、查词弹卡） |
| `/words` | 词表（收藏带原句、再用计数）与错误本（类别筛选/删除/导出/清空） |
| `/review` | 挖空复习（你自己的句子 + 两键评分，10min/1/3/7/14/30 天间隔） |
| `/settings` | 偏好 6 项、自定义模型、服务状态 |
| `/sdk/demo.html` | 嵌入 SDK 演示页（悬浮球/划词气泡/点词查词，一行 script 接入任意站点） |

## 验证

```bash
make verify-all     # P0..P7 各期出口门禁依次执行（证据落 docs/acceptance/P*/evidence/）
make test           # go test ./...
cd web && npm run build   # vue-tsc + vite
```

验收清单：[docs/acceptance/FR-CHECKLIST.md](docs/acceptance/FR-CHECKLIST.md)（FR/NFR/AD 逐条有证据）。
交付报告与走查顺序：[REPORT.md](REPORT.md)；遗留问题：[KNOWN-ISSUES.md](KNOWN-ISSUES.md)。

## 目录

```
cmd/server            装配与入口
cmd/mockupstream      假上游（测试基座：可注入成功/失败/超时/超长/空响应）
internal/errs         错误码 ↔ HTTP ↔ 用户文案 唯一映射（附录 B）
internal/httpx        统一响应包 {ok, data|error}
internal/config       环境变量（附录 A）+ models.json
internal/gateway      OpenAI 兼容客户端、多 Key、模型池与降级链、缓存、提示词
internal/service      业务编排（分段校验/配额/计费/子串校验/再用标记）
internal/store        GORM 模型与仓储（七表，无正文表）
internal/httpapi      路由、中间件、handler
internal/dict         ECDICT 精简词典（词频前 3000，embed）
internal/ratelimit    令牌桶（访客桶 + IP 桶）
web/src               Vue 3 前端（views/components/stores/api/lib/styles）
web/sdk               嵌入 SDK（零依赖，Shadow DOM）
deploy                Dockerfile / compose / LiteLLM 配置 / 部署说明
docs/11-rewrite-spec.md  唯一实现依据；docs/acceptance  各期验收
```
