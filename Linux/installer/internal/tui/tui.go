package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/apply"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/cleanup"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/doctor"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

var errBackToSections = errors.New("back to section selector")

type Options struct {
	Paths  paths.Options
	DryRun bool
}

type session struct {
	state    config.State
	secrets  config.Secrets
	paths    paths.Options
	dirty    map[string]bool
	dryRun   bool
	selected int
}

var sections = []string{
	"General",
	"User",
	"Passwords",
	"Region",
	"Display",
	"Shell",
	"Packages",
	"Services",
	"Dots",
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
		selected, cursor, err := chooseSection(s.state, s.secrets, s.selected)
		if err != nil {
			return err
		}
		s.selected = cursor
		if selected == "Quit" {
			return nil
		}
		previousState := s.state
		previousSecrets := s.secrets
		sectionSaved, err := handleSectionResult(s, previousState, previousSecrets, runSection(ctx, s, selected))
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			if err := showSectionError(err); err != nil {
				return err
			}
			continue
		}
		if sectionSaved && isDirtySection(selected) {
			s.dirty[selected] = true
			if err := config.Save(s.paths.DraftPath, s.state); err != nil {
				return fmt.Errorf("save draft state: %w", err)
			}
		}
	}
}

func handleSectionResult(
	s *session,
	previousState config.State,
	previousSecrets config.Secrets,
	err error,
) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errBackToSections) {
		s.state = previousState
		s.secrets = previousSecrets
		return false, nil
	}
	return false, err
}

func runSection(ctx context.Context, s *session, selected string) error {
	handlers := map[string]func() error{
		"General":   func() error { return editGeneral(s) },
		"User":      func() error { return editUser(s) },
		"Region":    func() error { return editRegion(s) },
		"Display":   func() error { return editDisplay(s) },
		"Shell":     func() error { return editShell(s) },
		"Packages":  func() error { return editPackages(s) },
		"Services":  func() error { return editServices(s) },
		"Dots":      func() error { return editDots(s) },
		"Passwords": func() error { return editSecrets(s) },
		"Cleanup":   func() error { return runCleanup(ctx, s) },
		"Doctor":    func() error { return showDoctor(ctx, s) },
		"Apply":     func() error { return runApply(ctx, s) },
	}
	handler, ok := handlers[selected]
	if !ok {
		return fmt.Errorf("unknown section %q", selected)
	}
	return handler()
}

