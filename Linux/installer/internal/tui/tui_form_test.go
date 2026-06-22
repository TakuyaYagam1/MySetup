package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var errTestValidation = errors.New("test validation")

func TestInstallerFormKeyMapInputUsesTabForFieldNavigation(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.Input.Next.Keys(), "tab")
	assertHasKey(t, keymap.Input.Prev.Keys(), "shift+tab")
	assertHasKey(t, keymap.Input.Prev.Keys(), "shift+enter")
	assertMissingKey(t, keymap.Input.Next.Keys(), "down")
	assertMissingKey(t, keymap.Input.Next.Keys(), "enter")
	assertHasKey(t, keymap.Input.Submit.Keys(), "ctrl+s")
	assertMissingKey(t, keymap.Input.Submit.Keys(), "ctrl+x")
	assertMissingKey(t, keymap.Input.Submit.Keys(), "shift+enter")
}

func TestInstallerFormKeyMapConfirmUsesTabForFieldNavigation(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.Confirm.Next.Keys(), "tab")
	assertHasKey(t, keymap.Confirm.Prev.Keys(), "shift+tab")
	assertHasKey(t, keymap.Confirm.Prev.Keys(), "shift+enter")
	assertMissingKey(t, keymap.Confirm.Next.Keys(), "down")
	assertMissingKey(t, keymap.Confirm.Next.Keys(), "enter")
	assertMissingKey(t, keymap.Confirm.Submit.Keys(), "enter")
	assertHasKey(t, keymap.Confirm.Submit.Keys(), "ctrl+s")
	assertMissingKey(t, keymap.Confirm.Submit.Keys(), "ctrl+x")
	assertMissingKey(t, keymap.Confirm.Submit.Keys(), "shift+enter")
	if keymap.Confirm.Accept.Enabled() || keymap.Confirm.Reject.Enabled() {
		t.Fatal("confirm y/n must be handled by the form wrapper so it does not auto-advance")
	}
}

func TestInstallerFormKeyMapSelectKeepsArrowsForOptionsAndTabForFields(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.Select.Up.Keys(), "up")
	assertHasKey(t, keymap.Select.Up.Keys(), "k")
	assertHasKey(t, keymap.Select.Down.Keys(), "down")
	assertHasKey(t, keymap.Select.Down.Keys(), "j")
	assertHasKey(t, keymap.Select.Next.Keys(), "tab")
	assertHasKey(t, keymap.Select.Prev.Keys(), "shift+tab")
	assertHasKey(t, keymap.Select.Prev.Keys(), "shift+enter")
	assertMissingKey(t, keymap.Select.Next.Keys(), "down")
	assertMissingKey(t, keymap.Select.Next.Keys(), "enter")
	assertHasKey(t, keymap.Select.Submit.Keys(), "ctrl+s")
	assertMissingKey(t, keymap.Select.Submit.Keys(), "ctrl+x")
	assertMissingKey(t, keymap.Select.Submit.Keys(), "shift+enter")
	assertHasKey(t, keymap.Select.SetFilter.Keys(), "esc")
	assertHasKey(t, keymap.Select.SetFilter.Keys(), "enter")
	if keymap.Select.Filter.Enabled() {
		t.Fatal("standard huh selects should not open the top-position filter; large lists use bottomFilterSelect")
	}
}

func TestInstallerFormKeyMapMultiSelectUsesSpaceForToggleAndTabForFields(t *testing.T) {
	keymap := installerFormKeyMap()
	assertHasKey(t, keymap.MultiSelect.Up.Keys(), "up")
	assertHasKey(t, keymap.MultiSelect.Up.Keys(), "k")
	assertHasKey(t, keymap.MultiSelect.Down.Keys(), "down")
	assertHasKey(t, keymap.MultiSelect.Down.Keys(), "j")
	assertHasKey(t, keymap.MultiSelect.Toggle.Keys(), " ")
	assertMissingKey(t, keymap.MultiSelect.Toggle.Keys(), "x")
	assertHasKey(t, keymap.MultiSelect.Next.Keys(), "tab")
	assertHasKey(t, keymap.MultiSelect.Prev.Keys(), "shift+tab")
	assertHasKey(t, keymap.MultiSelect.Prev.Keys(), "shift+enter")
	assertMissingKey(t, keymap.MultiSelect.Next.Keys(), "enter")
	assertHasKey(t, keymap.MultiSelect.Submit.Keys(), "ctrl+s")
	assertMissingKey(t, keymap.MultiSelect.Submit.Keys(), "ctrl+x")
	assertMissingKey(t, keymap.MultiSelect.Submit.Keys(), "shift+enter")
	if keymap.MultiSelect.SelectAll.Enabled() || keymap.MultiSelect.SelectNone.Enabled() {
		t.Fatal("multi-select bulk toggles must stay disabled; Space is the only toggle key")
	}
}

