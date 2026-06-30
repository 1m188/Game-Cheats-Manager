// Package tui 提供 Bubble Tea TUI 的状态机、键盘处理和视图渲染。
// 基于 Elm Architecture（Model/Init/Update/View）实现交互式终端界面。
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"fling-tui/internal/fling"
	"fling-tui/internal/store"
)

// logToFile 将消息追加写入 fling-data/fling-tui.log，用于记录完整错误信息。
func logToFile(msg string) {
	logPath := filepath.Join("fling-data", "fling-tui.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o750)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

type (
	// state 表示 TUI 状态机的当前状态。
	state int

	// dataLoadedMsg 表示数据加载完成（成功或失败）。
	dataLoadedMsg struct {
		index *fling.Index
		err   error
	}

	// searchDoneMsg 表示搜索完成，携带匹配结果列表。
	searchDoneMsg []fling.SearchResult

	// downloadDoneMsg 表示下载完成。
	downloadDoneMsg struct {
		err      error
		destPath string
	}

	// progressTickMsg 是下载进度更新的内部定时器消息。
	progressTickMsg struct{}

	// progressMsg 表示下载进度更新，在 downloader goroutine 与 TUI 之间传递。
	progressMsg fling.DownloadProgress

	// doneTimeoutMsg 表示下载完成后的倒计时到期。
	doneTimeoutMsg struct{}

	// configLoadedMsg 表示配置加载完成。
	configLoadedMsg struct {
		config *fling.Config
		err    error
	}

	// realHTTPFetcher 是 fling.HTTPFetcher 的生产环境实现，使用 net/http。
	realHTTPFetcher struct{}

	// model 是 Bubble Tea 应用的核心状态，包含所有 TUI 数据。
	model struct {
		state             state
		searchInput       textinput.Model
		results           []fling.SearchResult
		selectedIndex     int
		viewport          viewport.Model
		progress          fling.DownloadProgress
		statusMsg         string
		err               error
		config            *fling.Config
		index             *fling.Index
		downloadDestPath  string
		lastSearchKeyword string // 上次搜索的关键词，用于区分"重新搜索"和"下载选中项"
	}
)

const (
	stateLoading     state = iota // 正在加载数据
	stateReady                    // 就绪，等待输入
	stateSearching                // 正在搜索
	stateDownloading              // 正在下载
	stateDone                     // 下载完成（短暂显示后自动返回 ready）
	stateError                    // 错误状态

	statusReady = "就绪 — 输入游戏名搜索"

	statusDownloadFailed = "下载失败"
	statusDownloadDone   = "下载完成"

	progressBarWidth = 28 // 进度条内部宽度（不含括号）
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	resultStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	selectedStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))
)

// Get 执行 HTTP GET 请求并返回响应体字节，实现 fling.HTTPFetcher 接口。
func (realHTTPFetcher) Get(url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("网络错误: 创建 HTTP 请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络错误: HTTP 请求失败 (%s): %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // Close 在 defer 中，忽略错误是标准实践

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("网络错误: HTTP %d — 请求 %s 失败", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("网络错误: 读取响应体失败: %w", err)
	}
	return body, nil
}

// formatBytes 将字节数格式化为人类可读的字符串（如 "1.2MB", "450KB"）。
func formatBytes(n int64) string {
	const unit = 1024.0
	if n < 0 {
		return "0B"
	}
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	b := float64(n)
	if b < unit*1024 {
		return fmt.Sprintf("%dKB", int64(b/unit))
	}
	if b < unit*unit*1024 {
		return fmt.Sprintf("%.1fMB", b/(unit*unit))
	}
	return fmt.Sprintf("%.1fGB", b/(unit*unit*unit))
}

// loadConfigCmd 加载配置文件的异步命令。
func loadConfigCmd() tea.Msg { //nolint:ireturn // Bubble Tea 命令必须返回 tea.Msg
	cfg, err := store.LoadConfig()
	return configLoadedMsg{config: cfg, err: err}
}

// loadDataCmd 抓取并解析归档和主站 HTML 页面的异步命令。
func loadDataCmd() tea.Msg { //nolint:ireturn // Bubble Tea 命令必须返回 tea.Msg
	archiveHTML, err := store.FetchAndCache(
		"https://archive.flingtrainer.com/",
		"fling-data/cache/fling_archive.html",
	)
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("网络错误: 获取归档页面失败: %w", err)}
	}

	mainHTML, err := store.FetchAndCache(
		"https://flingtrainer.com/all-trainers-a-z/",
		"fling-data/cache/fling_main.html",
	)
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("网络错误: 获取主站页面失败: %w", err)}
	}

	archiveTrainers, err := fling.ParseArchiveHTML(archiveHTML)
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("解析错误: 解析归档页面失败: %w", err)}
	}

	mainTrainers, err := fling.ParseMainHTML(mainHTML)
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("解析错误: 解析主站页面失败: %w", err)}
	}

	return dataLoadedMsg{
		index: &fling.Index{
			Archive:   archiveTrainers,
			Main:      mainTrainers,
			FetchedAt: time.Now(),
		},
	}
}

