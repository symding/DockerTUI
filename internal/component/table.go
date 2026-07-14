package component

import (
	btable "github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

type FilterTable struct {
	Filter  string
	Table   string
	Status  string
	Padding int
}

func SetTableRows(t *btable.Model, columns []btable.Column, rows []btable.Row, cursor int) {
	t.SetStyles(TableStyles())
	t.SetRows(nil)
	t.SetColumns(columns)
	t.SetRows(rows)
	t.SetCursor(cursor)
}

func TableStyles() btable.Styles {
	styles := btable.DefaultStyles()
	styles.Selected = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("236"))
	return styles
}

func FilterTableView(table FilterTable) string {
	parts := make([]string, 0, 3)
	if table.Filter != "" {
		parts = append(parts, table.Filter)
	}
	parts = append(parts, table.Table, table.Status)
	return PaddedLeft(lipgloss.JoinVertical(lipgloss.Left, parts...), table.Padding)
}
