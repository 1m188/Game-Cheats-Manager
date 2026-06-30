// Package tui 提供 Bubble Tea TUI 的状态机模型测试。
//
//nolint:goconst // 测试数据中的字符串常量无需提取
package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"fling-tui/internal/fling"
)

// assertAnError 是一个实现了 error 接口的简单测试类型。
type assertAnError struct {
	msg string
}

func (e assertAnError) Error() string {
	return e.msg
}

func testConfig() *fling.Config {
	return &fling.Config{
		CacheTTLHours: 24,
		DownloadPath:  "/tmp/fling-test/trainers/",
	}
}

func TestInitialModel_stateLoading(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)

	if m.state != stateLoading {
		t.Errorf("期望初始状态为 stateLoading (%d), 实际为 %d", stateLoading, m.state)
	}

	if m.config != cfg {
		t.Error("期望 model.config 与传入的 config 相同")
	}

	if m.searchInput.Placeholder != "搜索游戏名..." {
		t.Errorf("期望 searchInput placeholder 为 '搜索游戏名...', 实际为 '%s'", m.searchInput.Placeholder)
	}

	if !m.searchInput.Focused() {
		t.Error("期望 searchInput 初始聚焦")
	}

	if m.selectedIndex != 0 {
		t.Errorf("期望 selectedIndex 初始为 0, 实际为 %d", m.selectedIndex)
	}
}

func TestInit_returnsCommands(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	cmd := m.Init()

	if cmd == nil {
		t.Error("期望 Init 返回非 nil 的 tea.Cmd")
	}
}

func TestUpdate_qKey_quits(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateReady

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("期望按 q 返回 quit 命令")
	}
}

func TestUpdate_ctrlC_quits(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("期望按 ctrl+c 返回 quit 命令")
	}
}

func TestUpdate_escFromError_returnsReady(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateError
	m.err = assertAnError{msg: "测试错误"}

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateReady {
		t.Errorf("期望状态从 error 切换为 ready (%d), 实际为 %d", stateReady, tm.state)
	}
	if tm.err != nil {
		t.Error("期望返回 ready 后 error 被清空")
	}
}

func TestUpdate_enterInReady_triggersSearch(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateReady
	m.searchInput = textinput.New()
	m.searchInput.SetValue("DREDGE")

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateSearching {
		t.Errorf("期望状态从 ready 切换为 searching (%d), 实际为 %d", stateSearching, tm.state)
	}
	if cmd == nil {
		t.Error("期望按 Enter 返回搜索命令")
	}
}

func TestUpdate_dataLoaded_success(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)

	dummyIndex := &fling.Index{}
	msg := dataLoadedMsg{index: dummyIndex, err: nil}

	newModel, _ := m.Update(msg)
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateReady {
		t.Errorf("期望数据加载成功后状态为 ready (%d), 实际为 %d", stateReady, tm.state)
	}
	if tm.index != dummyIndex {
		t.Error("期望 model.index 被正确设置")
	}
}

func TestUpdate_dataLoaded_failure(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)

	msg := dataLoadedMsg{index: nil, err: assertAnError{msg: "网络错误"}}

	newModel, _ := m.Update(msg)
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateError {
		t.Errorf("期望数据加载失败后状态为 error (%d), 实际为 %d", stateError, tm.state)
	}
	if tm.err == nil {
		t.Error("期望 model.err 不为 nil")
	}
}

func TestUpdate_searchDone_setsResults(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateSearching

	results := []fling.SearchResult{
		{GameName: "DREDGE", TrainerName: "[FL] DREDGE Trainer", Origin: "fling_archive"},
	}
	msg := searchDoneMsg(results)

	newModel, _ := m.Update(msg)
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateReady {
		t.Errorf("期望搜索完成后状态为 ready (%d), 实际为 %d", stateReady, tm.state)
	}
	if len(tm.results) != 1 {
		t.Errorf("期望 results 包含 1 个结果, 实际为 %d", len(tm.results))
	}
}

func TestUpdate_escInNonError_noChange(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateReady

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateReady {
		t.Errorf("期望在 non-error 状态按 Esc 不改变状态, 实际变为 %d", tm.state)
	}
}

func TestView_loading_displaysMessage(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)

	view := m.View()
	if view == "" {
		t.Error("期望 View 在 loading 状态返回非空字符串")
	}
}

func TestView_ready_displaysSearch(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateReady

	view := m.View()
	if view == "" {
		t.Error("期望 View 在 ready 状态返回非空字符串")
	}
}

