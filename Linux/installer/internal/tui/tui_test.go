package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

func TestLoadInitialStatePrefersMachineState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	draftPath := filepath.Join(dir, "draft.json")

	machine := config.Default()
	machine.Host.Hostname = "machine"
	if err := config.Save(statePath, machine); err != nil {
		t.Fatal(err)
	}
	draft := config.Default()
	draft.Host.Hostname = "draft"
	if err := config.Save(draftPath, draft); err != nil {
		t.Fatal(err)
	}

	got, err := loadInitialState(paths.Options{StatePath: statePath, DraftPath: draftPath})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host.Hostname != "machine" {
		t.Fatalf("expected machine state to win, got %q", got.Host.Hostname)
	}
}

func TestLoadInitialStateFallsBackToDraft(t *testing.T) {
	dir := t.TempDir()
	draftPath := filepath.Join(dir, "draft.json")
	draft := config.Default()
	draft.Host.Hostname = "draft"
	if err := config.Save(draftPath, draft); err != nil {
		t.Fatal(err)
	}

	got, err := loadInitialState(paths.Options{
		StatePath: filepath.Join(dir, "missing.json"),
		DraftPath: draftPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host.Hostname != "draft" {
		t.Fatalf("expected draft fallback, got %q", got.Host.Hostname)
	}
}

func TestLoadInitialStateReturnsStateLoadError(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := config.Save(statePath, config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := overwriteFile(statePath, "{ nope\n"); err != nil {
		t.Fatal(err)
	}

	if _, err := loadInitialState(paths.Options{StatePath: statePath}); err == nil {
		t.Fatal("expected invalid state load error")
	}
}

func TestSectionProgramUsesAltScreen(t *testing.T) {
	if len(sectionProgramOptions()) == 0 {
		t.Fatal("expected Bubble Tea program options")
	}
	source, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "tea.WithAltScreen()") {
		t.Fatal("expected section program to use tea.WithAltScreen")
	}
}

func TestNewSectionModelUsesInitialCursor(t *testing.T) {
	model := newSectionModel(config.Default(), config.Secrets{}, 1)
	if got := selectedSection(model); got != "User" {
		t.Fatalf("expected User to stay selected, got %q", got)
	}
}

func TestNewSectionModelClampsInitialCursor(t *testing.T) {
	if got := newSectionModel(config.Default(), config.Secrets{}, -1).cursor; got != 0 {
		t.Fatalf("expected negative cursor to clamp to 0, got %d", got)
	}
	if got := newSectionModel(config.Default(), config.Secrets{}, len(sections)+10).cursor; got != len(sections)-1 {
		t.Fatalf("expected cursor to clamp to last section, got %d", got)
	}
}

func TestLayoutForSmallTerminal(t *testing.T) {
	layout := layoutFor(50, 12)
	if layout.width != 50 || layout.height != 12 {
		t.Fatalf("unexpected layout bounds: %#v", layout)
	}
	if layout.sidebarWidth != 18 {
		t.Fatalf("expected compact sidebar width 18, got %d", layout.sidebarWidth)
	}
	if layout.contentWidth != 31 {
		t.Fatalf("expected content width 31, got %d", layout.contentWidth)
	}
	if layout.bodyHeight != 11 {
		t.Fatalf("expected body height 11, got %d", layout.bodyHeight)
	}
}

func TestLayoutForNormalTerminal(t *testing.T) {
	layout := layoutFor(120, 40)
	if layout.sidebarWidth != 30 {
		t.Fatalf("expected sidebar width 30, got %d", layout.sidebarWidth)
	}
	if layout.contentWidth != 89 {
		t.Fatalf("expected content width 89, got %d", layout.contentWidth)
	}
	if layout.bodyHeight != 39 {
		t.Fatalf("expected body height 39, got %d", layout.bodyHeight)
	}
}

func TestLayoutForWideTerminal(t *testing.T) {
	layout := layoutFor(240, 60)
	if layout.sidebarWidth != 32 {
		t.Fatalf("expected max sidebar width 32, got %d", layout.sidebarWidth)
	}
	if layout.contentWidth != 207 {
		t.Fatalf("expected content width 207, got %d", layout.contentWidth)
	}
	if layout.bodyHeight != 59 {
		t.Fatalf("expected body height 59, got %d", layout.bodyHeight)
	}
}

func TestWindowSizeMsgUpdatesSectionModel(t *testing.T) {
	model := sectionModel{state: config.Default()}
	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 140, Height: 44})
	if cmd == nil {
		t.Fatal("expected clear-screen command on resize")
	}
	got, ok := updated.(sectionModel)
	if !ok {
		t.Fatalf("expected sectionModel, got %T", updated)
	}
	if got.width != 140 || got.height != 44 {
		t.Fatalf("expected window size to be stored, got %dx%d", got.width, got.height)
	}
}

