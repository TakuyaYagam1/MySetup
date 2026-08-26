package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExistingMissingFileErrors(t *testing.T) {
	_, err := LoadExisting(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected missing state error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing state to wrap os.ErrNotExist, got %v", err)
	}
}

func TestLoadExistingInvalidJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{ nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadExisting(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestLoadExistingRejectsDuplicateJSONFieldsRecursively(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "top level", data: `{"schemaVersion":7,"schemaVersion":7}`},
		{name: "current nested object", data: `{"features":{"secureBoot":false,"secureBoot":true}}`},
		{name: "historical nested object", data: `{"schemaVersion":7,"zapret":{"enable":false,"enable":true,"config":"general"}}`},
		{name: "object nested in array", data: `{"display":{"extraMonitors":[{"name":"DP-1","name":"DP-2"}]}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadExisting(path); err == nil || !strings.Contains(err.Error(), "duplicate JSON field") {
				t.Fatalf("LoadExisting() error = %v, want recursive duplicate rejection", err)
			}
		})
	}
}

func TestLoadExistingRejectsNoncanonicalJSONFieldCasing(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "lone root alias", data: `{"User":{"username":"alice"}}`},
		{name: "root canonical plus alias", data: `{"user":{"username":"alice"},"User":{"username":"mallory"}}`},
		{name: "lone current nested alias", data: `{"features":{"SecureBoot":true}}`},
		{name: "current nested canonical plus alias", data: `{"features":{"secureBoot":false,"SecureBoot":true}}`},
		{name: "lone historical root alias", data: `{"schemaVersion":3,"Shell":{"profile":"caelestia"}}`},
		{name: "lone historical nested alias", data: `{"schemaVersion":7,"zapret":{"Enable":false,"config":"general"}}`},
		{name: "historical nested canonical plus alias", data: `{"schemaVersion":7,"zapret":{"enable":false,"Enable":true,"config":"general"}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadExisting(path); err == nil || !strings.Contains(err.Error(), "exact canonical JSON field") {
				t.Fatalf("LoadExisting() error = %v, want exact field-name rejection", err)
			}
		})
	}
}

func TestLoadExistingRejectsNullInGeneratedStateFields(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "root scalar", data: `{"schemaVersion":null}`},
		{name: "root object", data: `{"user":null}`},
		{name: "nested string", data: `{"user":{"username":null}}`},
		{name: "nested boolean", data: `{"features":{"secureBoot":null}}`},
		{name: "extra monitors array", data: `{"display":{"extraMonitors":null}}`},
		{name: "monitor entry", data: `{"display":{"extraMonitors":[null]}}`},
		{name: "monitor nested string", data: `{"display":{"extraMonitors":[{"name":null}]}}`},
		{name: "historical object", data: `{"schemaVersion":7,"zapret":null}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadExisting(path); err == nil || !strings.Contains(err.Error(), "must be a non-null JSON") {
				t.Fatalf("LoadExisting() error = %v, want strict non-null type rejection", err)
			}
		})
	}
}

func TestLoadExistingMigratesStateWithoutSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{
	  "user": {
	    "username": "alice"
  },
  "git": {
    "email": "alice@example.com"
  }
	}
	`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("expected migrated schema %d, got %d", SchemaVersion, got.SchemaVersion)
	}
	if got.User.Username != "alice" {
		t.Fatalf("expected existing username to be preserved, got %q", got.User.Username)
	}
	if got.Host.StateVersion != Default().Host.StateVersion {
		t.Fatalf("expected default stateVersion to be filled, got %q", got.Host.StateVersion)
	}
	if got.Source.Channel != SourceChannelStable {
		t.Fatalf("expected migrated source channel %q, got %q", SourceChannelStable, got.Source.Channel)
	}
}

func TestLoadGeneratedExistingRejectsPartialStateWithoutChangingDraftLoading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	payload := []byte(`{"user":{"username":"alice"}}` + "\n")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadExisting(path); err != nil {
		t.Fatalf("LoadExisting() rejected supported partial draft state: %v", err)
	}
	if _, err := LoadGeneratedExisting(path); err == nil || !strings.Contains(err.Error(), "missing required field") {
		t.Fatalf("LoadGeneratedExisting() error = %v, want incomplete generated-state rejection", err)
	} else if !IsGeneratedStateShapeError(err) {
		t.Fatalf("LoadGeneratedExisting() error = %v, want stable generated-state shape classification", err)
	}
}