func TestBottomFilterSelectReportsValidationErrors(t *testing.T) {
	value := "bad"
	field := newBottomFilterSelect().
		Options(huh.NewOption("good", "good")).
		Value(&value).
		Validate(func(value string) error {
			if value != "good" {
				return errTestValidation
			}
			return nil
		})
	if err := field.Error(); err == nil {
		t.Fatal("expected bottom filter select validation error")
	}
	value = "good"
	if err := field.Error(); err != nil {
		t.Fatalf("expected bottom filter select to validate: %v", err)
	}
}

func TestBottomFilterSelectValidationBlocksFormSubmit(t *testing.T) {
	value := "bad"
	field := newBottomFilterSelect().
		Options(huh.NewOption("good", "good")).
		Value(&value).
		Validate(func(value string) error {
			if value != "good" {
				return errTestValidation
			}
			return nil
		})
	form := newForm(field)
	form.form.SubmitCmd = tea.Quit
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		t.Fatal("expected invalid bottom filter select to block submit command")
	}
	got := updated.(submitOnEnterModel)
	if got.form.State != huh.StateNormal {
		t.Fatalf("expected invalid form to stay normal, got state %v", got.form.State)
	}

	value = "good"
	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("expected valid bottom filter select to submit")
	}
	got = updated.(submitOnEnterModel)
	if got.form.State != huh.StateCompleted {
		t.Fatalf("expected valid form to complete, got state %v", got.form.State)
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

func TestEnterOnOnlyNavigableConfirmSubmits(t *testing.T) {
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
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected init to focus confirm field, got %d", got)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if model.form.State != huh.StateCompleted {
		t.Fatalf("expected enter on the only navigable confirm to complete form, got state %v", model.form.State)
	}
	if !confirm {
		t.Fatal("expected Apply selection to remain true")
	}
}

func TestConfirmYNChangesValueWithoutAdvancing(t *testing.T) {
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
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		model = updateFormWithCmd(t, model, initCmd)
	}
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected init to focus confirm field, got %d", got)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if confirm {
		t.Fatal("expected n to choose the negative value")
	}
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected n to keep focus on confirm field, got %d", got)
	}
	if model.form.State != huh.StateNormal {
		t.Fatalf("expected n to keep form open, got state %v", model.form.State)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if !confirm {
		t.Fatal("expected y to choose the affirmative value")
	}
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected y to keep focus on confirm field, got %d", got)
	}
}

func TestEnterSubmitsSingleConfirm(t *testing.T) {
	confirm := true
	form := newForm(
		huh.NewConfirm().
			Title("Cleanup").
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
	if cmd == nil {
		t.Fatal("expected enter on single confirm to submit the form")
	}
	got := updated.(submitOnEnterModel)
	if got.form.State != huh.StateCompleted {
		t.Fatalf("expected enter on single confirm to complete form, got state %v", got.form.State)
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
		"Tab/Enter: next",
		"Shift+Tab/Shift+Enter: previous",
		"Ctrl+S: save",
		"Esc: menu",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected form view to contain key hint %q, got:\n%s", want, view)
		}
	}
}

func TestConfirmFormFooterShowsContinueKey(t *testing.T) {
	confirm := true
	form := newForm(huh.NewConfirm().Title("Apply").Value(&confirm))
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		model = updateFormWithCmd(t, model, initCmd)
	}

	view := model.View()
	if !strings.Contains(view, "Tab/Enter: continue") {
		t.Fatalf("expected confirm footer to show continue key, got:\n%s", view)
	}
}

func TestSingleFieldFormFooterShowsReturnKeys(t *testing.T) {
	form := newForm(huh.NewNote().Title("Doctor").Description("Report"))
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}

	view := model.View()
	if !strings.Contains(view, "Tab/Enter: return") {
		t.Fatalf("expected single-field footer to show return keys, got:\n%s", view)
	}
}

