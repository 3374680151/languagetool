# FR-CHECKLIST —— 规格（docs/11-rewrite-spec.md）全量验收清单

> 每条：验收标准（规格原文摘要）、验证方法、证据（命令输出摘要/接口响应/截图路径）、结论。
> 证据目录：`docs/acceptance/P*/evidence/`；一键复现：`make verify-all`（P7 的容器级验证受沙箱网络限制，见 KNOWN-ISSUES P7-01）。
> 结论图例：✅ 有证据通过 · 🔶 通过但带注记 · ⏭ 随后置期验证（均已闭环）。

## FR-1 写作台布局与输入

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-101 | 首开左英右中，默认英→中，无需手动选 | 打开 `/`，读工具栏默认值 | settings 默认 langPair=en/zh-Hans（internal/service/lang.go DefaultFrom/DefaultTo；e2e p1 全译走默认对） | ✅ |
| FR-102 | 任一行左右顶端偏差 ≤2px | e2e 逐行量 getBoundingClientRect | p1 e2e"逐行对齐"全断言（0px）；docs/acceptance/P1/evidence/e2e.txt | ✅ |
| FR-103 | 悬停行两格同亮、段号连续 | hover 断言 + 截图 | p1-02-hover-highlight.png | ✅ |
| FR-104 | 粘贴 4 段+3 空行 → 4 框不跳号 | e2e 粘贴 | p1 e2e"粘贴 4 段+空行" | ✅ |
| FR-105 | 粘贴空行不开空框；手动 Enter×2 开新段 | paste 事件 vs input 事件双路径 | SplitPane onPaste/onSourceInput + p1 e2e | ✅ |
| FR-106 | Ctrl/⌘+Enter 全译；+Shift 译光标段 | e2e 快捷键 | p1 e2e 两条用例 | ✅ |
| FR-107 | 每段独立状态 待译/翻译中/完成/失败 | e2e 失败隔离 | p1 e2e"1 段失败不影响其他段" | ✅ |
| FR-108 | 右栏直接可编辑无弹窗 | e2e fill 译文 | p2 e2e 手改用例 | ✅ |
| FR-109 | 手改后重译先确认 | e2e 弹确认框 | p2 retranslate-confirm.png | ✅ |
| FR-110 | 刷新不丢稿 | e2e 刷新断言 | p1 e2e"刷新不丢稿" | ✅ |
| FR-111 | 空文档说明 + 载入示例 | e2e 空态 | p2 state-empty.png | ✅ |
| FR-112 | 语言对可改，默认永远英→中 | 设置页改 + 服务端 prefs | p2 e2e 偏好持久化；GET /api/user/prefs 默认值 | ✅ |

## FR-2 纠错（核心）

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-201 | 拼写/用词/单复数被指出 | e2e 3 错 3 建议 | p3 proofread-3-suggestions.png | ✅ |
| FR-202 | 语法级错误被指出 | 同上（grammar 类别） | 同上 | ✅ |
| FR-203 | original→改法→类别→理由（教学解释）四字段 | e2e 逐字段断言 | p3 e2e | ✅ |
| FR-204 | original 非子串丢弃 | 接口级断言 | P3 proofread-api.json（xyzzy 片段被滤除） | ✅ |
| FR-205 | 逐条接受/忽略，接受后重定位 | e2e 逐条接受后原文正确 | p3 proofread-all-accepted.png | ✅ |
| FR-206 | 只检查英文段，中文段无入口 | e2e + 接口 | p3 proofread-cjk.json | ✅ |
| FR-207 | 不自动跑，手动触发 | e2e 输入过程断言面板未开 | p3 e2e | ✅ |
| FR-208 | 不做整段重写 | 界面无"一键改写" | p3 e2e 文案断言 | ✅ |
| FR-209 | 无错时明确说"没有发现问题" | e2e 空结果态 | p3 proofread-clean.png | ✅ |

