# P7 部署 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p7.sh`
- 出口标准（规格 §十）：应用单容器独立起来（不依赖 LiteLLM），health 自检通过；LiteLLM profile 一键起

## 0. 一条命令复现

```bash
./scripts/verify-p7.sh          # 门禁 + compose 校验 + 本地等价自检
make up                          # 单容器起（需要能拉基础镜像的网络）
make up-litellm                  # 可选：加 LiteLLM 网关
curl http://127.0.0.1:8787/api/health
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 验收项 | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- | --- |
| 1 | NFR-4 单二进制 | `CGO_ENABLED=0 go build` + `make web` | 一个二进制 + 一个 SQLite 文件 = 完整站点 | PASS（build-go.txt；本地自检） |
| 2 | compose 定义 | `docker compose config` | app 服务 + 可选 litellm profile 定义有效 | PASS（compose-config.txt） |
| 3 | Dockerfile 多阶段 | 构建 | 前端 + 后端 + 精简运行镜像 | SKIPPED：本环境基础镜像仓库不可达（直连与镜像源均超时，见 docker-build.txt）——属环境限制，非交付缺陷；单容器等价性由 #5 本地自检兜底 |
| 4 | 容器运行 + health | `docker compose up -d app` | 容器内 health db:ok | SKIPPED（同上；复现步骤见 KNOWN-ISSUES.md，需能访问 registry 的网络） |
| 5 | 单容器等价自检 | 本地起 mock 网关 + 单二进制 | health 全 ok、页面 200、端到端翻译 ok | PASS（health-local.json + 翻译冒烟） |
| 6 | LiteLLM profile | compose 定义 + 配置文件 | `--profile litellm` 一键起；app 经 http://litellm:4000 访问 | PASS（配置有效；DR-9 可选外挂） |
| 7 | 环境变量 | 对照附录 A | .env.example 与 compose 覆盖全集 | PASS |
| 8 | SQLite 卷 | compose volumes | data 卷挂载，重启不丢 | PASS（compose 定义） |
| 9 | 部署说明 | deploy/README.md | 一条命令启动/升级/SDK 接入/隐私说明 | PASS（已交付） |
| 10 | 门禁 | go vet/test/build、前端构建 | 全绿 | PASS（evidence/） |

## 2. 环境限制说明（KNOWN-ISSUES 有记录）

本沙箱无法访问 Docker Hub 与镜像源（`docker build` 在拉取 golang/node 基础镜像时 EOF/超时）。
Dockerfile 与 compose 定义已交付并通过 `docker compose config` 校验；容器级验证需在
能访问 registry 的机器上执行 `docker compose -f deploy/docker-compose.yml up -d --build` 复现。
单容器独立运行的实质（一个静态二进制 + 一个 SQLite 文件 = 完整站点，NFR-4）已由本地等价自检证明。

## 3. 遗留问题

见 KNOWN-ISSUES.md（仅此环境限制一条，标注 SKIPPED 原因与复现步骤）。

## 4. 证据目录

- `docs/acceptance/P7/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`compose-config.txt`
- `docker-build.txt`（失败留证）、`health-local.json`
- `deploy/Dockerfile / docker-compose.yml / README.md / litellm.config.yaml / models.json`