// searchCmd 执行异步搜索的命令，调用 fling.SearchTrainers 进行模糊匹配。
func searchCmd(index *fling.Index, keyword string) tea.Cmd {
	return func() tea.Msg {
		if index == nil {
			return searchDoneMsg(nil)
		}
		results := fling.SearchTrainers(index.Archive, index.Main, keyword)
		return searchDoneMsg(results)
	}
}

// downloadCmd 执行完整下载管线的异步命令。
// fling_main 来源需要先抓取详情页获取真实下载链接和版本号；
// fling_archive 来源的 URL 已经是直接下载链接，跳过详情页抓取。
func downloadCmd(trainer *fling.SearchResult, downloadPath string) tea.Cmd {
	return func() tea.Msg {
		downloadURL := trainer.URL
		version := trainer.Version

		// fling_main 来源：抓取详情页获取下载链接和版本号
		if trainer.Origin == "fling_main" {
			fetcher := &realHTTPFetcher{}
			dlURL, ver, fetchErr := fling.FetchTrainerDetails(trainer.URL, fetcher)
			if fetchErr != nil {
				return downloadDoneMsg{err: fmt.Errorf("获取详情页失败 (%s, URL=%s): %w", trainer.TrainerName, trainer.URL, fetchErr)}
			}
			downloadURL = dlURL
			version = ver
		}

		// 在下载路径下创建临时目录
		tempDir := filepath.Join(downloadPath, ".download_temp")
		archivePath := filepath.Join(tempDir, "trainer_download")

		// 下载文件（同步，不发送进度更新）
		actualName, err := fling.DownloadFile(downloadURL, archivePath, nil)
		if err != nil {
			_ = os.RemoveAll(tempDir) //nolint:errcheck // 清理操作，错误可忽略
			return downloadDoneMsg{err: fmt.Errorf("网络错误: 下载失败: %w", err)}
		}

		// 若 Content-Disposition 给出了更合适的文件名，重命名下载文件
		if actualName != "" && actualName != filepath.Base(archivePath) {
			renamedPath := filepath.Join(tempDir, actualName)
			if renameErr := os.Rename(archivePath, renamedPath); renameErr == nil {
				archivePath = renamedPath
			}
		}

		// 解压并找到修改器 .exe 文件
		extractDir := filepath.Join(tempDir, "extract")
		exeFiles, err := fling.ExtractAndFindTrainer(archivePath, extractDir)
		if err != nil {
			_ = os.RemoveAll(tempDir) //nolint:errcheck // 清理操作，错误可忽略
			return downloadDoneMsg{err: fmt.Errorf("解析错误: 解压失败: %w", err)}
		}

		// 组织文件到目标目录
		err = fling.OrganizeTrainer(exeFiles, extractDir, trainer.TrainerName, trainer.Origin, version, downloadPath)
		if err != nil {
			_ = os.RemoveAll(tempDir) //nolint:errcheck // 清理操作，错误可忽略
			if errors.Is(err, fling.ErrTrainerAlreadyExists) {
				safeName := fling.SymbolReplacement(trainer.TrainerName)
				return downloadDoneMsg{destPath: filepath.Join(downloadPath, safeName) + string(filepath.Separator)}
			}
			return downloadDoneMsg{err: fmt.Errorf("存储错误: 文件组织失败: %w", err)}
		}

		// 清理临时目录
		_ = os.RemoveAll(tempDir) //nolint:errcheck // 清理操作，错误可忽略

		safeName := fling.SymbolReplacement(trainer.TrainerName)
		destPath := filepath.Join(downloadPath, safeName) + string(filepath.Separator)

		return downloadDoneMsg{destPath: destPath}
	}
}

