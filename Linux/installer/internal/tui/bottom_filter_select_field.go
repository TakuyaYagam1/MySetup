package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func (s *bottomFilterSelect) Focus() tea.Cmd {
	s.focused = true
	return nil
}

func (s *bottomFilterSelect) Blur() tea.Cmd {
	s.focused = false
	s.filtering = false
	return nil
}

func (s *bottomFilterSelect) Error() error { return nil }

func (s *bottomFilterSelect) Run() error {
	program := tea.NewProgram(s)
	_, err := program.Run()
	return err
}

func (s *bottomFilterSelect) RunAccessible(w io.Writer, _ io.Reader) error {
	for i, option := range s.options {
		_, _ = fmt.Fprintf(w, "%d. %s\n", i+1, option.Key)
	}
	return nil
}

func (s *bottomFilterSelect) Skip() bool { return false }

func (s *bottomFilterSelect) Zoom() bool { return false }

func (s *bottomFilterSelect) KeyBinds() []key.Binding {
	if s.filtering {
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "results")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep search")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next field")),
		key.NewBinding(key.WithKeys("ctrl+x", "shift+enter"), key.WithHelp("ctrl+x", "save section")),
	}
}

func (s *bottomFilterSelect) WithTheme(theme *huh.Theme) huh.Field {
	s.theme = theme
	return s
}

func (s *bottomFilterSelect) WithAccessible(bool) huh.Field { return s }

func (s *bottomFilterSelect) WithKeyMap(*huh.KeyMap) huh.Field { return s }

func (s *bottomFilterSelect) WithWidth(width int) huh.Field {
	s.width = width
	return s
}

func (s *bottomFilterSelect) WithHeight(height int) huh.Field {
	s.height = height
	return s
}

func (s *bottomFilterSelect) WithPosition(huh.FieldPosition) huh.Field { return s }

func (s *bottomFilterSelect) GetKey() string { return s.key }

func (s *bottomFilterSelect) GetValue() any {
	return s.currentValue()
}

func (s *bottomFilterSelect) GetFiltering() bool {
	return s.filtering
}

func (s *bottomFilterSelect) HasFilter() bool {
	return s.filter != ""
}
