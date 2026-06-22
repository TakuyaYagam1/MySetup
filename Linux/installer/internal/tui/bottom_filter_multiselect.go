package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type bottomFilterMultiSelect struct {
	key         string
	title       string
	description string
	options     []huh.Option[string]
	filtered    []huh.Option[string]
	value       *[]string
	selected    map[string]bool
	cursor      int
	filter      string
	filtering   bool
	focused     bool
	width       int
	height      int
	theme       *huh.Theme
	validate    func([]string) error
}

func newBottomFilterMultiSelect() *bottomFilterMultiSelect {
	return &bottomFilterMultiSelect{
		key:      "bottom-filter-multiselect",
		width:    80,
		height:   12,
		filtered: []huh.Option[string]{},
		selected: map[string]bool{},
	}
}

func (s *bottomFilterMultiSelect) Title(title string) *bottomFilterMultiSelect {
	s.title = title
	return s
}

func (s *bottomFilterMultiSelect) Description(description string) *bottomFilterMultiSelect {
	s.description = description
	return s
}

func (s *bottomFilterMultiSelect) Options(options ...huh.Option[string]) *bottomFilterMultiSelect {
	s.options = append([]huh.Option[string]{}, options...)
	s.syncSelectedFromValue()
	s.refreshFilter()
	return s
}

func (s *bottomFilterMultiSelect) Height(height int) *bottomFilterMultiSelect {
	s.height = height
	return s
}

func (s *bottomFilterMultiSelect) Value(value *[]string) *bottomFilterMultiSelect {
	s.value = value
	s.syncSelectedFromValue()
	s.refreshFilter()
	return s
}

func (s *bottomFilterMultiSelect) Validate(validate func([]string) error) *bottomFilterMultiSelect {
	s.validate = validate
	return s
}

func (s *bottomFilterMultiSelect) Init() tea.Cmd {
	s.syncSelectedFromValue()
	s.refreshFilter()
	return nil
}

func (s *bottomFilterMultiSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (s *bottomFilterMultiSelect) handleFilterKey(keyMsg tea.KeyMsg) {
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
		s.moveCursor(-1)
	case "down", "ctrl+n":
		s.moveCursor(1)
	default:
		if len(keyMsg.Runes) > 0 {
			s.filter += string(keyMsg.Runes)
			s.refreshFilter()
		}
	}
}

func (s *bottomFilterMultiSelect) handleListKey(keyMsg tea.KeyMsg) {
	switch keyMsg.String() {
	case "/":
		s.filtering = true
	case "esc":
		if s.filter != "" {
			s.filter = ""
			s.refreshFilter()
		}
	case "up", "k", "ctrl+p":
		s.moveCursor(-1)
	case "down", "j", "ctrl+n":
		s.moveCursor(1)
	case "home", "g":
		s.cursor = 0
	case "end", "G":
		if len(s.filtered) > 0 {
			s.cursor = len(s.filtered) - 1
		}
	case " ":
		s.toggleCurrent()
	}
}

func (s *bottomFilterMultiSelect) refreshFilter() {
	current := s.currentCursorValue()
	s.filtered = s.filtered[:0]
	for _, option := range s.options {
		if s.filter == "" || strings.Contains(strings.ToLower(option.Key), strings.ToLower(s.filter)) {
			s.filtered = append(s.filtered, option)
		}
	}
	s.cursor = 0
	for i, option := range s.filtered {
		if option.Value == current {
			s.cursor = i
			break
		}
	}
	if len(s.filtered) > 0 && s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
}

func (s *bottomFilterMultiSelect) moveCursor(delta int) {
	if len(s.filtered) == 0 {
		return
	}
	s.cursor += delta
	if s.cursor < 0 {
		s.cursor = len(s.filtered) - 1
	}
	if s.cursor >= len(s.filtered) {
		s.cursor = 0
	}
}

func (s *bottomFilterMultiSelect) toggleCurrent() {
	if len(s.filtered) == 0 || s.cursor < 0 || s.cursor >= len(s.filtered) {
		return
	}
	value := s.filtered[s.cursor].Value
	s.selected[value] = !s.selected[value]
	if !s.selected[value] {
		delete(s.selected, value)
	}
	s.syncValue()
}

func (s *bottomFilterMultiSelect) syncSelectedFromValue() {
	if s.selected == nil {
		s.selected = map[string]bool{}
	}
	clear(s.selected)
	if s.value == nil {
		return
	}
	for _, value := range *s.value {
		s.selected[value] = true
	}
}

func (s *bottomFilterMultiSelect) syncValue() {
	if s.value == nil {
		return
	}
	next := make([]string, 0, len(s.selected))
	for _, option := range s.options {
		if s.selected[option.Value] {
			next = append(next, option.Value)
		}
	}
	*s.value = next
}

