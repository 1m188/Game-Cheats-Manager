# Game Cheats Manager Web — FLiNG 修改器搜索（纯静态）

纯前端实现的风灵月影（FLiNG）修改器**搜索 + 下载**：

- **搜索**：构建期把 FLiNG 两个列表页解析成离线索引（`data/trainers-data.js`），运行时基线零网络即过滤；匹配/排序规则与 `go/internal/fling`（[search.go](../go/internal/fling/search.go)）逐函数对齐。
- **下载**：直接打开官方页面/官方下载链接（浏览器原生处理 Cloudflare、Cookie 与 Referer），**无需 CORS**。文件落入浏览器下载目录。
- **新修改器不漏**：页面打开时经 WordPress REST（实测对浏览器 CORS 开放）增量拉取基线之后的新条目，失败静默回退离线索引。

## 为什么“HTML 解析在构建期、不运行时”

主站与归档页**不发 `Access-Control-Allow-Origin`**，浏览器运行时抓 HTML 必被 CORS 拦截；
而“构建期用 Node 抓一次，随 dist 分发”让运行时只剩两件天生合法的事：本地过滤、点击跳转。
HTML 页的时效性则由“运行时 WP REST 增量层”补齐（见下节）——这也是实测发现的最优解。

## 数据时效性：基线 + 在线增量（两层）

| 层 | 时机 | 内容 | 失效行为 |
|---|---|---|---|
| 基线层 | 构建期 (`npm run build:index`) | 完整索引，离线可搜，git 可提交快照 | 索引文件损坏才会缺功能 |
| 时效层 | 页面打开时 (`assets/refresh.js`) | `GET /wp-json/wp/v2/posts?after=<基线时间>` 增量拉新帖并入搜索 | 静默回退基线，页面不受影响 |

- **为什么增量层在纯静态里是合法的**：WordPress 核心的 REST API 会把请求 `Origin` 回显为
  `access-control-allow-origin`（本项目实测成立；HTML 列表页则完全无 CORS）。
- **为什么过时不用焦虑**：归档源为定格数据（页面 `last-modified: 2024-07`）；主站条目 URL/名称
  稳定，过时仅表现为“缺最新”；下载动作永远实时打开官方页面，不存在缓存过期。
- **定期重建仍是保底**：运行时增量只覆盖“新帖”，不负责站点改版后的解析适配
  （`build-index.mjs` 对解析条数设有下限阈值，骤降会直接报错而不是产残缺索引）；
  CI 工作流见下方"部署"。

## 目录结构

```
web/
├── index.html              # 页面入口（classic script，无 module，file:// 双击可用）
├── assets/
│   ├── style.css
│   ├── app.js              # UI 逻辑（DOM/渲染/快捷键/增量合并展示）
│   ├── search.js           # 搜索内核(纯函数): sanitize / fuzzy / 版本排序（对齐 go 搜索）
│   └── refresh.js          # 运行时增量层(纯函数+HTTP): WP REST after= 增量抓新帖
├── data/                   # 构建产物（git 可提交快照，npm run build:index 刷新）
│   ├── trainers-data.js    # 基线索引（globalThis 注入，无 fetch）
│   └── trainers.json       # 同内容 JSON（供其他工具引用）
├── scripts/
│   ├── build-index.mjs     # 抓两个来源页 → 解析（对齐 go 版解析规则）→ 写 data/*
│   ├── serve.mjs           # 零依赖本地静态服务（默认 127.0.0.1:8137）
│   └── verify-data.mjs     # 数据 + 内核 + 在线增量层 回归校验(含 go 版测试向量)
└── package.json            # 无任何依赖; 仅 npm scripts
```

## 快速开始

```bash
cd web
npm run build:index   # 生成/刷新基线索引（需可访问 flingtrainer.com）
npm run verify        # 数据 + 搜索内核 + 在线增量层 回归校验
npm start             # http://127.0.0.1:8137/ 打开页面
```

部署 = 把 `web/`（至少 `index.html + assets/ + data/`）原样丢到任意静态托管
（GitHub Pages、nginx、Edge/CF Pages…）。**没有构建步骤**，目录本身就是 dist。

> `file://` 双击打开也可用（数据以 classic script 注入）；在线增量层同样合法（WP REST 跨域放行）。
> 日常开发建议本地 `npm start` 或 http 托管，兼容性最好。

## 与 go/internal/fling 的行为对齐点

| 环节 | 对齐实现（web） | go 参考 |
|---|---|---|
| main 解析 | `div.letter-section > ul > li > a`，截尾部 ` Trainer` | `main_parser.go` |
| archive 解析 | `a[target="_self"]` + 版本后缀正则 + `_`→`: ` | `archive_parser.go` |
| 忽略列表 | 4 个游戏 + `Bright.Memory.Episode.1` 特例 | `archive_parser.go:26-78` |
| 搜索 | `sanitize`（数字→罗马）+ 子序列/子串/编辑距离(≤len/3) | `search.go` |
| 去重 | 按 URL；main 覆盖同 URL 的 archive 条目 | `search.go:181-220` |
| 排序 | 匹配质量 → 版本降序 → 名称字母序 | `search.go:230-244` |
| 展示名 | `[FL] {GameName} Trainer` | `search.go:225` |
| User-Agent | 与 `store/cache.go:18` 相同 | `build-index.mjs` |
| 运行时增量 | WP REST `after=` 增量，标题截尾同 main（go 无此层） | web 新增 |

搜索为纯英文关键词（同 go 版的既定决策：不做中英翻译）。
