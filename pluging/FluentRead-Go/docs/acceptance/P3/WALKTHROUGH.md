# P3 纠错 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p3.sh`（mockupstream + 被测服务 + Playwright）
- 出口标准（规格 §十）：3 处错误出 3 条建议，逐条接受后正确；FR-201~209 全过

## 0. 一条命令复现

```bash
./scripts/verify-p3.sh
# 手工走查：写作台写一段含错误的英文 → 点段落下方"纠错" → 逐条看建议 → 接受/忽略
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- |
| 1 | 写入含 3 处错误的段落，点"纠错"（FR-201/202） | 拼写/语法/用词错各出一条建议 | PASS（e2e"3 处错误出 3 条建议"；截图 proofread-3-suggestions.png） |
| 2 | 看每条建议的字段（FR-203） | 原文片段→改法→类别→教学理由，四字段齐全 | PASS（e2e 逐字段断言） |
| 3 | 逐条接受 3 条（出口标准） | 每接受一条，左栏原文立即改对；recieves→receives、a RAG→an RAG、make a conclusion→draw a conclusion | PASS（e2e 最终原文逐字断言；截图 proofread-all-accepted.png） |
| 4 | 接受第 1 条后看剩余建议（FR-205） | 第 2/3 条仍正确定位可接受 | PASS（e2e 重定位断言） |
| 5 | 点"忽略"（FR-205） | 该条移除，原文不动 | PASS |
| 6 | 检查完全正确的段落（FR-209） | 明确"这段没有发现问题"，不是空白 | PASS（截图 proofread-clean.png） |
| 7 | 中文段落（FR-206） | 没有纠错入口 | PASS（服务端也返回空建议：proofread-cjk.json） |
| 8 | 输入过程中（FR-207） | 不弹任何建议 | PASS（e2e 逐字输入断言面板未开） |
| 9 | 找"一键改写全文"（FR-208） | 不存在 | PASS（e2e 文案断言） |
| 10 | 模型返回不存在的片段（FR-204） | 服务端丢弃且不报错 | PASS（接口级 proofread-api.json：xyzzy 片段被滤除） |
| 11 | go vet / go test / 构建 / e2e | 全绿 | PASS（evidence/） |

## 2. 设计说明

- 纠错建议的"接受"直接修改左栏原文；被接受的条目从列表移除，剩余条目按子串校验重排。
- 纠错不进入访客翻译配额（FR-604 只管翻译段），站点用量另记 proofread_req。
- 纠错提示词要求输出 JSON 数组、禁止偏移量；服务端以"original 必须是子串"兜底（FR-204）。

## 3. 遗留问题

无新增。

## 4. 证据目录

- `docs/acceptance/P3/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`
- `proofread-api.json / proofread-cjk.json`
- `proofread-3-suggestions.png / proofread-all-accepted.png / proofread-clean.png`
