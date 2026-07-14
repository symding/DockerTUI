package component

import "github.com/charmbracelet/lipgloss"

func LogPanel(info, logs, status string) string {
	logTitle := ColoredTitle("Logs", "205")
	return lipgloss.JoinVertical(lipgloss.Left, info, logTitle, logs, status)
}

func LogModal(title string, width, height int, logs string) string {
	content := lipgloss.NewStyle().
		Padding(1, 2).
		Render(logs)
	panel := TitledPanel(title, "205", "63", width, height, content)
	return lipgloss.NewStyle().
		Margin(1, 2).
		Render(panel)
}