func isDirtySection(selected string) bool {
	return selected != "Apply" && selected != "Doctor" && selected != "Cleanup" && selected != "Passwords"
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
	if err := newForm(
		huh.NewNote().
			Title("Apply summary").
			Description(summary(s.state, s.secrets)),
		huh.NewConfirm().
			Title("Run dry-build and then ask before switch?").
			Affirmative("Apply").
			Negative("Back").
			Value(&confirm),
	).Run(); err != nil {
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

func showDoctor(ctx context.Context, s *session) error {
	report, err := doctor.Report(ctx, doctor.Options{Paths: s.paths, State: s.state})
	if err != nil {
		return err
	}
	return showNote("Doctor", report)
}

func runCleanup(ctx context.Context, s *session) error {
	report, err := cleanup.Report()
	if err != nil {
		return err
	}
	confirm := false
	if err := newForm(
		huh.NewConfirm().
			Title("Cleanup").
			Description(report + "\nOnly safe MySetup-managed leftovers are touched.\nThis removes preview wallpapers and Noctalia wallpaper cache files.").
			Value(&confirm),
	).Run(); err != nil {
		return err
	}
	if !confirm {
		return nil
	}
	var out bytes.Buffer
	if err := cleanup.Run(ctx, cleanup.Options{
		Paths:  s.paths,
		DryRun: s.dryRun,
		Yes:    true,
		Stdout: &out,
	}); err != nil {
		return err
	}
	return showNote("Cleanup complete", out.String())
}

func showNote(title, description string) error {
	return newForm(
		huh.NewNote().
			Title(title).
			Description(description + "\nPress Enter or Ctrl+X to return."),
	).Run()
}

func showSectionError(err error) error {
	if err == nil {
		return nil
	}
	if displayErr := showNote("Error", err.Error()); displayErr != nil && !errors.Is(displayErr, errBackToSections) {
		return displayErr
	}
	return nil
}

func summary(s config.State, secrets config.Secrets) string {
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
Passwords: linux-user=%s pgAdmin=%s
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
		secretSummaryStatus(secrets.UserPassword),
		secretSummaryStatus(secrets.PgAdminPassword),
	)
}

func chooseSection(state config.State, secrets config.Secrets, initialCursor int) (string, int, error) {
	model := newSectionModel(state, secrets, initialCursor)
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

func newSectionModel(state config.State, secrets config.Secrets, cursor int) sectionModel {
	return sectionModel{state: state, secrets: secrets, cursor: clampSectionCursor(cursor)}
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

type sectionModel struct {
	cursor    int
	cancelled bool
	state     config.State
	secrets   config.Secrets
	width     int
	height    int
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

func (m sectionModel) View() string {
	layout := layoutFor(m.width, m.height)
	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderSidebar(m, layout.sidebarWidth, layout.bodyHeight),
		spacerStyle.Width(layout.gapWidth).Height(layout.bodyHeight).Render(""),
		renderContent(m, layout.contentWidth, layout.bodyHeight),
	)
	footer := renderFooter(layout.width)
	return screenStyle.
		Width(layout.width).
		Height(layout.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, body, footer))
}

func renderSidebar(m sectionModel, width, height int) string {
	sidebar := stringsBuilder(func(b *builder) {
		b.line(sidebarText(width, titleStyle.Render(" MySetup")))
		b.line(sidebarText(width, mutedStyle.Render("Catppuccin Macchiato installer")))
		b.line("")
		for i, section := range sections {
			item := sidebarText(width, section)
			if i == m.cursor {
				item = selectedStyle.Width(maxInt(1, width-4)).Render(" " + section)
			} else {
				item = itemStyle.Render(item)
			}
			b.line(item)
		}
	})
	return sidebarStyle.Width(width).Height(height).Render(sidebar)
}

func renderContent(m sectionModel, width, height int) string {
	content := stringsBuilder(func(b *builder) {
		b.line(contentText(width, headerStyle.Render(selectedSection(m))))
		b.line("")
		for _, line := range sectionPreview(m) {
			b.line(renderPreviewLine(width, line))
		}
		b.line("")
		b.line(contentText(width, mutedStyle.Render("Select a section with ↑/↓ or k/j, then press Enter.")))
	})
	return panelStyle.Width(width).Height(height).Render(content)
}

func selectedSection(m sectionModel) string {
	if m.cursor < 0 || m.cursor >= len(sections) {
		return "MySetup"
	}
	return sections[m.cursor]
}

func sectionPreview(m sectionModel) []string {
	s := m.state
	switch selectedSection(m) {
	case "General":
		return previewSettings(
			previewSetting("Hostname", "Flake target and networking host name.", s.Host.Hostname),
			previewSetting("State version", "NixOS compatibility baseline for this machine.", s.Host.StateVersion),
		)
	case "User":
		return previewSettings(
			previewSetting("Username", "Linux account name used by users/user.nix and Home Manager.", s.User.Username),
			previewSetting("Full name", "Display name stored in the generated user module.", s.User.FullName),
			previewSetting("Home directory", "User home used by Home Manager and dots install paths.", s.User.HomeDirectory),
			previewSetting("Git identity", "Git user.name and user.email written by Home Manager.", fmt.Sprintf("%s <%s>", s.Git.Username, s.Git.Email)),
			previewSetting("pgAdmin email", "pgAdmin web UI login email; password is handled in Passwords.", s.Services.PgAdminEmail),
		)
	case "Region":
		return previewSettings(
			previewSetting("Timezone", "Nix time.timeZone value.", s.Locale.TimeZone),
			previewSetting("Default locale", "Primary glibc locale generated by NixOS.", s.Locale.DefaultLocale),
			previewSetting("Extra locale", "Additional generated locale for regional tools/input.", s.Locale.ExtraLocale),
			previewSetting("Console keymap", "TTY keyboard map only. Default is us; Hypr graphical layouts are configured in Display.", s.Locale.ConsoleKeyMap),
			previewSetting("Weather location", "City name used by shell/weather widgets.", s.Locale.WeatherLocation),
			previewSetting("Russia mode", "Region-specific defaults in this config.", formatBool(s.Features.RussiaMode)),
		)
	case "Display":
		return previewSettings(
			previewSetting("GPU", "Selects driver/session defaults for the system config.", s.Hardware.GPU),
			previewSetting("Monitor", "Generates: monitor = name, mode, position, scale.", fmt.Sprintf("%s, %s, %s, %s", s.Display.MonitorName, s.Display.MonitorMode, s.Display.MonitorPosition, s.Display.MonitorScale)),
			previewSetting("Keyboard layouts", "Hyprland kb_layout value.", s.Locale.KeyboardLayouts),
			previewSetting("Keyboard toggle", "Hyprland kb_options value for layout switching.", s.Locale.KeyboardToggle),
		)
	case "Shell":
		return previewSettings(
			previewSetting("Shell profile", "Controls which Quickshell profile Hyprland autostarts.", s.Shell.Profile),
			previewSettingWithLabel("Available profiles", "Supported shell choices in this installer.", "options", "caelestia, noctalia"),
		)
	case "Packages":
		return previewSettings(
			previewSetting("Package preset", "Personal keeps the full Takuya package set; other presets trim optional user layers.", s.Packages.Preset),
			previewSetting("CTF tools", "Adds heavy reverse/pwn/web/forensics tooling.", formatBool(s.Features.CTFTools)),
			previewSetting("Secure Boot", "Lanzaboote should stay disabled unless the machine is prepared.", formatBool(s.Features.SecureBoot)),
		)
	case "Services":
		settings := [][]string{
			previewSetting("OmniRouter", "Builds/enables the local OmniRouter package and module.", formatBool(s.Features.OmniRouter)),
			previewSetting("Zapret", "Enables the zapret service/config for Discord/YouTube bypass presets.", formatBool(s.Zapret.Enable)),
		}
		if shouldShowZapretConfig(s) {
			settings = append(settings, previewSetting("Zapret config", "Preset selected from the upstream zapret configs.", s.Zapret.Config))
		}
		return previewSettings(settings...)
	case "Dots":
		return previewSettings(
			previewSetting("Hypr dots", "Mirrors Linux/dots/hypr into ~/.config/hypr and reloads Hypr if running.", formatBool(s.Dots.Hypr)),
			previewSetting("Wallpapers", "Copies repo wallpapers into ~/Pictures/Wallpapers.", formatBool(s.Dots.Wallpapers)),
			previewSetting("Zen Catppuccin chrome", "Installs the Zen Browser Catppuccin chrome theme.", formatBool(s.Dots.ZenTheme)),
			previewSetting("Sine profile files", "Best-effort Zen Sine files install from pinned URLs.", formatBool(s.Dots.Sine)),
			previewSetting("Neovim config", "Backs up/syncs the repo Neovim config.", formatBool(s.Dots.Neovim)),
			previewSetting("v2rayN sing-box", "Copies sing-box support into v2rayN when detected.", formatBool(s.Dots.V2rayN)),
		)
	case "Passwords":
		return previewSettings(
			previewSettingWithLabel("Linux user password", "When ready, Apply writes hosts/NixOS/hashed-password.nix. Existing hash is preserved when this is not ready.", "status", secretStatus(m.secrets.UserPassword, "not entered")),
			previewSettingWithLabel("pgAdmin web password", "When ready, Apply writes /etc/nixos/secrets/pgadmin-password for the pgAdmin web login.", "status", secretStatus(m.secrets.PgAdminPassword, "not entered")),
			previewSettingWithLabel("PostgreSQL database role", "Not the same as pgAdmin. The installer does not change the postgres database role password.", "status", "not managed"),
			previewSettingWithLabel("State safety", "Plain passwords are kept only in memory for this installer run. They are never written to draft.json.", "status", "session-only"),
		)
	case "Cleanup":
		return previewSettings(
			previewSettingWithLabel("Managed cleanup", "Removes MySetup-managed stale files and caches.", "action", "explicit"),
			previewSettingWithLabel("Backups", "Unmanaged configs are backed up before removal candidates are touched.", "status", "enabled"),
		)
	case "Doctor":
		return previewSettings(
			previewSettingWithLabel("System checks", "Checks /etc/nixos mirror, hardware config and flake target.", "mode", "read-only"),
			previewSettingWithLabel("Hypr checks", "Checks shell profile, scripts and wallpaper state consistency.", "mode", "read-only"),
			previewSettingWithLabel("Recovery hints", "Shows backup paths and commands when something looks wrong.", "mode", "read-only"),
		)
	case "Apply":
		return previewSettings(
			previewSettingWithLabel("Stage config", "Builds a temporary mirror containing NixOS, dots, installer, host hardware, flake.lock and generated variables.", "order", "1"),
			previewSettingWithLabel("Dry-build", "Runs nixos-rebuild dry-build against the staging flake before touching /etc/nixos.", "order", "2"),
			previewSettingWithLabel("Mirror /etc/nixos", "After dry-build success, backs up /etc/nixos and syncs the staging mirror.", "order", "3"),
			previewSettingWithLabel("Apply dots", "Applies selected user dots in the same session and reloads Hypr when available.", "order", "4"),
			previewSettingWithLabel("Switch", "TUI always asks before nixos-rebuild switch.", "order", "5"),
		)
	case "Quit":
		return previewSettings(
			previewSettingWithLabel("Quit installer", "Exit without applying changes.", "action", "no-op"),
		)
	default:
		return previewSettings(
			previewSetting("Host", "Current NixOS host name.", s.Host.Hostname),
			previewSetting("User", "Current account name.", s.User.Username),
			previewSetting("Shell", "Current shell profile.", s.Shell.Profile),
			previewSetting("Packages", "Current package preset.", s.Packages.Preset),
			previewSetting("GPU", "Current GPU profile.", s.Hardware.GPU),
		)
	}
}

func previewSettings(settings ...[]string) []string {
	var lines []string
	for _, setting := range settings {
		lines = append(lines, setting...)
		lines = append(lines, "")
	}
	if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func previewSetting(name, description, value string) []string {
	return previewSettingWithLabel(name, description, "current", value)
}

func previewSettingWithLabel(name, description, label, value string) []string {
	return []string{
		previewHeadingPrefix + name,
		description,
		previewValuePrefix(label) + value,
	}
}

func formatBool(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func secretStatus(value, emptyStatus string) string {
	if value != "" {
		return "ready for apply (session only)"
	}
	return emptyStatus
}

func secretSummaryStatus(value string) string {
	if value != "" {
		return "ready"
	}
	return "not-entered"
}

const previewHeadingPrefix = "## "

func previewValuePrefix(label string) string {
	return label + ": "
}

func isPreviewValueLine(line string) bool {
	for _, label := range []string{"current", "options", "status", "action", "target", "order", "mode"} {
		if strings.HasPrefix(line, previewValuePrefix(label)) {
			return true
		}
	}
	return false
}

func renderPreviewLine(width int, line string) string {
	switch {
	case strings.HasPrefix(line, previewHeadingPrefix):
		return contentText(width, previewHeadingStyle.Render(line))
	case isPreviewValueLine(line):
		return contentText(width, previewValueStyle.Render(line))
	case line == "":
		return contentText(width, "")
	default:
		return contentText(width, previewDescriptionStyle.Render(line))
	}
}

func renderFooter(width int) string {
	return footerStyle.Width(width).Render("  ↑/↓ or k/j: move    Enter: open    q/Esc/Ctrl+C: quit")
}

func sidebarText(width int, text string) string {
	return lipgloss.NewStyle().PaddingLeft(2).Width(maxInt(1, width-2)).Render(text)
}

func contentText(width int, text string) string {
	return lipgloss.NewStyle().PaddingLeft(3).Width(maxInt(1, width-3)).Render(text)
}

type layoutSize struct {
	width        int
	height       int
	sidebarWidth int
	contentWidth int
	bodyHeight   int
	gapWidth     int
}

func layoutFor(width, height int) layoutSize {
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 30
	}
	gapWidth := 1
	sidebarWidth := minInt(32, maxInt(20, width/4))
	if width < 80 {
		sidebarWidth = minInt(24, maxInt(18, width/3))
	}
	if width < 44 {
		sidebarWidth = maxInt(14, width/3)
	}
	contentWidth := maxInt(12, width-sidebarWidth-gapWidth)
	bodyHeight := maxInt(8, height-1)
	return layoutSize{
		width:        width,
		height:       height,
		sidebarWidth: sidebarWidth,
		contentWidth: contentWidth,
		bodyHeight:   bodyHeight,
		gapWidth:     gapWidth,
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	tuiEmailRe           = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	tuiHostnameRe        = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	tuiMonitorModeRe     = regexp.MustCompile(`^[0-9]+x[0-9]+@[0-9]+(\.[0-9]+)?$`)
	tuiMonitorNameRe     = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	tuiMonitorPositionRe = regexp.MustCompile(`^-?[0-9]+x-?[0-9]+$`)
	tuiMonitorScaleRe    = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)
	tuiUsernameRe        = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
)

var (
	macchiatoBase     = lipgloss.Color("#24273a")
	macchiatoSurface0 = lipgloss.Color("#363a4f")
	macchiatoMantle   = lipgloss.Color("#1e2030")
	macchiatoText     = lipgloss.Color("#cad3f5")
	macchiatoSubtext  = lipgloss.Color("#a5adcb")
	macchiatoMauve    = lipgloss.Color("#c6a0f6")
	macchiatoLavender = lipgloss.Color("#b7bdf8")
	macchiatoRed      = lipgloss.Color("#ed8796")

	screenStyle             = lipgloss.NewStyle().Background(macchiatoBase).Foreground(macchiatoText)
	spacerStyle             = lipgloss.NewStyle().Background(macchiatoBase)
	sidebarStyle            = lipgloss.NewStyle().Background(macchiatoMantle)
	panelStyle              = lipgloss.NewStyle().Background(macchiatoSurface0)
	footerStyle             = lipgloss.NewStyle().Background(macchiatoBase).Foreground(macchiatoSubtext)
	titleStyle              = lipgloss.NewStyle().Bold(true).Foreground(macchiatoMauve)
	headerStyle             = lipgloss.NewStyle().Bold(true).Foreground(macchiatoLavender)
	mutedStyle              = lipgloss.NewStyle().Foreground(macchiatoSubtext)
	previewHeadingStyle     = lipgloss.NewStyle().Bold(true).Foreground(macchiatoMauve)
	previewDescriptionStyle = lipgloss.NewStyle().Foreground(macchiatoSubtext)
	previewValueStyle       = lipgloss.NewStyle().Bold(true).Foreground(macchiatoLavender)
	formErrorStyle          = lipgloss.NewStyle().Foreground(macchiatoRed)
	itemStyle               = lipgloss.NewStyle().Foreground(macchiatoText)
	selectedStyle           = lipgloss.NewStyle().Foreground(macchiatoBase).Background(macchiatoMauve).Bold(true)
)

func theme() *huh.Theme {
	t := huh.ThemeCatppuccin()
	return t
}

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

func installerFormKeyMap() *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	keymap.Input.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back field"))
	keymap.Input.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Input.Submit = key.NewBinding(key.WithKeys("ctrl+x", "shift+enter"), key.WithHelp("ctrl+x", "save section"))
	keymap.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back field"))
	keymap.Confirm.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Confirm.Submit = key.NewBinding(key.WithKeys("ctrl+x", "shift+enter"), key.WithHelp("ctrl+x", "save section"))
	keymap.Select.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back field"))
	keymap.Select.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Select.Submit = key.NewBinding(key.WithKeys("ctrl+x", "shift+enter"), key.WithHelp("ctrl+x", "save section"))
	keymap.Select.Filter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter"), key.WithDisabled())
	keymap.Select.SetFilter = key.NewBinding(key.WithKeys("enter", "esc"), key.WithHelp("enter/esc", "set filter"), key.WithDisabled())
	keymap.Select.ClearFilter = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter"), key.WithDisabled())
	keymap.Note.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "back field"))
	keymap.Note.Next = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	keymap.Note.Submit = key.NewBinding(key.WithKeys("ctrl+x", "shift+enter"), key.WithHelp("ctrl+x", "save section"))
	return keymap
}

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

