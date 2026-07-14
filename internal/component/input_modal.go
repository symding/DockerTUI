package component

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func NewTextInput(width int) textinput.Model {
	input := textinput.New()
	input.Width = width
	return input
}

func InputModal(width int, title, input string) string {
	title = ColoredTitle(title, "42")
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, input))
}
