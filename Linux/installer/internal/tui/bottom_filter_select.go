package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type bottomFilterSelect struct {
	key         string
	title       string
	description string
	options     []huh.Option[string]
	filtered    []huh.Option[string]
	value       *string
	selected    int
	filter      string
	filtering   bool
	focused     bool
	width       int
	height      int
	theme       *huh.Theme
	onChange    func(string)
}

func newBottomFilterSelect() *bottomFilterSelect {
	return &bottomFilterSelect{
		key:      "bottom-filter-select",
		width:    80,
		height:   12,
		filtered: []huh.Option[string]{},
	}
}

func (s *bottomFilterSelect) Title(title string) *bottomFilterSelect {
	s.title = title
	return s
}

func (s *bottomFilterSelect) Description(description string) *bottomFilterSelect {
	s.description = description
	return s
}

func (s *bottomFilterSelect) Options(options ...huh.Option[string]) *bottomFilterSelect {
	s.options = append([]huh.Option[string]{}, options...)
	s.filtered = append([]huh.Option[string]{}, options...)
	s.selectCurrentValue()
	return s
}

func (s *bottomFilterSelect) Height(height int) *bottomFilterSelect {
	s.height = height
	return s
}

func (s *bottomFilterSelect) Value(value *string) *bottomFilterSelect {
	s.value = value
	s.selectCurrentValue()
	return s
}

func (s *bottomFilterSelect) OnChange(onChange func(string)) *bottomFilterSelect {
	s.onChange = onChange
	return s
}

func (s *bottomFilterSelect) Init() tea.Cmd {
	s.refreshFilter()
	return nil
}

func (s *bottomFilterSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.filtering {
		s.handleFilterKey(keyMsg)
		return s, nil
	}
	s.handleListKey(keyMsg)
	return s, nil
}

func (s *bottomFilterSelect) handleFilterKey(keyMsg tea.KeyMsg) {
	switch keyMsg.String() {
	case "esc":
		s.filter = ""
		s.filtering = false
		s.refreshFilter()
	case "enter":
		s.filtering = false
	case "backspace", "delete":
		if s.filter != "" {
			runes := []rune(s.filter)
			s.filter = string(runes[:len(runes)-1])
			s.refreshFilter()
		}
	case "ctrl+u":
		s.filter = ""
		s.refreshFilter()
	case "up", "ctrl+p":
		s.moveSelected(-1)
	case "down", "ctrl+n":
		s.moveSelected(1)
	default:
		if len(keyMsg.Runes) > 0 {
			s.filter += string(keyMsg.Runes)
			s.refreshFilter()
		}
	}
}

func (s *bottomFilterSelect) handleListKey(keyMsg tea.KeyMsg) {
	switch keyMsg.String() {
	case "/":
		s.filtering = true
	case "esc":
		if s.filter != "" {
			s.filter = ""
			s.refreshFilter()
		}
	case "up", "k", "ctrl+p":
		s.moveSelected(-1)
	case "down", "j", "ctrl+n":
		s.moveSelected(1)
	case "home", "g":
		s.selected = 0
		s.updateValue()
	case "end", "G":
		if len(s.filtered) > 0 {
			s.selected = len(s.filtered) - 1
			s.updateValue()
		}
	}
}

func (s *bottomFilterSelect) refreshFilter() {
	current := s.currentValue()
	s.filtered = s.filtered[:0]
	for _, option := range s.options {
		if s.filter == "" || strings.Contains(strings.ToLower(option.Key), strings.ToLower(s.filter)) {
			s.filtered = append(s.filtered, option)
		}
	}
	s.selected = 0
	for i, option := range s.filtered {
		if option.Value == current {
			s.selected = i
			break
		}
	}
	if len(s.filtered) > 0 && !containsOptionValue(s.filtered, current) {
		s.updateValue()
	}
}

func (s *bottomFilterSelect) moveSelected(delta int) {
	if len(s.filtered) == 0 {
		return
	}
	s.selected += delta
	if s.selected < 0 {
		s.selected = len(s.filtered) - 1
	}
	if s.selected >= len(s.filtered) {
		s.selected = 0
	}
	s.updateValue()
}

func (s *bottomFilterSelect) selectCurrentValue() {
	current := s.currentValue()
	source := s.options
	if len(s.filtered) > 0 {
		source = s.filtered
	}
	for i, option := range source {
		if option.Value == current {
			s.selected = i
			return
		}
	}
}

func (s *bottomFilterSelect) updateValue() {
	if s.value != nil && s.selected >= 0 && s.selected < len(s.filtered) {
		next := s.filtered[s.selected].Value
		if *s.value != next {
			*s.value = next
			if s.onChange != nil {
				s.onChange(next)
			}
		}
	}
}

func (s *bottomFilterSelect) currentValue() string {
	if s.value == nil {
		return ""
	}
	return *s.value
}

func containsOptionValue(options []huh.Option[string], value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}
