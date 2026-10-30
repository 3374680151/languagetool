# P1.5 护栏 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p1.5.sh`（5 个实例：A 正常 / B 配额=3 / C 站点字符封顶=30+免费后备 / D 限流=5 / E 额度=5，独立数据库）
- 出口标准（规格 §十）：被拒请求一次都没打上游；FR-601~606 逐条过

## 0. 一条命令复现

```bash
./scripts/verify-p1.5.sh     # 全部 FR 逐条断言 + 双 e2e，证据落 docs/acceptance/P1.5/evidence/
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 操作（实例） | 预期 | 实测结果 |
| --- | --- | --- | --- |
| 1 | 翻译 5 段（3 段空白）（A） | 空白段不计费：`/api/quota` used=2 | PASS（fr604-translate.json；FR-604） |
| 2 | 站点付费字符上限=30，送 65 字段（C） | 不拒绝：响应 degraded:true，译文可用，来自免费方式 | PASS（fr603-degraded.json + 截图 p15-02；FR-603/306） |
| 3 | 同上，看上游计数（C） | 付费调用 0 次、免费调用 1 次 | PASS（__stats `chat:0,free:1`；FR-604 免费不计字符） |
| 4 | 用满 3 段配额后继续译（B） | 第 4 次 429 QUOTA_EXCEEDED | PASS（FR-601） |
| 5 | 重启实例 B（同一数据库）再译 | 仍 429——配额落库不归零 | PASS（FR-601） |
| 6 | 看 429 响应（B） | Retry-After 头 + 文案"今日额度已用完（已译 3 段），约 X 小时后恢复"，无内部词 | PASS（fr605-headers.txt / fr605-body.json；FR-605） |
| 7 | 限流=5，连发 12 次不同文本（D） | 放行 5 / 拒绝 7；上游 chat 调用恰为 5——被拒请求一次没打上游 | PASS（FR-602） |
| 8 | 额度充足时打开写作台（A，200 段） | 界面无任何额度文字（FR-606 不常驻） | PASS（e2e EXPECT_QUOTA=ok） |
| 9 | 额度=5 用掉 2 段后（E） | 出现横幅"今日还剩 3 段可翻译，约 20 小时后（明日 0 点）恢复" | PASS（e2e EXPECT_QUOTA=low + 截图 p15-01；FR-606） |
| 10 | 429 文案用词通读 | 不出现 上游/线路/配额/跳/回退/provider | PASS（脚本内 python 断言） |
| 11 | go vet / go test / 构建 / e2e | 全绿 | PASS（evidence/） |

## 2. 六态覆盖进度

- 限额：429 文案（#6）+ 低额横幅（#9）+ 用尽横幅（banner-quota-out，额度=0 时同款展示）
- 降级不是错误：degraded:true + 横幅（#2），段落照常出译文
- 其余态：见 P1 走查单；未配置态与超时态在 P2 出口逐态截图

## 3. 遗留问题

无新增。

## 4. 证据目录

- `docs/acceptance/P1.5/evidence/gates.txt`、`go-*.txt`、`build-*.txt`
- `fr604-translate.json / fr603-degraded.json / fr605-headers.txt / fr605-body.json`
- `p15-01-quota-low-banner.png / p15-02-degraded-banner.png`
- `e2e-ok.txt / e2e-low.txt / e2e-ok/ / e2e-low/`
