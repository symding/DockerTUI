package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type ConfirmFooter struct {
	Confirm string
	Cancel  string
}

func ConfirmDialog(header string, body []string, footer ConfirmFooter) string {
	footerText := strings.TrimSpace(strings.Join([]string{footer.Confirm, footer.Cancel}, "    "))
	footerView := strings.TrimSpace(strings.Join([]string{
		coloredFirstWord(footer.Confirm, "42"),
		coloredFirstWord(footer.Cancel, "196"),
	}, "    "))
	lines := []string{header, ""}
	lines = append(lines, body...)
	lines = append(lines, "", footerText)
	viewLines := append([]string(nil), lines...)
	viewLines[0] = ColoredTitle(header, "226")
	viewLines[len(viewLines)-1] = footerView
	return lipgloss.NewStyle().
		Width(confirmDialogWidth(lines)).
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(strings.Join(viewLines, "\n"))
}

func coloredFirstWord(value, color string) string {
	key, rest, _ := strings.Cut(value, " ")
	key = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(key)
	if rest == "" {
		return key
	}
	return key + " " + rest
}

func confirmDialogWidth(lines []string) int {
	width := 48
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line)+4)
	}
	return min(width, 72)
}