## FR-3 翻译/回译

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-301 | 默认英→中；语言集；非法码 400 | 接口级 | translate-3seg.json（400 族单测 service/translate_test.go） | ✅ |
| FR-302 | 风格枚举；拼错回落默认 + WARN | 单测 NormalizeStyle | internal/service/translate_test.go（日志留痕） | ✅ |
| FR-303 | 段落级隔离 | 接口 + e2e | translate-3seg.json（ok/failed/ok） | ✅ |
| FR-304 | 空白段 skipped 不计配额 | 接口 | P1.5 fr604-translate.json（5 段含 3 空 → used=2） | ✅ |
| FR-305 | 超长/超段数 400 + 拆分提示，无重试按钮 | 单测 + e2e（超长段无重试按钮） | service 单测 + SplitPane noRetry | ✅ |
| FR-306 | 触顶降级不拒绝，带 degraded | 接口 | P1.5 fr603-degraded.json | ✅ |
| FR-307 | 文案不暴露内部结构 | 断言无内部词 | P1 translate-3seg.json + P1.5 文案断言 | ✅ |
| FR-308 | 失败原因可行动 | 429 文案含剩余与恢复时间 | P1.5 fr605-body.json | ✅ |

## FR-4 模型选择

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-401 | 默认模型开箱即用 | e2e 新访客直接翻译 | p1 全译 + p3.5 下拉默认项 | ✅ |
| FR-402 | models.json 下拉带标签 | e2e 下拉内容 | P3.5 e2e + models-initial.json | ✅ |
| FR-403 | 自定义模型可测试连接并翻译 | e2e | P3.5 e2e（测试连接 + [my-own-model] 译文） | ✅ |
| FR-404 | 密钥经服务端转发、no-store、不落日志 | 接口 + 日志 grep | P3.5 model-test.json / headers / server.log 无密钥 | ✅ |
| FR-405 | 自定义失败回落默认并提示 | e2e | P3.5 custom-fallback-tag | ✅ |
| FR-406 | 任意 OpenAI 兼容端点；非 OpenAI 协议经 LiteLLM | 架构保证（唯一 OpenAI 兼容协议，DR-7/9） | internal/gateway/client.go；litellm.config.yaml | ✅ |
| FR-407 | 模型名不写死业务代码 | 代码评审 | models.json 驱动（config.Catalog）；加模型零代码 | ✅ |
| FR-408 | 两份清单一致，不一致报警 | 巡检对账 | P3.5 models-removed.json + WARN 日志 + health degraded | ✅ |
| FR-409 | 占位模型默认禁用，启用被拒 | 启动校验 | P3.5 fr409-reject.log + 单测 config_test.go | ✅ |

## FR-5 术语

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-501 | 增删改术语 | e2e | P4 glossary dialog + 截图 | ✅ |
| FR-502 | 术语真的进提示词 | mock 提示词指纹对比 | P4 e2e（fp1≠fp2） | ✅ |
| FR-503 | 改术语提示"译文是旧的"+一键重译 | e2e 横幅 | P4 banner-glossary-stale | ✅ |
| FR-504 | 一键重译遇手改先确认 | e2e | P4 e2e（确认框） | ✅ |
| FR-505 | 冲突按 站点>用户 消解 | 接口 | P4 glossary-merged.json | ✅ |

## FR-6 配额与限流

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-601 | 日配额落库，重启不归零 | 重启实例复测 | P1.5（429→重启→仍 429） | ✅ |
| FR-602 | 访客+IP 双桶；被拒不打上游 | mock 调用计数 | P1.5（放行5/拒绝7/chat=5） | ✅ |
| FR-603 | 站点日字符触顶降级不拒绝 | 接口 | P1.5 fr603-degraded.json | ✅ |
| FR-604 | 按段计费；空白不计；免费不计上游字符 | 接口 | P1.5 fr604（used=2）+ 触顶后 chat=0 | ✅ |
| FR-605 | 429 带 Retry-After 头 + 可读时间 | 接口头+体 | P1.5 fr605-headers.txt/body.json | ✅ |
| FR-606 | 快用完才提示，不常驻顶栏 | e2e 双态 | P1.5 e2e-ok/low + p15-01 截图 | ✅ |

## FR-7 词典与词表

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-701 | 弹卡按 §八 执行 | AD-1~12（下表） | P4 evidence | ✅ |
| FR-702 | 收藏带原句 | e2e | P5 words-collected.png | ✅ |
| FR-703 | 随访客存服务端 | 接口 + e2e | P5 words-other-visitor.json | ✅ |
| FR-704 | 空态说明 + 动作 | e2e | P5 words-empty.png | ✅ |

