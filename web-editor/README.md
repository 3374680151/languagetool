# LanguageTool 网页校对编辑器

单文件网页应用（`index.html`），无需构建、无任何外部依赖，可整体拷贝到你自己的服务里。

## 本地体验

```sh
cd web-editor
./start.sh          # 一键起 LanguageTool(8081) + 静态站(8091)
./start.sh stop     # 停掉
# 浏览器打开 http://127.0.0.1:8091/
```

手动起也可以：

```sh
# 1. LanguageTool 服务（必须带 --allow-origin，浏览器跨域需要）
cd /Users/wang/tools/LanguageTool-6.6
/Users/wang/tools/java21/Contents/Home/bin/java -cp languagetool-server.jar \
  org.languagetool.server.HTTPServer --port 8081 --allow-origin

# 2. 静态服务
cd web-editor && python3 -m http.server 8091 --bind 127.0.0.1
```

页面顶部会显示服务器连接状态；也可以直接在"高级设置"里填任意服务器地址。

## 文档

| 文件 | 内容 |
|---|---|
| `guide.html` | **《使用与配置说明》**：界面分区、写作到纠错流程、纠错弹窗每个按钮、词库、快捷键，以及前端配置项 / localStorage / 服务端启动参数 / properties 全部配置项 / 三档推荐配置 / 反代示例 / 故障排查。页面右上角 `?` 里有入口 |
| `server.properties.example` | 服务端 `--config` 配置模板，注释齐全，改一行放开一行 |
| `start.sh` | 一键起服务（LT 8081 + 静态站 8091） |

## 功能对照

### 编辑器与交互（v3：改错发生在错误出现的地方）
| 页面功能 | 对应 API 参数 |
|---|---|
| **就地纠错卡**：点正文里带下划线的词，卡片贴在那个词旁边弹出（不再藏在角落圆点里） | `matches[].offset/length` + `matches[].replacements` |
| 改完自动跳到下一处，卡片跟着走；上一条/下一条、⌥←/→、⌥↵ 采用第一条建议 | 前端状态机 |
| 卡片内显示：错误类型、规则 ID、上下文、建议、规则说明（`configinfo` 里的 description） | `matches[].message` + `configinfo.rules[]` |
| 右侧「审阅」队列：按 拼写/语法/标点/风格 筛选 + 比例条，卡片可 跳过 / 永久忽略 / 加入词典 / 忽略此类 | `rule.category.id` + `disabledRules` / `disabledCategories` |
| 审阅进度条（已修正 / 剩余），顶部徽章显示待处理数量，点击从第一个问题开始 | 前端统计 |
| 「全部应用」先弹 diff 确认清单，确认后才落笔（原来是盲改） | `replacements[0]` |
| 跳过的问题进「已跳过 N 条」，可一键恢复（区别于永久忽略） | 前端暂存 |
| 双向联动：点卡片定位正文并弹卡；点正文下划线弹卡 | `offset/length` |
| 改动记录：逐条 `原文 → 修正`，撤销一步 / 还原全文；开始打字自动退出审阅态 | 前端记录 |
| 草稿自动保存、拖拽或按钮导入 .txt/.md/.html | localStorage / FileReader |
| 复制修正稿、下载 .txt、导出 Markdown 批注报告、查看原始 JSON | — |
| 主题切换（跟随系统/浅色/深色）、移动端单列 | — |
| 快捷键：⌘/Ctrl+Enter 检查、Alt+←/→ 换问题、Alt+Enter 采用、Esc 关闭、⌘/Ctrl+Z 撤销、? 帮助 | — |

### 我的词库（v4）
| 页面功能 | 实现 |
|---|---|
| **页头「我的词库 N」按钮**：一键打开词库视图（也作为收藏数量的常驻指示） | `openBank()` |
| **选中即收藏**：正文里选中任意词/短语，浮出「＋ 收藏」，点一下就进词库 | 选区 + mirror div 算光标坐标定位浮条 |
| 纠错卡 / 审阅卡片里的「收藏」按钮 | 收藏被标红的词，同时不再报拼写错误 |
| 收藏后提示条带「查看词库」按钮，点一下直接跳过去 | `toast(msg, actionLabel, actionFn)` |
| 右侧面板切「词库」：搜索、写备注（释义/例句）、删除 | localStorage `lt-wordbank` |
| 「收藏的词不再报拼写错误」总开关 | 收藏词进 `localDict` 白名单，客户端过滤 |
| 导出 / 导入 JSON、清空 | 便于备份或换浏览器迁移 |
| 旧版「本地词典」自动迁移进词库 | 首次加载时从 `lt-web-editor-sets.dict` 迁移 |

### 写作时的提示节奏
| 行为 | 说明 |
|---|---|
| 停止输入 0.7s 后自动检查 | 不需要点按钮；按钮降级为「重查 ⌘↵」 |
| 检查中 | 顶部徽章变橙色脉冲「检查中…」 |
| 新增问题 | 徽章闪一下，正文出现下划线 |
| 开始打字 | 自动收起纠错卡和收藏浮条，不打断写作 |

### 检查参数
| 页面功能 | 对应 API 参数 |
|---|---|
| 语言下拉（自动从服务端拉取） | `GET /v2/languages` → `language` |
| 自动检测语言 | `language=auto`，结果显示 `language.detectedLanguage` |
| 检查级别（标准/严格/学术） | `level=default/picky/academic` |
| 检查范围（全部 / 仅文本级 / 排除文本级） | `mode=all/textLevelOnly/allButTextLevelOnly` |
| 母语提示 / 偏好变体 | `motherTongue` / `preferredVariants` |
| 仅启用指定规则（白名单） | `enabledOnly=true` + `enabledRules` |
| 分类白名单（只查勾选分类） | `enabledCategories` |
| 忽略规则 / 忽略分类（本地持久化） | `disabledRules` / `disabledCategories` |
| 跳过 Markdown 代码块（``` 与 `行内`） | 前端等长空格屏蔽，偏移不变 |
| 长文分段检查（超过阈值自动切片，偏移回填，显示 3/7 进度） | 多次 `POST /v2/check` + 偏移合并 |
| 本地词典（不报拼写错误，仅本浏览器） | 客户端过滤；服务端 `/v2/words` 需数据库，未接 |

### 服务端能力面板
| 页面功能 | 对应 API 参数 |
|---|---|
| 规则浏览器：6202 条规则可搜索、按分类筛，一键「忽略」或「只启用它」 | `GET /v2/configinfo?language=xx` → `rules[]` |
| 服务端能力条：版本 / 规则数 / 分类数 / 文本上限 | `configinfo.parameter.maxTextLength` |
| 分类勾选清单（24 个分类） | `configinfo.rules[].categoryId/categoryName` |

### 仍未接
| 项目 | 原因 |
|---|---|
| `GET/POST /v2/words` 服务端个人词典 | 需要配置数据库与 user id，本地版未开 |
| HTML/XML 标记校对（`data` + `markup`） | 实测该通道对本机 6.6 返回空结果，未启用；贴纯文本效果一致 |
| 鉴权 / 限流 | 开源版只有 properties 的 basic auth，建议在网关做 |

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