func TestView_error_displaysMessage(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateError
	m.err = assertAnError{msg: "测试错误"}

	view := m.View()
	if view == "" {
		t.Error("期望 View 在 error 状态返回非空字符串")
	}
}

// --- 下载进度相关测试 ---

func TestView_downloading_showsProgressBar(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDownloading
	m.results = []fling.SearchResult{
		{GameName: "DREDGE", TrainerName: "[FL] DREDGE Trainer", Origin: "fling_archive"},
	}
	m.selectedIndex = 0
	m.progress = fling.DownloadProgress{
		BytesDownloaded: 1200000,
		TotalBytes:      2800000,
		PercentComplete: 42.85,
	}

	view := m.View()

	if !strings.Contains(view, "1.1MB") {
		t.Errorf("期望 view 包含已下载字节数 '1.1MB', 实际: %s", view)
	}
	if !strings.Contains(view, "2.7MB") {
		t.Errorf("期望 view 包含总字节数 '2.7MB', 实际: %s", view)
	}
	if !strings.Contains(view, "[") || !strings.Contains(view, "]") {
		t.Error("期望 view 包含进度条括号")
	}
	if !strings.Contains(view, "42%") {
		t.Errorf("期望 view 包含百分比 '42%%', 实际: %s", view)
	}
	if !strings.Contains(view, "DREDGE") {
		t.Errorf("期望 view 包含修改器名称 'DREDGE', 实际: %s", view)
	}
}

func TestView_downloading_unknownTotalSize(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDownloading
	m.results = []fling.SearchResult{
		{GameName: "DREDGE", TrainerName: "[FL] DREDGE Trainer", Origin: "fling_archive"},
	}
	m.selectedIndex = 0
	m.progress = fling.DownloadProgress{
		BytesDownloaded: 500000,
		TotalBytes:      0, // 未知总大小
	}

	view := m.View()

	if !strings.Contains(view, "已下载") {
		t.Errorf("期望 view 在未知总大小时显示 '已下载', 实际: %s", view)
	}
	if strings.Contains(view, "%") {
		t.Error("期望 view 在未知总大小时不显示百分比")
	}
}

func TestUpdate_progressMsg_updatesProgress(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDownloading

	progress := fling.DownloadProgress{
		BytesDownloaded: 1024,
		TotalBytes:      4096,
		PercentComplete: 25.0,
	}

	newModel, _ := m.Update(progressMsg(progress))
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.progress.BytesDownloaded != 1024 {
		t.Errorf("期望 progress.BytesDownloaded = 1024, 实际为 %d", tm.progress.BytesDownloaded)
	}
	if tm.progress.TotalBytes != 4096 {
		t.Errorf("期望 progress.TotalBytes = 4096, 实际为 %d", tm.progress.TotalBytes)
	}
	if tm.progress.PercentComplete != 25.0 {
		t.Errorf("期望 progress.PercentComplete = 25.0, 实际为 %f", tm.progress.PercentComplete)
	}
}

func TestUpdate_progressMsg_ignoredWhenNotDownloading(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateReady
	m.progress = fling.DownloadProgress{
		BytesDownloaded: 100,
		TotalBytes:      200,
	}

	progress := fling.DownloadProgress{
		BytesDownloaded: 9999,
		TotalBytes:      9999,
	}

	newModel, _ := m.Update(progressMsg(progress))
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.progress.BytesDownloaded != 100 {
		t.Errorf("期望在非 downloading 状态 progress 不被更新, 实际 BytesDownloaded = %d", tm.progress.BytesDownloaded)
	}
}

func TestUpdate_downloadDone_success_transitionsToDone(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDownloading

	msg := downloadDoneMsg{
		err:      nil,
		destPath: "./fling-data/trainers/[FL] DREDGE Trainer/",
	}

	newModel, cmd := m.Update(msg)
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateDone {
		t.Errorf("期望下载成功后状态为 done (%d), 实际为 %d", stateDone, tm.state)
	}
	if tm.downloadDestPath != "./fling-data/trainers/[FL] DREDGE Trainer/" {
		t.Errorf("期望 downloadDestPath 被正确设置, 实际为 %s", tm.downloadDestPath)
	}
	if tm.statusMsg != statusDownloadDone {
		t.Errorf("期望 statusMsg 为 '%s', 实际为 %s", statusDownloadDone, tm.statusMsg)
	}
	if cmd == nil {
		t.Error("期望返回 tea.Tick 命令用于自动跳转")
	}
}

