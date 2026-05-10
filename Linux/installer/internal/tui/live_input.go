package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type liveInput struct {
	key         string
	title       string
	description string
	value       *string
	input       textinput.Model
	focused     bool
	width       int
	height      int
	theme       *huh.Theme
}

func newLiveInput() *liveInput {
	return &liveInput{
		key:    "live-input",
		input:  textinput.New(),
		width:  80,
		height: 3,
	}
}

func (i *liveInput) Title(title string) *liveInput {
	i.title = title
	return i
}

func (i *liveInput) Description(description string) *liveInput {
	i.description = description
	return i
}

func (i *liveInput) Value(value *string) *liveInput {
	i.value = value
	i.syncFromValue()
	return i
}

func (i *liveInput) Init() tea.Cmd {
	i.syncFromValue()
	i.input.Blur()
	return nil
}

func (i *liveInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if !i.focused {
		i.syncFromValue()
	}
	i.input, cmd = i.input.Update(msg)
	i.syncToValue()
	return i, cmd
}

func (i *liveInput) View() string {
	if !i.focused {
		i.syncFromValue()
	}
	styles := i.activeStyles()
	i.input.PlaceholderStyle = styles.TextInput.Placeholder
	i.input.PromptStyle = styles.TextInput.Prompt
	i.input.Cursor.Style = styles.TextInput.Cursor
	i.input.Cursor.TextStyle = styles.TextInput.CursorText
	i.input.TextStyle = styles.TextInput.Text

	var lines []string
	if i.title != "" {
		lines = append(lines, styles.Title.Render(i.title))
	}
	if i.description != "" {
		lines = append(lines, styles.Description.Render(i.description))
	}
	lines = append(lines, i.input.View())
	return styles.Base.Width(i.width).Height(i.height).Render(strings.Join(lines, "\n"))
}

func (i *liveInput) syncFromValue() {
	if i.value != nil && i.input.Value() != *i.value {
		i.input.SetValue(*i.value)
	}
}

func (i *liveInput) syncToValue() {
	if i.value != nil {
		*i.value = i.input.Value()
	}
}

func (i *liveInput) activeStyles() *huh.FieldStyles {
	t := i.theme
	if t == nil {
		t = theme()
	}
	if i.focused {
		return &t.Focused
	}
	return &t.Blurred
}

func (i *liveInput) Focus() tea.Cmd {
	i.focused = true
	return i.input.Focus()
}

func (i *liveInput) Blur() tea.Cmd {
	i.focused = false
	i.syncToValue()
	i.input.Blur()
	return nil
}

func (i *liveInput) Error() error { return nil }

func (i *liveInput) Run() error {
	program := tea.NewProgram(i)
	_, err := program.Run()
	return err
}

func (i *liveInput) RunAccessible(w io.Writer, _ io.Reader) error {
	_, _ = fmt.Fprintf(w, "%s: %s\n", i.title, i.currentValue())
	return nil
}

func (i *liveInput) Skip() bool { return false }

func (i *liveInput) Zoom() bool { return false }

func (i *liveInput) KeyBinds() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("tab", "enter"), key.WithHelp("tab/enter", "next field")),
		key.NewBinding(key.WithKeys("ctrl+x", "shift+enter"), key.WithHelp("ctrl+x", "save section")),
	}
}

func (i *liveInput) WithTheme(theme *huh.Theme) huh.Field {
	i.theme = theme
	return i
}

func (i *liveInput) WithAccessible(bool) huh.Field { return i }

func (i *liveInput) WithKeyMap(*huh.KeyMap) huh.Field { return i }

func (i *liveInput) WithWidth(width int) huh.Field {
	i.width = width
	i.input.Width = maxInt(1, width-4)
	return i
}

func (i *liveInput) WithHeight(height int) huh.Field {
	i.height = height
	return i
}

func (i *liveInput) WithPosition(huh.FieldPosition) huh.Field { return i }

func (i *liveInput) GetKey() string { return i.key }

func (i *liveInput) GetValue() any {
	return i.currentValue()
}

func (i *liveInput) currentValue() string {
	if i.value == nil {
		return i.input.Value()
	}
	return *i.value
}
