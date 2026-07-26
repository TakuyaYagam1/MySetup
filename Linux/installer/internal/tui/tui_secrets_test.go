package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
)

func TestPasswordsSectionExplainsManagedPassword(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Passwords"), state: config.Default()}
	preview := strings.Join(sectionPreview(model), "\n")

	for _, want := range []string{
		"## Linux user password",
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
		cursor:  sectionIndex(t, "Passwords"),
		state:   config.Default(),
		secrets: config.Secrets{UserPassword: "user-secret"},
	}
	preview := strings.Join(sectionPreview(model), "\n")

	if got := strings.Count(preview, "status: ready for apply (session only)"); got != 1 {
		t.Fatalf("expected Linux user password to be ready, count=%d preview:\n%s", got, preview)
	}
	if strings.Contains(preview, "user-secret") {
		t.Fatalf("preview must not leak plaintext secrets, got:\n%s", preview)
	}
}

func TestPasswordsSectionShowsExistingSecretStatus(t *testing.T) {
	model := sectionModel{
		cursor: sectionIndex(t, "Passwords"),
		state:  config.Default(),
		existingSecrets: secretAvailability{
			UserPassword: secretPresenceExists,
		},
	}
	preview := strings.Join(sectionPreview(model), "\n")

	if got := strings.Count(preview, "status: already exists (preserved if left blank)"); got != 1 {
		t.Fatalf("expected Linux user password to show existing status, count=%d preview:\n%s", got, preview)
	}
}

func TestDetectExistingSecrets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hashed-password.nix"), []byte("hash"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := detectExistingSecrets(paths.Options{NixOSDest: dir})
	if got.UserPassword != secretPresenceExists {
		t.Fatalf("expected existing Linux password hash, got %q", got.UserPassword)
	}
}

func TestDetectExistingSecretsAcceptsLegacyHashPath(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "hosts", "NixOS", "hashed-password.nix")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("hash"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := detectExistingSecrets(paths.Options{NixOSDest: dir})
	if got.UserPassword != secretPresenceExists {
		t.Fatalf("expected legacy Linux password hash to be accepted, got %q", got.UserPassword)
	}
}

func TestSummaryShowsPendingSecretsWithoutValues(t *testing.T) {
	state := config.Default()
	secrets := config.Secrets{UserPassword: "linux-secret"}

	got := summary(state, secrets, secretAvailability{}, "/etc/nixos/wahrwelt/state.json")
	if !strings.Contains(got, "Passwords: linux-user=ready") {
		t.Fatalf("expected summary to show ready secret status, got:\n%s", got)
	}
	if strings.Contains(got, "linux-secret") {
		t.Fatalf("summary must not leak plaintext secrets, got:\n%s", got)
	}
}

func TestSummaryShowsExistingSecretsWithoutValues(t *testing.T) {
	state := config.Default()
	existingSecrets := secretAvailability{
		UserPassword: secretPresenceExists,
	}

	got := summary(state, config.Secrets{}, existingSecrets, "/etc/nixos/wahrwelt/state.json")
	if !strings.Contains(got, "Passwords: linux-user=existing") {
		t.Fatalf("expected summary to show existing secret status, got:\n%s", got)
	}
}

func TestSummaryShowsGrafanaAccessWhenObservabilityEnabled(t *testing.T) {
	state := config.Default()
	if strings.Contains(summary(state, config.Secrets{}, secretAvailability{}, "/etc/nixos/wahrwelt/state.json"), "Grafana:") {
		t.Fatal("summary must not show Grafana access when observability is disabled")
	}

	state.Features.Observability = true
	got := summary(state, config.Secrets{}, secretAvailability{}, "/etc/nixos/wahrwelt/state.json")
	if !strings.Contains(got, "Grafana: http://127.0.0.1:3010 initial login admin/admin") {
		t.Fatalf("expected summary to show Grafana access, got:\n%s", got)
	}
}

func TestPasswordFormUsesDirectMaskedInputs(t *testing.T) {
	source, err := os.ReadFile("secrets.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`Title("Linux user password")`,
		`Title("Confirm Linux user password")`,
		`EchoMode(huh.EchoModePassword)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected password form source to contain %q", want)
		}
	}
	if strings.Contains(text, `Title("Set Linux user password")`) {
		t.Fatalf("password flow should not contain enable toggle title")
	}
}

func TestEditSecretsWithReaderStoresPasswordsOnlyInSession(t *testing.T) {
	state := config.Default()
	s := &session{state: state}

	err := editSecretsWithReader(s, func() (string, error) {
		return "linux-secret", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if s.secrets.UserPassword != "linux-secret" {
		t.Fatalf("expected Linux password in session secrets, got %q", s.secrets.UserPassword)
	}
	if !reflect.DeepEqual(s.state, state) {
		t.Fatal("passwords must not mutate persistent installer state")
	}
}

func TestEditSecretsWithReaderPropagatesError(t *testing.T) {
	wantErr := errors.New("password mismatch")
	s := &session{}

	err := editSecretsWithReader(s, func() (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected password reader error, got %v", err)
	}
	if s.secrets.UserPassword != "" {
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
		userPassword: "one",
		userConfirm:  "two",
	}, secretAvailability{})

	if got.userConfirm == "" {
		t.Fatal("expected Linux password confirmation mismatch error")
	}
}

func TestValidateSecretFormValuesAcceptsMatchingPasswords(t *testing.T) {
	got := validateSecretFormValues(secretFormValues{
		userPassword: "linux-secret",
		userConfirm:  "linux-secret",
	}, secretAvailability{})

	if !got.empty() {
		t.Fatalf("expected no password form errors, got %#v", got)
	}
}

func TestValidateSecretFormValuesAllowsBlankExistingSecrets(t *testing.T) {
	got := validateSecretFormValues(secretFormValues{}, secretAvailability{
		UserPassword: secretPresenceExists,
	})

	if !got.empty() {
		t.Fatalf("expected blank existing passwords to be accepted, got %#v", got)
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
