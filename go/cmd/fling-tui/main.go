// fling-tui 是 FLiNG 修改器下载器的终端交互界面（TUI）。
// 使用 Bubble Tea 框架实现 Elm Architecture 的终端应用。
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"fling-tui/internal/store"
	"fling-tui/internal/tui"
)

func main() {
	cfg, err := store.LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fling-tui 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 不使用 tea.WithAltScreen() — 保护终端历史记录
	m := tui.InitialModel(cfg)
	p := tea.NewProgram(&m)

	_, err = p.Run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fling-tui 运行出错: %v\n", err)
		os.Exit(1)
	}
}
