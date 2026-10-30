# P2 写作台 · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p2.sh`（4 实例：A 正常+免费后备 / B 未配置 / C 限额=2 / D 无后备）
- 出口标准（规格 §十）：§九 体验走查①②⑤；六态逐态截图

## 0. 一条命令复现

```bash
./scripts/verify-p2.sh
# 手工起服务后浏览器走查：
#   写一段英文 → Ctrl/⌘+Enter 全译 → 点纠错入口（P3 前置灰）→ 刷新 → 换深色 → 缩 400px
```

## 1. 六态逐态证据（NFR-8）

| 态 | 触发方式 | 证据 | 界面表现 |
| --- | --- | --- | --- |
| 加载 | 翻译中 | e2e"加载态" | 段级 el-skeleton 骨架屏，不是转圈不是空白 |
| 空 | 首次进入（未输入） | state-empty.png | 说明 + "开始写 / 载入示例"两个可点动作（FR-111） |
| 失败 | [[fail]] 标记段（无后备实例 D） | state-failed.png | "翻译服务暂时不可用，请稍后重试" + 重试本段（FR-303/307） |
| 超时 | 假上游 20s 慢响应 | state-timeout.png | "已等待 N 秒" + 取消翻译按钮；取消后段落回待译 |
| 限额 | 配额=2 译 3 段（实例 C） | state-quota.png | 每段"今日额度已用完（已译 2 段），约 X 小时后恢复"，不报 500 |
| 未配置 | 不配模型启动（实例 B） | state-unconfigured.png | "站点还没有配置翻译模型"，写作照常可用，不白屏（§5.2） |

## 2. 走查步骤（操作 → 预期 → 实测）

| # | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- |
| 1 | §九①：打开写作台直接写一段英文（逐字输入，非粘贴） | 输入即分段，无配置操作 | PASS（e2e 手改确认用例含输入路径；G-1/G-2 达成） |
| 2 | §九②：全译后读中文是否对得上 | 每段出中文回译 | PASS（加载态/深色用例断言 4 段 done） |
| 3 | §九⑤：刷新 → 关掉重开 → 换深色 → 缩 400px | 草稿还在；深浅色都可用；顶栏不溢出 | PASS（p1 草稿用例 + dark-1280/dark-400px 截图） |
| 4 | 手改译文后单段重译 | 弹确认框"重译会覆盖修改"，可取消 | PASS（retranslate-confirm.png；FR-109） |
| 5 | 导出 仅英文 / 仅中文 / 中英对照 | 三种都下载；对照为英文段、中文段交替 | PASS（e2e 断言文件内容；FR-801/802） |
| 6 | 设置页改风格=口语 → 刷新 | 仍为口语（服务端 user_prefs） | PASS（e2e；FR-1001/1105） |
| 7 | 抓 /api/config 响应搜密钥 | 无任何密钥形状字符串 | PASS（config-a.json/config-b.json 断言；FR-1002） |
| 8 | 未配置站点的 config features.translate | 如实为 false（前端不假装可用） | PASS（FR-1004） |
| 9 | 未配置实例 /api/health | gateway:"fail"，HTTP 200 | PASS（health-b.json；FR-1003） |
| 10 | 设置页"服务状态"区块 | 模型/数据库状态如实展示，无密钥输入框 | PASS（SettingsView + e2e） |
| 11 | go vet / go test / 构建 / 全部 e2e | 全绿 | PASS（evidence/） |

## 3. 六态之外的验收补充

- NFR-10：400px 截图（dark-400px.png）顶栏折叠、单列堆叠。
- NFR-11：横幅与错误文案通读无"上游/线路/配额/跳/回退/provider"。
- NFR-13：日志为结构化 JSON（方法/路径/状态码/耗时/字节），无正文。

## 4. 遗留问题

无新增（KNOWN-ISSUES.md）。

## 5. 证据目录

- `docs/acceptance/P2/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`
- `config-a.json / config-b.json / health-b.json`
- `state-*.png`（六态）、`retranslate-confirm.png`、`dark-1280.png`、`dark-400px.png`
- `e2e-unconfigured.txt / e2e-quota.txt / e2e-failed.txt`