func editGeneral(s *session) error {
	var errors generalFormErrors
	for {
		if err := runGeneralForm(s, errors); err != nil {
			return err
		}
		errors = validateGeneralForm(s.state)
		if errors.empty() {
			return nil
		}
	}
}

func runGeneralForm(s *session, errors generalFormErrors) error {
	return newForm(
		huh.NewInput().
			Title("Hostname").
			Description(fieldDescription("NixOS host name and flake target, for example NixOS.", errors.hostname)).
			Value(&s.state.Host.Hostname),
		huh.NewInput().
			Title("State version").
			Description("NixOS compatibility baseline. Do not bump casually on an existing machine.").
			Value(&s.state.Host.StateVersion),
	).Run()
}

type generalFormErrors struct {
	hostname string
}

func (e generalFormErrors) empty() bool {
	return e.hostname == ""
}

func validateGeneralForm(state config.State) generalFormErrors {
	var errors generalFormErrors
	if !tuiHostnameRe.MatchString(state.Host.Hostname) {
		errors.hostname = "hostname must be a single DNS label, for example NixOS or laptop-01."
	}
	return errors
}

func editUser(s *session) error {
	var errors userFormErrors
	for {
		if err := runUserForm(s, errors); err != nil {
			return err
		}
		errors = validateUserForm(s.state)
		if errors.empty() {
			return nil
		}
	}
}

