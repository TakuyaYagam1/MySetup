package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/apply"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/cleanup"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/doctor"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

type Options struct {
	Paths  paths.Options
	DryRun bool
}

type session struct {
	state   config.State
	secrets config.Secrets
	paths   paths.Options
	dirty   map[string]bool
	dryRun  bool
}

var sections = []string{
	"General",
	"User",
	"Region",
	"Display",
	"Shell",
	"Packages",
	"Services",
	"Dots",
	"Secrets",
	"Cleanup",
	"Doctor",
	"Apply",
	"Quit",
}

func Run(ctx context.Context, opts Options) error {
	state, err := loadInitialState(opts.Paths)
	if err != nil {
		return err
	}
	s := &session{
		state:  state,
		paths:  opts.Paths,
		dirty:  map[string]bool{},
		dryRun: opts.DryRun,
	}

	for {
		selected, err := chooseSection(s.state)
		if err != nil {
			return err
		}
		if selected == "Quit" {
			return nil
		}
		if err := runSection(ctx, s, selected); err != nil {
			return err
		}
		if isDirtySection(selected) {
			s.dirty[selected] = true
			if err := config.Save(s.paths.DraftPath, s.state); err != nil {
				return fmt.Errorf("save draft state: %w", err)
			}
		}
	}
}

func runSection(ctx context.Context, s *session, selected string) error {
	handlers := map[string]func() error{
		"General":  func() error { return editGeneral(s) },
		"User":     func() error { return editUser(s) },
		"Region":   func() error { return editRegion(s) },
		"Display":  func() error { return editDisplay(s) },
		"Shell":    func() error { return editShell(s) },
		"Packages": func() error { return editPackages(s) },
		"Services": func() error { return editServices(s) },
		"Dots":     func() error { return editDots(s) },
		"Secrets":  func() error { return editSecrets(s) },
		"Cleanup":  func() error { return cleanup.Run(ctx, cleanup.Options{Paths: s.paths, DryRun: s.dryRun}) },
		"Doctor":   func() error { return doctor.Run(ctx, doctor.Options{Paths: s.paths, State: s.state}) },
		"Apply":    func() error { return runApply(ctx, s) },
	}
	handler, ok := handlers[selected]
	if !ok {
		return fmt.Errorf("unknown section %q", selected)
	}
	return handler()
}

func isDirtySection(selected string) bool {
	return selected != "Apply" && selected != "Doctor" && selected != "Cleanup"
}

func loadInitialState(opts paths.Options) (config.State, error) {
	if _, err := os.Stat(opts.StatePath); err == nil {
		return config.LoadExisting(opts.StatePath)
	} else if !os.IsNotExist(err) {
		return config.State{}, fmt.Errorf("stat state %s: %w", opts.StatePath, err)
	}
	if _, err := os.Stat(opts.DraftPath); err == nil {
		return config.LoadExisting(opts.DraftPath)
	} else if !os.IsNotExist(err) {
		return config.State{}, fmt.Errorf("stat draft %s: %w", opts.DraftPath, err)
	}
	return config.Default(), nil
}

func runApply(ctx context.Context, s *session) error {
	if err := config.Validate(s.state); err != nil {
		return err
	}
	confirm := false
	if err := huh.NewForm(huh.NewGroup(
		huh.NewNote().
			Title("Apply summary").
			Description(summary(s.state)),
		huh.NewConfirm().
			Title("Run dry-build and then ask before switch?").
			Affirmative("Apply").
			Negative("Back").
			Value(&confirm),
	)).WithTheme(theme()).Run(); err != nil {
		return err
	}
	if !confirm {
		return nil
	}
	return apply.Run(ctx, apply.Options{
		Paths:     s.paths,
		State:     s.state,
		Secrets:   s.secrets,
		DryRun:    s.dryRun,
		AssumeYes: false,
	})
}

func summary(s config.State) string {
	return fmt.Sprintf(`Host: %s
User: %s (%s)
Shell: %s
Packages: %s
GPU: %s
Secure Boot: %t
CTF Tools: %t
OmniRouter: %t
Zapret: %t %s
Dots: hypr=%t zen=%t sine=%t nvim=%t v2rayN=%t wallpapers=%t
State: /etc/nixos/mysetup/state.json`,
		s.Host.Hostname,
		s.User.Username,
		s.User.HomeDirectory,
		s.Shell.Profile,
		s.Packages.Preset,
		s.Hardware.GPU,
		s.Features.SecureBoot,
		s.Features.CTFTools,
		s.Features.OmniRouter,
		s.Zapret.Enable,
		s.Zapret.Config,
		s.Dots.Hypr,
		s.Dots.ZenTheme,
		s.Dots.Sine,
		s.Dots.Neovim,
		s.Dots.V2rayN,
		s.Dots.Wallpapers,
	)
}