func TestGeneratedStateShapeErrorExcludesReadAndOwnershipFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{}"+strings.Repeat(" ", MaxGeneratedStateBytes)), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGeneratedExisting(path)
	if err == nil {
		t.Fatal("LoadGeneratedExisting() accepted oversized state")
	}
	if IsGeneratedStateShapeError(err) {
		t.Fatalf("oversized read error was misclassified as a stable shape rejection: %v", err)
	}
}

func TestLoadGeneratedExistingRequiresEveryVersionedRootField(t *testing.T) {
	required := map[int][]string{
		0: {"host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "shell", "services", "zapret"},
		1: {"host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "shell", "services", "zapret"},
		2: {"host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "shell", "services", "zapret"},
		3: {"host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "services", "zapret"},
		4: {"schemaVersion", "host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "services", "zapret"},
		5: {"schemaVersion", "host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "zapret"},
		6: {"schemaVersion", "host", "source", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "zapret"},
		7: {"schemaVersion", "host", "source", "user", "locale", "git", "packages", "display", "hardware", "features", "dots", "noctalia"},
	}

	for schema, fields := range required {
		for _, field := range fields {
			t.Run(fmt.Sprintf("schema-%d-without-%s", schema, field), func(t *testing.T) {
				payload := mutateSystemMigrationStatePayload(schema, func(state map[string]any) {
					delete(state, field)
				})
				path := filepath.Join(t.TempDir(), "state.json")
				if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := LoadGeneratedExisting(path); err == nil {
					t.Fatalf("LoadGeneratedExisting() accepted schema %d without required root field %q", schema, field)
				}
			})
		}
	}
}

func TestLoadGeneratedExistingRequiresVersionedNestedFields(t *testing.T) {
	required := map[int][]string{
		0: {"host.hostname", "host.stateVersion", "user.username", "user.fullName", "user.homeDirectory", "locale.timeZone", "locale.defaultLocale", "locale.extraLocale", "locale.consoleKeyMap", "locale.weatherLocation", "locale.keyboardLayouts", "locale.keyboardToggle", "git.username", "git.email", "packages.preset", "display.monitorName", "display.monitorMode", "display.monitorPosition", "display.monitorScale", "hardware.gpu", "features.secureBoot", "features.ctfTools", "features.omniRouter", "features.russiaMode", "dots.hypr", "dots.zenTheme", "dots.sine", "dots.neovim", "dots.v2rayN", "dots.wallpapers", "shell.profile", "services.pgAdminEmail", "zapret.enable", "zapret.config"},
		3: {"features.secureBoot", "features.ctfTools", "features.omniRouter", "features.russiaMode", "services.pgAdminEmail", "zapret.enable", "zapret.config"},
		4: {"features.secureBoot", "features.ctfTools", "features.omniRouter", "features.observability", "features.russiaMode", "dots.neovimCleanState", "services.pgAdminEmail", "zapret.enable", "zapret.config"},
		5: {"features.secureBoot", "features.ctfTools", "features.omniRouter", "features.observability", "dots.neovimCleanState", "zapret.enable", "zapret.config", "display.extraMonitors.0.name", "display.extraMonitors.0.mode", "display.extraMonitors.0.position", "display.extraMonitors.0.scale"},
		6: {"source.channel", "features.secureBoot", "features.ctfTools", "features.omniRouter", "features.observability", "zapret.enable", "zapret.config", "display.extraMonitors.0.name", "display.extraMonitors.0.mode", "display.extraMonitors.0.position", "display.extraMonitors.0.scale"},
		7: {"source.channel", "noctalia.version", "features.secureBoot", "features.ctfTools", "features.omniRouter", "features.observability", "display.extraMonitors.0.name", "display.extraMonitors.0.mode", "display.extraMonitors.0.position", "display.extraMonitors.0.scale"},
	}

	for schema, fields := range required {
		for _, field := range fields {
			t.Run(fmt.Sprintf("schema-%d-without-%s", schema, strings.ReplaceAll(field, ".", "-")), func(t *testing.T) {
				payload := mutateSystemMigrationStatePayload(schema, func(state map[string]any) {
					deleteGeneratedStateTestField(state, field)
				})
				path := filepath.Join(t.TempDir(), "state.json")
				if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := LoadGeneratedExisting(path); err == nil {
					t.Fatalf("LoadGeneratedExisting() accepted schema %d without required nested field %q", schema, field)
				}
			})
		}
	}
}

func TestLoadGeneratedExistingRejectsImpossibleReleasedTransitions(t *testing.T) {
	tests := []struct {
		name   string
		schema int
		mutate func(map[string]any)
	}{
		{
			name:   "schema 5 services without Russia mode",
			schema: 5,
			mutate: func(state map[string]any) {
				delete(state["features"].(map[string]any), "russiaMode")
			},
		},
		{
			name:   "schema 7 Zapret with Portainer",
			schema: 7,
			mutate: func(state map[string]any) {
				state["zapret"] = map[string]any{"enable": false, "config": "general"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(mutateSystemMigrationStatePayload(test.schema, test.mutate)), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadGeneratedExisting(path); err == nil || !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("LoadGeneratedExisting() error = %v, want impossible transition rejection", err)
			}
		})
	}
}

func TestLoadGeneratedExistingRejectsNullAndWrongTypes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "root scalar null", mutate: func(state map[string]any) { state["schemaVersion"] = nil }},
		{name: "root object null", mutate: func(state map[string]any) { state["user"] = nil }},
		{name: "nested scalar null", mutate: func(state map[string]any) { state["features"].(map[string]any)["secureBoot"] = nil }},
		{name: "nested scalar wrong type", mutate: func(state map[string]any) { state["host"].(map[string]any)["hostname"] = false }},
		{name: "monitor array null", mutate: func(state map[string]any) { state["display"].(map[string]any)["extraMonitors"] = nil }},
		{name: "monitor entry null", mutate: func(state map[string]any) { state["display"].(map[string]any)["extraMonitors"] = []any{nil} }},
		{name: "monitor field null", mutate: func(state map[string]any) {
			state["display"].(map[string]any)["extraMonitors"].([]any)[0].(map[string]any)["name"] = nil
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(mutateSystemMigrationStatePayload(7, test.mutate)), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadGeneratedExisting(path); err == nil {
				t.Fatal("LoadGeneratedExisting() accepted a null or wrong-type generated field")
			}
		})
	}
}

func TestLoadGeneratedExistingRejectsInvalidUTF8AndOversizedPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name: "invalid UTF-8",
			payload: bytes.Replace(
				[]byte(systemMigrationStatePayload(7)),
				[]byte("NixOS"),
				[]byte{'N', 'i', 'x', 0xff, 'O', 'S'},
				1,
			),
		},
		{
			name:    "over one MiB",
			payload: append([]byte(systemMigrationStatePayload(7)), bytes.Repeat([]byte{' '}, 1<<20)...),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, test.payload, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadGeneratedExisting(path); err == nil {
				t.Fatal("LoadGeneratedExisting() accepted unowned raw state bytes")
			}
		})
	}
}

