# 前端设计

Vue 3.5 + TypeScript + Vite + Pinia + vue-router。**不引 UI 框架。**

---

## 一、路由

| 路径 | 视图 | 说明 |
| --- | --- | --- |
| `/write` | `WriteView` | 默认页，写作台 |
| `/learn` | `LearnView` | 词表 |
| `/settings` | `SettingsView` | 语言、朗读、自定义模型 |
| `/login` | `LoginView` | 仅后台用 |

没有 `/` 落地页——打开就是写作台。多一个落地页只是多一次点击。

---

## 二、写作台组件树

```
WriteView
├── TopBar
│   ├── LangPairPicker      语言对 + 互换
│   ├── StylePicker         学术 / 正式 / 简洁 / 口语
│   ├── ModelPicker         来自 config.models，空则隐藏
│   ├── GlossaryBadge       "术语 3 条"，点击展开
│   └── Actions             翻译全部 / 纠错 / 复制 / 导出
├── SplitPane
│   ├── SourcePane
│   │   └── SegmentList
│   │       └── SegmentInput × N        textarea，自动增高
│   └── TargetPane
│       └── SegmentList
│           ├── SegmentOutput × N       可编辑
│           └── IssueList               该段的纠错条目
├── SelectionBubble          Teleport 到 body，跟选区走
└── VocabDrawer              右侧抽屉，词表
```

---

## 三、状态

### doc store

```ts
type SegmentStatus = 'idle' | 'pending' | 'done' | 'error'

interface Segment {
  id: string
  source: string
  target: string
  status: SegmentStatus
  edited: boolean          // 用户手改过译文
  issues: Issue[]
  error?: string
}

interface DocState {
  segments: Segment[]
  translating: boolean
  proofreading: boolean
  style: string
  model: string
  from: string
  to: string
}
```

**段落状态机**

```
idle ──翻译──► pending ──成功──► done
                   └──失败──► error ──重试──► pending
done ──用户编辑──► done(edited=true)
done ──重译──► 若 edited 则弹确认 ──► pending
```

`edited=true` 的段落在重译前必须确认，不能静默覆盖——那是用户的手工成果。

### 其他 store

| store | 内容 | 持久化 |
| --- | --- | --- |
| `config` | `/api/config` 结果，启动拉一次 | 内存 |
| `prefs` | 语言对、风格、朗读语速、主题 | localStorage |
| `vocab` | 词表列表与分页 | 服务端 |
| `ui` | 抽屉开关、toast 队列、当前选中段 | 内存 |

---

## 四、关键交互

| 交互 | 规则 |
| --- | --- |
| `Ctrl/Cmd + Enter` | 翻译全部未译段落 |
| `Ctrl/Cmd + Shift + Enter` | 只译光标所在段 |
| 输入停顿 | **不自动翻译**——写作时自动翻会打断思路 |
| 选中文字 | 400ms 后弹气泡；`Esc` 或点空白关闭 |
| 气泡定位 | 贴选区，**不遮挡被选文字**；空间不够时翻到上方 |
| 译文编辑 | 直接改，`edited=true`，不弹任何确认 |
| 纠错 | 只在点按钮时跑，逐条「接受 / 忽略」 |
| 接受纠错 | 用 `indexOf` 定位替换，替换后重新校验剩余条目是否仍匹配 |

**为什么不引富文本编辑器**：段落级翻译需要精确控制每个段落的 DOM 边界，
富文本编辑器的内部模型（ProseMirror / Slate）会和"按段落替换"打架。
用 `textarea` + 段落数组，行为可预测，代价是丢掉行内格式——写博客不需要行内格式。

---

## 五、六态（每个数据区域都要有）

| 态 | 表现 |
| --- | --- |
| 加载 | 骨架屏，不用无限转圈 |
| 空 | 说明 + 一个可点的动作（"载入示例段落"） |
| 失败 | 原因 + 重试按钮 + 备选（"已换备用线路"） |
| 超时 | 已等待秒数 + 取消按钮 |
| 限额 | 剩余额度 + 恢复时间 + 降级说明 |
| 未配置 | 缺什么 + 谁能配（"本站未开启纠错，联系站长"） |

缺一个态视为该页未完成。

---

## 六、设计 token

沿用旧站视觉：中性灰底 + 白卡片 + 细边框，青绿只作点缀。

```css
:root {
  --bg: #F3F4F6;
  --surface: #FFFFFF;
  --surface-2: #F7F8F9;
  --ink: #1B1F23;
  --ink-2: #4C545B;
  --ink-3: #747B83;
  --border: #E4E6E9;
  --primary: #0D9488;
  --primary-soft: #E4F3F1;
  --danger: #D13438;
  --warn: #955102;
  --radius: 10px;
  --font: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  --mono: ui-monospace, "SF Mono", Menlo, monospace;
}
[data-theme="dark"] {
  --bg: #131518;  --surface: #1B1E21;  --surface-2: #17191C;
  --ink: #E8EAED; --ink-2: #AAB0B7;    --ink-3: #7F868E;
  --border: #2B2F33; --primary: #2CC7BA;
}
```

主题：浅色 / 深色 / 跟随系统。**深色下必须逐页检查对比度**，不能只改背景。

---

## 七、API 客户端

```ts
// api/client.ts
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'content-type': 'application/json', ...init?.headers },
    credentials: 'same-origin',
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(body?.error?.code ?? 'UNKNOWN', body?.error?.message, body?.error?.hint)
  }
  return res.json() as Promise<T>
}
```

- 统一 `ApiError`，带 `hint`。
- 组件里不允许直接写 `fetch`。
- DTO 类型手写在 `api/types.ts`，与 `docs/02-api.md` 一一对应，改接口先改这里。

---

## 八、体积预算

| 项 | 预算 |
| --- | --- |
| 首屏 JS（gzip） | < 120KB |
| 首屏 CSS（gzip） | < 20KB |
| 首屏可交互 | < 1.5s（本地） |

超预算先砍依赖，不砍功能。这也是不引 UI 框架的原因之一。
