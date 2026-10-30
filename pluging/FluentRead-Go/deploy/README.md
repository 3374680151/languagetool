# 部署说明（P7）

## 0. 你会得到什么

一个静态二进制（前端已由 go:embed 打包）+ 一个 SQLite 文件 = 完整站点（NFR-4）。
LiteLLM 是**可选外挂**（DR-9）：不部署时 `FR_BASE_URL` 直指任意 OpenAI 兼容端点，行为完全一致。

## 1. 一条命令启动（单容器，不依赖 LiteLLM）

```bash
cp .env.example .env          # 至少改 FR_BASE_URL / FR_API_KEY / FR_MODEL
docker compose -f deploy/docker-compose.yml up -d --build
curl http://127.0.0.1:8787/api/health
# 期望：{"db":"ok","gateway":"ok","ok":true,...}（gateway 未配时为 "fail"，服务仍可用）
```

不用 Docker 也可以：

```bash
make web      # 构建前端到 web/dist
make build    # CGO_ENABLED=0 静态二进制 → bin/fluentread
FR_BASE_URL=http://127.0.0.1:4000 FR_API_KEY=sk-xxx ./bin/fluentread
```

## 2. 可选：一键加 LiteLLM 网关（DR-9）

```bash
# .env 里填 DEEPSEEK_API_KEY 等真实密钥
docker compose -f deploy/docker-compose.yml --profile litellm up -d --build
# app 容器经 http://litellm:4000 访问网关；站长侧的渠道/用量看板用 LiteLLM 自带 UI
```

加一家模型 = 改两处，不发版：`deploy/models.json` 加一条（下拉展示）、
`deploy/litellm.config.yaml` 加一条 model_list（实际可调用）。
巡检（FR-1103）每小时对账 `/v1/models`，两边不一致会 WARN + health degraded + 下拉剔除。

## 3. 数据与隐私

- SQLite 文件在 `data/fluentread.db`（compose 已挂卷，重启不丢）。
- 原文/译文与草稿一律不落库（只在浏览器 localStorage，NFR-6）；
  库里只有用量数字、术语、显式收藏的词与接受的纠错片段。

## 4. 嵌入 SDK（FR-902/906）

把别的站点接入：

```html
<script src="https://你的站点/sdk/fluentread-sdk.js"
        data-fr-server-url="https://你的站点" data-fr-from="en" data-fr-to="zh-Hans"></script>
```

并在 `.env` 里把对方域名加进 `FR_SDK_ORIGINS`（逗号分隔；空 = SDK 关闭）。
演示页：`/sdk/demo.html`（模拟第三方博客，可直接体验悬浮球/划词气泡/点词查词）。

## 5. 升级

```bash
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

SQLite 结构由 GORM AutoMigrate 启动时自动补齐。