func TestSelectedSectionFollowsCursor(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Region"), state: config.Default()}
	if got := selectedSection(model); got != "Region" {
		t.Fatalf("expected Region, got %q", got)
	}
}

func TestSectionOrderKeepsPasswordsUnderUser(t *testing.T) {
	user := sectionIndex(t, "User")
	passwords := sectionIndex(t, "Passwords")
	if passwords != user+1 {
		t.Fatalf("expected Passwords directly below User, user=%d passwords=%d sections=%v", user, passwords, sections)
	}
}

func TestSectionPreviewFollowsSelectedSection(t *testing.T) {
	state := config.Default()
	state.Locale.TimeZone = "Asia/Tokyo"
	model := sectionModel{cursor: sectionIndex(t, "Region"), state: state}

	preview := strings.Join(sectionPreview(model), "\n")
	if !strings.Contains(preview, "## Timezone\nNix time.timeZone value.\ncurrent: Asia/Tokyo") {
		t.Fatalf("expected region preview, got:\n%s", preview)
	}
	if strings.Contains(preview, "Hostname") {
		t.Fatalf("region preview should not show general host summary, got:\n%s", preview)
	}
}

func TestSectionPreviewUsesSettingBlocksWithBlankLines(t *testing.T) {
	state := config.Default()
	state.User.Username = "takuya"
	state.User.FullName = "Takuya"
	state.User.HomeDirectory = "/home/takuya"
	model := sectionModel{cursor: sectionIndex(t, "User"), state: state}
	preview := sectionPreview(model)
	joined := strings.Join(preview, "\n")

	for _, want := range []string{
		"## Username\nLinux account name used by users/user.nix and Home Manager.\ncurrent: takuya",
		"## Full name\nDisplay name stored in the generated user module.\ncurrent: Takuya",
		"## Home directory\nUser home used by Home Manager and dots install paths.\ncurrent: /home/takuya",
		"## Git identity\nGit user.name and user.email written by Home Manager.",
		"## pgAdmin email\npgAdmin web UI login email; password is handled in Passwords.",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected user preview block %q, got:\n%s", want, joined)
		}
	}
	if !hasBlankLineAfterBlock(preview, "current: takuya") {
		t.Fatalf("expected blank line between setting blocks, got %#v", preview)
	}
}

func TestRenderPreviewLineStylesHeadingsDescriptionsAndValues(t *testing.T) {
	source, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"previewHeadingStyle.Render(line)",
		"previewDescriptionStyle.Render(line)",
		"previewValueStyle.Render(line)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected preview renderer source to contain %q", want)
		}
	}

	cases := []struct {
		name string
		line string
	}{
		{name: "heading", line: "## Timezone"},
		{name: "description", line: "Nix time.timeZone value."},
		{name: "value", line: "current: Europe/Moscow"},
		{name: "options", line: "options: caelestia, noctalia, end4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := renderPreviewLine(80, tc.line)
			if !strings.Contains(rendered, tc.line) {
				t.Fatalf("expected rendered preview line to contain %q, got %q", tc.line, rendered)
			}
		})
	}
}

func TestShellPreviewUsesOptionsForAvailableProfiles(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Shell"), state: config.Default()}
	preview := strings.Join(sectionPreview(model), "\n")

	if !strings.Contains(preview, "current: caelestia") {
		t.Fatalf("expected current shell, got:\n%s", preview)
	}
	if !strings.Contains(preview, "options: caelestia, noctalia, end4") {
		t.Fatalf("expected available profiles to use options label, got:\n%s", preview)
	}
	if strings.Contains(preview, "current: caelestia, noctalia, end4") {
		t.Fatalf("available profiles must not render as current value, got:\n%s", preview)
	}
}

func TestPasswordsSectionExplainsManagedAndUnmanagedPasswords(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Passwords"), state: config.Default()}
	preview := strings.Join(sectionPreview(model), "\n")

	for _, want := range []string{
		"## Linux user password",
		"## pgAdmin web password",
		"## PostgreSQL database role",
		"status: not managed",
		"Plain passwords are kept only in memory",
		"status: session-only",
		"status: not entered",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected passwords preview to contain %q, got:\n%s", want, preview)
		}
	}
}

func TestPasswordsSectionShowsPendingSecretStatus(t *testing.T) {
	model := sectionModel{
		cursor: sectionIndex(t, "Passwords"),
		state:  config.Default(),
		secrets: config.Secrets{
			UserPassword:    "user-secret",
			PgAdminPassword: "pg-secret",
		},
	}
	preview := strings.Join(sectionPreview(model), "\n")

	if got := strings.Count(preview, "status: ready for apply (session only)"); got != 2 {
		t.Fatalf("expected both password secrets to be ready, count=%d preview:\n%s", got, preview)
	}
	if strings.Contains(preview, "user-secret") || strings.Contains(preview, "pg-secret") {
		t.Fatalf("preview must not leak plaintext secrets, got:\n%s", preview)
	}
}

