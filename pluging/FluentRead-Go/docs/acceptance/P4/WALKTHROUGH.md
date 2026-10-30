# P4 词典 + 术语 + 词卡 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p4.sh`（mockupstream + 被测服务，词典限频=3 便于演示）
- 出口标准（规格 §十）：查词可用；§八 AD-1~12 全过（AD-10 随 P5 词表页验证）；FR-501~505、701 全过

## 0. 一条命令复现

```bash
./scripts/verify-p4.sh
# 手工走查：写作台写一段英文 → 双击/划选单词看词卡 → 收藏 → 术语按钮增删改 → 重译
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 验收项 | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- | --- |
| 1 | AD-1 | 选词弹卡，Esc/点空白/滚动/开始输入 | 均能关闭 | PASS（e2e 逐项断言） |
| 2 | AD-2 | 连续单击 | 零误触（单击永不弹） | PASS（e2e 连击 5 次断言） |
| 3 | AD-3 | 划选 2-4 词 / ≥5 词 | 短语卡可查（≤4 词）；≥5 词不弹 | PASS（服务端限制 4 词 + 前端词数判断；短语卡随生僻词路径） |
| 4 | AD-4 | 查 recall（高频）/ gerrymander（生僻） | 词典态音标/词性/多条释义/标签 <100ms；生僻词"AI 生成，仅供参考" | PASS（接口 30ms；截图 p4-01/p4-02） |
| 5 | AD-5 | 查 recieves（拼错） | "可能是 receive 的拼写错误" | PASS（e2e word-typo 断言） |
| 6 | AD-6 | 点收藏 → 重复收藏 | 收藏幂等、卡片变已收藏、词表可见 | 部分 PASS：幂等与已收藏态在 P5 词表落地后全链验证（/api/words 属 P5 交付）；本期的收藏按钮已接通 P5 端点 |
| 7 | AD-7 | 查 recalled | 显示"查询的是 recalled—— 原形 recall"；与 FR-1202 同一实现 | PASS（接口 dict-recalled.json + 截图 p4-03） |
| 8 | AD-8 | 400px + 深色 | 卡片不溢出、对比度达标 | PASS（截图 p4-04-dark-card.png；token 体系与 NFR-9 校准一致） |
| 9 | AD-9 | 连续查 4 个生僻词；查词前后看配额 | 查词不计翻译配额；第 4 个生僻词（限频 3）返回 429+可读提示 | PASS（接口断言：配额不变 + 429） |
| 10 | AD-10 | 词表页搜索框 | 弹卡 | 推迟至 P5（词表页本期尚未交付，验收随 P5 走查单） |
| 11 | AD-11 | 设置页切三种触发模式 + 刷新 | 划选即弹（默认，替换式选中不闪卡）/浮标确认（只出浮标，点浮标才弹）/修饰键（Alt+划选）；刷新保留 | PASS（e2e 全流程断言） |
| 12 | AD-12 | 自动朗读开关 | 开：弹卡即读；关：仅朗读按钮发声 | PASS（e2e 以 speechSynthesis 记录器断言） |
| 13 | FR-501 | 术语增删改 | 列表可见 | PASS（e2e + 截图 p4-05） |
| 14 | FR-502 | 加术语 → 重译 | 术语真的进提示词（mock 在译文尾部附系统提示词指纹，前后指纹不同） | PASS（e2e 指纹对比） |
| 15 | FR-503 | 改术语不重译 | "术语表改过了，当前译文还是旧的"横幅 + 一键重译后消失 | PASS（e2e） |
| 16 | FR-504 | 有手改段时一键重译 | 先弹确认 | PASS（e2e） |
| 17 | FR-505 | 站点与用户各定义同一术语 | 站点 > 用户 | PASS（接口 glossary-merged.json） |
| 18 | FR-701 | 弹卡整体 | 位置锚定、两来源、收藏入口、限频 | PASS（截图 + e2e） |
| 19 | 门禁 | go vet/test/build、前端构建、e2e | 全绿 | PASS（evidence/） |

## 2. 自主决策记录

- ECDICT 精简库由 `scripts/gen-dict.py` 从上游 ECDICT CSV（skywind3000/ECDICT，词频前 3000）生成，
  生成物 `internal/dict/data/ecdict-slim.json`（591KB）随二进制 embed，可复现。
- 词典模型兜底的限频走独立桶，`FR_DICT_RATE_PER_MINUTE` 可调（默认 10，§八 8.5）。
- 术语"站点级"在无账号体系下对所有访客可编辑（规格 FR-501 的字面语义）。
- 词形还原优先用 ECDICT exchange 反查，缺省规则后缀还原（-ed/-ing/-s/-ies）；查词与 FR-1202 共用。

## 3. 遗留问题

无新增。AD-6 的"词表出现该词"与 AD-10 的"词表页搜索框"随 P5 词表页交付后全链验证。

## 4. 证据目录

- `docs/acceptance/P4/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`
- `dict-recall.json / dict-recalled.json / dict-gerrymander.json / dict-ratelimit.txt / glossary-merged.json`
- `p4-01-ecdict-card.png / p4-02-model-card.png / p4-03-lemma-collected.png / p4-04-dark-card.png / p4-05-glossary-dialog.png`
