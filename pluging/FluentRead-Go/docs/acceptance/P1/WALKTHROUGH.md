# P1 翻译链路 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p1.sh`（mockupstream + 被测服务 + Playwright）
- 出口标准（规格 §十）：3 段中 1 段失败另 2 段成功；NFR-1 计时达标

## 0. 一条命令复现

```bash
./scripts/verify-p1.sh
# 手工起服务：
./bin/mockupstream --addr 127.0.0.1:18091 --models deploy/models.json &
FR_LISTEN=:8787 FR_DB_PATH=data/fluentread.db \
FR_BASE_URL=http://127.0.0.1:18091 FR_API_KEY=testkey FR_MODEL=deepseek-chat ./bin/fluentread
# 浏览器打开 http://127.0.0.1:8787/，粘贴英文，Ctrl/⌘+Enter 翻译
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- |
| 1 | 粘贴 4 段 + 3 个空行的整篇英文 | 自动切成 4 个输入框，段号 1-4 连续，空行不开空框 | PASS（e2e"粘贴 4 段+空行"；FR-104/105） |
| 2 | 手动在某段末尾按两次回车 | 开出新段，光标落进新段 | PASS（normalizeBlanks 语义；store 单元路径 + 手测） |
| 3 | Ctrl/⌘+Enter 全译 | 4 段全部出中文，右栏"完成"态，可点击直接改 | PASS（e2e 全译用例；截图 p1-01） |
| 4 | 一段文本带故障标记（模拟上游失败），共 3 段 | 该段标红"翻译服务暂时不可用，请稍后重试"+重试按钮；另 2 段正常出译文 | PASS（e2e 失败隔离 + 接口级断言 translate-3seg.json；FR-303/307） |
| 5 | 失败文案检查 | 不出现"上游/线路/配额"等内部词（NFR-11） | PASS（e2e 断言 + 接口断言） |
| 6 | 修改失败段内容后点"重试本段" | 只有该段重译并变"完成" | PASS（e2e 重试路径） |
| 7 | 首段回译计时（假网关基线 ≤1s） | 秒回 | PASS（e2e 计时 <1000ms；接口级 3 段含失败共 31ms） |
| 8 | 25 段长文全译（NFR-2 ≤10s） | 全部完成，页面不卡死 | PASS（浏览器实测 147ms；截图 p1-03） |
| 9 | 逐行量左右两格顶端坐标（FR-102 ≤2px） | 每行偏差 0px | PASS（e2e 逐行 getBoundingClientRect 断言） |
| 10 | 悬停某行 | 左右两格同时高亮（FR-103） | PASS（截图 p1-02；CSS hover 行联动） |
| 11 | 改译文 → F5 刷新 | 段落数、译文、手改标记全部还原 | PASS（e2e"刷新不丢稿"；FR-110） |
| 12 | 光标点进第 2 段，Ctrl/⌘+Shift+Enter | 只译该段，其余 3 段保持待译 | PASS（e2e 单段译；FR-106） |
| 13 | 深色模式 + 400px 窄屏 | 可读、顶栏不溢出、单列堆叠 | PASS（截图 p1-04/p1-05） |
| 14 | 相同段落第二次翻译 | 秒回且上游计数不变（缓存，FR-1101 提前接入） | PASS（__stats chat 计数不变，见 P0 期记录） |
| 15 | go vet / go test / CGO_ENABLED=0 build / 前端构建 | 全绿 | PASS（evidence/go-*.txt、build-*.txt） |

## 2. 六态覆盖进度

- 加载：段 pending 时 el-skeleton（e2e 全译过程覆盖）
- 空：空文档给说明 + "载入示例"（P2 出口逐态截图）
- 失败：段级红色错误块 + 重试（本表 #4）
- 超时/限额/未配置：P1.5/P2 出口（护栏与横幅组件接入后逐态截图）

## 3. 遗留问题

无新增（KNOWN-ISSUES.md P1 区为空）。

## 4. 证据目录

- `docs/acceptance/P1/evidence/gates.txt` —— 门禁逐项 PASS 记录
- `docs/acceptance/P1/evidence/go-vet.txt / go-test.txt / build-go.txt / build-web.txt / e2e.txt`
- `docs/acceptance/P1/evidence/translate-3seg.json` —— 接口级"1 失败 2 成功"响应
- `docs/acceptance/P1/evidence/p1-01..05.png` —— 两栏/失败态/悬停/25 段/深色/400px
- `docs/acceptance/P1/evidence/e2e/` —— e2e 失败附件（本轮无失败）