func runUserForm(s *session, errors userFormErrors) error {
	return newForm(
		huh.NewInput().
			Title("Username").
			Description(fieldDescription("Linux account name used by users/user.nix and Home Manager.", errors.username)).
			Value(&s.state.User.Username),
		huh.NewInput().
			Title("Full name").
			Description(fieldDescription("Display name for account metadata. It can stay equal to the username.", errors.fullName)).
			Value(&s.state.User.FullName),
		huh.NewInput().
			Title("Home directory").
			Description(fieldDescription("Usually /home/<username>; dots and HM config are written relative to it.", errors.homeDirectory)).
			Value(&s.state.User.HomeDirectory),
		huh.NewInput().
			Title("Git user.name").
			Description(fieldDescription("Written to Home Manager git config.", errors.gitUsername)).
			Value(&s.state.Git.Username),
		huh.NewInput().
			Title("Git user.email").
			Description(fieldDescription("Written to Home Manager git config and used by commits.", errors.gitEmail)).
			Value(&s.state.Git.Email),
		huh.NewInput().
			Title("pgAdmin email").
			Description(fieldDescription("pgAdmin web UI login email. The password is set in the Passwords section.", errors.pgAdminEmail)).
			Value(&s.state.Services.PgAdminEmail),
	).Run()
}