// InitialModel 创建并返回一个配置好的初始 model。
// 初始状态为 stateLoading，输入框已聚焦。
//
//nolint:revive // 返回未导出类型符合 Bubble Tea 包内模型的设计惯例
func InitialModel(cfg *fling.Config) model {
	ti := textinput.New()
	ti.Placeholder = "搜索游戏名..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	vp := viewport.New(80, 20)

	return model{
		state:       stateLoading,
		searchInput: ti,
		viewport:    vp,
		config:      cfg,
	}
}

// Init 返回应用启动时的初始命令。
//
//nolint:revive // 接收器在 Bubble Tea Init() 中不需要引用
func (m *model) Init() tea.Cmd {
	return tea.Batch(
		loadConfigCmd,
		loadDataCmd,
	)
}

// Update 处理消息并返回更新后的模型和后续命令。
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 5 // 预留搜索框和状态栏空间
		m.updateViewport()
		return m, nil
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case dataLoadedMsg:
		return m.handleDataLoaded(msg)
	case searchDoneMsg:
		return m.handleSearchDone(msg)
	case downloadDoneMsg:
		return m.handleDownloadDone(msg)
	case configLoadedMsg:
		return m.handleConfigLoaded(msg)
	case progressMsg:
		return m.handleProgress(msg)
	case progressTickMsg:
		if m.state == stateDownloading {
			return m, progressTickCmd()
		}
	case doneTimeoutMsg:
		if m.state == stateDone {
			m.state = stateReady
			m.statusMsg = statusReady
			m.downloadDestPath = ""
			return m, nil
		}
	default:
	}

	// 在 ready 状态下，将其他消息委托给 textinput
	if m.state == stateReady {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKeyMsg 处理键盘事件。
func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// ctrl+c 始终退出；q 仅在非打字状态下退出
	if key == "ctrl+c" || (key == "q" && m.state != stateReady) {
		return m, tea.Quit
	}

	if key == "esc" {
		if m.state == stateError {
			m.state = stateReady
			m.err = nil
			m.statusMsg = statusReady
			return m, nil
		}
	}

	if key == "enter" {
		if m.state == stateReady {
			return m.handleEnterReady()
		}
	}

	if m.state == stateReady && len(m.results) > 0 {
		switch key {
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.updateViewport()
			}
			return m, nil
		case "down":
			if m.selectedIndex < len(m.results)-1 {
				m.selectedIndex++
				m.updateViewport()
			}
			return m, nil
		}
	}

	if m.state == stateReady {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleEnterReady 处理就绪状态下 Enter 键的逻辑：
// 若输入框内容与上次搜索关键词相同且有结果 → 触发下载；
// 否则若输入框有内容 → 触发新搜索。
func (m *model) handleEnterReady() (tea.Model, tea.Cmd) {
	// 输入内容与上次搜索相同且有结果 → 下载选中项
	if len(m.results) > 0 && m.searchInput.Value() == m.lastSearchKeyword {
		m.state = stateDownloading
		m.statusMsg = fmt.Sprintf("正在下载 %s...", m.results[m.selectedIndex].TrainerName)
		m.progress = fling.DownloadProgress{}
		return m, downloadCmd(&m.results[m.selectedIndex], m.config.DownloadPath)
	}
	// 输入内容有变化（或无结果）→ 重新搜索
	if m.searchInput.Value() != "" {
		m.state = stateSearching
		m.statusMsg = "正在搜索..."
		m.results = nil
		m.lastSearchKeyword = m.searchInput.Value()
		return m, searchCmd(m.index, m.searchInput.Value())
	}
	return m, nil
}

// handleConfigLoaded 处理配置加载完成消息。
func (m *model) handleConfigLoaded(msg configLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// 配置加载失败：config 已从 main.go 传入默认值，忽略此错误
		return m, nil
	}
	m.config = msg.config
	return m, nil
}

