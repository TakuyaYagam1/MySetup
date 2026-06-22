package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

type sectionModel struct {
	cursor          int
	cancelled       bool
	state           config.State
	secrets         config.Secrets
	existingSecrets secretAvailability
	width           int
	height          int
}

var keys = struct {
	up    key.Binding
	down  key.Binding
	enter key.Binding
	quit  key.Binding
}{
	up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	quit:  key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
}

func summary(s config.State, secrets config.Secrets, existingSecrets secretAvailability, statePath string) string {
	return fmt.Sprintf(`Host: %s
MySetup channel: %s
User: %s (%s)
Packages: %s
GPU: %s
Secure Boot: %t
CTF Tools: %t
OmniRouter: %t
Observability: %t
Zapret: %t %s
Dots: hypr=%t zen=%t sine=%t nvim=%t nvimClean=%t v2rayN=%t wallpapers=%t
Passwords: linux-user=%s
State: %s%s`,
		s.Host.Hostname,
		sourceChannelLabel(s.Source.Channel),
		s.User.Username,
		s.User.HomeDirectory,
		s.Packages.Preset,
		s.Hardware.GPU,
		s.Features.SecureBoot,
		s.Features.CTFTools,
		s.Features.OmniRouter,
		s.Features.Observability,
		s.Zapret.Enable,
		s.Zapret.Config,
		s.Dots.Hypr,
		s.Dots.ZenTheme,
		s.Dots.Sine,
		s.Dots.Neovim,
		s.Dots.NeovimCleanState,
		s.Dots.V2rayN,
		s.Dots.Wallpapers,
		secretSummaryStatus(secrets.UserPassword, existingSecrets.UserPassword),
		statePath,
		observabilityAccessSummary(s.Features.Observability),
	)
}

func observabilityAccessSummary(enabled bool) string {
	if !enabled {
		return ""
	}
	return "\nGrafana: http://127.0.0.1:3010 initial login admin/admin"
}

func chooseSection(
	state config.State,
	secrets config.Secrets,
	existingSecrets secretAvailability,
	initialCursor int,
) (string, int, error) {
	model := newSectionModel(state, secrets, existingSecrets, initialCursor)
	program := newSectionProgram(model)
	final, err := program.Run()
	if err != nil {
		return "", initialCursor, err
	}
	m, ok := final.(sectionModel)
	if !ok {
		return "Quit", initialCursor, nil
	}
	if m.cancelled {
		return "Quit", m.cursor, nil
	}
	return sections[m.cursor], m.cursor, nil
}

func newSectionModel(state config.State, secrets config.Secrets, existingSecrets secretAvailability, cursor int) sectionModel {
	return sectionModel{
		state:           state,
		secrets:         secrets,
		existingSecrets: existingSecrets,
		cursor:          clampSectionCursor(cursor),
	}
}

func clampSectionCursor(cursor int) int {
	if cursor < 0 {
		return 0
	}
	if cursor >= len(sections) {
		return len(sections) - 1
	}
	return cursor
}

func newSectionProgram(model sectionModel) *tea.Program {
	return tea.NewProgram(model, sectionProgramOptions()...)
}

func sectionProgramOptions() []tea.ProgramOption {
	return []tea.ProgramOption{tea.WithAltScreen()}
}

func (m sectionModel) Init() tea.Cmd { return tea.ClearScreen }

func (m sectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.ClearScreen
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m sectionModel) handleKey(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(keyMsg, keys.quit):
		m.cancelled = true
		return m, tea.Quit
	case key.Matches(keyMsg, keys.up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(keyMsg, keys.down):
		if m.cursor < len(sections)-1 {
			m.cursor++
		}
	case key.Matches(keyMsg, keys.enter):
		return m, tea.Quit
	}
	return m, nil
}
