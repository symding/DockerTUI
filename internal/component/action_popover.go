package component

import (
	btable "github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func SetActionRows(t *btable.Model, rows []btable.Row, popoverWidth int) {
	t.SetStyles(ActionTableStyles())
	t.SetRows(nil)
	t.SetColumns(actionColumns(popoverWidth))
	t.SetRows(rows)
	t.SetWidth(popoverWidth - 4)
	t.SetHeight(max(2, len(rows)+1))
}

func ResizeActionTable(t *btable.Model, popoverWidth int) {
	t.SetColumns(actionColumns(popoverWidth))
	t.SetWidth(popoverWidth - 4)
	t.SetHeight(max(2, len(t.Rows())+1))
}

func actionColumns(popoverWidth int) []btable.Column {
	return []btable.Column{{Title: "Action", Width: popoverWidth - 6}}
}

func ActionTableStyles() btable.Styles {
	styles := TableStyles()
	styles.Cell = lipgloss.NewStyle().Padding(0, 0, 0, 2)
	return styles
}

func ActionPopover(width int, content string) string {
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		Render(content)
}
