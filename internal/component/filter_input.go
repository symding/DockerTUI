package component

import "github.com/charmbracelet/bubbles/textinput"

func NewFilterInput() textinput.Model {
	input := textinput.New()
	input.Prompt = "Filter: "
	input.Placeholder = "type to filter"
	return input
}
