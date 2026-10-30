# REPORT · FluentRead 写作台交付报告

> 日期：2026-10-31 · 执行者：AI（隔夜 goal 模式）
> 唯一实现依据：docs/11-rewrite-spec.md（2026-10-31 定稿版）
> 状态：**P0–P7 全部分期完成，出口门禁全绿，等待用户走查**（用户走查从 docs/acceptance/P0/WALKTHROUGH.md 开始）。

---

## 1. 交付了什么

一句话：**左英右中的回译写作台 + 纠错核心 + 学习闭环（查词收藏→再用→挖空复习→错误本）+ 读者侧 SDK**，
Go 1.25 + Gin + GORM（glebarez/sqlite，纯 Go 无 CGO）+ Vue 3 + Element Plus，单二进制 + 单 SQLite 文件。

| 模块 | 内容 |
| --- | --- |
| 后端 | `cmd/server`（装配）、`internal/{errs,httpx,config,gateway,service,store,httpapi,ratelimit}`，Gin 路由 + GORM AutoMigrate（§7.5 七表） |
| 翻译链路 | /api/translate：限流→分段校验→配额预扣→并发(5)→降级链（站点线路→MyMemory 免费后备，DR-10）→结果校验→结算计费；内存 LRU 缓存（FR-1101） |
| 模型选择 | models.json（primary/pool，占位模板启动校验 FR-409）+ 访客自定义端点转发（FR-403/404）+ 巡检对账剔除（FR-1103） |
| 纠错 | /api/proofread：教学解释建议、子串校验（FR-204）、类别枚举、接受/忽略/重定位 |
| 学习闭环 | /api/words（收藏带原句、幂等、排序）、/api/mistakes（接受写入/筛选/删除/清空）、/api/review（挖空自己的句子、两键评分、10min/1/3/7/14/30 天间隔、没记住回退）、/api/assets（导出/清空） |
| 词典 | ECDICT 精简库（词频前 3000，591KB embed）+ 词形还原（与再用标记同一实现，AD-7）+ 模型兜底（独立限频） |
| 术语 | site/user 双作用域（站点>用户）、进提示词、指纹过期横幅 + 一键重译 |
| 前端 | Vue 3 + Element Plus（唯一 UI 库）+ Pinia + tokens.css 设计 token（深浅色，NFR-9 对比度校准）；写作台/词表/复习/设置四页；查词弹卡（§八全量）；六态齐备 |
| SDK | 零依赖 vanilla JS（gzip 5.3KB，FR-905 预算 30KB）：悬浮球/快捷面板/划词气泡/点词查词卡/收藏；Shadow DOM 隔离；Origin 白名单（FR-906） |
| 部署 | deploy/Dockerfile（多阶段，CGO_ENABLED=0）+ compose（app 单容器 + litellm 可选 profile，DR-9）+ README |

## 2. FR 覆盖情况

- **全量清单见 docs/acceptance/FR-CHECKLIST.md**：FR-1xx~12xx（50 条）、NFR-1~14、§八 AD-1~12，逐条有验收标准/验证方法/证据/结论，无一条"无证据的已完成"。
- 每期出口标准（规格 §十 最后一列）逐条过：各期 `docs/acceptance/P*/WALKTHROUGH.md` 以"操作 → 预期 → 实测"格式给出走查单，门禁记录在同目录 evidence/。
- 门禁（NFR-14）：`go vet ./...`、`go test ./...`（8 包全绿）、`CGO_ENABLED=0 go build`、前端构建（vue-tsc + vite）、mock 上游驱动的 Playwright e2e、health 三态、mock 调用计数——每期 `scripts/verify-pN.sh` 一键复现。

## 3. 怎么一条命令启动

```bash
# 本地（不用 Docker）
make build                                     # CGO_ENABLED=0 → bin/fluentread
FR_BASE_URL=http://127.0.0.1:4000 FR_API_KEY=sk-xxx ./bin/fluentread   # 默认 :8787
# 或 Docker
docker compose -f deploy/docker-compose.yml up -d --build              # 单容器
docker compose -f deploy/docker-compose.yml --profile litellm up -d    # 可选：加网关
```

未配置模型时站点处于"未配置"态：写作、草稿、导出照常可用（六态之一，FR-1003/1004）。
测试链路可用 `bin/mockupstream`（假 OpenAI 兼容上游，可注入成功/失败/超时/超长/空响应/回显/按密钥 401/模型隐藏）。

