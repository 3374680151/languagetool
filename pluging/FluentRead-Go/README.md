# FluentRead Go

> 用 Go + Vue 重写的双语写作与翻译网站。给博客作者自己用，也给博客读者划词用。
> **不安装任何东西，访客零配置。**

---

## 一句话目标

在自己博客上写中英文文章时，能随手翻译和纠错；读者打开博客就自带划词翻译，不用装插件、不用注册、不用填 key。

---

## 技术栈

| 层 | 选型 | 为什么 |
| --- | --- | --- |
| 后端 | **Go 1.25** + `chi` 路由 | 单二进制部署，无运行时依赖；chi 兼容标准库 `http.Handler`，不绑架框架 |
| 数据访问 | **sqlc** + SQLite（`modernc.org/sqlite`，纯 Go 无 CGO） | SQL 可见可审，生成类型安全代码；无 CGO 才能交叉编译 |
| 前端 | **Vue 3.5 + TypeScript + Vite** | 组合式 API，类型完整 |
| 状态 | **Pinia** | 官方方案，够用 |
| 路由 | **vue-router 4** | 3 个页面，无需更复杂的东西 |
| UI | **自建组件 + CSS 变量** | 只有 3 个页面，引 UI 框架反而更重、样式还会打架 |
| 嵌入 SDK | **原生 TS + Vite lib 模式** | 单文件 IIFE，Shadow DOM 隔离，预算 < 30KB gzip |
| 模型网关 | **LiteLLM**（独立容器） | 加一家模型只改配置，不改代码 |
| 部署 | **docker-compose**（web + litellm + db） | 一条命令起 |

---

## 目录结构

```
FluentRead-Go/
├── cmd/
│   └── server/main.go          程序入口：装配依赖、启动 HTTP
├── internal/
│   ├── config/                 环境变量解析与校验
│   ├── httpapi/                handler、路由、中间件
│   │   ├── middleware/         鉴权、限流、请求日志、恢复
│   │   └── dto/                请求与响应结构
│   ├── translate/              翻译编排：provider 注册、换线、并发
│   │   └── provider/           各翻译服务实现（llm / microsoft / mymemory）
│   ├── ai/                     LiteLLM 网关客户端（chat、models、流式）
│   ├── proofread/              纠错：prompt 构造、JSON 解析、片段定位
│   ├── dict/                   词典：ECDICT 查询 + 兜底
│   ├── glossary/               术语：三层作用域与冲突消解
│   ├── vocab/                  词表：收藏、搜索、导入导出
│   ├── store/                  sqlc 生成代码 + 迁移
│   │   ├── migrations/         编号 SQL 迁移
│   │   └── query/              手写 SQL（sqlc 输入）
│   └── errs/                   统一错误类型与错误码
├── web/                        Vue 前端
│   ├── src/
│   │   ├── views/              Write / Learn / Settings / Login
│   │   ├── components/         段落编辑、纠错列表、词卡、词表抽屉
│   │   ├── stores/             config / prefs / doc / vocab / ui
│   │   ├── api/                fetch 封装与 DTO 类型
│   │   └── styles/             设计 token
│   └── index.html
├── sdk/                        嵌入 SDK 源码（构建产物进 web 静态目录）
├── deploy/
│   ├── docker-compose.yml
│   ├── Dockerfile
│   ├── litellm.config.yaml
│   └── models.json             模型展示信息（质量/速度/成本/是否开放）
├── docs/                       本目录：设计文档
└── scripts/                    词典导入、冒烟测试
```

---

## 文档索引

| 文档 | 内容 |
| --- | --- |
| [01-architecture.md](docs/01-architecture.md) | 分层、依赖方向、关键接口、错误与日志约定 |
| [02-api.md](docs/02-api.md) | 全部接口契约与错误码 |
| [03-data-model.md](docs/03-data-model.md) | 数据表、索引、迁移策略 |
| [04-frontend.md](docs/04-frontend.md) | 路由、组件树、状态机、交互规则、设计 token |
| [05-sdk.md](docs/05-sdk.md) | 嵌入 SDK 的接入方式与交互规范 |
| [06-plan.md](docs/06-plan.md) | 阶段划分、任务清单、每个阶段的出口标准 |
| [07-verification.md](docs/07-verification.md) | 逐功能验证与体验迭代流程（R0–R5） |
| [08-handoff-prompt.md](docs/08-handoff-prompt.md) | 交接提示词 v1（P1.5 → P2 时代，任务编号已过时，仅环境坑仍有效） |
| [09-handoff-v2.md](docs/09-handoff-v2.md) | 交接文档 v2：目标、设计思路、现状、给执行 agent 的提示词、验收清单 |
| [10-requirements.md](docs/10-requirements.md) | **需求规格**：逐条 FR + 验收标准，可直接交付开发照着做完整个项目 |

---

## 快速开始

```bash
cp .env.example .env          # 填模型 key
docker compose -f deploy/docker-compose.yml up -d
open http://localhost:8080
```

开发模式：

```bash
go run ./cmd/server           # 后端 :8080
pnpm -C web dev               # 前端 :5173，代理到 8080
```

本机验证（注意绕过系统代理，否则 curl 会打到代理上）：

```bash
go build -o bin/fluentread ./cmd/server
FR_ADDR=127.0.0.1:18080 FR_DB_PATH=./data/fluentread.db ./bin/fluentread &
curl -s --noproxy '*' http://127.0.0.1:18080/api/health
```

---

## 依赖版本约束

`modernc.org/sqlite` 与 `golang.org/x/sys` 的新版本要求 Go ≥ 1.26，
会自动触发工具链升级，导致 `deploy/Dockerfile` 里的 `golang:1.25-alpine` 构建失败。

因此**以下版本已固定，不要随手 `go get -u`**：

| 模块 | 固定版本 | 原因 |
| --- | --- | --- |
| `modernc.org/sqlite` | v1.34.5 | 更高版本要求 Go 1.26 |
| `modernc.org/libc` | v1.55.3 | 跟随 sqlite |
| `golang.org/x/sys` | v0.30.0 | v0.49.0 要求 Go 1.26 |

`go.mod` 的 `go` 指令固定为 `1.25.0`。要升级 Go 版本时，三处一起改：`go.mod`、`Dockerfile`、本节表格。


---

## 与旧项目的关系

旧项目 `FluentRead`（浏览器扩展，WXT + Vue）**不参与本次构建**，只作为参考：

- **复用思路**：划词卡的交互细节、术语冲突处理、段落级翻译的切分策略。
- **不复用代码**：扩展的 content script、offscreen、消息中枢全部不需要。
- **不移植 40 家 provider 适配器**：改用 LiteLLM 网关统一接入，只在 Go 侧实现 3 个兜底免费线路。

---

## 边界

不做：PDF/DOCX 导入、复习调度、背单词、学习统计、多引擎对比、多租户、圈选翻译、视频字幕。

判断标准：**新功能若不能加快"30 分钟写完一篇双语博客"，就不做。**