func deleteGeneratedStateTestField(state map[string]any, path string) {
	parts := strings.Split(path, ".")
	current := any(state)
	for _, part := range parts[:len(parts)-1] {
		if part == "0" {
			current = current.([]any)[0]
			continue
		}
		current = current.(map[string]any)[part]
	}
	delete(current.(map[string]any), parts[len(parts)-1])
}

func TestLoadExistingRejectsFutureSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":999}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadExisting(path)
	if err == nil {
		t.Fatal("expected schema error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported schema error, got %v", err)
	}
}

func TestLoadExistingRejectsNegativeSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":-1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadExisting(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("LoadExisting() error = %v, want unsupported negative schema", err)
	}
}

func TestLoadExistingRejectsUnknownStateFields(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "top level", data: `{"owner":"human"}`},
		{name: "current nested object", data: `{"features":{"owner":"human"}}`},
		{name: "historical nested object", data: `{"schemaVersion":7,"zapret":{"enable":false,"config":"general","owner":"human"}}`},
		{name: "historical field with invalid shape", data: `{"schemaVersion":3,"shell":"caelestia"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadExisting(path); err == nil || !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("LoadExisting() error = %v, want unknown field rejection", err)
			}
		})
	}
}

func TestLoadExistingAcceptsAllowlistedHistoricalStateFields(t *testing.T) {
	// These shapes are the removed managed fields from state.go history:
	// schema 3 had shell/services, schema 5 had russiaMode, schema 6 had
	// neovimCleanState, and schema 7 had zapret.
	tests := []struct {
		name string
		data string
	}{
		{
			name: "schema 0 all historical fields",
			data: `{
  "shell": {"profile": "caelestia"},
  "services": {"pgAdminEmail": "admin@localhost.local"},
  "features": {"russiaMode": false},
  "dots": {"neovimCleanState": false},
  "zapret": {"enable": false, "config": "general (FAKE_TLS_AUTO_ALT3)"}
}`,
		},
		{
			name: "schema 2 shell services russia and zapret",
			data: `{
  "schemaVersion": 2,
  "shell": {"profile": "caelestia"},
  "services": {"pgAdminEmail": "admin@localhost.local"},
  "features": {"russiaMode": false},
  "zapret": {"enable": false, "config": "general (FAKE_TLS_AUTO_ALT3)"}
}`,
		},
		{
			name: "schema 3 shell services russia and zapret",
			data: `{
  "schemaVersion": 3,
  "shell": {"profile": "caelestia"},
  "services": {"pgAdminEmail": "admin@localhost.local"},
  "features": {"russiaMode": false},
  "zapret": {"enable": false, "config": "general (FAKE_TLS_AUTO_ALT3)"}
}`,
		},
		{
			name: "schema 4 neovim cleanup services russia and zapret",
			data: `{
  "schemaVersion": 4,
  "services": {"pgAdminEmail": "admin@localhost.local"},
  "features": {"russiaMode": false},
  "dots": {"neovimCleanState": false},
  "zapret": {"enable": false, "config": "general (FAKE_TLS_AUTO_ALT3)"}
}`,
		},
		{
			name: "schema 6 neovim cleanup and zapret",
			data: `{
  "schemaVersion": 6,
  "dots": {"neovimCleanState": false},
  "zapret": {"enable": true, "config": "general (FAKE_TLS_AUTO_ALT3)"}
}`,
		},
		{
			name: "schema 7 zapret",
			data: `{
  "schemaVersion": 7,
  "zapret": {"enable": false, "config": "general (FAKE_TLS_AUTO_ALT3)"}
}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadExisting(path); err != nil {
				t.Fatalf("LoadExisting() rejected historical managed state: %v", err)
			}
		})
	}
}