// handleDataLoaded 处理数据加载完成消息。
func (m *model) handleDataLoaded(msg dataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		logToFile("数据加载失败: " + msg.err.Error())
		m.state = stateError
		m.err = msg.err
		m.statusMsg = "数据加载失败"
		return m, nil
	}

	m.index = msg.index
	m.state = stateReady
	m.statusMsg = statusReady
	return m, nil
}

// handleSearchDone 处理搜索完成消息。
func (m *model) handleSearchDone(msg searchDoneMsg) (tea.Model, tea.Cmd) {
	m.results = msg
	m.state = stateReady
	m.selectedIndex = 0

	if len(msg) == 0 {
		m.statusMsg = "未找到匹配结果"
	} else {
		m.statusMsg = fmt.Sprintf("找到 %d 个结果，按 ↑↓ 选择，Enter 下载", len(msg))
	}
	m.updateViewport()

	return m, nil
}

// handleProgress 处理下载进度更新消息。
func (m *model) handleProgress(msg progressMsg) (tea.Model, tea.Cmd) {
	if m.state == stateDownloading {
		m.progress = fling.DownloadProgress(msg)
	}
	return m, nil
}

// handleDownloadDone 处理下载完成消息。
func (m *model) handleDownloadDone(msg downloadDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		logToFile("下载失败: " + msg.err.Error())
		m.state = stateError
		m.err = msg.err
		m.statusMsg = statusDownloadFailed
		return m, nil
	}

	m.state = stateDone
	m.downloadDestPath = msg.destPath
	m.statusMsg = statusDownloadDone
	return m, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
		return doneTimeoutMsg{}
	})
}

// updateViewport 根据当前搜索结果更新 viewport 内容。
func (m *model) updateViewport() {
	var b strings.Builder

	for i, r := range m.results {
		line := fmt.Sprintf("[%d] %s", i+1, r.TrainerName)
		if i == m.selectedIndex {
			_, _ = b.WriteString(selectedStyle.Render(line))
		} else {
			_, _ = b.WriteString(resultStyle.Render(line))
		}
		_, _ = b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())

	// 确保选中的项目在可视区域内
	if m.selectedIndex < m.viewport.Height {
		m.viewport.SetYOffset(0)
	} else {
		m.viewport.SetYOffset(m.selectedIndex - m.viewport.Height + 2)
	}
}

// View 根据当前状态渲染 TUI 界面。
func (m *model) View() string {
	switch m.state {
	case stateLoading:
		return viewLoading()
	case stateReady:
		return m.viewReady()
	case stateSearching:
		return viewSearching()
	case stateDownloading:
		return m.viewDownloading()
	case stateDone:
		return m.viewDone()
	case stateError:
		return m.viewError()
	default:
		return ""
	}
}

func viewLoading() string {
	return titleStyle.Render("\n  FLiNG 修改器下载器\n\n") +
		"  正在加载 FLiNG 修改器数据...\n\n" +
		helpStyle.Render("  按 Ctrl+C 或 q 退出")
}

// viewReady 渲染就绪界面（搜索框 + 结果列表 + 状态栏）。
func (m *model) viewReady() string {
	var b strings.Builder

	_, _ = b.WriteString(titleStyle.Render("FLiNG 修改器下载器"))
	_, _ = b.WriteString("\n\n")
	_, _ = b.WriteString(m.searchInput.View())
	_, _ = b.WriteString("\n\n")

	if len(m.results) > 0 {
		_, _ = b.WriteString(m.viewport.View())
		_, _ = b.WriteString("\n")
	} else {
		_, _ = b.WriteString("  No results\n\n")
	}

	_, _ = b.WriteString(statusStyle.Render(m.statusMsg))
	_, _ = b.WriteString("\n")
	_, _ = b.WriteString(helpStyle.Render("Press ↑↓ to navigate, Enter to download, q to quit"))

	return b.String()
}

