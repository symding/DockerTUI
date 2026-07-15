package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func AppFrame(width int, server, nav, main string) string {
	body := lipgloss.JoinVertical(lipgloss.Left, nav, main)
	return lipgloss.JoinVertical(lipgloss.Left, AppTitle(width, server), OperationHint(width), body)
}

func AppTitle(width int, server string) string {
	if width <= 0 {
		width = 100
	}
	title := " Docker TUI"
	if server != "" {
		server = "Server: " + server
		spaces := width - ansi.StringWidth(title) - ansi.StringWidth(server)
		if spaces > 0 {
			title += strings.Repeat(" ", spaces) + server
		}
	}
	return lipgloss.NewStyle().
		Width(width).
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Background(lipgloss.Color("236")).
		Render(title)
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
		Render("[Tab] switch [/] filter  [SPACE] refresh")
}