func TestLoadExistingRejectsHistoricalFieldsOutsideTheirSchema(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "shell after schema 3", data: `{"schemaVersion":4,"shell":{"profile":"caelestia"}}`},
		{name: "services after schema 5", data: `{"schemaVersion":6,"services":{"pgAdminEmail":"admin@localhost.local"}}`},
		{name: "russia mode after schema 5", data: `{"schemaVersion":6,"features":{"russiaMode":false}}`},
		{name: "neovim cleanup before schema 4", data: `{"schemaVersion":3,"dots":{"neovimCleanState":false}}`},
		{name: "neovim cleanup after schema 6", data: `{"schemaVersion":7,"dots":{"neovimCleanState":false}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadExisting(path); err == nil || !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("LoadExisting() error = %v, want schema-aware historical field rejection", err)
			}
		})
	}
}

func TestDefaultConsoleKeyMapIsUS(t *testing.T) {
	state := Default()
	if state.Locale.ConsoleKeyMap != "us" {
		t.Fatalf("expected safe TTY console keymap default us, got %q", state.Locale.ConsoleKeyMap)
	}
	if state.Locale.KeyboardLayouts != "us,ru" {
		t.Fatalf("expected graphical Hypr layouts to keep ru support, got %q", state.Locale.KeyboardLayouts)
	}
}

func TestDefaultSourceChannelIsStable(t *testing.T) {
	state := Default()
	if state.Source.Channel != SourceChannelStable {
		t.Fatalf("expected default source channel %q, got %q", SourceChannelStable, state.Source.Channel)
	}
}

func TestDefaultFeatureAndDotsToggles(t *testing.T) {
	state := Default()
	if state.Features.SecureBoot {
		t.Fatal("secure boot must be disabled by default")
	}
	if !state.Dots.Sine {
		t.Fatal("sine profile install should be enabled by default")
	}
	if !state.Dots.Neovim {
		t.Fatal("neovim sync should be enabled by default")
	}
	if state.Noctalia.Version != NoctaliaVersionV5 {
		t.Fatalf("expected noctalia version default %q, got %q", NoctaliaVersionV5, state.Noctalia.Version)
	}
}

func TestValidateRejectsUnsafeHomeDirectory(t *testing.T) {
	for name, home := range map[string]string{
		"traversal":       "/home/../root",
		"relative":        "home/takuya",
		"wrong username":  "/home/other",
		"unclean subpath": "/home/takuya/..",
	} {
		t.Run(name, func(t *testing.T) {
			state := Default()
			state.User.Username = "takuya"
			state.User.HomeDirectory = home

			err := Validate(state)
			if err == nil {
				t.Fatalf("expected unsafe home directory %q to fail validation", home)
			}
		})
	}
}
