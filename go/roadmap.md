# fling-tui — 实现路线图 (完整版)

> **Git 跟踪**: 此文件在 `go/roadmap.md`，由 git 版本控制，是项目的持久化开发计划。  
> **审查状态**: ✅ 已通过 Momus 高精度审查，5 项 WARNING 已修复。  
> **最终更新**: 2026-06-29  
> **关联文件**: `.omo/plans/fling-tui-go.md`（工作计划原稿，含详细依赖矩阵）
>
> **源码位置**: 所有 Go 代码在 `go/` 目录下（`go/cmd/`, `go/internal/`），以 `go/go.mod` 为模块根。
>
> **运行时目录**: 编译产物旁仅有一个 `fling-data/` 目录，收敛所有缓存/下载/配置：
> ```
> 任意目录/
> ├── fling-tui(.exe)         # 编译出的二进制
> └── fling-data/             # 全部运行时文件
>     ├── config.json
>     ├── cache/              # HTML 缓存
>     ├── downloads/          # 临时下载（自动清理）
>     └── trainers/           # 已下载修改器
> ```

---

## 目录

1. [TL;DR — 人类可读摘要](#tldr)
2. [范围边界](#范围)
3. [技术栈](#技术栈)
4. [项目目录结构](#目录结构)
5. [数据模型](#数据模型)
6. [FLiNG 爬虫管线详解](#fling-爬虫管线)
7. [Bubble Tea 状态机与键盘映射](#bubble-tea-状态机)
8. [实现路线 — 5 Waves × 19 Tasks](#实现路线)
9. [依赖矩阵](#依赖矩阵)
10. [详细任务分解 (Task 1–19)](#详细任务分解)
11. [最终验证清单](#最终验证清单)
12. [提交策略](#提交策略)
13. [成功标准](#成功标准)
14. [Python → Go 技术对照表](#python--go-技术对照表)
15. [参考代码位置](#参考代码位置)

---

## TL;DR

**你将得到**: 一个跨平台的 Go TUI 应用，输入游戏名 → 从 FLiNG 网站搜索结果 → 选择 → 下载修改器到 `./fling-data/trainers/`，全程无需任何 API 密钥。

**核心决策**:
- **Bubble Tea** Elm Architecture 管理多状态 TUI（loading/ready/searching/downloading/error）
- **goquery** 解析 HTML（对标 Python BeautifulSoup，CSS 选择器）
- **公开爬虫** 取代 secret_config.py，零依赖私有 API
- **单一数据目录** `fling-data/` 收敛所有缓存/下载/配置，二进制旁就这一个目录
- **源码在 `go/` 下** — 这是 Game-Cheats-Manager 仓库中的 Go 子项目

**不会做的事**: 其他修改器源（小幸姐、CT表、GCM）、中英双语搜索、BGM移除、自动更新、WeMod/Cheat Evolution

**工作量**: Medium（19 个 todo，5 个并行 Wave） | **风险**: Medium（FLiNG 网页 HTML 结构改版会导致解析失效）

---

## 范围

### Must have
- 抓取 + 缓存 `flingtrainer.com` HTML 页面（archive + main A-Z）
- 解析 HTML 为修改器索引（游戏名 + URL 对）
- 模糊搜索 + 去重（fling_main 优先于 fling_archive）
- 抓取单个修改器页面 → 提取下载链接 + 版本号 (YYYY.MM.DD regex)
- 流式下载修改器压缩包 + 实时进度
- 提取 .zip（stdlib）+ .rar（纯 Go rardecode），定位 trainer .exe
- 处理多版本压缩包（分拆子目录）
- 移动文件到 `./fling-data/trainers/[FL] Game Name Trainer/`，写入 gcm_info.json
- Bubble Tea TUI：搜索输入框、可滚动结果列表、进度条、状态消息
- 配置持久化（config.json）：缓存 TTL、下载路径
- 所有运行时文件收敛于 `./fling-data/` 中

### Must NOT have（守卫规则）
- ❌ 其他修改器源（仅 FLiNG）
- ❌ API 密钥或 S3 端点——仅公开爬虫
- ❌ 中英双语搜索——仅英文模糊匹配
- ❌ BGM 移除、反作弊检测、任何 Windows 专有功能
- ❌ 自动更新、后台定时器、社区上传
- ❌ FLTK / Qt——仅 TUI
- ❌ 并行分块下载

---

## 技术栈

| 模块 | 库 | 版本策略 | 用途 |
|------|-----|---------|------|
| TUI 框架 | `github.com/charmbracelet/bubbletea` | latest stable | Elm Architecture 状态管理 |
| TUI 组件 | `github.com/charmbracelet/bubbles` | latest stable | textinput, viewport, spinner |
| 终端样式 | `github.com/charmbracelet/lipgloss` | latest stable | ANSI 颜色和布局 |
| HTML 解析 | `github.com/PuerkitoBio/goquery` | latest stable | jQuery 风格 CSS 选择器 |
| RAR 解压 | `github.com/nwaples/rardecode` | latest stable | 纯 Go RAR reader（无 CGO） |
| 模糊搜索 | `github.com/lithammer/fuzzysearch` | latest stable | 子串模糊匹配 |

**不需要的依赖**: cloudscraper, 任何 CGO 库, Windows-Only API

---

## 目录结构

```
Game-Cheats-Manager/                 # 仓库根
├── go/                              # ★ Go 子项目（所有源码在此）
│   ├── go.mod                       # Go 模块定义
│   ├── go.sum
│   ├── .gitignore                   # 忽略 fling-data/ 等运行时产物
│   ├── roadmap.md                   # 本文件 — 实现路线图
│   ├── cmd/
│   │   └── fling-tui/
│   │       └── main.go              # 程序入口: tea.NewProgram(model).Run()
│   └── internal/
│       ├── fling/
│       │   ├── models.go           # 数据模型 (Trainer, Index, Config, DownloadProgress)
│       │   ├── archive_parser.go   # ParseArchiveHTML() — archive.flingtrainer.com
│       │   ├── main_parser.go      # ParseMainHTML() — flingtrainer.com/all-trainers-a-z/
│       │   ├── search.go           # SearchTrainers(), Sanitize() — 模糊搜索 + 去重
│       │   ├── trainer_page.go     # FetchTrainerDetails() — 详情页下载链接+版本
│       │   ├── downloader.go       # DownloadFile() — 流式下载 + progress channel
│       │   ├── extractor.go        # ExtractAndFindTrainer() — .zip + .rar 解压
│       │   ├── organizer.go        # OrganizeTrainer() — 文件组织 + 多版本 + gcm_info
│       │   ├── symbols.go          # SymbolReplacement() — 文件名安全转换
│       │   └── index.go            # BuildIndex() — 数据刷新管线
│       ├── store/
│       │   ├── cache.go            # FetchAndCache(), LoadFromCache() — HTTP + 磁盘缓存
│       │   └── config.go           # LoadConfig(), SaveConfig() — config.json 持久化
│       └── tui/
│           └── model.go            # Bubble Tea Model / Init / Update / View + 状态机
│       └── testdata/                      # HTML fixtures（git 跟踪）
│           ├── fling_archive.html
│           ├── fling_main.html
│           └── trainer_page.html
│
└── ... (Game-Cheats-Manager 原有文件不受影响)

编译运行后的磁盘布局（二进制可置于任意目录）:
  任意目录/
  ├── fling-tui(.exe)                # 编译出的可执行文件
  └── fling-data/                    # ★ 单一数据目录（全部运行时文件）
      ├── config.json                # 运行时配置
      ├── cache/                     # HTML 缓存
      │   ├── fling_archive.html     #   archive.flingtrainer.com 的缓存
      │   └── fling_main.html        #   flingtrainer.com/all-trainers-a-z/ 的缓存
      ├── downloads/                 # 临时下载目录（下载完成后自动清理）
      └── trainers/                  # 已下载修改器
          └── [FL] DREDGE Trainer/
              ├── DREDGETrainer.exe
              └── gcm_info.json
```

**关键约定**:
- 源码全部在 `go/` 下，`go.mod` 在此，Go 模块路径由 `go.mod` 决定
- 编译: `cd go && go build -o ../fling-tui ./cmd/fling-tui/`（或 `go run ./cmd/fling-tui/`）
- 二进制启动后在自身所在目录创建/使用 `./fling-data/`
- `config.go` 的 `LoadConfig()` 读 `./fling-data/config.json`（相对二进制，不是源码目录）
- `fling-data/` 整体加入 `.gitignore`

---

## 数据模型

所有结构体定义在 `go/internal/fling/models.go`：

```go
// Trainer — 单个修改器的所有信息（解析+下载填充）
type Trainer struct {
    GameName    string `json:"game_name"`    // 清洗后的游戏名（去版本号后的干净名）
    URL         string `json:"url"`          // flingtrainer.com 完整 URL
    Origin      string `json:"origin"`       // "fling_main" | "fling_archive"
    Version     string `json:"version"`      // YYYY.MM.DD 格式（下载阶段才填充）
    TrainerName string `json:"trainer_name"` // 展示名: "[FL] Game Name Trainer"
}

// Index — 内存中的修改器索引（由 BuildIndex 构建，供搜索和 TUI 使用）
type Index struct {
    Archive   []Trainer `json:"archive"`
    Main      []Trainer `json:"main"`
    FetchedAt time.Time `json:"fetched_at"`
}

// Config — 用户配置（持久化到 ./fling-data/config.json）
type Config struct {
    CacheTTLHours int       `json:"cache_ttl_hours"` // 默认 24
    DownloadPath  string    `json:"download_path"`   // 默认 "./fling-data/trainers/"
    LastFetch     time.Time `json:"last_fetch"`
}

// DownloadProgress — 下载进度（通过 channel 从 downloader 传递到 TUI）
type DownloadProgress struct {
    BytesDownloaded int64   `json:"bytes_downloaded"`
    TotalBytes      int64   `json:"total_bytes"`
    PercentComplete float64 `json:"percent_complete"`
}
```

**禁止添加的字段**: `gcm_url`, `extension`, `author`, `custom_name`, `custom_name_en`, `custom_name_zh`（这些是 CT/GCM 来源专用，不在范围内）

---

## FLiNG 爬虫管线

### 阶段 1: 数据获取 (BuildIndex — `go/internal/fling/index.go`)

```
URL 1: https://archive.flingtrainer.com/
URL 2: https://flingtrainer.com/all-trainers-a-z/

抓取方式: net/http GET, Chrome 96 User-Agent
缓存位置: ./fling-data/cache/fling_archive.html, ./fling-data/cache/fling_main.html
缓存策略: 24h TTL，基于文件修改时间。超时重新抓取。
刷新方式: 应用启动时检查 TTL，手动重启触发刷新（无后台定时器）。
```

### 阶段 2: HTML 解析

#### Archive 解析 (`go/internal/fling/archive_parser.go`)

| 项目 | 值 |
|------|-----|
| **CSS 选择器** | `a[target="_self"]` |
| **名称提取** | 正则去除版本后缀（见下），`_`→`: ` |
| **URL 处理** | `url.JoinPath("https://archive.flingtrainer.com/", href)` |

**版本去除正则**（Python `download_display_thread.py:166` 精确复制）:
```regex
  v[\d.]+.*|\.\bv.*| \d+\.\d+\.\d+.*| Plus\s\d+.*|Build\s\d+.*|(\d+\.\d+-Update.*)|Update\s\d+.*|\(Update\s.*| Early Access .*|\.Early.Access.*
```

**硬编码忽略列表**（Python `download_display_thread.py:168-173`）:
- Dying Light The Following Enhanced Edition
- Monster Hunter World
- Street Fighter V
- World War Z

**特殊处理**: `"Bright.Memory.Episode.1"` → `"Bright Memory: Episode 1"`

**返回**: `[]Trainer` — `Origin: "fling_archive"`, `Version: ""`, `TrainerName: ""`

#### Main 解析 (`go/internal/fling/main_parser.go`)

| 项目 | 值 |
|------|-----|
| **CSS 选择器** | `div.letter-section ul li a` |
| **名称提取** | `strings.TrimSpace(link.Text())`，然后去掉 `" Trainer"` 后缀（等价 Python: `.rsplit(" Trainer", 1)[0]`） |
| **URL 处理** | 直接使用 `href`（已经是完整 URL，无需拼接） |

**返回**: `[]Trainer` — `Origin: "fling_main"`, `Version: ""`, `TrainerName: ""`

### 阶段 3: 模糊搜索 (`go/internal/fling/search.go`)

**Sanitize 函数**（复制 Python `download_base_thread.py:295-298`）:
```
1. 所有数字 → 罗马数字 (1→I, 2→II, ..., 10→X, 11→XI, ..., 3999→MMMCMXCIX)
   使用 strconv.Atoi + 标准转换逻辑（参考 Python download_base_thread.py:276-293 的 arabic_to_roman 实现）
2. 去除所有标点符号，保留 &（正则: [^\w&]）
3. 去除所有空白字符
4. 转小写
```

**匹配逻辑**（替代 Python 的 `fuzz.partial_ratio >= 80`）:
- 使用 `fuzzy.Match(sanitizedKeyword, sanitizedGameName)` from `lithammer/fuzzysearch`
- 此库不返回分数，加入后备逻辑：`strings.Contains(sanitizedGameName, sanitizedKeyword)` OR Levenshtein 距离 < len(sanitizedKeyword)/3
- 两种策略都通过则匹配

**去重规则**（Python `download_display_thread.py:41-53`）:
- 以 `Sanitize(gameName)` 为键
- `fling_main` 始终覆盖 `fling_archive`（同款游戏主站优先）

**展示名**（Python `download_base_thread.py:338-412`，仅 FL 前缀部分）:
```
英文: "[FL] {GameName} Trainer"
```
**禁止**: 无中文翻译、无按来源排序

**返回**: `[]SearchResult`，按 GameName 字母排序

### 阶段 4: 抓取详情页 (`go/internal/fling/trainer_page.go`)

```
GET https://flingtrainer.com/games/{game-slug}-trainer/  （即搜索结果的 URL）

下载链接提取:
  goquery: doc.Find("a[target='_self']").FilterFunction(
    func(i int, s *goquery.Selection) bool {
      href, _ := s.Attr("href")
      return strings.Contains(href, "flingtrainer.com")
    })
  → 获取 href 属性作为下载 URL

版本号提取 (YYYY.MM.DD):
  定位 <div class="entry">，获取文本内容
  正则: options.*game\s*version.*last\s*updated:\s*(\d{4}\.[0-1]?\d\.[0-3]?\d)
  → 返回 group(1) 例如 "2024.03.15"

如果任一缺失 → 返回 error
```

**Python 参考**: `download_trainers_thread.py:212-228`

### 阶段 5: 下载 + 解压 + 组织

#### 5a. 下载 (`go/internal/fling/downloader.go`)
```
1. GET 请求（流式，不全量缓冲）
2. 读取 response body（32KB chunks）
3. 每 chunk 写入文件，发送 DownloadProgress 到 channel（非阻塞: select with default）
4. 文件名: 从 Content-Disposition header 提取（支持 filename*=, filename=, RFC 5987）
   回退到 URL 路径 basename
5. 成功关闭文件；失败清理部分文件
保存位置: ./fling-data/downloads/
```
**Python 参考**: `download_base_thread.py:63-83`（request_download）, `download_base_thread.py:183-197`（文件名提取）
**禁止**: 并行分块下载、断点续传、哈希校验

#### 5b. 解压 (`go/internal/fling/extractor.go`)
```
1. 通过魔数或扩展名检测类型
2. .zip → archive/zip (stdlib)，全部提取到 destDir
3. .rar → nwaples/rardecode，迭代文件逐个提取（跳过损坏文件，记录日志）
4. 扫描 destDir: 文件名含 "trainer"（case-insensitive）且以 ".exe" 结尾
5. 无 .exe → 返回描述性错误（提示杀毒软件可能拦截）
```
**Python 参考**: `download_trainers_thread.py:245-255`（7z 提取）, `download_trainers_thread.py:258-267`（exe 检测）
**禁止**: 外部工具（7z, unrar）、反作弊文件检测、instructionDst 目录创建

#### 5c. 文件组织 (`go/internal/fling/organizer.go`)
```
1. 文件名安全化: SymbolReplacement(name)
   ": "→" - ", ":"→"-", "/"→"_", "?"→""

2. 目标路径: ./fling-data/trainers/{sanitizedName}/

3. 多版本处理:
   如果 >1 个 .exe:
     fling_main 来源: regex 'trainer(.*)\.exe' (case-insensitive) → 子目录 {name}_v1.0/
     fling_archive 来源: regex "\s+Update.*|\s+v\d+.*" → 子目录 {name} Update 2/

4. 单文件: 移动到 ./fling-data/trainers/{sanitizedName}/{exeFile}

5. 写入 gcm_info.json: {"game_name": name, "origin": origin, "version": version}

6. 清理临时文件（./fling-data/downloads/*）
```
**Python 参考**: `download_trainers_thread.py:284-315`（多版本）, `download_trainers_thread.py:61-88`（文件移动）, `download_trainers_thread.py:108-114`（gcm_info.json）, `download_base_thread.py:300-302`（symbol_replacement）
**禁止**: BGM 移除、反作弊说明文件、.ini 设置修改

---

## Bubble Tea 状态机

### 状态流转图

```
         ┌──────────┐
    ┌───►│ loading  │───┐  抓取+解析HTML，完成后自动跳转
    │    └──────────┘   │
    │         │         │
    │    ┌────▼─────┐   │
    │    │   ready  │◄──┤  显示搜索框（焦点在输入）
    │    └────┬─────┘   │
    │         │ Enter   │
    │    ┌────▼─────┐   │
    │    │ searching│   │  执行同步搜索（瞬间完成）
    │    └────┬─────┘   │
    │         │         │
    │    ┌────▼─────┐   │
    └────┤   ready  │   │  显示结果列表 + 结果计数
         │ (results)│   │  Up/Down 浏览，Enter 选中下载
         └────┬─────┘   │
              │ Enter   │  选中结果
         ┌────▼──────┐  │
         │downloading│  │  进度条 + 状态行 + filename
         └────┬──────┘  │
              │ 完成     │
         ┌────▼─────┐   │
         │   done   │───┘  显示 "Saved to ..." 3秒，自动回 ready
         └──────────┘
              │ 失败
         ┌────▼─────┐
         │  error   │───► 显示错误 + "Press Esc to go back"
         └──────────┘     Esc → 返回 ready
```

### 键盘操作

| 按键 | 状态 | 行为 |
|------|------|------|
| 输入文字 | ready | 输入游戏名到搜索框 |
| Enter | ready（搜索框聚焦） | 执行搜索 → 跳转结果显示 |
| ↑ / ↓ | ready（结果列表聚焦） | 浏览搜索结果 |
| Enter | ready（结果列表聚焦） | 下载选中项 → 跳转 downloading |
| Esc | downloading | 取消/忽略，返回 ready |
| Esc | error | 关闭错误，返回 ready |
| Ctrl+C | 所有状态 | 立即退出 |
| q | 所有状态 | 立即退出 |

### Bubble Tea 消息类型

```go
// 自定义消息（在 go/internal/tui/model.go 中定义）
type progressMsg DownloadProgress      // 下载进度更新
type downloadDoneMsg struct {          // 下载完成
    err      error
    destPath string
}
type dataLoadedMsg struct {            // HTML 加载完成
    index *Index
    err   error
}
type searchDoneMsg []SearchResult      // 搜索完成
```

### Model 结构体

```go
type model struct {
    state            state            // loading|ready|searching|downloading|error|done
    searchInput      textinput.Model  // 搜索输入框（bubbles textinput）
    results          []SearchResult   // 当前搜索结果
    selectedIndex    int             // 结果列表中选中的索引
    viewport         viewport.Model  // 结果列表滚动（bubbles viewport）
    progress         DownloadProgress // 当前下载进度
    statusMsg        string          // 状态栏消息
    err              error           // 最近错误
    config           *Config         // 应用配置
    index            *Index          // 修改器索引（Archive + Main）
    downloadDestPath string          // 下载完成后的目标路径
}
```

**TUI 约束**:
- 使用 inline mode（非 alt-screen），保护终端历史
- 不支持鼠标
- 键盘快捷键仅限上述列表
- 颜色仅限 Bubble Tea/lipgloss 默认主题

---

## 实现路线

### Wave 概览

| Wave | Todos | 并行策略 | 产出物 |
|------|-------|---------|--------|
| **Wave 1** | 1–4 | 1 先行；2,3,4 可并行 | 项目骨架 + 数据模型 + HTTP缓存 + 配置 |
| **Wave 2** | 5–8 | 5,6 并行; 7 依赖 5,6; 8 独立并行 | HTML 解析器 + 搜索引擎 + 详情页抓取 |
| Wave 3 | 9–12 | 9→10→(11需12完成)串行链；12可与9,10并行 | 下载 + 解压 + 文件组织 + 符号替换 |
| Wave 4 | 13–16 | 13 先行；14,15 可彼此并行；16 依赖 13–15 | TUI 外壳 + 搜索视图 + 进度视图 + 前后端连线 |
| **Wave 5** | 17–19 | 串行 (17→18→19) | 数据管线 + 错误处理 + E2E 测试 |

---

## 依赖矩阵

| # | 任务 | 依赖 | 阻塞 | 可并行 |
|---|------|------|------|--------|
| 1 | 项目脚手架 | — | 2,3,4 | 2,3,4 |
| 2 | 数据模型 | 1 | 5,6,7,8,9,10,11,13,17 | 1,3,4 |
| 3 | HTTP + 缓存 | 1 | 5,8 | 1,2,4 |
| 4 | 配置持久化 | 1 | 5,8,17 | 1,2,3 |
| 5 | Archive 解析器 | 3 | 7,8 | 6 |
| 6 | Main 解析器 | 3 | 7 | 5 |
| 7 | 搜索引擎 | 2,5,6 | 8,17 | — |
| 8 | 详情页抓取 | 3 | 17 | — |
| 9 | 文件下载器 | 2 | 10,17 | — |
| 10 | 压缩包提取器 | 9 | 11 | — |
| 11 | 文件组织器 | 10 | 17 | — |
| 12 | 符号替换 | 2 | 11,17 | — |
| 13 | TUI 外壳 | 2 | 16,17 | 14,15 |
| 14 | 搜索+结果视图 | 2,13 | 16 | 13,15 |
| 15 | 下载+进度视图 | 2,13 | 16 | 13,14 |
| 16 | 前后端连线 | 13,14,15 | 17 | — |
| 17 | 数据刷新管线 | 2,3,4,5,6 | 18,19 | — |
| 18 | 错误处理 | 17 | 19 | — |
| 19 | E2E 烟雾测试 | 18 | — | — |

---

## 详细任务分解

> **规则**: 实现 + 测试 = 1 个 todo。绝不分离。  
> 每个 todo 必须包含: What to do / Must NOT do / Python 参考 / 验收标准 / QA 场景 / 提交信息  
> **源码位置**: 所有文件路径均相对于 `go/` 目录（Go 模块根）

---

### Task 1: 项目脚手架

**What to do**: 在 `go/` 目录下初始化 Go 项目。`go mod init`（模块名自行决定，例如 `fling-tui`）。添加依赖: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/lipgloss`, `github.com/PuerkitoBio/goquery`, `github.com/nwaples/rardecode`, `github.com/lithammer/fuzzysearch`。创建目录树: `go/cmd/fling-tui/`, `go/internal/fling/`, `go/internal/store/`, `go/internal/tui/`。添加 `go/.gitignore` 排除 `fling-data/`, 二进制文件。

**Must NOT do**: 不创建除 `func main() { fmt.Println("fling-tui starting") }` 之外的任何逻辑。不添加 API 密钥或 Windows 专有依赖。

**Python 参考**: 无（全新项目）。

**验收标准**: `cd go && go mod tidy && go build ./...` 成功。`golangci-lint run ./...` 通过。所有空 package 的目录树存在。

**QA 场景**: `go test ./...`（尚无测试，仅编译验证）。

**提交**: `chore(init): scaffold Go module with Bubble Tea deps and directory structure`

---

### Task 2: 数据模型

**What to do**: 在 `go/internal/fling/models.go` 中定义所有结构体:
- `Trainer`: `GameName string`, `URL string`, `Origin string` ("fling_main"|"fling_archive"), `Version string`, `TrainerName string`
- `SearchResult`: 同 Trainer
- `Config`: `CacheTTLHours int` (默认 24), `DownloadPath string` (默认 `"./fling-data/trainers/"`), `LastFetch time.Time`
- `DownloadProgress`: `BytesDownloaded int64`, `TotalBytes int64`, `PercentComplete float64`
- `Index`: `Archive []Trainer`, `Main []Trainer`, `FetchedAt time.Time`

**Must NOT do**: 不包含 S3/GCM 字段（gcm_url, extension）。不包含 author, custom_name 字段。不包含翻译字段。

**Python 参考**: trainer_urls dict shape — `download_display_thread.py:181-192`, translate_trainer — `download_base_thread.py:338-412`

**验收标准**: Package 可编译。所有结构体有 JSON 标签。

**QA 场景**: `go test ./internal/fling/ -run TestModels` — 验证 Config 的 JSON marshal/unmarshal 往返。

**提交**: `feat(models): define Trainer, SearchResult, Config, DownloadProgress, Index structs`

---

### Task 3: HTTP 客户端 + HTML 磁盘缓存

**What to do**: 实现 `go/internal/store/cache.go`:
- `FetchAndCache(url, cachePath string) ([]byte, error)` — Chrome 96 UA GET，保存到 cachePath
- `LoadFromCache(cachePath string, maxAge time.Duration) ([]byte, error)` — mtime 在 maxAge 内返回内容
- `CacheExists(cachePath string) bool`
- 缓存路径: `./fling-data/cache/`，如 `./fling-data/cache/fling_archive.html`
- 测试: mock HTTP server → 验证写入、加载新鲜缓存、过期缓存返回 sentinel error
- 启动时自动创建 `fling-data/cache/` 目录（`os.MkdirAll`）

**Must NOT do**: 不使用 cloudscraper。不自定义 TLS。不重试。不内存缓存。

**Python 参考**: `get_webpage_content()` — `download_base_thread.py:44-61`, `save_html_content()` — `download_base_thread.py:415-419`, headers — `download_base_thread.py:34-37`

**验收标准**: `FetchAndCache` 写入文件。`LoadFromCache` 新鲜时返回内容，过期返回 sentinel error。

**QA 场景**: `go test ./internal/store/ -run TestCache -v`

**提交**: `feat(store): HTTP fetch + disk cache under ./fling-data/cache/ with max-age TTL`

---

### Task 4: 配置持久化

**What to do**: 实现 `go/internal/store/config.go`:
- `LoadConfig() (*Config, error)` — 从 `./fling-data/config.json` 读取（相对二进制所在目录 `os.Executable()`），缺失则创建默认
- `SaveConfig(c *Config) error` — 写 JSON
- 默认值: `CacheTTLHours: 24`, `DownloadPath: "./fling-data/trainers/"`
- 启动时自动创建 `fling-data/` 目录（`os.MkdirAll`）
- 测试: 往返 save/load, 缺失文件创建默认, 无效 JSON 返回错误

**Must NOT do**: 不使用环境变量覆盖。不使用 ~/.config 或 %APPDATA%。不热重载。

**Python 参考**: settings.json 默认值 — `config.py:191-252`

**验收标准**: `go test ./internal/store/ -run TestConfig` — 默认创建, save→load 往返保留全部字段。

**QA 场景**: `go test ./internal/store/ -run TestConfig -v`

**提交**: `feat(store): config.json persistence at ./fling-data/config.json`

---

### Task 5: FLiNG Archive HTML 解析器

**What to do**: 实现 `go/internal/fling/archive_parser.go`:
- `ParseArchiveHTML(html []byte) ([]Trainer, error)`
- goquery 选择器: `a[target="_self"]`
- 版本去除正则（精确复制 Python），`_`→`: `
- 硬编码忽略 4 个游戏 + Bright Memory 特殊处理
- URL: `url.JoinPath("https://archive.flingtrainer.com/", href)`
- 返回 `[]Trainer`, `Origin: "fling_archive"`, `Version: ""`, `TrainerName: ""`

**Must NOT do**: 不存储原始链接文本。严格遵循 Python——gameName 是版本去除后的清洗名。不存储未去除版本号的名称。

**Python 参考**: `download_display_thread.py:152-192`, `download_display_thread.py:166`（正则）, `download_display_thread.py:168-177`（忽略列表）

**验收标准**: 解析已知 HTML fixture。返回数量 >0。"DREDGE" 出现且 URL 正确。忽略列表中的 trainer 不出现。空 HTML → 空 slice + 无 error。

**QA 场景**: `go test ./internal/fling/ -run TestParseArchive -v`

**提交**: `feat(fling): parse archive.flingtrainer.com HTML into Trainer list`

---

### Task 6: FLiNG Main A-Z HTML 解析器

**What to do**: 实现 `go/internal/fling/main_parser.go`:
- `ParseMainHTML(html []byte) ([]Trainer, error)`
- goquery 选择器: `div.letter-section ul li a`
- 去掉 `" Trainer"` 后缀，直接使用 `href`
- 返回 `[]Trainer`, `Origin: "fling_main"`, `Version: ""`, `TrainerName: ""`

**Must NOT do**: 不加回 "Trainer"。不拼接 base URL。

**Python 参考**: `download_display_thread.py:236-257`

**验收标准**: origin 为 "fling_main"，游戏名不含 "Trainer" 后缀，URL 以 "https://flingtrainer.com/" 开头。

**QA 场景**: `go test ./internal/fling/ -run TestParseMain -v`

**提交**: `feat(fling): parse flingtrainer.com/all-trainers-a-z/ HTML`

---

### Task 7: 搜索引擎（模糊匹配 + 去重 + sanitize）

**What to do**: 实现 `go/internal/fling/search.go`:
- `SearchTrainers(archive, main []Trainer, keyword string) []SearchResult`
- `Sanitize(s string) string` — 数字→罗马数字(1-3999)，去标点保留&，去空白，小写
- 模糊匹配: `fuzzy.Match()` + 后备 `strings.Contains` OR Levenshtein < len/3
- 去重: 相同 sanitized game name → fling_main 替换 fling_archive
- 展示名: `"[FL] {GameName} Trainer"`
- 返回按 GameName 字母排序的结果

**Must NOT do**: 无翻译查找。无中文支持。无按来源排序。

**Python 参考**: `sanitize()` — `download_base_thread.py:295-298`, `keyword_match()` — `download_display_thread.py:140-148`, 去重 — `download_display_thread.py:41-53`

**验收标准**: "DREDGE" → "[FL] DREDGE Trainer"。大小写不敏感。无结果返回空。main 覆盖 archive。

**QA 场景**: `go test ./internal/fling/ -run TestSearch -v`

**提交**: `feat(fling): fuzzy search with sanitization and main-over-archive dedup`

---

### Task 8: 修改器详情页抓取器

**What to do**: 实现 `go/internal/fling/trainer_page.go`:
- `FetchTrainerDetails(trainerURL string, cache httpCache) (downloadURL string, version string, err error)`
- 下载 URL: goquery FilterFunction 找 href 含 "flingtrainer.com" 的 `<a target="_self">`
- 版本: `<div class="entry">` → 正则 `options.*game\s*version.*last\s*updated:\s*(\d{4}\.[0-1]?\d\.[0-3]?\d)`
- 任一缺失 → 返回 error

**Must NOT do**: 无 S3 模式。无 GCM 路径。无默认/回退 URL。

**Python 参考**: `download_trainers_thread.py:212-228`

**验收标准**: 使用 HTML fixture 测试。验证 URL 和版本被提取。

**QA 场景**: `go test ./internal/fling/ -run TestTrainerPage -v`

**提交**: `feat(fling): scrape trainer detail page for download URL and version`

---

### Task 9: 文件下载器（流式 + 进度 channel）

**What to do**: 实现 `go/internal/fling/downloader.go`:
- `DownloadFile(url, destPath string, progress chan<- DownloadProgress) error`
- 流式 GET，32KB chunks，每 chunk 非阻塞发送 Progress
- 文件名: Content-Disposition → RFC 5987 → URL basename 回退
- 保存到 `./fling-data/downloads/`

**Must NOT do**: 不并行分块。不断点续传。不哈希校验。

**Python 参考**: `download_base_thread.py:63-83`, `download_base_thread.py:183-197`

**验收标准**: httptest server 测试。文件大小匹配 Content-Length。progress channel 有更新。

**QA 场景**: `go test ./internal/fling/ -run TestDownload -v`

**提交**: `feat(fling): streaming file downloader with progress channel`

---

### Task 10: 压缩包提取器（.zip + .rar + 定位 trainer .exe）

**What to do**: 实现 `go/internal/fling/extractor.go`:
- `ExtractAndFindTrainer(archivePath, destDir string) ([]string, error)`
- .zip → archive/zip (stdlib)。.rar → nwaples/rardecode（跳过损坏文件）
- 扫描文件名含 "trainer"（case-insensitive）且以 ".exe" 结尾
- 无 .exe → 描述性错误（提示杀毒软件）

**Must NOT do**: 不使用外部工具（7z, unrar）。不检测反作弊文件。

**Python 参考**: `download_trainers_thread.py:245-255`, `download_trainers_thread.py:258-267`

**验收标准**: .zip 含 "DREDGE Trainer.exe" + "readme.txt" → 仅返回 exe。.rar 测试。无 exe → 错误。

**QA 场景**: `go test ./internal/fling/ -run TestExtract -v`

**提交**: `feat(fling): archive extractor for .zip (stdlib) and .rar (rardecode)`

---

### Task 11: 文件组织器（多版本分拆 + 元数据写入）

**What to do**: 实现 `go/internal/fling/organizer.go`:
- `OrganizeTrainer(exeFiles []string, tempDir string, trainerName string, origin string, version string, downloadPath string) error`
- SymbolReplacement 安全化文件名
- 目标: `./fling-data/trainers/{sanitizedName}/`
- 多版本: fling_main regex `trainer(.*)\.exe`（case-insensitive），fling_archive regex `\s+Update.*|\s+v\d+.*`
- 写 gcm_info.json: `{"game_name", "origin", "version"}`
- 清理 `./fling-data/downloads/` 临时文件

**Must NOT do**: 不 BGM 移除。不创建反作弊说明目录。不修改 .ini。

**Python 参考**: `download_trainers_thread.py:284-315`, `download_trainers_thread.py:61-88`, `download_trainers_thread.py:108-114`, `download_base_thread.py:300-302`

**验收标准**: 单 exe → 一个目录含 gcm_info.json。2 个 exes → 两个子目录。

**QA 场景**: `go test ./internal/fling/ -run TestOrganize -v`

**提交**: `feat(fling): file organizer with multi-version handling and gcm_info.json`

---

### Task 12: 符号替换工具

**What to do**: 实现 `go/internal/fling/symbols.go`:
- `SymbolReplacement(name string) string` — 4 条规则: `": "→" - "`, `":"→"-"`, `"/"→"_"`, `"?"→""`
- 测试: `"Game: Subtitle"` → `"Game- Subtitle"`

**Must NOT do**: 不添加额外替换。

**Python 参考**: `download_base_thread.py:300-302`

**验收标准**: 全部 4 条规则单独和组合的单元测试通过。

**QA 场景**: `go test ./internal/fling/ -run TestSymbolReplacement -v`

**提交**: `feat(fling): filesystem-safe name conversion (SymbolReplacement)`

---

### Task 13: TUI 外壳（Bubble Tea Model/Init/Update/View 骨架）

**What to do**: 实现 `go/internal/tui/model.go` 和 `go/cmd/fling-tui/main.go`:
- Model: state (loading|ready|searching|downloading|error), searchInput (textinput.Model), results, viewport, progress, statusMsg, err, config (*Config), index (*Index)
- Init: `tea.Batch(loadConfigCmd, loadDataCmd)` — 加载 HTML 并解析
- Update: Enter=搜索, Esc=返回, Ctrl+C/q=退出, Up/Down=浏览结果
- View: 基于状态用 lipgloss 渲染
- main.go: `tea.NewProgram(initialModel).Run()` — **不使用 `tea.WithAltScreen()`**

**Must NOT do**: 不使用 alt-screen。不支持鼠标。颜色不超出默认。

**Python 参考**: Bubble Tea tutorial。`main.py` GameCheatsManager 类。

**验收标准**: 启动 → "Loading..." → 搜索框。Ctrl+C 干净退出。无 panic。

**QA 场景**: 手动 QA: 验证 loading→ready 跳转, Ctrl+C 退出。

**提交**: `feat(tui): Bubble Tea app shell with state machine (loading/ready/searching/downloading/error)`

---

### Task 14: 搜索输入 + 结果列表视图

**What to do**: 实现搜索交互:
- textinput placeholder "Search game name..."
- Enter → `fling.SearchTrainers(index.Archive, index.Main, input.Value())` → 显示结果计数
- viewport 可滚动列表: `[N] [FL] Game Name Trainer`。Up/Down 选择。高亮选中行。
- Enter 选中 → 发送下载消息。空结果 → "No results found"。

**Must NOT do**: 不同步异步搜索。不显示翻译。不使用来源颜色。

**Python 参考**: `main.py` UI 布局, `download_display_thread.py` 结果格式化。

**验收标准**: "DREDGE" → "[FL] DREDGE Trainer"。"zzzz" → "No results found"。Up/Down 浏览。

**QA 场景**: 手动 QA 用真实或 mock 数据。

**提交**: `feat(tui): search input with scrollable results list and keyboard navigation`

---

### Task 15: 下载进度视图

**What to do**: 实现下载显示:
- 进度条: `[=====>    ] 45% 1.2MB / 2.8MB`
- 状态: "Downloading [FL] DREDGE Trainer from flingtrainer.com..."
- 完成: "Download complete! Saved to ./fling-data/trainers/[FL] DREDGE Trainer/" 3秒 → ready
- 错误: "Download failed: {error}" + "Press Esc to go back"
- Bubble Tea `tea.Batch` + goroutine 连接 progress channel

**Must NOT do**: 不暂停/恢复。不下载队列。不并行下载。

**Python 参考**: `download_trainers_thread.py:234`, `custom_widgets.py` SegmentedProgressBar（概念）

**验收标准**: 进度条百分比正确。完成/错误消息正确。

**QA 场景**: 手动 QA 用真实下载。

**提交**: `feat(tui): download progress bar with real-time status updates`

---

### Task 16: 前后端连线

**What to do**: 完成 `Update` 函数调用真实后端:
- Loading: `store.FetchAndCache()` + fling parsers → index
- Search: `fling.SearchTrainers()`
- Download: `FetchTrainerDetails()` → `DownloadFile()` (progress channel) → `ExtractAndFindTrainer()` → `OrganizeTrainer()`
- progress channel → Bubble Tea `tea.Batch`

**Must NOT do**: 下载 goroutine 之外无并发。初始加载后无后台获取。

**Python 参考**: `main.py: __init__()` 线程信号到 UI slot 的连线

**验收标准**: 全流程: 启动 → 搜索 "DREDGE" → 选择 → 下载 → 文件在 `./fling-data/trainers/` 含 gcm_info.json

**QA 场景**: 端到端手动测试。

**提交**: `feat(tui): wire Bubble Tea Update to fling/search/download backend`

---

### Task 17: 数据刷新管线

**What to do**: 实现 `go/internal/fling/index.go`:
- `BuildIndex(cache store.Cache, config store.Config) (*Index, error)`
- 获取/缓存 `fling_archive.html` 和 `fling_main.html`（缓存位置 `./fling-data/cache/`）
- 解析两者
- 返回 `Index{Archive, Main, FetchedAt}`
- TTL 检查 — 新鲜则跳过获取

**Must NOT do**: 不后台自动刷新。不并发获取。

**Python 参考**: `other_threads.py:145-205`, `download_display_thread.py`

**验收标准**: 返回含非空 Archive 和 Main 的 Index。TTL 内第二次调用无网络请求。

**QA 场景**: `go test ./internal/fling/ -run TestBuildIndex -v` — mock HTTP server

**提交**: `feat(fling): data refresh pipeline with cache-aware fetch and parse`

---

### Task 18: 错误处理通行

**What to do**: 审查所有公共函数。`fmt.Errorf("context: %w", err)` 包装。确保:
- 网络错误 → "Network error: cannot connect to flingtrainer.com"
- 解析错误 → "Parse error: FLiNG website may have changed"
- 磁盘错误 → "Storage error: check disk space and permissions"
- 零 panic。不用 log.Fatal。
- TUI error 显示错误 + "Press Esc to go back"

**Must NOT do**: 不 sentry/telemetry。不错误聚合。暂不重试。

**Python 参考**: `download_trainers_thread.py` 错误处理

**验收标准**: 所有错误路径覆盖。`golangci-lint run ./...` 通过。离线启动 → 有意义错误，不崩溃。

**QA 场景**: `go test ./... -count=1` + 手动离线/无效 URL 测试。

**提交**: `fix: comprehensive error wrapping and graceful TUI error display`

---

### Task 19: 端到端烟雾测试

**What to do**: 在 `go/internal/fling/e2e_test.go` 写集成测试。6 个场景:
1. Happy: 搜索 "DREDGE" → 至少 1 个 fling_main 结果
2. No results: "zzzznotagame" → 空
3. Cache hit: 构建 index 两次，第二次用缓存
4. Config roundtrip: save→load 字段一致
5. Symbol replacement: `"Game: Sub/Title?"` → `"Game- Sub_Title"`
6. Sanitize: `"Test 7"` → `"testvii"`

使用 `httptest.NewServer` 提供 HTML fixtures。无真实网络。

**Must NOT do**: 不做真实下载。不直接测试 flingtrainer.com。

**Python 参考**: HTML fixtures 来自 Python 应用缓存

**验收标准**: `go test ./... -count=1 -race` 通过。全部 6 个场景通过。

**QA 场景**: `go test ./... -count=1 -race -v`

**提交**: `test(e2e): integration tests for search, cache, config, sanitize using test fixtures`

---

## 最终验证清单

> 在全部 19 个 todo 完成后并行执行。全部必须 APPROVE。

- [ ] **F1. 计划合规审查** — 验证 19 个 todo 全部完成，全部 Scope OUT 项目不存在
- [ ] **F2. 代码质量** — `cd go && golangci-lint run ./...` 通过，无 Windows-Only API
- [ ] **F3. 真实手动 QA** — 编译 → 启动 → 搜索 "DREDGE" → 下载 → 验证 `./fling-data/trainers/` 文件结构
- [ ] **F4. 范围忠实度** — 确认无 XiaoXing/CT/GCM/S3 代码，无中文搜索，无 BGM 移除

---

## 提交策略

- 每个 todo 一次提交（共 19 次）
- 原子提交——每次提交前: cd go && go mod tidy && golangci-lint fmt ./... && golangci-lint run ./... && go test -race ./... && go build -o ../fling-tui ./cmd/fling-tui/
- Conventional Commits: `feat(scope): description` / `fix(scope):` / `test(scope):` / `chore:`
- 每个 task 完成后按 [AGENTS.md](./AGENTS.md) 的 MPLR 四层审查自检（L1 运行期安全 → L4 语义正确性）
- 提交顺序遵从依赖矩阵: 基础先行 → 解析器 → TUI → 集成

---

## 成功标准

1. `cd go && go run ./cmd/fling-tui/` 启动 Bubble Tea TUI
2. 输入游戏名 → Enter → 显示 FLiNG 搜索结果
3. 选择结果 → Enter → 下载进度 → 文件保存到 `./fling-data/trainers/`
4. 二进制旁仅一个 `fling-data/` 目录，收敛全部运行时文件
5. 零 API 密钥、零 S3 依赖、零 Windows 专有代码
6. 跨平台编译运行 (Linux / macOS / Windows)
7. `cd go && go test ./... -race` 全部通过

---

## Python → Go 技术对照表

| Python 原版 | Go 移植 | 文件位置 |
|------------|---------|---------|
| `BeautifulSoup` HTML 解析 | `goquery` (CSS 选择器) | `go/internal/fling/archive_parser.go`, `main_parser.go` |
| `fuzz.partial_ratio >= 80` | `strings.Contains` + Levenshtein 后备 | `go/internal/fling/search.go` |
| `requests.get()` HTTP | `net/http` GET | `go/internal/store/cache.go`, `downloader.go` |
| `cloudscraper` 反爬 | **不使用**（FLiNG 不需要） | — |
| `7z.exe` 解压 | `archive/zip` (stdlib) + `rardecode` | `go/internal/fling/extractor.go` |
| `ResourceHacker.exe` BGM 移除 | **不做**（仅 Windows） | — |
| `PyQt6` GUI | Bubble Tea TUI | `go/internal/tui/model.go` |
| `secret_config.py` API 密钥 | **不需要**（公开爬虫） | — |
| `gettext` 国际化 | **不做**（纯英文） | — |
| `arabic_to_roman()` 数字转换 | 自实现（1–3999） | `go/internal/fling/search.go` |
| `gcm_info.json` 写入 | `encoding/json` Marshal | `go/internal/fling/organizer.go` |
| `settings.json` 配置 | `./fling-data/config.json` | `go/internal/store/config.go` |
| `DATABASE_PATH = %APPDATA%/GCM Settings/db` | `./fling-data/cache/` | `go/internal/store/cache.go` |
| `DOWNLOAD_TEMP_DIR = %TEMP%` | `./fling-data/downloads/` | `go/internal/fling/downloader.go` |
| `symbol_replacement()` 文件名清理 | `SymbolReplacement()` | `go/internal/fling/symbols.go` |
| `sanitize()` 关键词清理 | `Sanitize()` | `go/internal/fling/search.go` |
| `translate_trainer()` 展示名构建 | FL 前缀硬编码 `"[FL] {name} Trainer"` | `go/internal/fling/search.go` |

---

## 参考代码位置

所有 Python 参考代码在 `src/scripts/` 下（相对于 Game-Cheats-Manager 仓库根）:

| 文件 | 行号 | 内容 |
|------|------|------|
| `src/scripts/threads/other_threads.py` | 145–205 | FetchFlingSite — 数据获取 |
| `src/scripts/threads/other_threads.py` | 164–166 | FLiNG Archive URL 抓取 |
| `src/scripts/threads/other_threads.py` | 184–185 | FLiNG Main URL 抓取 |
| `src/scripts/threads/download_display_thread.py` | 152–192 | Archive HTML 解析逻辑 |
| `src/scripts/threads/download_display_thread.py` | 166 | 版本去除正则（关键！） |
| `src/scripts/threads/download_display_thread.py` | 168–177 | 忽略列表 + Bright Memory 特殊处理 |
| `src/scripts/threads/download_display_thread.py` | 181–192 | trainer_urls 数据结构 |
| `src/scripts/threads/download_display_thread.py` | 236–257 | Main HTML 解析逻辑 |
| `src/scripts/threads/download_display_thread.py` | 41–53 | 去重逻辑（main > archive） |
| `src/scripts/threads/download_trainers_thread.py` | 192–334 | download_fling() — 下载管线 |
| `src/scripts/threads/download_trainers_thread.py` | 212–228 | 详情页抓取 — 下载 URL + 版本 |
| `src/scripts/threads/download_trainers_thread.py` | 245–255 | 7z 提取 |
| `src/scripts/threads/download_trainers_thread.py` | 258–267 | trainer .exe 检测 |
| `src/scripts/threads/download_trainers_thread.py` | 284–315 | 多版本处理 |
| `src/scripts/threads/download_trainers_thread.py` | 61–88 | 文件移动 + gcm_info 写入 |
| `src/scripts/threads/download_trainers_thread.py` | 108–114 | gcm_info.json 结构 |
| `src/scripts/threads/download_base_thread.py` | 34–37 | HTTP headers (Chrome 96 UA) |
| `src/scripts/threads/download_base_thread.py` | 44–61 | get_webpage_content() |
| `src/scripts/threads/download_base_thread.py` | 63–83 | request_download() |
| `src/scripts/threads/download_display_thread.py` | 140–148 | keyword_match() — 模糊匹配 |
| `src/scripts/threads/download_base_thread.py` | 183–197 | 文件名提取 (Content-Disposition) |
| `src/scripts/threads/download_base_thread.py` | 276–293 | arabic_to_roman() — 数字→罗马 |
| `src/scripts/threads/download_base_thread.py` | 295–298 | sanitize() — 字符串清理 |
| `src/scripts/threads/download_base_thread.py` | 300–302 | symbol_replacement() — 文件名清理 |
| `src/scripts/threads/download_base_thread.py` | 338–412 | translate_trainer() — 展示名构建 |
| `src/scripts/threads/download_base_thread.py` | 415–419 | save_html_content() — 缓存保存 |
| `src/scripts/config.py` | 191–252 | settings.json 默认值 |

---

> **审查记录**: 此计划于 2026-06-29 通过 Momus 高精度审查。  
> **审查结论**: APPROVE（5 项 WARNING 已修复）  
> **审查范围**: 结构完整性、依赖链正确性、歧义清除、验收标准可验证性