func chooseSection(state config.State) (string, error) {
	model := sectionModel{state: state}
	program := tea.NewProgram(model)
	final, err := program.Run()
	if err != nil {
		return "", err
	}
	m, ok := final.(sectionModel)
	if !ok {
		return "Quit", nil
	}
	if m.cancelled {
		return "Quit", nil
	}
	return sections[m.cursor], nil
}

type sectionModel struct {
	cursor    int
	cancelled bool
	state     config.State
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

func (m sectionModel) Init() tea.Cmd { return nil }

func (m sectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
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

func (m sectionModel) View() string {
	sidebar := stringsBuilder(func(b *builder) {
		b.line(titleStyle.Render("󰣇 MySetup"))
		b.line(mutedStyle.Render("Catppuccin Mocha installer"))
		b.line("")
		for i, section := range sections {
			item := "  " + section
			if i == m.cursor {
				item = selectedStyle.Render("  " + section + "  ")
			} else {
				item = itemStyle.Render(item)
			}
			b.line(item)
		}
	})
	content := stringsBuilder(func(b *builder) {
		b.line(headerStyle.Render("Current State"))
		b.line("")
		b.line(fmt.Sprintf("Host:  %s", m.state.Host.Hostname))
		b.line(fmt.Sprintf("User:  %s", m.state.User.Username))
		b.line(fmt.Sprintf("Shell: %s", m.state.Shell.Profile))
		b.line(fmt.Sprintf("Pkgs:  %s", m.state.Packages.Preset))
		b.line(fmt.Sprintf("GPU:   %s", m.state.Hardware.GPU))
		b.line("")
		b.line(mutedStyle.Render("Enter opens a section. q exits."))
	})
	return appStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, sidebarStyle.Render(sidebar), panelStyle.Render(content)))
}

type builder struct {
	s string
}

func (b *builder) line(s string) {
	b.s += s + "\n"
}

func stringsBuilder(fn func(*builder)) string {
	var b builder
	fn(&b)
	return b.s
}

var (
	mochaBase     = lipgloss.Color("#1e1e2e")
	mochaSurface0 = lipgloss.Color("#313244")
	mochaText     = lipgloss.Color("#cdd6f4")
	mochaSubtext  = lipgloss.Color("#a6adc8")
	mochaMauve    = lipgloss.Color("#cba6f7")
	mochaLavender = lipgloss.Color("#b4befe")

	appStyle      = lipgloss.NewStyle().Background(mochaBase).Foreground(mochaText).Padding(1, 2)
	sidebarStyle  = lipgloss.NewStyle().Width(28).Padding(1, 2).Background(lipgloss.Color("#181825"))
	panelStyle    = lipgloss.NewStyle().Width(58).Padding(1, 3).Background(mochaSurface0)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(mochaMauve)
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(mochaLavender)
	mutedStyle    = lipgloss.NewStyle().Foreground(mochaSubtext)
	itemStyle     = lipgloss.NewStyle().Foreground(mochaText)
	selectedStyle = lipgloss.NewStyle().Foreground(mochaBase).Background(mochaMauve).Bold(true)
)

func theme() *huh.Theme {
	t := huh.ThemeCatppuccin()
	return t
}

func editGeneral(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Hostname").Value(&s.state.Host.Hostname),
		huh.NewInput().Title("State version").Value(&s.state.Host.StateVersion),
	)).WithTheme(theme()).Run()
}

func editUser(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Username").Value(&s.state.User.Username),
		huh.NewInput().Title("Full name").Value(&s.state.User.FullName),
		huh.NewInput().Title("Home directory").Value(&s.state.User.HomeDirectory),
		huh.NewInput().Title("Git user.name").Value(&s.state.Git.Username),
		huh.NewInput().Title("Git user.email").Value(&s.state.Git.Email),
	)).WithTheme(theme()).Run()
}

func editRegion(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Timezone").Value(&s.state.Locale.TimeZone),
		huh.NewInput().Title("Default locale").Value(&s.state.Locale.DefaultLocale),
		huh.NewInput().Title("Extra locale").Value(&s.state.Locale.ExtraLocale),
		huh.NewInput().Title("Console keymap").Value(&s.state.Locale.ConsoleKeyMap),
		huh.NewInput().Title("Weather location").Value(&s.state.Locale.WeatherLocation),
		huh.NewConfirm().Title("Russia mode").Value(&s.state.Features.RussiaMode),
	)).WithTheme(theme()).Run()
}

