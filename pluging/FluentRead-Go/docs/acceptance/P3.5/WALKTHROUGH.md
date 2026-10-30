# P3.5 模型选择 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p3.5.sh`（mockupstream + 被测服务，巡检间隔 2s 便于演示剔除/恢复）
- 出口标准（规格 §十）：三层模型都能用；FR-401~409 全过

## 0. 一条命令复现

```bash
./scripts/verify-p3.5.sh
# 手工走查：写作台顶栏"模型"下拉 → 站点池/自定义；设置页"自定义模型"表单 → 测试连接
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- |
| 1 | 全新访客打开写作台直接翻译（FR-401） | 默认模型开箱即用，0 配置 | PASS（p1 起各用例持续覆盖；G-2） |
| 2 | 打开顶栏模型下拉（FR-402） | enabled 模型出现并带 质量·速度·成本 标签 | PASS（e2e"下拉含默认/站点池/自定义"） |
| 3 | 检查下拉里有没有 my-custom（FR-409） | 禁用的占位模板不出现；启用会被启动校验拒绝 | PASS（e2e + fr409-reject.log） |
| 4 | 设置页填自定义端点+密钥+模型名 → 测试连接（FR-403） | "连接成功（Nms）" | PASS（e2e 走真端点） |
| 5 | 保存后到写作台选"自定义模型…" → 翻译 | 译文来自自定义端点（mock 带 [my-own-model] 前缀） | PASS（e2e） |
| 6 | 自定义端点指向死端口 → 测试连接 | 明确"连接失败：…" | PASS（e2e） |
| 7 | 自定义模型翻译失败（FR-405） | 段落标记"自定义模型不可用，已用默认模型"，译文来自默认模型 | PASS（e2e custom-fallback-tag） |
| 8 | 抓 /api/models/test 响应与响应头（FR-404） | 不回显密钥；no-store；服务端日志无密钥 | PASS（model-test.json + headers + 日志 grep） |
| 9 | 刷新页面后再翻译（FR-404） | 密钥已清空（只在会话内存），需重填才能再用自定义 | PASS（e2e FR-403 用例的刷新路径） |
| 10 | mock 隐藏 qwen-plus（模拟拔掉上游），等 2 轮巡检（FR-408/1103） | /api/models 不再出现该模型；health degraded；WARN 日志 | PASS（models-removed.json + server.log） |
| 11 | 恢复上游，等 1 轮巡检 | 模型自动回到下拉，degraded 解除 | PASS（models-recovered.json） |
| 12 | /api/config 的模型清单（FR-402/408） | 与下拉一致，剔除的不下发 | PASS（初始清单断言） |
| 13 | go vet / go test / 构建 / e2e | 全绿 | PASS（evidence/） |

## 2. 自主决策记录（记入 REPORT）

- 巡检间隔新增测试钩子 `FR_INSPECT_INTERVAL_SECONDS`（默认 3600，规格只要求"每小时"）。
- 自定义模型选择记在本地偏好（model:"custom"）；baseURL/model 落 localStorage，**apiKey 只存会话内存**（FR-404 不落盘的从严解释）。
- 段落级回落提示用持久标签（比 toast 更符合"提示要说清"）。

## 3. 遗留问题

无新增。

## 4. 证据目录

- `docs/acceptance/P3.5/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`
- `fr409-reject.log / models-initial.json / models-removed.json / models-recovered.json`
- `model-test.json / model-test-headers.txt`