func (s *bottomFilterMultiSelect) currentCursorValue() string {
	if len(s.filtered) == 0 || s.cursor < 0 || s.cursor >= len(s.filtered) {
		return ""
	}
	return s.filtered[s.cursor].Value
}

func (s *bottomFilterMultiSelect) Focus() tea.Cmd {
	s.focused = true
	return nil
}

func (s *bottomFilterMultiSelect) Blur() tea.Cmd {
	s.focused = false
	s.filtering = false
	s.syncValue()
	return nil
}

func (s *bottomFilterMultiSelect) Error() error {
	if s.validate == nil {
		return nil
	}
	return s.validate(s.currentValue())
}

func (s *bottomFilterMultiSelect) Run() error {
	program := tea.NewProgram(s)
	_, err := program.Run()
	return err
}

func (s *bottomFilterMultiSelect) RunAccessible(w io.Writer, _ io.Reader) error {
	for i, option := range s.options {
		_, _ = fmt.Fprintf(w, "%d. %s\n", i+1, option.Key)
	}
	return nil
}

func (s *bottomFilterMultiSelect) Skip() bool { return false }

func (s *bottomFilterMultiSelect) Zoom() bool { return false }

func (s *bottomFilterMultiSelect) KeyBinds() []key.Binding {
	if s.filtering {
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down", "ctrl+p", "ctrl+n"), key.WithHelp("↑/↓", "results")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "keep search")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear search")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down", "k", "j"), key.WithHelp("↑/↓/j/k", "choose")),
		key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		key.NewBinding(key.WithKeys("tab", "enter"), key.WithHelp("tab/enter", "next field")),
		key.NewBinding(key.WithKeys("shift+tab", "shift+enter"), key.WithHelp("shift+tab", "back field")),
		key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save section")),
	}
}

func (s *bottomFilterMultiSelect) WithTheme(theme *huh.Theme) huh.Field {
	s.theme = theme
	return s
}

func (s *bottomFilterMultiSelect) WithAccessible(bool) huh.Field { return s }

func (s *bottomFilterMultiSelect) WithKeyMap(*huh.KeyMap) huh.Field { return s }

func (s *bottomFilterMultiSelect) WithWidth(width int) huh.Field {
	s.width = width
	return s
}

func (s *bottomFilterMultiSelect) WithHeight(height int) huh.Field {
	s.height = height
	return s
}

func (s *bottomFilterMultiSelect) WithPosition(huh.FieldPosition) huh.Field { return s }

func (s *bottomFilterMultiSelect) GetKey() string { return s.key }

func (s *bottomFilterMultiSelect) GetValue() any {
	return s.currentValue()
}

func (s *bottomFilterMultiSelect) GetFiltering() bool {
	return s.filtering
}

func (s *bottomFilterMultiSelect) HasFilter() bool {
	return s.filter != ""
}

func (s *bottomFilterMultiSelect) currentValue() []string {
	if s.value == nil {
		return nil
	}
	return append([]string{}, (*s.value)...)
}

func (s *bottomFilterMultiSelect) View() string {
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

func (s *bottomFilterMultiSelect) visibleOptions(styles *huh.FieldStyles) []string {
	if len(s.filtered) == 0 {
		return []string{styles.Description.Render("  No matches")}
	}
	height := s.optionHeight()
	start := s.cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > len(s.filtered) {
		start = maxInt(0, len(s.filtered)-height)
	}
	end := minInt(len(s.filtered), start+height)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		cursor := strings.Repeat(" ", lipgloss.Width(styles.MultiSelectSelector.String()))
		if i == s.cursor {
			cursor = styles.MultiSelectSelector.String()
		}
		prefix := styles.UnselectedPrefix
		optionStyle := styles.UnselectedOption
		if s.selected[s.filtered[i].Value] {
			prefix = styles.SelectedPrefix
			optionStyle = styles.SelectedOption
		}
		lines = append(lines, cursor+prefix.String()+optionStyle.Render(s.filtered[i].Key))
	}
	return lines
}

func (s *bottomFilterMultiSelect) filterView(styles *huh.FieldStyles) string {
	if s.filtering {
		return styles.TextInput.Prompt.Render("Search: /") + styles.TextInput.Text.Render(s.filter)
	}
	if s.filter != "" {
		return styles.Description.Render("Search: /" + s.filter + "  (Esc clears)")
	}
	return styles.Description.Render("Search: press / to filter")
}

func (s *bottomFilterMultiSelect) optionHeight() int {
	used := 3
	if s.title != "" {
		used++
	}
	if s.description != "" {
		used += lipgloss.Height(s.description)
	}
	return maxInt(3, s.height-used)
}

func (s *bottomFilterMultiSelect) activeStyles() *huh.FieldStyles {
	t := s.theme
	if t == nil {
		t = theme()
	}
	if s.focused {
		return &t.Focused
	}
	return &t.Blurred
}
