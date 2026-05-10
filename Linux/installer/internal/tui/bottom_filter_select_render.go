package tui

import (
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

func (s *bottomFilterSelect) View() string {
	styles := s.activeStyles()
	var lines []string
	if s.title != "" {
		lines = append(lines, styles.Title.Render(s.title))
	}
	if s.description != "" {
		lines = append(lines, styles.Description.Render(s.description))
	}
	lines = append(lines, s.visibleOptions(styles)...)
	lines = append(lines, "")
	lines = append(lines, s.filterView(styles))
	return styles.Base.Width(s.width).Height(s.height).Render(strings.Join(lines, "\n"))
}

func (s *bottomFilterSelect) visibleOptions(styles *huh.FieldStyles) []string {
	if len(s.filtered) == 0 {
		return []string{styles.Description.Render("  No matches")}
	}
	height := s.optionHeight()
	start := s.selected - height/2
	if start < 0 {
		start = 0
	}
	if start+height > len(s.filtered) {
		start = maxInt(0, len(s.filtered)-height)
	}
	end := minInt(len(s.filtered), start+height)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		prefix := "  "
		optionStyle := styles.UnselectedOption
		if i == s.selected {
			prefix = styles.SelectSelector.String()
			optionStyle = styles.SelectedOption
		}
		lines = append(lines, prefix+optionStyle.Render(s.filtered[i].Key))
	}
	return lines
}

func (s *bottomFilterSelect) filterView(styles *huh.FieldStyles) string {
	if s.filtering {
		return styles.TextInput.Prompt.Render("Search: /") + styles.TextInput.Text.Render(s.filter)
	}
	if s.filter != "" {
		return styles.Description.Render("Search: /" + s.filter + "  (Esc clears)")
	}
	return styles.Description.Render("Search: press / to filter")
}

func (s *bottomFilterSelect) optionHeight() int {
	used := 3
	if s.title != "" {
		used++
	}
	if s.description != "" {
		used += lipgloss.Height(s.description)
	}
	return maxInt(3, s.height-used)
}

func (s *bottomFilterSelect) activeStyles() *huh.FieldStyles {
	t := s.theme
	if t == nil {
		t = theme()
	}
	if s.focused {
		return &t.Focused
	}
	return &t.Blurred
}