## FR-8 导出

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-801 | 三种 Markdown | e2e 下载内容断言 | P2 e2e 导出用例 | ✅ |
| FR-802 | 段落顺序与空行保留 | e2e 内容断言 | 同上 | ✅ |

## FR-9 读者侧 / SDK

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-901 | 读者直接读或一键翻译，首段快 | 演示页 e2e | P6 sdk-panel.png | ✅ |
| FR-902 | 一行接入 SDK：悬浮球/面板/划词气泡/点词查词/收藏 | e2e | P6 三张截图 + e2e | ✅ |
| FR-903 | 同一套额度与模型 | 接口 | P6 quota.used 计入 SDK 调用 | ✅ |
| FR-904 | Shadow DOM 隔离 | e2e computedStyle | P6 e2e 样式断言 | ✅ |
| FR-905 | ≤30KB gzip | 构建产物实测 | P6 sdk-size.txt（gzip 5.3KB） | ✅ |
| FR-906 | 白名单控制，未授权 403 | 接口三态 | P6 sdk-forbidden.json + 关闭实例 | ✅ |

## FR-10 设置与运维

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-1001 | 设置页 6 项偏好，刷新保留 | e2e + 服务端 prefs | P2/P4 e2e | ✅ |
| FR-1002 | 密钥只在环境变量 | /api/config 抓包搜密钥 | P2 config-a/b.json | ✅ |
| FR-1003 | health 三态 + 设置页状态区块 | 接口 + e2e | P0 health-*.json + TestHealthDBFail + SettingsView | ✅ |
| FR-1004 | 未做的功能不写"已开启" | features 如实上报 | P2 config 断言（translate:false） | ✅ |

## FR-11 系统与数据

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-1101 | 内存 LRU 缓存（5000/32MiB/24h），命中不调上游不计配额，可关 | 单测 + mock 计数 | gateway_test.go + P1 冒烟（chat 计数不变） | ✅ |
| FR-1102 | 多 Key 轮换冷却 | httptest 注入 401 | gateway_test.go TestClientMultiKeyRotation | ✅ |
| FR-1103 | 巡检对账，2 次失败剔除+degraded+WARN，恢复回来 | 接口 | P3.5 models-removed/recovered.json + WARN | ✅ |
| FR-1104 | 匿名 cookie 60 天滑动 | e2e + 单测 | P0 cookie 用例 + middleware 单测 | ✅ |
| FR-1105 | 词表/偏好随访客存服务端 | e2e | P2 偏好 + P5 词表隔离 | ✅ |

## FR-12 写作学习闭环

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| FR-1201 | 查词收藏带所在句 | e2e | P5 words-collected.png | ✅ |
| FR-1202 | 再使用 +1（词形还原），不打断；排序 | 接口 + e2e | P5 words-after-reuse.json + words-reused.png | ✅ |
| FR-1203 | 接受入错误本、忽略不入、筛选、删除 | e2e + 接口 | P5.5 mistakes-list.png | ✅ |
| FR-1204 | 挖空自己的句子、两键评分、间隔调度回退、到期入口徽标 | 接口 + e2e | P5.5 review-card-revealed.png + review-score-*.json | ✅ |
| FR-1205 | JSON 导出、确认后清空 | 接口 + e2e | P5.5 assets-export.json + mistakes-cleared.png | ✅ |

## NFR 非功能

