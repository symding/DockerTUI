package component

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func NewNavDelegate(width int) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		PaddingLeft(1)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Width(width).
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Background(lipgloss.Color("236")).
		PaddingLeft(1)
	return delegate
}

func NavBar(titles []string, selected int) string {
	parts := make([]string, 0, len(titles))
	for i, title := range titles {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)
		if i == selected {
			style = style.
				Bold(true).
				Foreground(lipgloss.Color("39")).
				Background(lipgloss.Color("236"))
		}
		parts = append(parts, style.Render(title))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