type userFormErrors struct {
	username      string
	fullName      string
	homeDirectory string
	gitUsername   string
	gitEmail      string
	pgAdminEmail  string
}

func (e userFormErrors) empty() bool {
	return e.username == "" &&
		e.fullName == "" &&
		e.homeDirectory == "" &&
		e.gitUsername == "" &&
		e.gitEmail == "" &&
		e.pgAdminEmail == ""
}

func validateUserForm(state config.State) userFormErrors {
	var errors userFormErrors
	if !tuiUsernameRe.MatchString(state.User.Username) {
		errors.username = "username must start with a lowercase letter or underscore and use only lowercase letters, digits, _ or -."
	}
	if strings.TrimSpace(state.User.FullName) == "" {
		errors.fullName = "full name cannot be empty."
	}
	if !validHomeDirectory(state.User.HomeDirectory, state.User.Username) {
		errors.homeDirectory = "home directory must be a clean /home/<username> path."
	}
	if strings.TrimSpace(state.Git.Username) == "" {
		errors.gitUsername = "git user.name cannot be empty."
	}
	if !tuiEmailRe.MatchString(state.Git.Email) {
		errors.gitEmail = "git user.email must look like name@example.com."
	}
	if state.Services.PgAdminEmail != "" && !tuiEmailRe.MatchString(state.Services.PgAdminEmail) {
		errors.pgAdminEmail = "pgAdmin email must look like name@example.com."
	}
	return errors
}

func validHomeDirectory(home, username string) bool {
	clean := filepath.Clean(home)
	if home == "" || !filepath.IsAbs(home) || clean != home || !strings.HasPrefix(clean, "/home/") {
		return false
	}
	return username == "" || clean == filepath.Join("/home", username)
}

func editRegion(s *session) error {
	return newForm(
		newBottomFilterSelect().
			Title("Timezone").
			Description("Nix time.timeZone value. Press / to search like archinstall, Esc clears search.").
			Options(timeZoneOptions(s.state.Locale.TimeZone)...).
			Height(14).
			Value(&s.state.Locale.TimeZone).
			OnChange(func(timeZone string) {
				s.state.Locale.WeatherLocation = weatherLocationFromTimeZone(timeZone)
			}),
		newBottomFilterSelect().
			Title("Default locale").
			Description("Primary glibc locale. Keep the .UTF-8 suffix.").
			Options(localeOptions(s.state.Locale.DefaultLocale)...).
			Height(12).
			Value(&s.state.Locale.DefaultLocale),
		newBottomFilterSelect().
			Title("Extra locale").
			Description("Additional generated locale, usually ru_RU.UTF-8 for Russian input/tools.").
			Options(localeOptions(s.state.Locale.ExtraLocale)...).
			Height(12).
			Value(&s.state.Locale.ExtraLocale),
		newBottomFilterSelect().
			Title("Console keymap").
			Description("TTY keyboard map only. Default is us; Hypr graphical layouts are configured in Display.").
			Options(consoleKeymapOptions(s.state.Locale.ConsoleKeyMap)...).
			Height(12).
			Value(&s.state.Locale.ConsoleKeyMap),
		newLiveInput().
			Title("Weather location").
			Description("City name used by shell/weather widgets if enabled.").
			Value(&s.state.Locale.WeatherLocation),
		huh.NewConfirm().
			Title("Russia mode").
			Description("Enables region-specific defaults in this config. VPN/proxy services are separate.").
			Value(&s.state.Features.RussiaMode),
	).Run()
}

func weatherLocationFromTimeZone(timeZone string) string {
	if timeZone == "" {
		return ""
	}
	parts := strings.Split(timeZone, "/")
	city := parts[len(parts)-1]
	city = strings.ReplaceAll(city, "_", " ")
	return city
}

func timeZoneOptions(current string) []huh.Option[string] {
	timeZones := discoverTimeZones(zoneInfoDirs())
	if current != "" && !containsString(timeZones, current) {
		timeZones = append([]string{current}, timeZones...)
	}
	return stringOptions(timeZones)
}

func zoneInfoDirs() []string {
	return []string{"/etc/zoneinfo", "/usr/share/zoneinfo"}
}

func discoverTimeZones(dirs []string) []string {
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			root = dir
		}
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			timeZone := filepath.ToSlash(rel)
			if validTimeZoneName(timeZone) {
				seen[timeZone] = struct{}{}
			}
			return nil
		}); err != nil {
			continue
		}
	}
	for _, timeZone := range fallbackTimeZones() {
		seen[timeZone] = struct{}{}
	}
	timeZones := make([]string, 0, len(seen))
	for timeZone := range seen {
		timeZones = append(timeZones, timeZone)
	}
	sort.Strings(timeZones)
	return timeZones
}