| 编号 | 验收标准 | 验证方法 | 证据 | 结论 |
| --- | --- | --- | --- | --- |
| NFR-1 | 首段回译 ≤3s（假网关 ≤1s） | e2e 计时 | P1 计时用例（实测 <1s） | ✅ |
| NFR-2 | 25 段 ≤10s 不卡死 | 浏览器实测 | P1 走查（147ms）+ p1-03 截图 | ✅ |
| NFR-3 | 只用 Element Plus 一套 + CSS token | package.json 依赖审查 | web/package.json（唯一 UI 库 element-plus） | ✅ |
| NFR-4 | 单二进制（web/dist embed）；不依赖 LiteLLM | 构建 + 本地自检 | P7 build + health-local.json | ✅ |
| NFR-5 | 纯 Go 驱动无 CGO | CGO_ENABLED=0 构建 | 各期 build-go.txt + go.mod 无 mattn | ✅ |
| NFR-6 | 隐私红线：正文/草稿不落库 | 单测扫列 + 红线自查 | store_test.go TestPrivacyNoContentColumns | ✅ |
| NFR-7 | 日志不记正文 | handler/service 日志审查 | 结构化日志仅 长度/耗时/码/模型 | ✅ |
| NFR-8 | 六态齐全 | 逐态截图 | P2 state-*.png（6 态）+ P5.5 复习页 | ✅ |
| NFR-9 | 深色对比度 ≥4.5:1 | token 校准 + 截图 | tokens.css 注释（正文≥7:1）+ dark 截图 | ✅ |
| NFR-10 | 400px 可用 | e2e + 截图 | P2 dark-400px.png | ✅ |
| NFR-11 | 文案说人话（无内部词） | 断言 + 通读 | P1/P1.5 文案断言 | ✅ |
| NFR-12 | 统一响应包与错误码唯一出口 | 单测 + 代码审查 | httpx_test.go + errs_test.go | ✅ |
| NFR-13 | 结构化日志（请求+上游两层） | 日志抽查 | server.log（JSON） | ✅ |
| NFR-14 | 验证门禁全绿 | make verify-all | docs/acceptance/P*/evidence/gates.txt | ✅ |

## §八 查词弹卡 AD

| 编号 | 验收项 | 证据 | 结论 |
| --- | --- | --- | --- |
| AD-1 | 弹卡锚定与四种关闭 | P4 e2e | ✅ |
| AD-2 | 单击零误触 | P4 e2e 连击 5 次 | ✅ |
| AD-3 | 2-4 词短语卡；≥5 词不弹 | 服务端 4 词限制 + 前端词数判断；短语走模型兜底路径 | 🔶 短语卡走在线态（ECDICT 无短语），规格 8.3 允许 |
| AD-4 | 高频词词典态 <100ms；生僻词 AI 标注 | 接口 30ms + p4-01/02 截图 | ✅ |
| AD-5 | 拼写错误提示 | P4 e2e word-typo | ✅ |
| AD-6 | 收藏态/幂等/原句 | P5 e2e（词表出现、幂等、原句一致） | ✅ |
| AD-7 | 词形还原与 FR-1202 同一实现 | dict-recalled.json + words-after-reuse.json（同一 dict.Lemma） | ✅ |
| AD-8 | 400px 不溢出 + 深色对比度 | p4-04-dark-card.png | ✅ |
| AD-9 | 查词不计配额；限频 10/min 可读提示 | P4 dict-ratelimit.txt | ✅ |
| AD-10 | 词表页搜索框弹卡；空 context 收藏退化 | P5 words-search-card.png | ✅ |
| AD-11 | 三种触发模式可切换刷新保留 | P4 e2e 全流程 | ✅ |
| AD-12 | 自动朗读开关 | P4 e2e（speechSynthesis 记录器） | ✅ |

## §7.4 API 端点覆盖

GET /api/health · GET /api/config · POST /api/translate · POST /api/proofread · GET /api/quota ·
GET/POST/PUT/DELETE /api/glossary · POST /api/models/test · GET /api/dict/:word ·
GET/POST/DELETE /api/words · GET/POST/DELETE /api/mistakes · GET /api/review · POST /api/review/score ·
GET/PUT /api/user/prefs · GET /api/assets/export · POST /api/assets/clear ·
GET /sdk/fluentread-sdk.js · POST /api/sdk/translate · GET/POST /api/sdk/word —— 全部实现并有 e2e/接口证据。

## 交付物

| 交付物 | 位置 | 结论 |
| --- | --- | --- |
| 单二进制（内嵌 web/dist + SDK） | bin/fluentread（CGO_ENABLED=0，默认 :8787） | ✅ |
| Dockerfile + compose（litellm 可选 profile）+ 部署说明 | deploy/ | ✅（容器级运行验证受沙箱网络限制，见 KNOWN-ISSUES P7-01） |
| FR-CHECKLIST（本文件） | docs/acceptance/FR-CHECKLIST.md | ✅ |
| 每期 WALKTHROUGH + evidence | docs/acceptance/P0..P7/ | ✅ |
| KNOWN-ISSUES | KNOWN-ISSUES.md（仅 1 条环境限制） | ✅ |
| REPORT | REPORT.md（仓库根） | ✅ |