func TestSummaryShowsPendingSecretsWithoutValues(t *testing.T) {
	state := config.Default()
	secrets := config.Secrets{
		UserPassword:    "linux-secret",
		PgAdminPassword: "pgadmin-secret",
	}

	got := summary(state, secrets)
	if !strings.Contains(got, "Passwords: linux-user=ready pgAdmin=ready") {
		t.Fatalf("expected summary to show ready secret status, got:\n%s", got)
	}
	if strings.Contains(got, "linux-secret") || strings.Contains(got, "pgadmin-secret") {
		t.Fatalf("summary must not leak plaintext secrets, got:\n%s", got)
	}
}

func TestApplyPreviewDescribesTransactionalOrder(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Apply"), state: config.Default()}
	preview := strings.Join(sectionPreview(model), "\n")
	for _, want := range []string{
		"## Stage config",
		"NixOS, dots, installer",
		"## Dry-build",
		"before touching /etc/nixos",
		"## Mirror /etc/nixos",
		"## Apply dots",
		"## Switch",
		"TUI always asks before nixos-rebuild switch.",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("apply preview missing %q, got:\n%s", want, preview)
		}
	}
	if strings.Contains(preview, "before dry-build") {
		t.Fatalf("apply preview should not claim user dots apply before dry-build, got:\n%s", preview)
	}
	if strings.Contains(preview, "--yes") {
		t.Fatalf("TUI preview should not mention CLI --yes behavior, got:\n%s", preview)
	}
}

