# 嵌入 SDK 设计

> 给博客读者用：选中就译、点词就查。**不装扩展、不注册、不填 key。**

---

## 一、接入

```html
<script src="https://你的域名/sdk.js" data-fr-site="站点token" data-fr-to="zh-Hans" defer></script>
```

一行。`defer` 必须有，否则会阻塞首屏。

### 配置项

| 属性 | 默认 | 说明 |
| --- | --- | --- |
| `data-fr-site` | 必填 | 站点 token，服务端按它校验域名与限流 |
| `data-fr-to` | `zh-Hans` | 目标语言 |
| `data-fr-from` | `auto` | 源语言 |
| `data-fr-ball` | `true` | 是否显示悬浮球 |
| `data-fr-selection` | `true` | 是否开启划词 |
| `data-fr-theme` | `auto` | `light` / `dark` / `auto` |

也支持 `window.FluentReadConfig = { ... }`，脚本标签属性优先。

---

## 二、交互规范

| 场景 | 行为 |
| --- | --- |
| 选中单词 | 弹词卡：音标、词性、释义、收藏、朗读 |
| 选中短语/句子 | 弹译卡：译文、复制、朗读、收藏 |
| 弹出时机 | 鼠标松开后 400ms |
| 关闭 | `Esc` / 点击空白 / 再次选中 |
| 定位 | 贴选区，**不遮挡被选文字**；下方空间不足时翻到上方 |
| 连续选词 | 旧卡立即让位，不叠加 |
| 收藏成功 | 按钮变"已收藏"，**不弹 toast** 打断阅读 |
| 词典查不到 | 明确写"未收录，以下为机器翻译"，不伪装 |

---

## 三、Shadow DOM 结构

```html
<fluentread-root>            <!-- attachShadow({ mode: 'open' }) -->
  #shadow-root
    <style>…</style>          <!-- 全部样式内联，不污染宿主 -->
    <div class="fr-bubble">…</div>
    <button class="fr-fab">译</button>
</fluentread-root>
```

- 宿主页面的 CSS **不能**影响卡片，卡片的 CSS **不能**影响宿主——这是必须用 Shadow DOM 的唯一理由。
- 不用 `!important` 堆砌来对抗宿主样式。
- 挂载节点 id 固定 `fluentread-root`，重复加载时先检测、不重复挂。

---

## 四、请求

```ts
POST /api/sdk/translate
X-FR-Site: <token>
{ "text": "…", "from": "auto", "to": "zh-Hans" }

→ { "text": "…", "kind": "word", "entry": { … } }
```

**一次请求决定弹什么卡**：服务端判断 `kind`，是词就带 `entry`，是句子就只带 `text`。
不要前端先猜"这是不是一个词"——判错会导致弹错卡。

---

## 五、体积与性能

| 项 | 预算 |
| --- | --- |
| `sdk.js`（gzip） | < 30KB |
| 首屏影响 | 不阻塞渲染，`defer` 加载 |
| 内存 | 不常驻定时器；关闭卡片即释放 |
| 网络 | 用户不选中就**不发任何请求** |

**不许**在页面加载时预取词典或模型列表——博客读者可能根本不划词。

---

## 六、边界

| 不做 | 原因 |
| --- | --- |
| 整页翻译 | 那是另一个产品（translate.js 已经做得很好），且会破坏博客排版 |
| 修改宿主 DOM | 只追加自己的节点，绝不改博客内容 |
| 收集阅读行为 | 只记收藏与用量计数，不埋点、不追踪 |
| 弹窗索要任何配置 | 读者不该看到任何设置 |

---

## 七、加载失败时

- 脚本 404 → 页面完全不受影响（`defer` + 无全局副作用）。
- 接口失败 → 卡片显示"翻译服务暂时不可用"，不静默、不重试到卡死。
- token 无效 → 静默禁用，不向读者报错（这是站长配置问题，不该打扰读者）。
