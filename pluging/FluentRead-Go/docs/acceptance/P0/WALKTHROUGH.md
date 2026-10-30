# P0 骨架 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p0.sh` 一键起 mockupstream + 被测服务；`CGO_ENABLED=0` 单二进制
- 出口标准（规格 §十）：服务能起；health 三态正确；`CGO_ENABLED=0` 构建通过；页面能开不白屏

## 0. 一条命令复现

```bash
cd FluentRead-Go
./scripts/verify-p0.sh          # 全量门禁 + health 三态 + e2e 冒烟，证据落 docs/acceptance/P0/evidence/
# 手工起服务（供浏览器逐页走查）：
./bin/mockupstream --addr 127.0.0.1:18091 --models deploy/models.json &
FR_LISTEN=127.0.0.1:8787 FR_DB_PATH=data/fluentread.db \
FR_BASE_URL=http://127.0.0.1:18091 FR_API_KEY=testkey ./bin/fluentread
# 浏览器打开 http://127.0.0.1:8787/
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- |
| 1 | `CGO_ENABLED=0 go build` 并启动二进制 | 一条命令启动，默认 :8787，无 panic | PASS：`bin/fluentread` 启动即用；日志为结构化 JSON（NFR-13） |
| 2 | 打开 `http://127.0.0.1:8787/` | 写作台页面打开，不白屏，顶栏四入口 | PASS（截图 p0-01-write-light.png） |
| 3 | 依次点 顶栏"词表/复习/设置" | 三页都能打开，不白屏，空态有说明文案 | PASS（e2e 用例"四个页面都能打开且不白屏"） |
| 4 | 设置页切"深色"，再刷新 | 深色立即生效且刷新保留（html.dark + localStorage） | PASS（截图 p0-03-settings-dark.png；e2e 断言刷新后仍 dark） |
| 5 | 缩窄窗口到 400px | 顶栏不溢出、页面可用 | PASS（截图 p0-04-write-400px.png） |
| 6 | `curl /api/health`（已配假上游） | `{"ok":true,"db":"ok","gateway":"ok"}` | PASS（evidence/health-ok.json） |
| 7 | 启动一个不配 `FR_BASE_URL` 的实例，`curl /api/health` | `gateway:"fail"`，HTTP 仍 200 | PASS（evidence/health-gateway-fail.json） |
| 8 | 数据库故障态 | `db:"fail"` 可演示 | PASS：单测 `TestHealthDBFail`（关闭连接池后 db:"fail"，go-test.txt 有记录） |
| 9 | `curl /api/not-exists` | 统一响应包 JSON 400，不是 HTML | PASS（e2e 用例"未实现的接口返回统一响应包"） |
| 10 | 首次打开页面查 cookie | `fr_vid` = UUID v4，HttpOnly，60 天 | PASS（e2e 用例"访客 cookie 已下发"） |
| 11 | 未知前端路径 `/no/such/page` | 回落写作台，不 404 白屏 | PASS（e2e 用例"未知前端路由回落写作台"） |
| 12 | 静态资源缓存 | `assets/*` 带 hash 长缓存；index.html 不缓存 | PASS（curl -D- 实测 `Cache-Control: public, max-age=31536000, immutable`） |
| 13 | `go vet ./... && go test ./...` | 全绿 | PASS（evidence/go-vet.txt、go-test.txt：8 个包 ok） |
| 14 | `cd web && npm run build` | vue-tsc 类型检查 + vite 构建成功 | PASS（evidence/build-web.txt） |

## 2. 六态基线说明（P0 只要求"页面能开"）

P0 出口不含六态全覆盖（那是 P2 的出口）。但骨架已预留六态的组件基础：
加载（el-skeleton 可用）、空（el-empty 已用于各占位页）、失败/超时/限额/未配置
（api/client.ts 的 ApiError 已带 code/retryable/retryAfter，横幅组件 P1.5/P2 落地）。

## 3. 遗留问题

无（KNOWN-ISSUES.md P0 区为空）。

## 4. 证据目录

- `docs/acceptance/P0/evidence/gates.txt` —— 门禁逐项 PASS 记录
- `docs/acceptance/P0/evidence/go-vet.txt / go-test.txt / build-go.txt / build-web.txt / e2e.txt`
- `docs/acceptance/P0/evidence/health-ok.json / health-gateway-fail.json`
- `docs/acceptance/P0/evidence/p0-01..04.png` —— 页面截图（浅色/深色/400px）
- `docs/acceptance/P0/evidence/e2e/` —— e2e 失败附件（本轮无失败，目录为空属正常）