func TestPasswordFormUsesDirectMaskedInputs(t *testing.T) {
	source, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`Title("Linux user password")`,
		`Title("Confirm Linux user password")`,
		`Title("pgAdmin web password")`,
		`Title("Confirm pgAdmin web password")`,
		`EchoMode(huh.EchoModePassword)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected password form source to contain %q", want)
		}
	}
	for _, gone := range []string{
		`Title("Set Linux user password")`,
		`Title("Set pgAdmin web password")`,
	} {
		if strings.Contains(text, gone) {
			t.Fatalf("password flow should not contain enable toggle title %q", gone)
		}
	}
}

func TestEditSecretsWithReaderStoresPasswordsOnlyInSession(t *testing.T) {
	state := config.Default()
	s := &session{state: state}

	err := editSecretsWithReader(s, func() (string, string, error) {
		return "linux-secret", "pgadmin-secret", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if s.secrets.UserPassword != "linux-secret" {
		t.Fatalf("expected Linux password in session secrets, got %q", s.secrets.UserPassword)
	}
	if s.secrets.PgAdminPassword != "pgadmin-secret" {
		t.Fatalf("expected pgAdmin password in session secrets, got %q", s.secrets.PgAdminPassword)
	}
	if s.state != state {
		t.Fatal("passwords must not mutate persistent installer state")
	}
}

func TestEditSecretsWithReaderPropagatesError(t *testing.T) {
	wantErr := errors.New("password mismatch")
	s := &session{}

	err := editSecretsWithReader(s, func() (string, string, error) {
		return "", "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected password reader error, got %v", err)
	}
	if s.secrets.UserPassword != "" || s.secrets.PgAdminPassword != "" {
		t.Fatalf("secrets should stay empty on error, got %#v", s.secrets)
	}
}

func TestPasswordsSectionDoesNotDirtyDraftState(t *testing.T) {
	if isDirtySection("Passwords") {
		t.Fatal("passwords are process-local secrets and must not be persisted to draft state")
	}
}

func TestValidateSecretFormValuesReportsInlineErrors(t *testing.T) {
	got := validateSecretFormValues(secretFormValues{
		userPassword:    "one",
		userConfirm:     "two",
		pgAdminPassword: "",
		pgAdminConfirm:  "",
	})

	if got.userConfirm == "" {
		t.Fatal("expected Linux password confirmation mismatch error")
	}
	if got.pgAdminPassword == "" || got.pgAdminConfirm == "" {
		t.Fatalf("expected pgAdmin password errors, got %#v", got)
	}
}

func TestValidateSecretFormValuesAcceptsMatchingPasswords(t *testing.T) {
	got := validateSecretFormValues(secretFormValues{
		userPassword:    "linux-secret",
		userConfirm:     "linux-secret",
		pgAdminPassword: "pg-secret",
		pgAdminConfirm:  "pg-secret",
	})

	if !got.empty() {
		t.Fatalf("expected no password form errors, got %#v", got)
	}
}

func TestFieldDescriptionRendersInlineError(t *testing.T) {
	got := fieldDescription("Base description.", "field cannot be empty.")
	if !strings.Contains(got, "Base description.") {
		t.Fatalf("expected original description, got %q", got)
	}
	if !strings.Contains(got, "error: field cannot be empty.") {
		t.Fatalf("expected inline error, got %q", got)
	}
}

func TestValidateGeneralFormReportsHostnameError(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "-bad"

	got := validateGeneralForm(state)
	if got.hostname == "" {
		t.Fatal("expected hostname validation error")
	}
}

func TestValidateUserFormReportsFieldErrors(t *testing.T) {
	state := config.Default()
	state.User.Username = "Bad User"
	state.User.FullName = ""
	state.User.HomeDirectory = "/tmp/takuya"
	state.Git.Username = ""
	state.Git.Email = "not-email"
	state.Services.PgAdminEmail = "also-not-email"

	got := validateUserForm(state)
	if got.empty() {
		t.Fatal("expected user form validation errors")
	}
	for name, value := range map[string]string{
		"username":      got.username,
		"fullName":      got.fullName,
		"homeDirectory": got.homeDirectory,
		"gitUsername":   got.gitUsername,
		"gitEmail":      got.gitEmail,
		"pgAdminEmail":  got.pgAdminEmail,
	} {
		if value == "" {
			t.Fatalf("expected %s validation error, got %#v", name, got)
		}
	}
}

func TestValidateDisplayFormReportsFieldErrors(t *testing.T) {
	got := validateDisplayForm(config.Display{
		MonitorName:     "bad monitor",
		MonitorMode:     "2560x1600",
		MonitorPosition: "zero",
		MonitorScale:    "big",
	})

	if got.empty() {
		t.Fatal("expected display form validation errors")
	}
	for name, value := range map[string]string{
		"monitorName":     got.monitorName,
		"monitorMode":     got.monitorMode,
		"monitorPosition": got.monitorPosition,
		"monitorScale":    got.monitorScale,
	} {
		if value == "" {
			t.Fatalf("expected %s validation error, got %#v", name, got)
		}
	}
}

func TestServicesPreviewHidesZapretConfigUntilEnabled(t *testing.T) {
	state := config.Default()
	state.Zapret.Enable = false
	model := sectionModel{cursor: sectionIndex(t, "Services"), state: state}
	preview := strings.Join(sectionPreview(model), "\n")
	if strings.Contains(preview, "## Zapret config") {
		t.Fatalf("disabled zapret must hide config from services preview, got:\n%s", preview)
	}
	if strings.Contains(preview, "pgAdmin email") {
		t.Fatalf("pgAdmin email belongs to User preview, got:\n%s", preview)
	}

	state.Zapret.Enable = true
	model.state = state
	preview = strings.Join(sectionPreview(model), "\n")
	if !strings.Contains(preview, "## Zapret config") {
		t.Fatalf("enabled zapret must show config in services preview, got:\n%s", preview)
	}
}

func TestServiceFieldsHideZapretConfigUntilEnabled(t *testing.T) {
	disabled := &session{state: config.Default()}
	disabled.state.Zapret.Enable = false
	if got := len(serviceFields(disabled)); got != 2 {
		t.Fatalf("disabled zapret should render only OmniRouter and Zapret toggles, got %d fields", got)
	}

	enabled := &session{state: config.Default()}
	enabled.state.Zapret.Enable = true
	if got := len(serviceFields(enabled)); got != 3 {
		t.Fatalf("enabled zapret should render OmniRouter, Zapret and config fields, got %d fields", got)
	}
}

func TestZapretConfigOptionsExposeCurrentTopLevelUpstreamChoices(t *testing.T) {
	names := zapretConfigNames()
	want := []string{
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
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected zapret config names\ngot:  %v\nwant: %v", names, want)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "old configs/") {
			t.Fatalf("legacy zapret configs should not be shown in the default selector: %v", names)
		}
	}
}

func TestZapretConfigOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := zapretConfigOptions("custom")
	if len(options) == 0 || options[0].Value != "custom" {
		t.Fatalf("expected custom current zapret config to be first option, got %#v", options)
	}
}

func TestDiscoverTimeZonesFromZoneInfoDir(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"Europe/Amsterdam",
		"Europe/Berlin",
		"posix/Europe/Moscow",
		"zone.tab",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("tz"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	timeZones := discoverTimeZones([]string{root})
	for _, want := range []string{"Europe/Amsterdam", "Europe/Berlin"} {
		if !containsString(timeZones, want) {
			t.Fatalf("expected discovered timezone %q in %v", want, timeZones)
		}
	}
	if containsString(timeZones, "posix/Europe/Moscow") {
		t.Fatalf("posix timezone mirror should be ignored: %v", timeZones)
	}
	if containsString(timeZones, "zone.tab") {
		t.Fatalf("zone.tab metadata should be ignored: %v", timeZones)
	}
}

func TestTimeZoneOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := timeZoneOptions("Custom/MoonBase")
	if len(options) == 0 || options[0].Value != "Custom/MoonBase" {
		t.Fatalf("expected custom current timezone to be first option, got %#v", options)
	}
}

func TestLocaleOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := localeOptions("eo_EO.UTF-8")
	if len(options) == 0 || options[0].Value != "eo_EO.UTF-8" {
		t.Fatalf("expected custom current locale to be first option, got %#v", options)
	}
	if !containsOption(options, "en_US.UTF-8") {
		t.Fatalf("expected fallback locale list to include en_US.UTF-8, got %#v", options)
	}
}

func TestDiscoverLocalesFromSupportedFile(t *testing.T) {
	dir := t.TempDir()
	supported := filepath.Join(dir, "SUPPORTED")
	if err := os.WriteFile(supported, []byte(`
# comment
en_US.UTF-8 UTF-8
ru_RU.UTF-8 UTF-8
de_DE/UTF-8 UTF-8
`), 0o644); err != nil {
		t.Fatal(err)
	}

	locales := discoverLocales([]string{supported})
	for _, want := range []string{"en_US.UTF-8", "ru_RU.UTF-8", "de_DE.UTF-8"} {
		if !containsString(locales, want) {
			t.Fatalf("expected locale %q in %v", want, locales)
		}
	}
}

func TestConsoleKeymapOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := consoleKeymapOptions("custom-map")
	if len(options) == 0 || options[0].Value != "custom-map" {
		t.Fatalf("expected custom keymap to be first option, got %#v", options)
	}
	if !containsOption(options, "us") {
		t.Fatalf("expected fallback keymap list to include us, got %#v", options)
	}
}

func TestDiscoverConsoleKeymapsFromDir(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		"i386/qwerty/us.map.gz",
		"i386/qwerty/de.map",
		"README",
	} {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("keymap"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	keymaps := discoverConsoleKeymaps([]string{dir})
	for _, want := range []string{"us", "de"} {
		if !containsString(keymaps, want) {
			t.Fatalf("expected keymap %q in %v", want, keymaps)
		}
	}
	if containsString(keymaps, "README") {
		t.Fatalf("README should not be a keymap: %v", keymaps)
	}
}

func TestWeatherLocationFromTimeZone(t *testing.T) {
	cases := map[string]string{
		"Europe/Amsterdam":               "Amsterdam",
		"America/Argentina/Buenos_Aires": "Buenos Aires",
		"UTC":                            "UTC",
	}
	for timeZone, want := range cases {
		if got := weatherLocationFromTimeZone(timeZone); got != want {
			t.Fatalf("expected %q from %q, got %q", want, timeZone, got)
		}
	}
}

func TestLiveInputReflectsExternalValueChanges(t *testing.T) {
	value := "Moscow"
	field := newLiveInput().
		Title("Weather location").
		Description("City name used by shell/weather widgets if enabled.").
		Value(&value)

	if view := field.View(); !strings.Contains(view, "Moscow") {
		t.Fatalf("expected initial live value in view, got:\n%s", view)
	}

	value = "Oslo"
	if view := field.View(); !strings.Contains(view, "Oslo") {
		t.Fatalf("expected changed live value in view, got:\n%s", view)
	}
}

func TestTimezoneChangeUpdatesLiveWeatherLocation(t *testing.T) {
	weatherLocation := "Moscow"
	timezone := "Europe/Moscow"
	timezoneField := newBottomFilterSelect().
		Title("Timezone").
		Options(
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
			huh.NewOption("Europe/Oslo", "Europe/Oslo"),
		).
		Value(&timezone).
		OnChange(func(timeZone string) {
			weatherLocation = weatherLocationFromTimeZone(timeZone)
		})
	weatherField := newLiveInput().
		Title("Weather location").
		Value(&weatherLocation)

	_, _ = timezoneField.Update(tea.KeyMsg{Type: tea.KeyDown})
	if weatherLocation != "Oslo" {
		t.Fatalf("expected timezone change to update weather location, got %q", weatherLocation)
	}
	if view := weatherField.View(); !strings.Contains(view, "Oslo") {
		t.Fatalf("expected live weather field to render updated value, got:\n%s", view)
	}
}

func TestRegionTimezoneUsesBottomFilterSelect(t *testing.T) {
	source, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`Options(timeZoneOptions(s.state.Locale.TimeZone)...)`,
		`Options(localeOptions(s.state.Locale.DefaultLocale)...)`,
		`Options(localeOptions(s.state.Locale.ExtraLocale)...)`,
		`Options(consoleKeymapOptions(s.state.Locale.ConsoleKeyMap)...)`,
		`weatherLocationFromTimeZone(timeZone)`,
		`newLiveInput().`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected region form source to contain %q", want)
		}
	}
}

func TestBottomFilterSelectRendersSearchBelowOptions(t *testing.T) {
	value := "Europe/Moscow"
	field := newBottomFilterSelect().
		Title("Timezone").
		Description("Pick a timezone.").
		Options(
			huh.NewOption("Europe/Amsterdam", "Europe/Amsterdam"),
			huh.NewOption("Europe/Berlin", "Europe/Berlin"),
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
		).
		Height(10).
		Value(&value)

	view := field.View()
	optionIndex := strings.Index(view, "Europe/Moscow")
	searchIndex := strings.Index(view, "Search: press / to filter")
	if optionIndex < 0 || searchIndex < 0 {
		t.Fatalf("expected options and bottom search prompt, got:\n%s", view)
	}
	if searchIndex < optionIndex {
		t.Fatalf("expected search prompt below options, got:\n%s", view)
	}
}

func TestBottomFilterSelectEscapeClearsSearch(t *testing.T) {
	value := "Europe/Moscow"
	field := newBottomFilterSelect().
		Title("Timezone").
		Options(
			huh.NewOption("Europe/Amsterdam", "Europe/Amsterdam"),
			huh.NewOption("Europe/Berlin", "Europe/Berlin"),
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
		).
		Value(&value)

	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Berlin")})
	if !field.GetFiltering() {
		t.Fatal("expected filter mode to be active")
	}
	if len(field.filtered) != 1 {
		t.Fatalf("expected one filtered timezone, got %#v", field.filtered)
	}

	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if field.GetFiltering() {
		t.Fatal("expected escape to leave filter mode")
	}
	if field.filter != "" {
		t.Fatalf("expected escape to clear filter, got %q", field.filter)
	}
	if len(field.filtered) != len(field.options) {
		t.Fatalf("expected escape to restore all options, got %#v", field.filtered)
	}
}

func TestSectionPreviewPackagesExplainsPresetAndRiskyToggles(t *testing.T) {
	state := config.Default()
	model := sectionModel{cursor: sectionIndex(t, "Packages"), state: state}

	preview := strings.Join(sectionPreview(model), "\n")
	for _, want := range []string{
		"Personal keeps the full Takuya package set",
		"CTF tools",
		"Lanzaboote should stay disabled",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected package preview to contain %q, got:\n%s", want, preview)
		}
	}
}

func TestPackagesFormDocumentsSecureBoot(t *testing.T) {
	source, err := os.ReadFile("tui.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`Title("Secure Boot (Lanzaboote)")`,
		`huh.NewSelect[bool]()`,
		"Enable only on machines already prepared for Secure Boot",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected package form source to contain %q", want)
		}
	}
}

func TestRenderContentUsesSelectedSectionHeader(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Region"), state: config.Default()}
	view := renderContent(model, 80, 20)
	if !strings.Contains(view, "Region") {
		t.Fatalf("expected Region header in content view, got:\n%s", view)
	}
	if strings.Contains(view, "Current State") {
		t.Fatalf("content view should not be pinned to Current State, got:\n%s", view)
	}
}

func TestSectionFooterShowsAvailableKeys(t *testing.T) {
	footer := renderFooter(100)
	for _, want := range []string{"↑/↓ or k/j: move", "Enter: open", "q/Esc/Ctrl+C: quit"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("expected section footer to contain %q, got %q", want, footer)
		}
	}
}

func TestInstallerFormKeyMapInputUsesTabForFieldNavigation(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.Input.Next.Keys(), "tab")
	assertHasKey(t, keymap.Input.Prev.Keys(), "shift+tab")
	assertMissingKey(t, keymap.Input.Next.Keys(), "down")
	assertMissingKey(t, keymap.Input.Next.Keys(), "enter")
	assertHasKey(t, keymap.Input.Submit.Keys(), "ctrl+x")
	assertHasKey(t, keymap.Input.Submit.Keys(), "shift+enter")
}

func TestInstallerFormKeyMapConfirmUsesTabForFieldNavigation(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.Confirm.Next.Keys(), "tab")
	assertHasKey(t, keymap.Confirm.Prev.Keys(), "shift+tab")
	assertMissingKey(t, keymap.Confirm.Next.Keys(), "down")
	assertMissingKey(t, keymap.Confirm.Next.Keys(), "enter")
	assertHasKey(t, keymap.Confirm.Submit.Keys(), "ctrl+x")
	assertHasKey(t, keymap.Confirm.Submit.Keys(), "shift+enter")
}

func TestInstallerFormKeyMapSelectKeepsArrowsForOptionsAndTabForFields(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.Select.Up.Keys(), "up")
	assertHasKey(t, keymap.Select.Down.Keys(), "down")
	assertHasKey(t, keymap.Select.Next.Keys(), "tab")
	assertHasKey(t, keymap.Select.Prev.Keys(), "shift+tab")
	assertMissingKey(t, keymap.Select.Next.Keys(), "down")
	assertMissingKey(t, keymap.Select.Next.Keys(), "enter")
	assertHasKey(t, keymap.Select.Submit.Keys(), "ctrl+x")
	assertHasKey(t, keymap.Select.Submit.Keys(), "shift+enter")
	assertHasKey(t, keymap.Select.SetFilter.Keys(), "esc")
	assertHasKey(t, keymap.Select.SetFilter.Keys(), "enter")
	if keymap.Select.Filter.Enabled() {
		t.Fatal("standard huh selects should not open the top-position filter; large lists use bottomFilterSelect")
	}
}

func TestEnterMovesToNextFieldWithoutSubmitting(t *testing.T) {
	value := "takuya"
	form := newForm(
		huh.NewInput().Title("Username").Value(&value),
		huh.NewInput().Title("Full name"),
	)

	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to move to next field")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateNormal {
		t.Fatalf("expected form to stay open before the last field, got state %v", got.form.State)
	}
	if got.focusedFieldIndex() != 1 {
		t.Fatalf("expected enter to focus next field, got %d", got.focusedFieldIndex())
	}
}

func TestEnterCyclesFromLastFieldToFirstWithoutSubmitting(t *testing.T) {
	value := "takuya"
	confirm := "takuya"
	form := newForm(
		huh.NewInput().Title("Password").Value(&value),
		huh.NewInput().Title("Confirm password").Value(&confirm),
	)

	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(submitOnEnterModel)
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected first enter to focus confirm field, got %d", got)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on last field to cycle focus")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateNormal {
		t.Fatalf("expected enter to keep form open, got state %v", got.form.State)
	}
	if got.focusedFieldIndex() != 0 {
		t.Fatalf("expected enter on last field to cycle to first, got %d", got.focusedFieldIndex())
	}
}

func TestEnterSubmitsSingleFieldNote(t *testing.T) {
	form := newForm(huh.NewNote().Title("Doctor").Description("Report"))
	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter on single note to submit form")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateCompleted {
		t.Fatalf("expected single note to complete on enter, got state %v", got.form.State)
	}
}

func TestFormViewAlwaysShowsAvailableKeys(t *testing.T) {
	value := "takuya"
	form := newForm(
		huh.NewInput().Title("Username").Value(&value),
		huh.NewInput().Title("Full name"),
	)
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	model = updated.(submitOnEnterModel)

	view := model.View()
	for _, want := range []string{
		"Tab/Enter: next field",
		"Shift+Tab: previous",
		"Ctrl+X: save section",
		"Esc: back without saving",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected form view to contain key hint %q, got:\n%s", want, view)
		}
	}
}

func TestSingleFieldFormFooterShowsReturnKeys(t *testing.T) {
	form := newForm(huh.NewNote().Title("Doctor").Description("Report"))
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}

	view := model.View()
	if !strings.Contains(view, "Enter/Ctrl+X: return") {
		t.Fatalf("expected single-field footer to show return keys, got:\n%s", view)
	}
}

func TestShiftEnterSubmitsForm(t *testing.T) {
	value := "takuya"
	form := newForm(
		huh.NewInput().Title("Username").Value(&value),
		huh.NewInput().Title("Full name"),
	)

	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("shift+enter")})
	if cmd == nil {
		t.Fatal("expected shift+enter to submit form")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateCompleted {
		t.Fatalf("expected form to complete on shift+enter, got state %v", got.form.State)
	}
}

func TestCtrlXSubmitsForm(t *testing.T) {
	value := "takuya"
	form := newForm(
		huh.NewInput().Title("Username").Value(&value),
		huh.NewInput().Title("Full name"),
	)

	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd == nil {
		t.Fatal("expected ctrl+x to submit form")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateCompleted {
		t.Fatalf("expected form to complete on ctrl+x, got state %v", got.form.State)
	}
}

func TestEscapeReturnsToSectionSelector(t *testing.T) {
	form := newForm(huh.NewInput().Title("Username"))
	model := submitOnEnterModel{form: form.form, fields: form.fields}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected escape to quit the section form")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if !got.back {
		t.Fatal("expected escape to mark form as back-to-sections")
	}
	if got.form.State != huh.StateNormal {
		t.Fatalf("expected escape back to keep form unsubmitted, got state %v", got.form.State)
	}
}

func TestEscapeCancelsFocusedFilterInsteadOfLeavingSection(t *testing.T) {
	value := "Europe/Moscow"
	field := newBottomFilterSelect().
		Title("Timezone").
		Options(
			huh.NewOption("Europe/Berlin", "Europe/Berlin"),
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
		).
		Value(&value)
	form := newForm(field, huh.NewInput().Title("Default locale"))
	model := submitOnEnterModel{form: form.form, fields: form.fields}

	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Berlin")})
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("expected escape inside filter to stay in the section")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.back {
		t.Fatal("escape inside an active filter must not return to section selector")
	}
	if field.GetFiltering() || field.filter != "" {
		t.Fatalf("expected escape to clear only the active filter, filtering=%t filter=%q", field.GetFiltering(), field.filter)
	}
}

func TestEscapeClearsRetainedFilterInsteadOfLeavingSection(t *testing.T) {
	value := "Europe/Moscow"
	field := newBottomFilterSelect().
		Title("Timezone").
		Options(
			huh.NewOption("Europe/Berlin", "Europe/Berlin"),
			huh.NewOption("Europe/Moscow", "Europe/Moscow"),
		).
		Value(&value)
	form := newForm(field, huh.NewInput().Title("Default locale"))
	model := submitOnEnterModel{form: form.form, fields: form.fields}

	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Berlin")})
	_, _ = field.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if field.GetFiltering() || !field.HasFilter() {
		t.Fatalf("expected retained inactive filter, filtering=%t filter=%q", field.GetFiltering(), field.filter)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("expected escape to clear retained filter without leaving the section")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.back {
		t.Fatal("escape with retained filter must not return to section selector")
	}
	if field.HasFilter() {
		t.Fatalf("expected escape to clear retained filter, got %q", field.filter)
	}
}

func TestBackToSectionsRestoresPreviousState(t *testing.T) {
	previousState := config.Default()
	previousState.Host.Hostname = "before"
	previousSecrets := config.Secrets{UserPassword: "old-password"}
	s := &session{
		state:   config.Default(),
		secrets: config.Secrets{UserPassword: "new-password"},
	}
	s.state.Host.Hostname = "after"

	saved, err := handleSectionResult(s, previousState, previousSecrets, errBackToSections)
	if err != nil {
		t.Fatal(err)
	}
	if saved {
		t.Fatal("expected back-to-sections to skip draft save")
	}
	if s.state.Host.Hostname != "before" {
		t.Fatalf("expected previous state to be restored, got %q", s.state.Host.Hostname)
	}
	if s.secrets.UserPassword != "old-password" {
		t.Fatalf("expected previous secrets to be restored, got %q", s.secrets.UserPassword)
	}
}

func TestTabCyclesForwardFromLastFieldToFirst(t *testing.T) {
	form := newForm(
		huh.NewInput().Title("First"),
		huh.NewConfirm().Title("Second"),
	)
	model := submitOnEnterModel{form: form.form, fields: form.fields}

	if got := model.focusedFieldIndex(); got != 0 {
		t.Fatalf("expected initial field 0, got %d", got)
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(submitOnEnterModel)
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected tab to move to field 1, got %d", got)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(submitOnEnterModel)
	if got := model.focusedFieldIndex(); got != 0 {
		t.Fatalf("expected tab on last field to cycle to field 0, got %d", got)
	}
	if model.form.State != huh.StateNormal {
		t.Fatalf("expected cycling tab to keep form open, got state %v", model.form.State)
	}
}

func TestShiftTabCyclesBackwardFromFirstFieldToLast(t *testing.T) {
	form := newForm(
		huh.NewInput().Title("First"),
		huh.NewConfirm().Title("Second"),
	)
	model := submitOnEnterModel{form: form.form, fields: form.fields}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model = updated.(submitOnEnterModel)
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected shift+tab on first field to cycle to field 1, got %d", got)
	}
	if model.form.State != huh.StateNormal {
		t.Fatalf("expected cycling shift+tab to keep form open, got state %v", model.form.State)
	}
}

func assertHasKey(t *testing.T, keys []string, want string) {
	t.Helper()
	for _, got := range keys {
		if got == want {
			return
		}
	}
	t.Fatalf("expected key %q in %v", want, keys)
}

func assertMissingKey(t *testing.T, keys []string, want string) {
	t.Helper()
	for _, got := range keys {
		if got == want {
			t.Fatalf("did not expect key %q in %v", want, keys)
		}
	}
}

func containsOption(options []huh.Option[string], want string) bool {
	for _, option := range options {
		if option.Value == want {
			return true
		}
	}
	return false
}

func overwriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func sectionIndex(t *testing.T, want string) int {
	t.Helper()
	for i, section := range sections {
		if section == want {
			return i
		}
	}
	t.Fatalf("section %q not found", want)
	return 0
}

func hasBlankLineAfterBlock(lines []string, value string) bool {
	for i, line := range lines {
		if line == value && i+1 < len(lines) && lines[i+1] == "" {
			return true
		}
	}
	return false
}
