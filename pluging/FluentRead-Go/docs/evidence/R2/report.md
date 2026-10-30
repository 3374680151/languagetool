# R2 修复轮报告

- 日期：2026-10-30
- 范围：R0 留下的两条 P2（R0-07 额度噪音、R0-08 译文对应弱）
- 原则：不写新 bash 脚本，代码改动直接落地

## R0-07 顶栏额度降噪

**问题**：顶栏状态行一直显示 `今日剩余 1000/1000 段`（或 `今日已用 N 段`）。
写作者打开页面就看到这个数字，但它对写作没有任何帮助——额度是站长该关心的事。

**改法**：`TopBar.vue` 的 `quotaText` 计算逻辑改成：

```ts
if (q.unlimited) return ''
const threshold = Math.max(10, Math.round(q.dailyLimit * 0.1))
if (q.remaining > threshold) return ''
return `今日剩余 ${q.remaining}/${q.dailyLimit} 段`
```

- 无限额度 → 完全不显示。
- 有限额度 → 只在剩余 <= max(10, 日上限 10%) 时显示。
- 低额度时仍保留 warn 色（`.topbar__quota--low`）和恢复时间提示。

**结果**：额度充足时顶栏干净只剩"X/Y 段已译"和线路信息，低额度时才预警。

## R0-08 悬停高亮对应行

**问题**：每段译文只有一个 6px 小圆点表示状态（绿=已译、蓝=翻译中、红=失败）。
滚动到长文中间后，左右两栏的段号虽然对齐了（R1-01），但视觉上仍然无法
一眼看出"这段译文属于哪段原文"——得靠鼠标来回点才能确认。

**改法**：`SplitPane.vue` 增加一行 CSS：

```css
.split__scroll :deep(.split-row:hover > :nth-child(1)),
.split__scroll :deep(.split-row:hover > :nth-child(2)) {
  background: var(--surface-2);
  border-radius: 4px;
  transition: background 0.1s ease;
}
```

`.split-row` 是 `display: contents` 的包装，它本身不产生盒子，但 `:hover`
在现代浏览器里仍能捕获鼠标悬停在其后代上的事件。左右两个网格单元格
（`:nth-child(1)` 和 `:nth-child(2)`）同时变深，0.1s 过渡，鼠标划到哪行
哪行就亮起来。

**结果**：长文滚动时，鼠标悬停即可确认对应关系，不需要再数段号。

## R2 出口

本轮没有新 P0/P1。R1 已清零，R2 也未新增问题。
连续两轮 P0=0 且 P1=0 ✅，但按 `docs/07-verification.md` 至少还要跑 R3–R5。

R3 待查：
- 失败路径（网关卡住时作者知不知道发生了什么）
- 长文（20+ 段，滚动性能、段号、悬停高亮是否还跟得住）
- 移动端窄屏（顶栏控件溢出）
