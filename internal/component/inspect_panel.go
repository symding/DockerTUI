package component

import "github.com/charmbracelet/lipgloss"

func InspectPanel(inspect, status string, padding int) string {
	return PaddedLeft(lipgloss.JoinVertical(lipgloss.Left, inspect, status), padding)
}
