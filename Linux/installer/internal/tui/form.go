package tui

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type installerForm struct {
	form   *huh.Form
	fields []huh.Field
}

func newForm(fields ...huh.Field) *installerForm {
	form := huh.NewForm(huh.NewGroup(fields...)).
		WithTheme(theme()).
		WithKeyMap(installerFormKeyMap()).
		WithShowHelp(false)
	return &installerForm{form: form, fields: fields}
}

func (f *installerForm) Run() error {
	f.form.SubmitCmd = tea.Quit
	f.form.CancelCmd = tea.Interrupt
	model := submitOnEnterModel{form: f.form, fields: f.fields}
	final, err := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithReportFocus(), tea.WithAltScreen()).Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return huh.ErrUserAborted
		}
		if errors.Is(err, tea.ErrProgramKilled) {
			return huh.ErrTimeout
		}
		return fmt.Errorf("huh: %w", err)
	}
	if finalModel, ok := final.(submitOnEnterModel); ok && finalModel.form.State == huh.StateAborted {
		return huh.ErrUserAborted
	} else if ok && finalModel.back {
		return errBackToSections
	}
	return nil
}

type submitOnEnterModel struct {
	form   *huh.Form
	fields []huh.Field
	back   bool
	width  int
	height int
}

func (m submitOnEnterModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m submitOnEnterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
		return m.updateForm(tea.WindowSizeMsg{Width: size.Width, Height: m.bodyHeight()})
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "ctrl+s" {
			return m, m.form.NextGroup()
		}
		if m.focusedFieldFiltering() {
			return m.updateForm(msg)
		}
		if keyMsg.String() == "esc" && m.focusedFieldHasFilter() {
			return m.updateForm(msg)
		}
		switch keyMsg.String() {
		case "y", "Y":
			if m.focusedFieldIsConfirm() {
				return m.setFocusedConfirmValue(true)
			}
		case "n", "N":
			if m.focusedFieldIsConfirm() {
				return m.setFocusedConfirmValue(false)
			}
		case "enter":
			return m, m.nextFieldCircular()
		case "tab":
			return m, m.nextFieldCircular()
		case "shift+tab", "shift+enter":
			return m, m.prevFieldCircular()
		case "esc":
			m.back = true
			return m, tea.Quit
		}
	}
	return m.updateForm(msg)
}

func (m submitOnEnterModel) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.form.Update(msg)
	if form, ok := model.(*huh.Form); ok {
		m.form = form
	}
	return m, cmd
}

func (m submitOnEnterModel) View() string {
	body := m.form.View()
	footer := formFooterText(m)
	if m.width <= 0 || m.height <= 0 {
		return lipgloss.JoinVertical(lipgloss.Left, body, "", footerStyle.Render("  "+footer))
	}

	bodyHeight := m.bodyHeight()
	renderedBody := lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(body)
	renderedFooter := footerStyle.Width(m.width).Height(1).MaxHeight(1).Render("  " + footer)
	return screenStyle.
		Width(m.width).
		Height(m.height).
		MaxHeight(m.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, renderedBody, renderedFooter))
}

func (m submitOnEnterModel) bodyHeight() int {
	return maxInt(0, m.height-1)
}

func formFooterText(m submitOnEnterModel) string {
	switch {
	case m.focusedFieldFiltering():
		return "Search: type filter    Enter: keep search    Esc: clear search    Up/Down: results    Ctrl+S: save"
	case m.focusedFieldIsConfirm():
		return "y/n or arrows: choose    Tab/Enter: continue    Shift+Tab/Shift+Enter: previous    Ctrl+S: save    Esc: menu    Ctrl+C: quit"
	case len(m.fields) <= 1:
		return "Tab/Enter: return    Ctrl+S: save    Esc: menu    Ctrl+C: quit"
	default:
		return "Tab/Enter: next    Shift+Tab/Shift+Enter: previous    Ctrl+S: save    Esc: menu    Ctrl+C: quit"
	}
}

func (m submitOnEnterModel) focusedFieldFiltering() bool {
	field, ok := m.form.GetFocusedField().(interface{ GetFiltering() bool })
	return ok && field.GetFiltering()
}

func (m submitOnEnterModel) focusedFieldHasFilter() bool {
	field, ok := m.form.GetFocusedField().(interface{ HasFilter() bool })
	return ok && field.HasFilter()
}

func (m submitOnEnterModel) focusedFieldIsConfirm() bool {
	_, ok := m.form.GetFocusedField().(*huh.Confirm)
	return ok
}

func (m submitOnEnterModel) setFocusedConfirmValue(value bool) (tea.Model, tea.Cmd) {
	current, ok := m.focusedConfirmValue()
	if !ok || current == value {
		return m, nil
	}
	return m.updateForm(tea.KeyMsg{Type: tea.KeyRight})
}

func (m submitOnEnterModel) focusedConfirmValue() (bool, bool) {
	confirm, ok := m.form.GetFocusedField().(*huh.Confirm)
	if !ok {
		return false, false
	}
	value, ok := confirm.GetValue().(bool)
	return value, ok
}

func (m submitOnEnterModel) nextFieldCircular() tea.Cmd {
	navigable := m.navigableFieldIndexes()
	if len(navigable) == 0 {
		return nil
	}
	if len(navigable) == 1 {
		return m.form.NextGroup()
	}
	current := m.focusedFieldIndex()
	position := indexOfInt(navigable, current)
	if position < 0 {
		return m.moveFocusForward(navigable[0])
	}
	if position < len(navigable)-1 {
		return m.form.NextField()
	}
	return m.moveFocusBackward(position)
}

func (m submitOnEnterModel) prevFieldCircular() tea.Cmd {
	navigable := m.navigableFieldIndexes()
	if len(navigable) <= 1 {
		return nil
	}
	current := m.focusedFieldIndex()
	position := indexOfInt(navigable, current)
	if position < 0 {
		return m.moveFocusBackward(len(m.fields) - 1 - navigable[len(navigable)-1])
	}
	if position > 0 {
		return m.form.PrevField()
	}
	return m.moveFocusForward(len(navigable) - 1)
}

func (m submitOnEnterModel) navigableFieldIndexes() []int {
	if len(m.fields) == 1 {
		return []int{0}
	}
	indexes := make([]int, 0, len(m.fields))
	for i, field := range m.fields {
		if !field.Skip() {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func indexOfInt(values []int, target int) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func (m submitOnEnterModel) focusedFieldIndex() int {
	focused := m.form.GetFocusedField()
	for i, field := range m.fields {
		if focused == field {
			return i
		}
	}
	return -1
}

func (m submitOnEnterModel) moveFocusBackward(steps int) tea.Cmd {
	if steps <= 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, steps)
	for range steps {
		cmds = append(cmds, m.form.PrevField())
	}
	return tea.Batch(cmds...)
}

func (m submitOnEnterModel) moveFocusForward(steps int) tea.Cmd {
	if steps <= 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, steps)
	for range steps {
		cmds = append(cmds, m.form.NextField())
	}
	return tea.Batch(cmds...)
}