## 4. 用户走查顺序（§十一 第 3 步）

1. **P0** docs/acceptance/P0/WALKTHROUGH.md —— 骨架与健康三态
2. **P1** P1/WALKTHROUGH.md —— 写作台两栏、分段、快捷键、失败隔离
3. **P1.5** —— 配额/限流/降级/Retry-After
4. **P2** —— 六态逐态、导出、深色、400px、偏好持久化
5. **P3** —— 纠错面板逐条接受/忽略
6. **P3.5** —— 模型三层与巡检
7. **P4** —— 查词弹卡（AD-1~12）与术语闭环
8. **P5** —— 词表与收藏（带原句、再用 +1）
9. **P5.5** —— 错误本与挖空复习
10. **P6** —— SDK 演示页（/sdk/demo.html）
11. **P7** —— 部署形态

每期走查单都是可复现的（脚本一键搭环境 + 逐步操作/预期/实测）。

## 5. 自主决策记录（规格未覆盖处，按默认方案与惯例自行决定）

1. **复习评分调度**：`记住`进一档、`没记住`回退一档（最低回第一档）；间隔序列默认 10min/1/3/7/14/30 天；测试钩子 `FR_REVIEW_STAGES`（分钟序列）可覆盖。
2. **巡检间隔**：默认每小时（FR-1103），测试钩子 `FR_INSPECT_INTERVAL_SECONDS`。
3. **词典兜底限频**：独立于翻译限流的桶，`FR_DICT_RATE_PER_MINUTE`（默认 10，§八 8.5 的"可调"落地）。
4. **纠错计费**：不进访客翻译配额（FR-604 只管翻译段），站点用量记 proofread_req，不计付费字符（字符封顶针对翻译体量）。
5. **自定义模型密钥**：只存会话内存（Pinia），baseURL/model 落 localStorage——对 FR-404"不落盘"的从严解释；服务端同样不落盘不写日志。
6. **同源 SDK 调用**：不带 Origin 的请求在 SDK 开启时放行（演示页同源部署场景），白名单约束跨站嵌入；SDK 关闭时同源也 403。
7. **术语"站点级"**在无账号体系下对所有访客可编辑（FR-501 字面语义）。
8. **错误本入口**挂在词表页"错误本" tab（规格页面清单无独立 /mistakes 路由，不新增导航项）。
9. **模型下拉默认项**用哨兵值 `default`（el-select 空串显示 placeholder 的问题）。
10. **P4 期收藏**依赖 P5 的 /api/words：P4 交付弹卡与收藏入口，AD-6 的词表闭环在 P5 全链验证。
11. **mockupstream** 目标语言感知（假译文与目标语言一致，否则 §7.6 语种校验会正确拒绝）+ 段级故障标记 `[[fail]]/[[timeout]]/[[empty]]/[[echo]]` + 按密钥注入状态码 + `/v1/models` 模型隐藏——为逐项验收提供确定性故障注入。
12. **旧方案清理**：chi 路由、手写 SQL（sqlc 时代）、自建 UI 组件、旧 verify 脚本与旧 env 命名全部移除（规格 DR-2/0.1 的重写要求）。

## 6. 已知限制

- **P7-01（环境限制，非缺陷）**：本沙箱访问不了 Docker Hub/镜像源，容器级构建与运行验证标记 SKIPPED（复现步骤与证据见 KNOWN-ISSUES.md 与 docs/acceptance/P7/evidence/docker-build.txt）。交付物本身完整并通过 `docker compose config` 校验；"单容器独立运行 + health 自检"的实质由本地等价自检证明（同一二进制 + 假网关，health 全 ok + 端到端翻译冒烟通过）。在能访问 registry 的机器上按 deploy/README.md 复现即可补跑。
- 复习间隔的 e2e 观察依赖 `FR_REVIEW_STAGES` 钩子缩短首档（生产不设即用规格默认）。

## 7. 等待用户走查

- 从 **docs/acceptance/P0/WALKTHROUGH.md** 开始逐期执行 §十一 的第 3–6 步（走查 → 提意见 → 修复 → 收块）。
- 每期意见记入 docs/acceptance/<期号>/ISSUES.md，修复后回到第 3 步复验，该块 P0=P1=0 才收块。
- 全量门禁随时可重跑：`make verify-all`。
