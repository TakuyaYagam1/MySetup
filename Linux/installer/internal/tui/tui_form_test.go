package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

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
	assertHasKey(t, keymap.Confirm.Submit.Keys(), "enter")
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

func TestEnterChoosesFocusedConfirm(t *testing.T) {
	confirm := true
	form := newForm(
		huh.NewNote().
			Title("Apply summary").
			Description("Summary"),
		huh.NewConfirm().
			Title("Run dry-build and then ask before switch?").
			Affirmative("Apply").
			Negative("Back").
			Value(&confirm),
	)
	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		model = updateFormWithCmd(t, model, initCmd)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected first enter to focus confirm field, got %d", got)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if model.form.State != huh.StateCompleted {
		t.Fatalf("expected enter on focused confirm to complete form, got state %v", model.form.State)
	}
	if !confirm {
		t.Fatal("expected Apply selection to remain true")
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

func updateFormWithCmd(t *testing.T, model submitOnEnterModel, cmd tea.Cmd) submitOnEnterModel {
	t.Helper()
	for i := 0; cmd != nil && i < 8; i++ {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, batchCmd := range batch {
				model = updateFormWithCmd(t, model, batchCmd)
			}
			return model
		}
		updated, nextCmd := model.Update(msg)
		var ok bool
		model, ok = updated.(submitOnEnterModel)
		if !ok {
			t.Fatalf("expected submitOnEnterModel, got %T", updated)
		}
		cmd = nextCmd
	}
	if cmd != nil {
		t.Fatal("form command chain did not settle")
	}
	return model
}
