package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func Overlay(base, popup string, width, height int) string {
	if width <= 0 {
		width = lipgloss.Width(base)
	}
	if height <= 0 {
		height = lipgloss.Height(base)
	}
	popupW := lipgloss.Width(popup)
	popupH := lipgloss.Height(popup)
	x := max(0, (width-popupW)/2)
	y := max(0, (height-popupH)/2)
	return OverlayAt(base, popup, width, height, x, y)
}

func OverlayAt(base, popup string, width, height, x, y int) string {
	if width <= 0 {
		width = lipgloss.Width(base)
	}
	if height <= 0 {
		height = lipgloss.Height(base)
	}
	popupW := lipgloss.Width(popup)
	popupH := lipgloss.Height(popup)
	x = min(max(0, x), max(0, width-popupW))
	y = min(max(0, y), max(0, height-popupH))
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	popupLines := strings.Split(popup, "\n")
	for i, popupLine := range popupLines {
		row := y + i
		if row >= len(baseLines) {
			break
		}
		line := PadANSI(baseLines[row], width)
		left := ansi.Cut(line, 0, x)
		right := ansi.Cut(line, x+ansi.StringWidth(popupLine), width)
		baseLines[row] = left + popupLine + right
	}
	return strings.Join(baseLines, "\n")
}
