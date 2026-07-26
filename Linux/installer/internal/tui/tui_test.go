package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

func TestSectionProgramUsesAltScreen(t *testing.T) {
	if len(sectionProgramOptions()) == 0 {
		t.Fatal("expected Bubble Tea program options")
	}
	source, err := os.ReadFile("section_model.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "tea.WithAltScreen()") {
		t.Fatal("expected section program to use tea.WithAltScreen")
	}
}

func TestNewSectionModelUsesInitialCursor(t *testing.T) {
	model := newSectionModel(config.Default(), config.Secrets{}, secretAvailability{}, 1)
	if got := selectedSection(model); got != "User" {
		t.Fatalf("expected User to stay selected, got %q", got)
	}
}

func TestNewSectionModelClampsInitialCursor(t *testing.T) {
	if got := newSectionModel(config.Default(), config.Secrets{}, secretAvailability{}, -1).cursor; got != 0 {
		t.Fatalf("expected negative cursor to clamp to 0, got %d", got)
	}
	if got := newSectionModel(config.Default(), config.Secrets{}, secretAvailability{}, len(sections)+10).cursor; got != len(sections)-1 {
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
	for _, want := range []string{"↑/↓ or j/k: move", "Enter: open", "Esc/Ctrl+C: quit"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("expected section footer to contain %q, got %q", want, footer)
		}
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
