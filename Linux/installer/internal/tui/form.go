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
		WithKeyMap(installerFormKeyMap())
	return &installerForm{form: form, fields: fields}
}

func (f *installerForm) Run() error {
	f.form.SubmitCmd = tea.Quit
	f.form.CancelCmd = tea.Interrupt
	model := submitOnEnterModel{form: f.form, fields: f.fields}
	final, err := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithReportFocus()).Run()
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
		return m.updateForm(msg)
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "ctrl+x" || keyMsg.String() == "shift+enter" {
			return m, m.form.NextGroup()
		}
		if m.focusedFieldFiltering() {
			return m.updateForm(msg)
		}
		if keyMsg.String() == "esc" && m.focusedFieldHasFilter() {
			return m.updateForm(msg)
		}
		switch keyMsg.String() {
		case "enter":
			return m, m.nextFieldCircular()
		case "tab":
			return m, m.nextFieldCircular()
		case "shift+tab":
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

	bodyHeight := maxInt(0, m.height-1)
	renderedBody := lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		Render(body)
	renderedFooter := footerStyle.Width(m.width).Render("  " + footer)
	return screenStyle.
		Width(m.width).
		Height(m.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, renderedBody, renderedFooter))
}

func formFooterText(m submitOnEnterModel) string {
	switch {
	case m.focusedFieldFiltering():
		return "type: filter    Enter: select    Esc: clear search    Tab: next field    Ctrl+X: save"
	case len(m.fields) <= 1:
		return "Enter/Ctrl+X: return    Esc: back    Ctrl+C: quit"
	default:
		return "Tab/Enter: next field    Shift+Tab: previous    Ctrl+X: save section    Esc: back without saving    Ctrl+C: quit"
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

func (m submitOnEnterModel) nextFieldCircular() tea.Cmd {
	if len(m.fields) <= 1 {
		return m.form.NextGroup()
	}
	current := m.focusedFieldIndex()
	if current < 0 || current < len(m.fields)-1 {
		return m.form.NextField()
	}
	return m.moveFocusBackward(len(m.fields) - 1)
}

func (m submitOnEnterModel) prevFieldCircular() tea.Cmd {
	current := m.focusedFieldIndex()
	if current < 0 || current > 0 {
		return m.form.PrevField()
	}
	return m.moveFocusForward(len(m.fields) - 1)
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
