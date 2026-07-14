package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TitledPanel(title, titleColor, borderColor string, width, height int, content string) string {
	return TitledPanelActive(title, titleColor, borderColor, width, height, content, false)
}

func TitledPanelActive(title, titleColor, borderColor string, width, height int, content string, active bool) string {
	width = max(3, width)
	height = max(3, height)
	innerW := width - 2
	innerH := height - 2
	content = lipgloss.NewStyle().
		Width(innerW).
		Height(innerH).
		Render(content)

	lines := strings.Split(content, "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	side := "│"
	bottomLeft := "└"
	bottom := "─"
	bottomRight := "┘"
	if active {
		side = "║"
		bottomLeft = "╚"
		bottom = "═"
		bottomRight = "╝"
	}
	var out strings.Builder
	out.WriteString(titledPanelTop(title, titleColor, borderColor, width, active))
	for _, line := range lines {
		line = PadANSI(ansi.Cut(line, 0, innerW), innerW)
		out.WriteByte('\n')
		out.WriteString(borderStyle.Render(side))
		out.WriteString(line)
		out.WriteString(borderStyle.Render(side))
	}
	out.WriteByte('\n')
	out.WriteString(borderStyle.Render(bottomLeft + strings.Repeat(bottom, innerW) + bottomRight))
	return out.String()
}

func titledPanelTop(title, titleColor, borderColor string, width int, active bool) string {
	title = ansi.Cut(title, 0, width-3)
	titleW := ansi.StringWidth(title)
	left := "┌─"
	fillerChar := "─"
	right := "┐"
	if active {
		left = "╔═"
		fillerChar = "═"
		right = "╗"
	}
	filler := strings.Repeat(fillerChar, max(0, width-3-titleW))
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(titleColor))
	return borderStyle.Render(left) + titleStyle.Render(title) + borderStyle.Render(filler+right)
}

func PaddedLeft(content string, padding int) string {
	return lipgloss.NewStyle().PaddingLeft(padding).Render(content)
}