func validTimeZoneName(name string) bool {
	if name == "" || strings.HasPrefix(name, "posix/") || strings.HasPrefix(name, "right/") {
		return false
	}
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") ||
		strings.HasSuffix(base, ".tab") ||
		strings.HasSuffix(base, ".zi") ||
		base == "leapseconds" ||
		base == "localtime" ||
		base == "posixrules" {
		return false
	}
	return strings.Contains(name, "/") || name == "UTC"
}

func fallbackTimeZones() []string {
	return []string{
		"UTC",
		"America/Chicago",
		"America/Los_Angeles",
		"America/New_York",
		"Asia/Tokyo",
		"Europe/Amsterdam",
		"Europe/Berlin",
		"Europe/London",
		"Europe/Moscow",
		"Europe/Paris",
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func localeOptions(current string) []huh.Option[string] {
	locales := discoverLocales(localeFiles())
	for _, locale := range fallbackLocales() {
		locales = appendUnique(locales, locale)
	}
	sort.Strings(locales)
	if current != "" && !containsString(locales, current) {
		locales = append([]string{current}, locales...)
	}
	return stringOptions(locales)
}

func localeFiles() []string {
	return []string{
		"/etc/locale.gen",
		"/run/current-system/sw/share/i18n/SUPPORTED",
		"/usr/share/i18n/SUPPORTED",
	}
}

func discoverLocales(files []string) []string {
	seen := map[string]struct{}{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
				continue
			}
			locale := normalizeLocale(fields[0])
			if validLocaleName(locale) {
				seen[locale] = struct{}{}
			}
		}
	}
	locales := make([]string, 0, len(seen))
	for locale := range seen {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	locale = strings.Replace(locale, "/UTF-8", ".UTF-8", 1)
	locale = strings.Replace(locale, ".utf8", ".UTF-8", 1)
	locale = strings.Replace(locale, ".UTF8", ".UTF-8", 1)
	return locale
}

func validLocaleName(locale string) bool {
	return strings.Contains(locale, ".") && strings.Contains(locale, "_")
}

func fallbackLocales() []string {
	return []string{
		"en_US.UTF-8",
		"ru_RU.UTF-8",
		"de_DE.UTF-8",
		"en_GB.UTF-8",
		"es_ES.UTF-8",
		"fr_FR.UTF-8",
		"it_IT.UTF-8",
		"ja_JP.UTF-8",
		"nl_NL.UTF-8",
		"pl_PL.UTF-8",
		"pt_BR.UTF-8",
		"tr_TR.UTF-8",
		"uk_UA.UTF-8",
	}
}

func consoleKeymapOptions(current string) []huh.Option[string] {
	keymaps := discoverConsoleKeymaps(consoleKeymapDirs())
	for _, keymap := range fallbackConsoleKeymaps() {
		keymaps = appendUnique(keymaps, keymap)
	}
	sort.Strings(keymaps)
	if current != "" && !containsString(keymaps, current) {
		keymaps = append([]string{current}, keymaps...)
	}
	return stringOptions(keymaps)
}

func consoleKeymapDirs() []string {
	return []string{
		"/run/current-system/sw/share/keymaps",
		"/run/current-system/sw/share/kbd/keymaps",
		"/usr/share/keymaps",
		"/usr/share/kbd/keymaps",
	}
}

func discoverConsoleKeymaps(dirs []string) []string {
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			keymap, ok := keymapNameFromPath(path)
			if ok {
				seen[keymap] = struct{}{}
			}
			return nil
		}); err != nil {
			continue
		}
	}
	keymaps := make([]string, 0, len(seen))
	for keymap := range seen {
		keymaps = append(keymaps, keymap)
	}
	sort.Strings(keymaps)
	return keymaps
}

func keymapNameFromPath(path string) (string, bool) {
	base := filepath.Base(path)
	switch {
	case strings.HasSuffix(base, ".map.gz"):
		base = strings.TrimSuffix(base, ".map.gz")
	case strings.HasSuffix(base, ".map"):
		base = strings.TrimSuffix(base, ".map")
	default:
		return "", false
	}
	if base == "" {
		return "", false
	}
	return base, true
}

func fallbackConsoleKeymaps() []string {
	return []string{
		"us",
		"ru",
		"de",
		"fr",
		"es",
		"it",
		"jp",
		"pl",
		"pt",
		"uk",
		"br-abnt2",
		"ruwin_alt_sh-UTF-8",
	}
}

func appendUnique(values []string, value string) []string {
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}

func stringOptions(values []string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(values))
	for _, value := range values {
		options = append(options, huh.NewOption(value, value))
	}
	return options
}

func editDisplay(s *session) error {
	var errors displayFormErrors
	for {
		if err := runDisplayForm(s, errors); err != nil {
			return err
		}
		errors = validateDisplayForm(s.state.Display)
		if errors.empty() {
			return nil
		}
	}
}

