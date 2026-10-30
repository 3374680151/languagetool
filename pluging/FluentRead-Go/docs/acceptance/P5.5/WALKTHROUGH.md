# P5.5 学习闭环 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p5.5.sh`（复习首档间隔经 FR_REVIEW_STAGES 缩短为约 2 秒以便 e2e 观察）
- 出口标准（规格 §十）：FR-1203~1205 全过

## 0. 一条命令复现

```bash
./scripts/verify-p5.5.sh
# 手工走查：纠错 → 接受建议 → 错误本回看 → 收藏词 10 分钟后到 /review 翻卡
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 验收项 | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- | --- |
| 1 | FR-1203 | 纠错 3 条：忽略 1 条 + 接受 2 条 | 错误本出现 2 条；被忽略的不出现 | PASS（e2e 断言 2 行且无 recieves） |
| 2 | FR-1203 | 错误本四要素 | 原片段/改法/类别/理由齐全 | PASS（e2e + 截图 mistakes-list.png） |
| 3 | FR-1203 | 类别筛选 | 只看该类别 | PASS（接口 + e2e） |
| 4 | FR-1203 | 单条删除 | 二次确认后删除 | PASS（e2e） |
| 5 | FR-1204 | 收藏词到期 → 词表页入口 | 到期数量徽标显示 1（不弹窗、不常驻顶栏） | PASS（截图 review-entry-badge.png） |
| 6 | FR-1204 | 复习翻卡 | 挖空**你自己的句子**（The judge will ____ ...），提示=释义 | PASS（截图 review-card-revealed.png） |
| 7 | FR-1204 | 两键评分·没记住 | 回退一档，很快再次到期 | PASS（e2e + 接口 review-score-forgot.json） |
| 8 | FR-1204 | 两键评分·记住 | 进下一档，队列清空 | PASS（e2e + 接口；截图 review-remembered.png） |
| 9 | FR-1204 | 错误本到期卡 | 挖空正确改法（原片段→____），理由可看 | PASS（接口级 + e2e 材料） |
| 10 | FR-1205 | 导出 JSON | 含词表与错误本条目 | PASS（接口 assets-export.json + e2e 下载内容断言） |
| 11 | FR-1205 | 清空 | 两次确认后词表与错误本为空 | PASS（e2e + 接口 mistakes-cleared.json） |
| 12 | 六态 | 复习页 | 加载骨架/空（无到期）/失败可重试 | PASS（截图 review-empty.png；e2e） |
| 13 | 门禁 | go vet/test/build、前端构建、e2e | 全绿 | PASS（evidence/） |

## 2. 自主决策记录

- 复习间隔序列默认 10min/1/3/7/14/30 天（FR-1204）；新增测试钩子 `FR_REVIEW_STAGES`（分钟序列）供 e2e 观察"到期/回退"，生产不设该变量即用默认。
- "没记住"回退 = 阶段索引 -1（最低回到第一档）；"记住" = +1。
- 错误本入口挂在词表页的"错误本" tab（规格页面清单只有 /words /review /settings，不再加导航项）。
- 挖空实现：服务端按 token 匹配收藏词或其词形变体（与查词卡同一 Lemma），替换为 ____。

## 3. 遗留问题

无新增。

## 4. 证据目录

- `docs/acceptance/P5.5/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`、`e2e-review.txt`
- `mistakes-all.json / mistakes-filtered.json / review-due.json / review-score-*.json / assets-export.json / mistakes-cleared.json`
- `mistakes-list.png / review-entry-badge.png / review-card-revealed.png / review-remembered.png / review-empty.png / mistakes-cleared.png`
