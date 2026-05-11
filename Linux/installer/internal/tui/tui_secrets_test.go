package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

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

func TestPasswordsSectionShowsExistingSecretStatus(t *testing.T) {
	model := sectionModel{
		cursor: sectionIndex(t, "Passwords"),
		state:  config.Default(),
		existingSecrets: secretAvailability{
			UserPassword:    secretPresenceExists,
			PgAdminPassword: secretPresenceExists,
		},
	}
	preview := strings.Join(sectionPreview(model), "\n")

	if got := strings.Count(preview, "status: already exists (preserved if left blank)"); got != 2 {
		t.Fatalf("expected both password secrets to show existing status, count=%d preview:\n%s", got, preview)
	}
}

func TestDetectExistingSecrets(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hosts", "NixOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hosts", "NixOS", "hashed-password.nix"), []byte("hash"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets", "pgadmin-password"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := detectExistingSecrets(paths.Options{NixOSDest: dir})
	if got.UserPassword != secretPresenceExists {
		t.Fatalf("expected existing Linux password hash, got %q", got.UserPassword)
	}
	if got.PgAdminPassword != secretPresenceExists {
		t.Fatalf("expected existing pgAdmin password secret, got %q", got.PgAdminPassword)
	}
}

func TestSummaryShowsPendingSecretsWithoutValues(t *testing.T) {
	state := config.Default()
	secrets := config.Secrets{
		UserPassword:    "linux-secret",
		PgAdminPassword: "pgadmin-secret",
	}

	got := summary(state, secrets, secretAvailability{}, "/etc/nixos/mysetup/state.json")
	if !strings.Contains(got, "Passwords: linux-user=ready pgAdmin=ready") {
		t.Fatalf("expected summary to show ready secret status, got:\n%s", got)
	}
	if strings.Contains(got, "linux-secret") || strings.Contains(got, "pgadmin-secret") {
		t.Fatalf("summary must not leak plaintext secrets, got:\n%s", got)
	}
}

func TestSummaryShowsExistingSecretsWithoutValues(t *testing.T) {
	state := config.Default()
	existingSecrets := secretAvailability{
		UserPassword:    secretPresenceExists,
		PgAdminPassword: secretPresenceExists,
	}

	got := summary(state, config.Secrets{}, existingSecrets, "/etc/nixos/mysetup/state.json")
	if !strings.Contains(got, "Passwords: linux-user=existing pgAdmin=existing") {
		t.Fatalf("expected summary to show existing secret status, got:\n%s", got)
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
	if !reflect.DeepEqual(s.state, state) {
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
	}, secretAvailability{})

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
	}, secretAvailability{})

	if !got.empty() {
		t.Fatalf("expected no password form errors, got %#v", got)
	}
}

func TestValidateSecretFormValuesAllowsBlankExistingSecrets(t *testing.T) {
	got := validateSecretFormValues(secretFormValues{}, secretAvailability{
		UserPassword:    secretPresenceExists,
		PgAdminPassword: secretPresenceExists,
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
