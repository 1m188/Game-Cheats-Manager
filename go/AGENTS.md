# fling-tui — Go 移植项目

## 项目概述

本项目将 Python（PyQt6）实现的 [Game-Cheats-Manager](https://github.com/dyang886/Game-Cheats-Manager) 的 FLiNG 修改器抓取功能独立移植到 Go 语言，以 Bubble Tea TUI 提供终端交互界面。

### 功能

- **搜索** — 输入游戏英文名，从 FLiNG 网站模糊搜索匹配的修改器
- **下载** — 选择搜索结果，自动下载修改器压缩包、解压、组织到本地
- **缓存** — HTML 页面磁盘缓存（24h TTL），减少重复网络请求

### 背景

原 Python 项目 Game-Cheats-Manager 聚合了 FLiNG、小幸姐、Cheat Tables、GCM 等多个修改器来源。本项目仅移植 FLiNG（风灵月影）来源，通过公开 Web 爬虫实现，无需任何 API 密钥。

---

## 架构设计

### 目录结构

```
> **⚠️ 注意**: 以下为项目目标结构。目前仅有文档文件（AGENTS.md, CLAUDE.md, roadmap.md, .golangci.yml）存在，源代码待实现。详见 [roadmap.md](./roadmap.md)。

go/                                  # Go 模块根
├── go.mod
├── go.sum
├── AGENTS.md                        # 本文件（项目说明书，给人看 + 给 AI 看）
├── CLAUDE.md                        # 指向 AGENTS.md
├── roadmap.md                       # 实现路线图（19-task 详细计划）
├── .golangci.yml                    # golangci-lint 静态检查配置
├── .gitignore                      # 排除 fling-data/、二进制文件
├── cmd/
│   └── fling-tui/
│       └── main.go                  # 程序入口: tea.NewProgram(model).Run()
├── internal/
│   ├── fling/
│   │   ├── models.go               # 数据模型 (Trainer, Index, Config, DownloadProgress)
│   │   ├── archive_parser.go       # ParseArchiveHTML() — archive.flingtrainer.com
│   │   ├── main_parser.go          # ParseMainHTML() — flingtrainer.com/all-trainers-a-z/
│   │   ├── search.go               # SearchTrainers(), Sanitize() — 模糊搜索 + 去重
│   │   ├── trainer_page.go         # FetchTrainerDetails() — 详情页抓取（下载链接+版本）
│   │   ├── downloader.go           # DownloadFile() — 流式下载 + progress channel
│   │   ├── extractor.go            # ExtractAndFindTrainer() — .zip (stdlib) + .rar (rardecode)
│   │   ├── organizer.go            # OrganizeTrainer() — 文件组织 + 多版本 + gcm_info
│   │   ├── symbols.go              # SymbolReplacement() — 文件名安全转换
│   │   └── index.go                # BuildIndex() — 数据刷新管线
│   ├── store/
│   │   ├── cache.go                # FetchAndCache(), LoadFromCache() — HTTP + 磁盘缓存
│   │   └── config.go               # LoadConfig(), SaveConfig() — ./fling-data/config.json
│   └── tui/
│       └── model.go                # Bubble Tea Model / Init / Update / View + 状态机
└── testdata/                        # HTML fixtures 测试数据（git 跟踪）
    ├── fling_archive.html           #   archive.flingtrainer.com 页面快照
    ├── fling_main.html              #   flingtrainer.com/all-trainers-a-z/ 页面快照
    └── trainer_page.html            #   单个修改器详情页快照
```

### 分层职责

| 层 | 路径 | 对应 Python | 职责 |
|----|------|------------|------|
| 入口层 | `cmd/fling-tui/` | `main.py` | Bubble Tea 程序启动、初始状态注入 |
| TUI 层 | `internal/tui/` | `main.py`（UI部分） | Elm Architecture 状态管理、视图渲染、键盘处理 |
| 业务逻辑层 | `internal/fling/` | `download_display_thread.py`, `download_trainers_thread.py` | 搜索、下载、解压、文件组织等核心逻辑 |
| 数据存储层 | `internal/store/` | `config.py`, `download_base_thread.py`（缓存部分） | HTTP 请求、磁盘缓存、配置持久化 |
| 纯数据层 | `internal/fling/models.go` | 隐式散落在各线程中的数据字典 | 类型化的 Go struct，JSON 序列化 |

### 数据流

```
用户输入 → textinput.Model
              ↓
         SearchTrainers(index.Archive, index.Main, keyword)
              ↓
         返回 []SearchResult → viewport 渲染列表
              ↓ 用户选择
         FetchTrainerDetails(url) → (downloadURL, version, err)
              ↓
         DownloadFile(url, destPath, progress chan) → 流式下载进度
              ↓
         ExtractAndFindTrainer(archive, tempDir) → []string (exe 文件名)
              ↓
         OrganizeTrainer(exes, name, origin, version, path)
              ↓
         ./fling-data/trainers/[FL] Game Name Trainer/*.exe + gcm_info.json
```

### 运行时文件布局

代码源码全部在 `go/` 下。编译后二进制可置于任意目录，所有运行时数据收敛到二进制旁的单一目录中：

```
任意目录/
├── fling-tui(.exe)          # 编译后的二进制
└── fling-data/              # 全部运行时文件
    ├── config.json           # 用户配置
    ├── cache/                # HTML 缓存
    │   ├── fling_archive.html
    │   └── fling_main.html
    ├── downloads/            # 临时下载（自动清理）
    └── trainers/             # 已下载修改器
        └── [FL] DREDGE Trainer/
            ├── DREDGETrainer.exe
            └── gcm_info.json
```

### Bubble Tea 状态机

```
loading → error（Esc→重启）
loading → ready（搜索框聚焦）
             ↓ Enter
          searching → ready（结果列表聚焦）
                         ↓ Enter
                      downloading → done（3秒）→ ready
                      downloading → error（Esc→ready）
Ctrl+C / q → 退出
```

---

## 技术选型

| 需求 | 选型 | 理由 |
|------|------|------|
| TUI 框架 | `github.com/charmbracelet/bubbletea` | Go 生态标准 TUI 框架，Elm Architecture |
| TUI 组件 | `github.com/charmbracelet/bubbles` | textinput（输入框）、viewport（可滚动列表）、spinner（加载动画） |
| 终端样式 | `github.com/charmbracelet/lipgloss` | ANSI 颜色和布局 |
| HTML 解析 | `github.com/PuerkitoBio/goquery` | jQuery 风格 CSS 选择器，对标 Python BeautifulSoup |
| RAR 解压 | `github.com/nwaples/rardecode` | 纯 Go RAR reader，无 CGO |
| 模糊搜索 | `github.com/lithammer/fuzzysearch` | 子串模糊匹配 |
| ZIP 解压 | `archive/zip`（标准库） | 零外部依赖 |
| HTTP | `net/http`（标准库） | 零外部依赖 |
| JSON | `encoding/json`（标准库） | 零外部依赖 |
| 测试 | `testing`（标准库）+ `net/http/httptest` | Go 原生测试框架 |
| 格式化 | `golangci-lint fmt`（gofmt + goimports） | 统一风格 |
| 静态检查 | `golangci-lint`（最严格配置） | 45+ linter 全覆盖 |
| CLI | （不使用外部 CLI 框架） | Bubble Tea 本身就是交互式接口 |

---

## 测试策略

### 测试分层

所有测试通过 `go test ./...` 一键运行。

#### 第一层：单元测试（httptest + 内存数据）

不依赖外部网络。使用 `httptest.NewServer` 提供模拟 HTTP 响应。

| 被测包 | 测试文件 | 测试内容 |
|--------|----------|----------|
| `internal/store` | `cache_test.go` | FetchAndCache（写入文件）、LoadFromCache（新鲜/过期/缺失）、sentinel error |
| `internal/store` | `config_test.go` | LoadConfig/SaveConfig 往返、默认值、无效 JSON |
| `internal/fling` | `archive_parser_test.go` | 解析 archive 页面（正常/空 HTML）、忽略列表、Bright Memory 特殊处理、URL 拼接 |
| `internal/fling` | `main_parser_test.go` | 解析 main 页面（正常/空 HTML）、去除 "Trainer" 后缀、URL 完整性 |
| `internal/fling` | `search_test.go` | Sanitize（数字→罗马数字、去标点、小写）、SearchTrainers（精确匹配、大小写不敏感、无结果、去重 main>archive） |
| `internal/fling` | `trainer_page_test.go` | 提取下载链接和版本号、任一缺失返回 error |
| `internal/fling` | `downloader_test.go` | 流式下载 + progress channel 更新、Content-Disposition 文件名提取、Content-Length 校验 |
| `internal/fling` | `extractor_test.go` | .zip 解压 + exe 检测、.rar 解压、无 .exe 返回 error |
| `internal/fling` | `organizer_test.go` | 单文件组织、多版本分拆（fling_main / fling_archive 来源）、gcm_info.json 内容 |
| `internal/fling` | `symbols_test.go` | SymbolReplacement 4 条规则逐条+组合 |
| `internal/fling` | `index_test.go` | BuildIndex（首次获取、二次命中缓存）、TTL 过期重新获取 |

#### 第二层：集成测试（HTML fixtures）

| 被测功能 | 测试数据 | 测试内容 |
|----------|----------|----------|
| 搜索管线 | `testdata/fling_archive.html`, `testdata/fling_main.html` | 完整搜索流程：解析→搜索→去重→展示名 |
| 下载管线 | `testdata/trainer_page.html` + 模拟下载 | 详情页抓取→下载→解压→组织 |

#### 第三层：E2E 烟雾测试

| 被测功能 | 测试内容 |
|----------|----------|
| 全流程 | 搜索 "DREDGE" → 验证结果 → 搜索 "zzzznotagame" → 验证空结果 |
| 缓存 | BuildIndex 两次调用，第二次命中缓存 |
| 配置 | 配置往返 save→load |

### 测试编写原则

1. **表驱动测试** — 使用 `[]struct{name, input, want}` 模式
2. **测试名使用中文** — 如 `测试Sanitize/数字转罗马数字`
3. **三个维度全覆盖** — 正常路径、边界条件（空字符串、零值）、异常输入（截断HTML、无效格式）
4. **每个公开函数** — 至少一个测试用例
5. **零外部网络** — 单元测试全部使用 `httptest.NewServer`，不连外网

### Bubble Tea TUI 测试策略

Bubble Tea 程序通过 `tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil)).Run()` 可在无终端环境下测试。TUI 测试关注：
1. **状态跳转** — 按键后 state 是否正确切换
2. **数据流** — 搜索结果是否正确填充到 model.results
3. **错误处理** — 网络/解析错误是否跳到 error 状态

---

## 开发工作流

所有开发者（人类和 AI）必须严格遵循此工作流。违反工作流的代码不得合入。

### 开发方法：测试驱动开发（TDD）

所有功能开发必须遵循 TDD 模式：**先写测试，后写实现**。

```
明确需求与设计
  ↓
[RED] 编写测试代码（预期失败）           # 阶段 1: 用测试定义功能行为
  ↓ 测试未通过 ← 确认测试有效
[GREEN] 编写最小实现代码（通过测试）    # 阶段 2: 只写让测试通过的代码
  ↓ 测试全部通过
[REFACTOR] 优化代码结构、添加注释        # 阶段 3: 重构但不改变行为
  ↓
  ├→ 任何重构后重新运行测试，确保仍通过
  ↓
提交
```

### TDD 原则

| 原则 | 说明 |
|------|------|
| 先测试后代码 | 不写测试就写功能代码 = 不合规 |
| 测试即文档 | 测试描述功能行为，替代额外的规格说明 |
| 最小实现 | 只写让当前测试通过的最少代码 |
| 红→绿→重构 | 严格遵守 RED → GREEN → REFACTOR 循环 |
| 测试覆盖表驱动 | 使用 Go 标准表驱动模式，覆盖正常路径、边界条件、异常输入 |
| 每次提交前 | 必须经过完整的 `golangci-lint fmt` + `golangci-lint run` + `go test -race ./...` |

### 工作流步骤

```
[RED] 编写测试代码
  ↓
go mod tidy                           # 步骤 1: 清理多余依赖（仅当引入新导入时）
  ↓
golangci-lint fmt ./...               # 步骤 2: 代码格式化
  ↓
golangci-lint run ./...               # 步骤 3: 全量静态分析
  ↓ 有告警 → 修复代码 → 回到步骤 1
  ↓ 无告警
go test -race ./...                   # 步骤 4: 验证测试框架（新测试应失败）
  ↓ ┌─ 测试失败（预期）→ 测试有效，进入步骤 5
  │ └─ 测试通过 → 测试无意义，重写测试
  ↓
[GREEN] 编写功能实现代码
  ↓
golangci-lint fmt ./...               # 步骤 5: 格式化新代码
  ↓
golangci-lint run ./...               # 步骤 6: 静态分析新代码
  ↓ 有告警 → 修复代码 → 回到步骤 5
  ↓ 无告警
go test -race ./...                   # 步骤 7: 测试应全部通过
  ↓ 有失败 → 修复代码 → 回到步骤 5
  ↓ 全部通过
[REFACTOR] 优化代码 + 补全注释
  ↓
同步更新注释和文档                    # 步骤 8: 检查所有注释与代码一致
  ↓
MPLR 分层审查                         # 步骤 8.5: 按 L1→L4 逐层自检
  ↓ 有问题 → 修复 → 回到步骤 2
  ↓ 无问题
go test -race ./...                   # 步骤 9: 重构后确认测试通过
  ↓
go build -o ../fling-tui ./cmd/fling-tui/ # 步骤 10: 构建可执行文件
  ↓
完成 ✓
```

### 快速参考

```bash
# 进入项目
cd go

# 完整工作流（一键）
go mod tidy && golangci-lint fmt ./... && golangci-lint run ./... && go test -race ./... && go build -o ../fling-tui ./cmd/fling-tui/

# 仅静态分析
golangci-lint run ./...

# 仅格式化
golangci-lint fmt ./...

# 仅测试（含竞态检测）
go test -race ./...

# 带自动修复的静态分析
golangci-lint run --fix ./...
```

---

## 代码审查方法：多视角分层审查（MPLR）

单次审查不可能发现所有问题。MPLR 将审查拆为 **4 层独立视角**，每层只关注一类问题，彻底查完再进入下一层。与 TDD 互补——TDD 保证**功能正确**，MPLR 保证**代码健壮**。

### 四层审查

```
L1 运行期安全 (Crash Layer)
  → 会 panic 吗？会 goroutine 泄漏吗？channel 会死锁吗？
  → 检查点: slice 索引越界、nil 指针解引用、类型断言、channel 关闭发送、
    tea.Batch 中 goroutine panic 未捕获、progress channel 未关闭导致 goroutine 泄漏

L2 数据完整性 (Data Layer)
  → HTML 解析结果与 Python 原版一致吗？gcm_info.json 格式正确吗？
  → 检查点: 对比 Python 解析结果、验证 JSON schema、config 往返一致性

L3 边界与异常 (Boundary Layer)
  → 空 HTML、截断 HTML、网络超时、磁盘满、权限不足每种情况正确处理？
  → 检查点: 对每个输入字段穷举边界表 (nil, "", 超大值, 网络断开)

L4 语义正确性 (Semantic Layer)
  → 搜索匹配逻辑与 Python 版 fuzz.partial_ratio 行为一致吗？
    版本号提取正则匹配吗？去重规则正确吗？
  → 检查点: 对照 Python 参考代码，逐条件/逐正则比对
```

### 审查检查清单

代码提交前必须逐项自检：

```
□ L1 [运行期安全]
  - 所有 slice[n]/slice[m:n] 是否验证了边界？
  - 所有 goquery Selection 结果是否检查了 Length() > 0？
  - channel 发送前是否检查了 channel 非 nil？
  - tea.Batch 中的 goroutine 是否有 recover？
  - 文件操作后是否 defer Close()？
  - HTTP response body 是否 defer Close()？

□ L2 [数据完整性]
  - 解析的 Trainer 结构体字段是否与 Python trainer_urls dict 对应？
  - gcm_info.json 的字段名是否与 Python 版一致？
  - config.json 读写往返后所有字段保留？
  - Sanitize() 输出是否与 Python sanitize() 一致？

□ L3 [边界]
  - 空 HTML → 返回空 slice（不是 nil slice）？
  - 网络超时 → 返回带上下文的 error？
  - 磁盘满 → WriteFile 失败 → 清理部分文件？
  - progress channel 满时 → 非阻塞跳过（select default）？
  - 关键词为空字符串 → SearchTrainers 返回空？
  - 零字节下载 → 返回错误？

□ L4 [语义]
  - 版本去除正则是否与 Python download_display_thread.py:166 完全一致？
  - 版本号提取正则是否与 Python download_trainers_thread.py:222 完全一致？
  - 去重逻辑: fling_main 是否总是覆盖 fling_archive？
  - 忽略列表中的 4 个游戏是否被正确跳过？
  - "Bright.Memory.Episode.1" → "Bright Memory: Episode 1" 特殊处理？
  - 展示名格式: "[FL] {GameName} Trainer"？
```

### 审查流程

MPLR 作为工作流步骤 **8.5**，位于同步文档之后、重构验证之前：

```
同步更新注释和文档      # 步骤 8
  ↓
MPLR 分层审查           # 步骤 8.5: 按 L1→L4 逐层自检
  ↓ 有问题 → 修复 → 回到步骤 2
  ↓ 无问题
go test -race            # 步骤 9
  ↓
go build                 # 步骤 10
```

---

## 注释与文档撰写规范

**核心原则**：注释必须时刻与代码保持同步一致。

### 注释层级规范

```
层级 1: 文件头注释
   位置: 每个 .go 文件的 package 声明之前
   内容: 描述本文件/模块的整体功能和职责
   示例: // Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。

层级 2: 类型注释
   位置: 每个公开 struct/interface/enum 定义之前
   内容: 用途和设计意图
   示例: // Index 表示从 FLiNG 网站解析出的修改器索引，供搜索使用。

层级 3: 函数/方法注释
   位置: 每个公开函数或方法定义之前
   内容: 功能、参数含义、返回值、注意事项
   示例: // SearchTrainers 在 archive 和 main 索引中搜索匹配关键词的修改器，
         // 返回去重后的结果列表（main 来源优先）。

层级 4: 常量/变量注释
   位置: 每个公开常量或变量定义之前或同行

    示例: // DefaultCacheTTL 是缓存默认过期时间，单位：小时。
层级 5: 内部逻辑注释
   位置: 复杂算法、非直观逻辑、边界条件处理处
   内容: 解释为什么这样写，而不仅仅是描述做了什么
   示例: // 使用 strings.Contains 作为主要匹配 + Levenshtein 后备，
         // 等价于 Python fuzzywuzzy 的 fuzz.partial_ratio >= 80
```

### 注释语言

- 所有注释使用**简体中文**
- 技术术语保留英文（如 goquery、Bubble Tea、Levenshtein）
- 代码/文件引用使用反引号（如 `internal/fling/search.go`）
- Python 参考使用行号引用（如 `download_display_thread.py:166`）

### 注释同步要求

| 触发场景 | 必须更新的内容 |
|----------|---------------|
| 新增文件 | 文件头注释描述模块功能 |
| 新增公开类型/函数/常量 | 添加中文注释 |
| 修改函数行为 | 同步更新该函数的注释 |
| 修改函数签名 | 同步更新参数和返回值的说明 |
| 删除函数 | 删除对应的注释 |
| 正则表达式 | 注释中标注对应 Python 文件的精确行号 |

### 文档

- `AGENTS.md` — 本文档，项目说明书，给人看 + 给 AI 看
- `CLAUDE.md` — 指向 AGENTS.md
- `roadmap.md` — 实现路线图（19-task 详细计划）
- `.golangci.yml` — 静态检查配置
- 测试代码 — 测试名称和注释即功能规格说明

---

## 编码规范

### 错误处理

- 使用 Go 标准 `error` 接口，不 panic
- 外部 error 必须用 `fmt.Errorf("context: %w", err)` 包装
- 自定义 sentinel error（如 `ErrCacheStale`、`ErrTrainerNotFound`）
- HTTP 响应体必须 `defer resp.Body.Close()`

### 命名

- 包名使用小写单词（`fling`, `store`, `tui`）
- 文件名使用蛇形命名（`archive_parser.go`）
- 常量使用驼峰命名，不使用全大写
- 测试文件使用 `_test.go` 后缀

### 函数设计

- 单个函数不超过 80 行 / 40 条语句（`funlen` 约束）
- 圈复杂度不超过 15（`gocyclo` 约束）
- 嵌套深度不超过 4 层（`nestif` 约束）
- 优先 early return 扁平化逻辑

### HTTP 规范

- 使用固定 User-Agent: `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110 Safari/537.36`
- 请求必须设置合理的超时（`http.Client{Timeout: 30s}`）
- 不使用 cloudscraper 或任何反机器人绕过

### Bubble Tea 规范

- Model 状态用 iota 枚举，不散落字符串比较
- 所有 `tea.Cmd` 用 `tea.Batch` 组合
- progress channel 必须非阻塞发送（`select { case ch <- msg: default: }`）
- `tea.Quit` 前清理资源（关闭 channel、移除临时文件）

---

## 静态检查覆盖范围

`.golangci.yml` 配置文件位于 `go/` 目录，启用以下 linter 类别：

| 类别 | Linter | 覆盖内容 |
|------|--------|----------|
| 正确性 | `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign` | 错误忽略、可疑构造、死代码 |
| 安全 | `gosec`, `noctx`, `bodyclose`, `canonicalheader` | 安全漏洞、HTTP 规范 |
| 质量 | `gocritic`, `revive`, `goconst`, `dupword`, `misspell`, `unconvert`, `prealloc` | 代码风格、重复代码、预分配 |
| 复杂度 | `gocyclo`, `funlen`, `gocognit`, `cyclop`, `nestif`, `nakedret`, `maintidx` | 圈复杂度、函数长度、嵌套深度 |
| 测试 | `testableexamples`, `testifylint`, `thelper`, `tparallel` | 测试最佳实践 |
| 健壮性 | `nilerr`, `nilnesserr`, `durationcheck`, `makezero`, `errorlint`, `wrapcheck` | nil 错误、时间计算、错误包装 |

---

## 设计决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| 搜索 | 纯英文，不做中英翻译 | 无翻译 API 可用；FLiNG 网站本身就是英文 |
| TUI 框架 | Bubble Tea | Go 生态最成熟的 TUI 框架，Elm Architecture |
| HTML 解析 | goquery | jQuery 风格 API，对标 Python BeautifulSoup |
| 运行时文件 | 单一 `fling-data/` 目录 | 二进制旁仅一个目录，干净整洁 |
| 源码位置 | `go/` 子目录 | 作为 Game-Cheats-Manager 仓库内的 Go 子项目 |
| 文件安全性 | SymbolReplacement（4 规则） | 精确复制 Python download_base_thread.py:300-302 |
| 压缩包 | .zip 用 stdlib，.rar 用 rardecode | 零外部二进制依赖，跨平台 |
| 下载模式 | 单线程流式 | 简单可靠，TUI 场景无并行需求 |
| 静态检查 | golangci-lint（最严格配置） | 与 RePKG 项目相同标准 |
| 开发方法 | TDD | 保证代码可测试性 |
| 审查方法 | MPLR 四层审查 | 覆盖安全、数据、边界、语义 |
| 注释语言 | 简体中文 | 开发者是中文用户 |

---

## 快速开始

```bash
# 初始化开发环境
cd go
go mod tidy

# 安装 golangci-lint
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# 运行全部测试
go test -race ./...

# 运行静态检查
golangci-lint run ./...

# 构建
go build -o ../fling-tui ./cmd/fling-tui/

# 启动
./fling-tui
```