func runDisplayForm(s *session, errors displayFormErrors) error {
	return newForm(
		huh.NewSelect[string]().
			Title("GPU").
			Description("Selects driver/session defaults for the system config.").
			Options(
				huh.NewOption("AMD", "amd"),
				huh.NewOption("Intel", "intel"),
				huh.NewOption("NVIDIA", "nvidia"),
				huh.NewOption("Other / VM", "other"),
			).
			Value(&s.state.Hardware.GPU),
		huh.NewInput().
			Title("Hypr keyboard layouts").
			Description("Hyprland kb_layout value, for example us,ru.").
			Value(&s.state.Locale.KeyboardLayouts),
		huh.NewInput().
			Title("Monitor name").
			Description(fieldDescription("Output name from hyprctl monitors, for example eDP-1 or DP-1.", errors.monitorName)).
			Value(&s.state.Display.MonitorName),
		huh.NewInput().
			Title("Resolution@Hz").
			Description(fieldDescription("Only the mode part, for example 2560x1600@120.", errors.monitorMode)).
			Value(&s.state.Display.MonitorMode),
		huh.NewInput().
			Title("Position").
			Description(fieldDescription("Monitor position in Hyprland syntax, for example 0x0.", errors.monitorPosition)).
			Value(&s.state.Display.MonitorPosition),
		huh.NewInput().
			Title("Scale").
			Description(fieldDescription("Monitor scale in Hyprland syntax, for example 1 or 1.25.", errors.monitorScale)).
			Value(&s.state.Display.MonitorScale),
		huh.NewSelect[string]().
			Title("Keyboard toggle").
			Description("Hyprland kb_options value for layout switching.").
			Options(
				huh.NewOption("Alt+Shift", "grp:alt_shift_toggle"),
				huh.NewOption("Win+Space", "grp:win_space_toggle"),
				huh.NewOption("Ctrl+Shift", "grp:ctrl_shift_toggle"),
				huh.NewOption("CapsLock", "grp:caps_toggle"),
			).
			Value(&s.state.Locale.KeyboardToggle),
	).Run()
}

type displayFormErrors struct {
	monitorName     string
	monitorMode     string
	monitorPosition string
	monitorScale    string
}

func (e displayFormErrors) empty() bool {
	return e.monitorName == "" &&
		e.monitorMode == "" &&
		e.monitorPosition == "" &&
		e.monitorScale == ""
}

func validateDisplayForm(display config.Display) displayFormErrors {
	var errors displayFormErrors
	if !tuiMonitorNameRe.MatchString(display.MonitorName) {
		errors.monitorName = "monitor name must look like eDP-1, DP-1 or HDMI-A-1."
	}
	if !tuiMonitorModeRe.MatchString(display.MonitorMode) {
		errors.monitorMode = "resolution must look like 2560x1600@120."
	}
	if !tuiMonitorPositionRe.MatchString(display.MonitorPosition) {
		errors.monitorPosition = "position must look like 0x0 or -1920x0."
	}
	if !tuiMonitorScaleRe.MatchString(display.MonitorScale) {
		errors.monitorScale = "scale must be a number like 1 or 1.25."
	}
	return errors
}

func editShell(s *session) error {
	return newForm(
		huh.NewSelect[string]().
			Title("Shell profile").
			Description("Chooses which Quickshell-based shell Hyprland autostarts.").
			Options(
				huh.NewOption("Caelestia", "caelestia"),
				huh.NewOption("Noctalia", "noctalia"),
			).
			Value(&s.state.Shell.Profile),
	).Run()
}

func editPackages(s *session) error {
	return newForm(
		huh.NewSelect[string]().
			Title("Package preset").
			Description("Controls optional package layers. Personal keeps Takuya's full setup; Minimal/Desktop/Developer trim app groups.").
			Options(
				huh.NewOption("Personal (Takuya's config)", "personal"),
				huh.NewOption("Minimal", "minimal"),
				huh.NewOption("Desktop", "desktop"),
				huh.NewOption("Developer", "developer"),
			).
			Value(&s.state.Packages.Preset),
		huh.NewConfirm().
			Title("Enable CTF tools").
			Description("Adds heavy CTF layers: reverse, pwn, web, forensics, OSINT and related tools.").
			Value(&s.state.Features.CTFTools),
		huh.NewSelect[bool]().
			Title("Secure Boot (Lanzaboote)").
			Description("Enable only on machines already prepared for Secure Boot. Leave disabled for normal installs.").
			Options(
				huh.NewOption("Disabled (normal installs)", false),
				huh.NewOption("Enabled (prepared Secure Boot only)", true),
			).
			Value(&s.state.Features.SecureBoot),
	).Run()
}

func editServices(s *session) error {
	wasZapretEnabled := s.state.Zapret.Enable
	if err := newForm(serviceFields(s)...).Run(); err != nil {
		return err
	}
	if !wasZapretEnabled && s.state.Zapret.Enable {
		return newForm(zapretConfigField(&s.state.Zapret.Config)).Run()
	}
	return nil
}

func serviceFields(s *session) []huh.Field {
	fields := []huh.Field{
		huh.NewConfirm().
			Title("Enable OmniRouter").
			Description("Builds/enables the local OmniRouter package and module used by this config.").
			Value(&s.state.Features.OmniRouter),
		huh.NewConfirm().
			Title("Enable Zapret").
			Description("Enables the zapret service/config for Discord/YouTube bypass presets.").
			Value(&s.state.Zapret.Enable),
	}
	if shouldShowZapretConfig(s.state) {
		fields = append(fields, zapretConfigField(&s.state.Zapret.Config))
	}
	return fields
}

func shouldShowZapretConfig(state config.State) bool {
	return state.Zapret.Enable
}

func zapretConfigField(value *string) huh.Field {
	return huh.NewSelect[string]().
		Title("Zapret config").
		Description("Choose one of the current top-level upstream configs from the pinned flake.").
		Options(zapretConfigOptions(*value)...).
		Value(value)
}

func zapretConfigOptions(current string) []huh.Option[string] {
	configs := zapretConfigNames()
	if current != "" && !containsString(configs, current) {
		configs = append([]string{current}, configs...)
	}
	return stringOptions(configs)
}

