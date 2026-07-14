package component

import "github.com/charmbracelet/lipgloss"

func ColoredTitle(title, color string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(color)).
		Render(title)
}
