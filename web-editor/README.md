# LanguageTool 网页校对编辑器

单文件网页应用（`index.html`），无需构建、无任何外部依赖，可整体拷贝到你自己的服务里。

## 本地体验

```sh
# 1. 启动 LanguageTool 服务（必须带 --allow-origin，浏览器跨域需要）
cd /Users/wang/tools/LanguageTool-6.6
/Users/wang/tools/java21/Contents/Home/bin/java -cp languagetool-server.jar \
  org.languagetool.server.HTTPServer --port 8081 --allow-origin

# 2. 起一个静态服务打开页面
cd web-editor
python3 -m http.server 8090
# 浏览器打开 http://localhost:8090
```

页面顶部会显示服务器连接状态；也可以直接在"高级设置"里填任意服务器地址。

## 功能对照

| 页面功能 | 对应 API 参数 |
|---|---|
| 编辑区右下角错误计数圆点（检查中脉冲、无错变绿 ✓） | 基于 `matches` 数量 |
| 圆点纠错浮层：逐个纠错、上一个/下一个、全部修正、忽略 | `matches[].replacements` + `disabledRules` |
| 点击正文下划线词直接在浮层定位该错误 | `matches[].offset/length` |
| 实时错误下划线 + 错误卡片 + 一键替换 | `POST /v2/check` 返回的 `matches[].replacements` |
| 语言下拉（自动从服务端拉取） | `GET /v2/languages` → `language` |
| 自动检测语言 | `language=auto`，结果显示 `language.detectedLanguage` |
| 检查级别（标准/严格/学术） | `level=default/picky/academic` |
| 母语提示 | `motherTongue` |
| 偏好变体 | `preferredVariants` |
| 仅启用指定规则 | `enabledOnly=true` + `enabledRules` |
| 忽略规则（本地持久化） | `disabledRules` |
| 忽略整个分类（本地持久化） | `disabledCategories` |
| 本地词典：加入词典的词不再报拼写错误（浏览器端持久化） | 客户端过滤；接后端可切换为 `/v2/words` 服务端词典（需配数据库） |
| 主题切换（跟随系统/浅色/深色，本地持久化） | — |
| 快捷键：Ctrl/⌘+Enter 检查、Esc 关浮层、←/→ 或 Alt+←/→ 浮层内切换 | — |
| 字符/词数/错误数/耗时统计 | — |
| 原始 JSON 查看 | 完整响应，便于对接你自己的解析逻辑 |

## 接入你自己的服务时需要改的地方

### 前端（index.html）
1. **服务器地址**：默认读"高级设置"里的地址，集成时直接把 `#serverUrl` 的默认值改成你的后端地址，或改成同域相对路径。
2. **同域反代（推荐）**：在你的 Nginx/网关上把 `/lt/` 反代到 LanguageTool 服务，前端就完全不存在 CORS 问题；此时启动服务**不要**带 `--allow-origin *`。

### 服务端（java 启动参数 / properties 配置）
| 项目 | 说明 |
|---|---|
| `--allow-origin https://你的域名` | 生产环境指定具体域名，**不要**用空参数（即 `*`） |
| `--public` / 默认 localhost | 部署到服务器必须加 `--public`；配合防火墙限制来源 |
| 鉴权 | 开源版仅支持 properties 里配置 basic auth（`password=...`）；正式接入建议在你的网关做登录态/JWT 校验后再转发 |
| 限流 | properties：`requestLimit`、`requestLimitPeriodInSeconds` |
| 最大文本长度 | properties：`maxTextLength`、`maxTextHardLength` |
| 性能 | JVM 堆（`-Xmx2g` 起）；properties：`cacheSize`、`cacheDurationSeconds`、`pipelineCaching=true` |
| 准确率增强 | `--languageModel <ngram目录>`（挂 n-gram 语言模型，英文混淆词大幅提升） |
| 个人词典 | `GET/POST /v2/words` 需配置数据库（properties 的 `database*` 项）；不需要就保持关闭 |
| HTTPS | 改用 `org.languagetool.server.HTTPSServer` + keystore 配置 |

### 建议改造顺序
1. 同域反代 + 指定 `--allow-origin`
2. 网关层加鉴权和限流
3. 按需挂 n-gram 语言模型
4. 把页面里的"忽略规则/偏好"改为存到你自己的用户系统