func TestWindowSizedBottomFilterFormKeepsFocusedFieldHeaderVisible(t *testing.T) {
	timeZone := "Europe/Moscow"
	locale := "en_US.UTF-8"
	extraLocale := "ru_RU.UTF-8"
	consoleKeymap := "us"
	form := newForm(
		newBottomFilterSelect().
			Title("Timezone").
			Description("Nix time.timeZone value.").
			Options(
				huh.NewOption("Europe/Malta", "Europe/Malta"),
				huh.NewOption("Europe/Mariehamn", "Europe/Mariehamn"),
				huh.NewOption("Europe/Minsk", "Europe/Minsk"),
				huh.NewOption("Europe/Monaco", "Europe/Monaco"),
				huh.NewOption("Europe/Moscow", "Europe/Moscow"),
				huh.NewOption("Europe/Nicosia", "Europe/Nicosia"),
				huh.NewOption("Europe/Oslo", "Europe/Oslo"),
				huh.NewOption("Europe/Paris", "Europe/Paris"),
			).
			Height(14).
			Value(&timeZone),
		newBottomFilterSelect().
			Title("Default locale").
			Description("Primary glibc locale.").
			Options(
				huh.NewOption("de_DE.UTF-8", "de_DE.UTF-8"),
				huh.NewOption("en_GB.UTF-8", "en_GB.UTF-8"),
				huh.NewOption("en_US.UTF-8", "en_US.UTF-8"),
				huh.NewOption("es_ES.UTF-8", "es_ES.UTF-8"),
			).
			Height(12).
			Value(&locale),
		newBottomFilterSelect().
			Title("Extra locale").
			Description("Additional generated locale.").
			Options(
				huh.NewOption("ja_JP.UTF-8", "ja_JP.UTF-8"),
				huh.NewOption("nl_NL.UTF-8", "nl_NL.UTF-8"),
				huh.NewOption("ru_RU.UTF-8", "ru_RU.UTF-8"),
				huh.NewOption("uk_UA.UTF-8", "uk_UA.UTF-8"),
			).
			Height(12).
			Value(&extraLocale),
		newBottomFilterSelect().
			Title("Console keymap").
			Description("TTY keyboard map only.").
			Options(
				huh.NewOption("de", "de"),
				huh.NewOption("fr", "fr"),
				huh.NewOption("ru", "ru"),
				huh.NewOption("us", "us"),
				huh.NewOption("uk", "uk"),
			).
			Height(12).
			Value(&consoleKeymap),
	)
	model := submitOnEnterModel{form: form.form, fields: form.fields}
	if initCmd := model.Init(); initCmd != nil {
		_, _ = model.Update(initCmd())
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	model = updated.(submitOnEnterModel)
	if view := model.View(); !strings.Contains(view, "Timezone") || !strings.Contains(view, "Nix time.timeZone value.") {
		t.Fatalf("expected focused first field header to stay visible, got:\n%s", view)
	} else {
		assertWindowSizedFormView(t, view, 30, "Timezone")
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if view := model.View(); !strings.Contains(view, "Default locale") || !strings.Contains(view, "Primary glibc locale.") {
		t.Fatalf("expected focused next field header to stay visible, got:\n%s", view)
	} else {
		assertWindowSizedFormView(t, view, 30, "Default locale")
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updateFormWithCmd(t, updated.(submitOnEnterModel), cmd)
	if view := model.View(); !strings.Contains(view, "Extra locale") || !strings.Contains(view, "Additional generated locale.") {
		t.Fatalf("expected focused third field header to stay visible, got:\n%s", view)
	} else {
		assertWindowSizedFormView(t, view, 30, "Extra locale")
	}
}

func assertWindowSizedFormView(t *testing.T, view string, height int, firstVisibleText string) {
	t.Helper()
	if got := lipgloss.Height(view); got != height {
		t.Fatalf("expected form view height to fit the terminal (%d), got %d:\n%s", height, got, view)
	}
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	firstContentLine := ""
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstContentLine = line
			break
		}
	}
	if !strings.Contains(firstContentLine, firstVisibleText) {
		t.Fatalf("expected first visible form content line to contain %q, got:\n%s", firstVisibleText, view)
	}
	if strings.Contains(view, "save section") {
		t.Fatalf("expected custom footer to replace huh help footer, got:\n%s", view)
	}
}

func TestShiftEnterMovesToPreviousField(t *testing.T) {
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
		t.Fatal("expected enter to move to the next field")
	}
	model = updated.(submitOnEnterModel)
	if got := model.focusedFieldIndex(); got != 1 {
		t.Fatalf("expected enter to focus second field, got %d", got)
	}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("shift+enter")})
	if cmd == nil {
		t.Fatal("expected shift+enter to move to the previous field")
	}
	got := updated.(submitOnEnterModel)
	if got.form.State != huh.StateNormal {
		t.Fatalf("expected shift+enter to keep form open, got state %v", got.form.State)
	}
	if got.focusedFieldIndex() != 0 {
		t.Fatalf("expected shift+enter to focus previous field, got %d", got.focusedFieldIndex())
	}
}

func TestCtrlSSubmitsForm(t *testing.T) {
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
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("expected ctrl+s to submit form")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateCompleted {
		t.Fatalf("expected form to complete on ctrl+s, got state %v", got.form.State)
	}
}

func TestCtrlXDoesNotSubmitForm(t *testing.T) {
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
	if cmd != nil {
		t.Fatal("expected ctrl+x to be ignored by the form wrapper")
	}
	got, ok := updated.(submitOnEnterModel)
	if !ok {
		t.Fatalf("expected submitOnEnterModel, got %T", updated)
	}
	if got.form.State != huh.StateNormal {
		t.Fatalf("expected form to stay open on ctrl+x, got state %v", got.form.State)
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
		if msg == nil {
			return model
		}
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
