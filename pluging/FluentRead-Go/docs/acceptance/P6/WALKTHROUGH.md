# P6 SDK · 用户走查单

- 日期：2026-10-31
- 执行人：AI（隔夜模式第 1、2 步；第 3 步用户走查待天亮后进行）
- 环境：`./scripts/verify-p6.sh`（主实例白名单含自身 + SDK 关闭实例；演示页 /sdk/demo.html 模拟第三方博客）
- 出口标准（规格 §十）：一行接入演示站可用；FR-901~906 全过

## 0. 一条命令复现

```bash
./scripts/verify-p6.sh
# 手工走查：浏览器打开 http://127.0.0.1:8787/sdk/demo.html
#   右下角悬浮球 → 点开快捷面板 → 输入英文 → 翻译
#   选中正文英文 → 译文气泡；双击生词 → 释义卡 → 收藏 → 回 /words 看同一个词表
```

## 1. 走查步骤（操作 → 预期 → 实测）

| # | 验收项 | 操作 | 预期 | 实测结果 |
| --- | --- | --- | --- | --- |
| 1 | FR-901 | 打开演示页 → 悬浮球 → 面板翻译 | 不装任何东西、不填 key，直接能翻 | PASS（e2e + 截图 sdk-panel.png） |
| 2 | FR-902 | 选中正文英文 | 译文气泡 | PASS（截图 sdk-bubble.png；演示页译文为 mock 确定性输出） |
| 3 | FR-902 | 双击生词 | 释义卡（词典态/在线态）+ 收藏按钮 | PASS（截图 sdk-word-card.png） |
| 4 | FR-902×1201 | 卡片上点收藏 → 回主站 /words | 收藏进**同一个**词表（同一访客身份） | PASS（e2e 断言主站词表出现该词） |
| 5 | FR-903 | SDK 翻译后查 /api/quota | 与主站同一配额（计入访客用量） | PASS（接口级 quota.used ≥ 1） |
| 6 | FR-904 | 宿主页 Georgia 样式 vs SDK | Shadow DOM 隔离：悬浮球/卡片样式不受宿主影响 | PASS（e2e computedStyle 断言） |
| 7 | FR-905 | SDK 体积 | ≤30KB gzip | PASS（实测 gzip 5309B ≈ 5.2KB，原始 15KB） |
| 8 | FR-906 | 白名单外来源 POST /api/sdk/* | 403 ORIGIN_FORBIDDEN + "本站点未开通翻译服务，请联系站长添加域名" | PASS（接口 sdk-forbidden.json + e2e） |
| 9 | FR-906 | 白名单为空启动 | SDK 关闭：同源调用也 403 | PASS（独立实例） |
| 10 | FR-906 | 白名单内来源 | 放行 + CORS 凭据头 | PASS（接口级） |
| 11 | 接入方式 | 查看 demo 页源码 | 一行 `<script src="/sdk/fluentread-sdk.js" data-fr-...>` | PASS（web/sdk/demo.html） |
| 12 | 门禁 | go vet/test/build、前端构建、e2e | 全绿 | PASS（evidence/） |

## 2. 自主决策记录

- SDK 为零依赖手写 vanilla JS（web/sdk/fluentread-sdk.js，约 15KB 原始 / 5.3KB gzip），由 go:embed 下发，不做前端构建——比打包链路更可控，体积余量大（FR-905）。
- 同源部署（演示页）的 /api/sdk/* 请求不带 Origin 头，在 SDK 开启时放行；白名单约束的是"嵌入到别的站点"这一场景。跨站请求必须带 Origin 且在白名单内（FR-906）。
- SDK 关闭（FR_SDK_ORIGINS 空）时，同源调用也 403（关闭语义）；演示页与脚本仍可下发但功能不可用，脚本给出可读提示。
- 配置沿用 data-fr-server-url / data-fr-from / data-fr-to / data-fr-theme 与 window.FluentReadConfig（§7.9）；serverUrl 缺省取脚本自身来源（同站部署零配置）。

## 3. 遗留问题

无新增。（跨站第三方 cookie 的浏览器限制按 §7.9 的既定口径：白名单域内走同站部署或子域共享 cookie。）

## 4. 证据目录

- `docs/acceptance/P6/evidence/gates.txt`、`go-*.txt`、`build-*.txt`、`e2e.txt`
- `sdk-size.txt / sdk-js-headers.txt / config-sdk-on.json / sdk-forbidden.json / fluentread-sdk.js`
- `sdk-panel.png / sdk-bubble.png / sdk-word-card.png`