func TestUpdate_downloadDone_failure_transitionsToError(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDownloading

	downloadErr := assertAnError{msg: "网络连接中断"}
	msg := downloadDoneMsg{
		err:      downloadErr,
		destPath: "",
	}

	newModel, cmd := m.Update(msg)
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateError {
		t.Errorf("期望下载失败后状态为 error (%d), 实际为 %d", stateError, tm.state)
	}
	if tm.err == nil {
		t.Error("期望 err 被设置")
	}
	if tm.statusMsg != statusDownloadFailed {
		t.Errorf("期望 statusMsg 为 '%s', 实际为 %s", statusDownloadFailed, tm.statusMsg)
	}
	if cmd != nil {
		t.Error("期望下载失败不返回后续命令")
	}
}

func TestUpdate_doneTimeout_transitionsToReady(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDone
	m.downloadDestPath = "./fling-data/trainers/some-trainer/"
	m.statusMsg = statusDownloadDone

	newModel, _ := m.Update(doneTimeoutMsg{})
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateReady {
		t.Errorf("期望倒计时到期后状态为 ready (%d), 实际为 %d", stateReady, tm.state)
	}
	if tm.downloadDestPath != "" {
		t.Errorf("期望 downloadDestPath 被清空, 实际为 %s", tm.downloadDestPath)
	}
	if tm.statusMsg != statusReady {
		t.Errorf("期望 statusMsg 恢复为就绪状态, 实际为 %s", tm.statusMsg)
	}
}

func TestView_done_displaysSuccessMessage(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDone
	m.downloadDestPath = "./fling-data/trainers/[FL] DREDGE Trainer/"
	m.statusMsg = statusDownloadDone

	view := m.View()

	if !strings.Contains(view, statusDownloadDone) {
		t.Errorf("期望 view 包含 '%s', 实际: %s", statusDownloadDone, view)
	}
	if !strings.Contains(view, "./fling-data/trainers/[FL] DREDGE Trainer/") {
		t.Errorf("期望 view 包含下载路径, 实际: %s", view)
	}
}

func TestView_downloading_showsStatusLine(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateDownloading
	m.results = []fling.SearchResult{
		{GameName: "DREDGE", TrainerName: "[FL] DREDGE Trainer", Origin: "fling_archive"},
	}
	m.selectedIndex = 0

	view := m.View()

	if !strings.Contains(view, "正在下载") {
		t.Errorf("期望 view 包含 '正在下载', 实际: %s", view)
	}
	if !strings.Contains(view, "[FL] DREDGE Trainer") {
		t.Errorf("期望 view 包含修改器名称 '[FL] DREDGE Trainer', 实际: %s", view)
	}
}

func TestUpdate_enterOnResults_triggersDownload(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateReady
	m.results = []fling.SearchResult{
		{GameName: "DREDGE", TrainerName: "[FL] DREDGE Trainer", Origin: "fling_main"},
	}
	m.selectedIndex = 0

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm, ok := newModel.(*model)
	if !ok {
		t.Fatalf("期望 newModel 为 *model 类型, 实际为 %T", newModel)
	}

	if tm.state != stateDownloading {
		t.Errorf("期望在有结果时按 Enter 切换为 downloading (%d), 实际为 %d", stateDownloading, tm.state)
	}
	if cmd == nil {
		t.Error("期望按 Enter 触发下载命令")
	}
	if tm.progress.BytesDownloaded != 0 || tm.progress.TotalBytes != 0 {
		t.Error("期望下载开始时 progress 被重置为零值")
	}
}

func TestView_error_showsDownloadFailedPrefix(t *testing.T) {
	cfg := testConfig()
	m := InitialModel(cfg)
	m.state = stateError
	m.err = assertAnError{msg: "网络连接中断"}
	m.statusMsg = statusDownloadFailed

	view := m.View()

	if !strings.Contains(view, statusDownloadFailed) {
		t.Errorf("期望 view 包含 '%s', 实际: %s", statusDownloadFailed, view)
	}
	if !strings.Contains(view, "网络连接中断") {
		t.Errorf("期望 view 包含错误详情, 实际: %s", view)
	}
	if !strings.Contains(view, "按 Esc 返回") {
		t.Errorf("期望 view 包含 '按 Esc 返回', 实际: %s", view)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"bytes", 500, "500B"},
		{"zero bytes", 0, "0B"},
		{"negative bytes", -1, "0B"},
		{"kilobytes", 1024, "1KB"},
		{"kilobytes round", 1536, "1KB"},
		{"megabytes", 1048576, "1.0MB"},
		{"megabytes point", 1572864, "1.5MB"},
		{"gigabytes", 1073741824, "1.0GB"},
		{"decimal megabytes", 2300000, "2.2MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.n)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