func zapretConfigNames() []string {
	return []string{
		"general",
		"general (FAKE_TLS_AUTO)",
		"general (FAKE_TLS_AUTO_ALT)",
		"general (FAKE_TLS_AUTO_ALT2)",
		"general (FAKE_TLS_AUTO_ALT3)",
		"general (SIMPLE FAKE)",
		"general (SIMPLE FAKE ALT)",
		"general (SIMPLE_FAKE_ALT2)",
		"general(ALT)",
		"general(ALT2)",
		"general(ALT3)",
		"general(ALT4)",
		"general(ALT5)",
		"general(ALT6)",
		"general(ALT7)",
		"general(ALT8)",
		"general(ALT9)",
		"general(ALT10)",
		"general(ALT11)",
	}
}

func editDots(s *session) error {
	return newForm(
		huh.NewConfirm().
			Title("Sync Hypr dots").
			Description("Mirrors Linux/dots/hypr into ~/.config/hypr, chmods scripts and reloads Hypr if running.").
			Value(&s.state.Dots.Hypr),
		huh.NewConfirm().
			Title("Copy wallpapers").
			Description("Copies Linux/NixOS/Wallpapers into ~/Pictures/Wallpapers and removes preview-* files.").
			Value(&s.state.Dots.Wallpapers),
		huh.NewConfirm().
			Title("Install Zen Catppuccin chrome").
			Description("Finds a Zen Browser profile and installs the Catppuccin chrome theme.").
			Value(&s.state.Dots.ZenTheme),
		huh.NewConfirm().
			Title("Install Sine profile files").
			Description("Best-effort Zen Sine files install; pinned URLs are checked and skipped with a warning if missing.").
			Value(&s.state.Dots.Sine),
		huh.NewConfirm().
			Title("Sync Neovim config").
			Description("Backs up/syncs the repo Neovim config into ~/.config/nvim when enabled.").
			Value(&s.state.Dots.Neovim),
		huh.NewConfirm().
			Title("Seed v2rayN sing-box").
			Description("Copies sing-box support into v2rayN when the target directory is detected.").
			Value(&s.state.Dots.V2rayN),
	).Run()
}

func editSecrets(s *session) error {
	return editSecretsWithReader(s, readSecrets)
}

func editSecretsWithReader(
	s *session,
	read func() (userPassword string, pgAdminPassword string, err error),
) error {
	userPassword, pgAdminPassword, err := read()
	if err != nil {
		return err
	}
	s.secrets.UserPassword = userPassword
	s.secrets.PgAdminPassword = pgAdminPassword
	return nil
}

func readSecrets() (string, string, error) {
	values := secretFormValues{}
	errors := secretFormErrors{}
	for {
		if err := runSecretForm(&values, errors); err != nil {
			return "", "", err
		}
		errors = validateSecretFormValues(values)
		if errors.empty() {
			return values.userPassword, values.pgAdminPassword, nil
		}
	}
}

type secretFormValues struct {
	userPassword    string
	userConfirm     string
	pgAdminPassword string
	pgAdminConfirm  string
}

type secretFormErrors struct {
	userPassword    string
	userConfirm     string
	pgAdminPassword string
	pgAdminConfirm  string
}

func (e secretFormErrors) empty() bool {
	return e.userPassword == "" &&
		e.userConfirm == "" &&
		e.pgAdminPassword == "" &&
		e.pgAdminConfirm == ""
}

func runSecretForm(values *secretFormValues, errors secretFormErrors) error {
	if err := newForm(
		huh.NewInput().
			Title("Linux user password").
			Description(fieldDescription("Required for this password step. Written as hosts/NixOS/hashed-password.nix during Apply.", errors.userPassword)).
			EchoMode(huh.EchoModePassword).
			Value(&values.userPassword),
		huh.NewInput().
			Title("Confirm Linux user password").
			Description(fieldDescription("Repeat the Linux user password.", errors.userConfirm)).
			EchoMode(huh.EchoModePassword).
			Value(&values.userConfirm),
		huh.NewInput().
			Title("pgAdmin web password").
			Description(fieldDescription("Required for this password step. Written to /etc/nixos/secrets/pgadmin-password during Apply.", errors.pgAdminPassword)).
			EchoMode(huh.EchoModePassword).
			Value(&values.pgAdminPassword),
		huh.NewInput().
			Title("Confirm pgAdmin web password").
			Description(fieldDescription("Repeat the pgAdmin web password. This is not the PostgreSQL postgres role password.", errors.pgAdminConfirm)).
			EchoMode(huh.EchoModePassword).
			Value(&values.pgAdminConfirm),
	).Run(); err != nil {
		return err
	}
	return nil
}

func validateSecretFormValues(values secretFormValues) secretFormErrors {
	var errors secretFormErrors
	if values.userPassword == "" {
		errors.userPassword = "Linux user password cannot be empty."
	}
	if values.userConfirm == "" {
		errors.userConfirm = "Linux user password confirmation cannot be empty."
	} else if values.userPassword != values.userConfirm {
		errors.userConfirm = "Linux user password does not match confirmation."
	}
	if values.pgAdminPassword == "" {
		errors.pgAdminPassword = "pgAdmin web password cannot be empty."
	}
	if values.pgAdminConfirm == "" {
		errors.pgAdminConfirm = "pgAdmin web password confirmation cannot be empty."
	} else if values.pgAdminPassword != values.pgAdminConfirm {
		errors.pgAdminConfirm = "pgAdmin web password does not match confirmation."
	}
	return errors
}

func fieldDescription(description, errorMessage string) string {
	if errorMessage == "" {
		return description
	}
	return description + "\n" + formErrorStyle.Render("error: "+errorMessage)
}