func viewSearching() string {
	return titleStyle.Render("\n  FLiNG 修改器下载器\n\n") +
		"  正在搜索...\n\n" +
		helpStyle.Render("  按 Ctrl+C 或 q 退出")
}

// viewDownloading 渲染下载进度界面，包含状态行和 ASCII 进度条。
func (m *model) viewDownloading() string {
	var b strings.Builder

	_, _ = b.WriteString(titleStyle.Render("\n  FLiNG 修改器下载器\n\n"))

	// 状态行：显示正在下载的修改器名称
	if m.selectedIndex < len(m.results) {
		_, _ = fmt.Fprintf(&b, "  正在下载 %s...\n\n", m.results[m.selectedIndex].TrainerName)
	} else {
		_, _ = b.WriteString("  准备下载...\n\n")
	}

	// 进度条
	_, _ = b.WriteString(renderProgressBar(m.progress))
	_, _ = b.WriteString("\n\n")

	_, _ = b.WriteString(helpStyle.Render("  按 Ctrl+C 或 q 退出"))

	return b.String()
}

// renderProgressBar 根据下载进度渲染 ASCII 进度条。
// 格式: "[=====>    ] 45% 1.2MB / 2.8MB"
// 当 TotalBytes 为 0（未知大小）时只显示已下载字节数。
func renderProgressBar(p fling.DownloadProgress) string {
	if p.TotalBytes <= 0 {
		return fmt.Sprintf("  %s 已下载", formatBytes(p.BytesDownloaded))
	}

	ratio := float64(p.BytesDownloaded) / float64(p.TotalBytes)
	filled := int(ratio * float64(progressBarWidth))
	if filled > progressBarWidth {
		filled = progressBarWidth
	}

	var bar strings.Builder
	_, _ = bar.WriteString("[")
	for i := range progressBarWidth {
		switch {
		case i < filled-1 && filled > 0 && filled < progressBarWidth:
			_, _ = bar.WriteString("=")
		case i == filled-1 && filled > 0 && filled < progressBarWidth:
			_, _ = bar.WriteString(">")
		default:
			if filled >= progressBarWidth {
				_, _ = bar.WriteString("=")
			} else {
				_, _ = bar.WriteString(" ")
			}
		}
	}
	_, _ = bar.WriteString("]")

	pct := fmt.Sprintf("%d%%", int(p.PercentComplete))
	return fmt.Sprintf("  %s %s %s / %s", bar.String(), pct, formatBytes(p.BytesDownloaded), formatBytes(p.TotalBytes))
}

// viewDone 渲染下载完成界面。
func (m *model) viewDone() string {
	var b strings.Builder

	_, _ = b.WriteString(titleStyle.Render("\n  FLiNG 修改器下载器\n\n"))
	_, _ = b.WriteString(successStyle.Render("  下载完成！已保存到 " + m.downloadDestPath))
	_, _ = b.WriteString("\n\n")
	_, _ = b.WriteString(helpStyle.Render("  按 Ctrl+C 或 q 退出"))

	return b.String()
}

// viewError 渲染错误界面。长错误信息会被截断，完整内容写入 fling-data/fling-tui.log。
func (m *model) viewError() string {
	errText := "未知错误"
	if m.err != nil {
		errText = m.err.Error()
		// 截断过长的单行错误消息（保留首行约 70 字符）
		if len(errText) > 70 {
			if idx := strings.Index(errText, ": "); idx > 0 && idx < 70 {
				errText = errText[:idx]
			} else {
				errText = errText[:70]
			}
		}
	}

	errLine := "  错误: " + errText
	if m.statusMsg == statusDownloadFailed {
		errLine = "  下载失败: " + errText
	}

	return titleStyle.Render("\n  FLiNG 修改器下载器\n\n") +
		errorStyle.Render(errLine) +
		"\n\n" +
		helpStyle.Render("  按 Esc 返回") +
		"\n" +
		helpStyle.Render("  完整错误信息见 fling-data/fling-tui.log")
}

// progressTickCmd 返回模拟下载进度更新的定时器命令（桩实现）。
func progressTickCmd() tea.Cmd {
	return nil
}
