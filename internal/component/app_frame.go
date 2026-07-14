package component

import "github.com/charmbracelet/lipgloss"

func AppFrame(width int, nav, main string) string {
	body := lipgloss.JoinVertical(lipgloss.Left, nav, main)
	return lipgloss.JoinVertical(lipgloss.Left, AppTitle(width), OperationHint(width), body)
}

func AppTitle(width int) string {
	if width <= 0 {
		width = 100
	}
	return lipgloss.NewStyle().
		Width(width).
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Background(lipgloss.Color("236")).
		PaddingLeft(1).
		Render("Docker TUI")
}

func OperationHint(width int) string {
	if width <= 0 {
		width = 100
	}
	return lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color("244")).
		Background(lipgloss.Color("236")).
		PaddingLeft(1).
		Render("Tab 切换导航  ↑/↓ 切换列表选项  /筛选列表")
}
