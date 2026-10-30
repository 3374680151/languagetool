# P5 词表 + 收藏闭环 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p5.sh`（mockupstream + 被测服务 + Playwright）
- 出口标准（规格 §十）：FR-702~704、1201、1202 全过

## 0. 一条命令复现

```bash
./scripts/verify-p5.sh
# 手工走查：写作台划选生词 → 收藏（带原句）→ 词表页查看/删除/排序 → 搜索框查词收藏
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 验收项 | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- | --- |
| 1 | FR-1201/AD-6 | 写作台查词 → 点收藏 | 卡片变"✓ 已收藏"；词表出现该词且原句=所在句 | PASS（e2e + 截图 words-collected.png） |
| 2 | AD-6 幂等 | 重复收藏同一词 | 不报错、不重复建行（接口 created=true→false，同 id） | PASS（接口 words-create1/2.json + e2e） |
| 3 | FR-702 | 看词表行 | 词 + 原句上下文 + 使用次数 + 复习到期时间 | PASS（e2e） |
| 4 | FR-1202 | 写一段含收藏词的新文字并翻译 | "用过 N 次"+1；输入与翻译过程无任何弹窗 | PASS（e2e 断言 0→1 且 popup count=0；截图 words-reused.png） |
| 5 | FR-1202 词形还原 | 新段落写 recalled（原形 recall 已收藏） | 同样命中（与查词卡同一词形还原实现，AD-7） | PASS（接口 words-after-reuse.json：一次请求 +1） |
| 6 | FR-1202 排序 | 切"最少使用" | 0 次的排前面 | PASS（e2e） |
| 7 | FR-703 | 换访客身份查词表 | 词表为空（匿名隔离属预期）；同访客刷新后仍在（服务端存储） | PASS（接口 words-other-visitor.json + e2e） |
| 8 | FR-704 | 空词表 | 说明 + 下一步动作（查词引导），不空白 | PASS（截图 words-empty.png） |
| 9 | 删除 | 词表删除 | 二次确认后删除 | PASS（e2e） |
| 10 | AD-10 | 词表页搜索框查词 | 固定位置弹卡（不受触发模式影响）；无原句收藏 → 复习时退化为词→释义卡 | PASS（e2e + 截图 words-search-card.png） |
| 11 | 门禁 | go vet/test/build、前端构建、e2e | 全绿 | PASS（evidence/） |

## 2. 自主决策记录

- "再使用"口径：翻译请求结算时，对本批非空段源文分词 + 词形还原（与查词卡同一 Lemma 实现），
  命中收藏词则每词每请求 +1（一段重复出现不重复计）。
- 词表排序：最近收藏（默认）/最少使用；收藏即进入复习调度（next_due_at=+10 分钟，P5.5 消费）。

## 3. 遗留问题

无新增。

## 4. 证据目录

- `docs/acceptance/P5/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`
- `words-create1.json / words-create2.json / words-after-reuse.json / words-other-visitor.json`
- `words-empty.png / words-collected.png / words-reused.png / words-search-card.png`