func editDisplay(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("GPU").
			Options(
				huh.NewOption("AMD", "amd"),
				huh.NewOption("Intel", "intel"),
				huh.NewOption("NVIDIA", "nvidia"),
				huh.NewOption("Other / VM", "other"),
			).
			Value(&s.state.Hardware.GPU),
		huh.NewInput().Title("Hypr keyboard layouts").Value(&s.state.Locale.KeyboardLayouts),
		huh.NewInput().Title("Monitor name").Value(&s.state.Display.MonitorName),
		huh.NewInput().Title("Resolution@Hz").Value(&s.state.Display.MonitorMode),
		huh.NewInput().Title("Position").Value(&s.state.Display.MonitorPosition),
		huh.NewInput().Title("Scale").Value(&s.state.Display.MonitorScale),
		huh.NewSelect[string]().
			Title("Keyboard toggle").
			Options(
				huh.NewOption("Alt+Shift", "grp:alt_shift_toggle"),
				huh.NewOption("Win+Space", "grp:win_space_toggle"),
				huh.NewOption("Ctrl+Shift", "grp:ctrl_shift_toggle"),
				huh.NewOption("CapsLock", "grp:caps_toggle"),
			).
			Value(&s.state.Locale.KeyboardToggle),
	)).WithTheme(theme()).Run()
}

func editShell(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Shell profile").
			Options(
				huh.NewOption("Caelestia", "caelestia"),
				huh.NewOption("Noctalia", "noctalia"),
			).
			Value(&s.state.Shell.Profile),
	)).WithTheme(theme()).Run()
}

func editPackages(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Package preset").
			Description("personal keeps Takuya's full config; the others trim app groups for reusable installs.").
			Options(
				huh.NewOption("Personal (Takuya's config)", "personal"),
				huh.NewOption("Minimal", "minimal"),
				huh.NewOption("Desktop", "desktop"),
				huh.NewOption("Developer", "developer"),
			).
			Value(&s.state.Packages.Preset),
		huh.NewConfirm().Title("Enable CTF tools").Value(&s.state.Features.CTFTools),
		huh.NewConfirm().Title("Enable Secure Boot / Lanzaboote").Value(&s.state.Features.SecureBoot),
	)).WithTheme(theme()).Run()
}

func editServices(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Enable OmniRouter").Value(&s.state.Features.OmniRouter),
		huh.NewConfirm().Title("Enable Zapret").Value(&s.state.Zapret.Enable),
		huh.NewInput().Title("Zapret config").Value(&s.state.Zapret.Config),
		huh.NewInput().Title("pgAdmin email").Value(&s.state.Services.PgAdminEmail),
	)).WithTheme(theme()).Run()
}

func editDots(s *session) error {
	return huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Sync Hypr dots").Value(&s.state.Dots.Hypr),
		huh.NewConfirm().Title("Copy wallpapers").Value(&s.state.Dots.Wallpapers),
		huh.NewConfirm().Title("Install Zen Catppuccin chrome").Value(&s.state.Dots.ZenTheme),
		huh.NewConfirm().Title("Install Sine profile files").Value(&s.state.Dots.Sine),
		huh.NewConfirm().Title("Sync Neovim config").Value(&s.state.Dots.Neovim),
		huh.NewConfirm().Title("Seed v2rayN sing-box").Value(&s.state.Dots.V2rayN),
	)).WithTheme(theme()).Run()
}

func editSecrets(s *session) error {
	resetUser := false
	resetPgAdmin := false
	if err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Reset initial user password").Value(&resetUser),
		huh.NewConfirm().Title("Write pgAdmin password").Value(&resetPgAdmin),
	)).WithTheme(theme()).Run(); err != nil {
		return err
	}
	if resetUser {
		password, err := readPasswordPair("User password")
		if err != nil {
			return err
		}
		s.secrets.UserPassword = password
	} else {
		s.secrets.UserPassword = ""
	}
	if resetPgAdmin {
		password, err := readPasswordPair("pgAdmin password")
		if err != nil {
			return err
		}
		s.secrets.PgAdminPassword = password
	} else {
		s.secrets.PgAdminPassword = ""
	}
	return nil
}

func readPasswordPair(title string) (string, error) {
	password := ""
	confirm := ""
	if err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title(title).EchoMode(huh.EchoModePassword).Value(&password),
		huh.NewInput().Title("Confirm "+title).EchoMode(huh.EchoModePassword).Value(&confirm),
	)).WithTheme(theme()).Run(); err != nil {
		return "", err
	}
	if password == "" || password != confirm {
		return "", fmt.Errorf("%s is empty or does not match confirmation", title)
	}
	return password, nil
}
