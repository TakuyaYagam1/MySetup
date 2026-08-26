//go:build linux

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const validLegacyHyprMarker = `{"manager":"mysetup","kind":"hypr","version":2}` + "\n"
const validCanonicalHyprMarker = `{"manager":"wahrwelt","kind":"hypr","version":2}` + "\n"

func writeMarkerSchemaFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}

func runMarkerSchemaCheck(home, configHome, oldMarker, newMarker string) (string, error) {
	output, err := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"check", oldMarker, newMarker, configHome, home,
	).CombinedOutput()
	return string(output), err
}

func TestHomeManagerLegacyMarkerMigrationRejectsInvalidSchemaWithoutMutation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		legacy          string
		canonical       string
		canonicalExists bool
	}{
		{name: "legacy missing version", legacy: `{"manager":"mysetup","kind":"hypr"}` + "\n"},
		{name: "legacy wrong version", legacy: `{"manager":"mysetup","kind":"hypr","version":1}` + "\n"},
		{name: "legacy missing kind", legacy: `{"manager":"mysetup","version":2}` + "\n"},
		{name: "legacy wrong kind", legacy: `{"manager":"mysetup","kind":"nvim","version":2}` + "\n"},
		{name: "legacy malformed", legacy: `{"manager":"mysetup",` + "\n"},
		{
			name:            "canonical missing version",
			legacy:          validLegacyHyprMarker,
			canonical:       `{"manager":"wahrwelt","kind":"hypr"}` + "\n",
			canonicalExists: true,
		},
		{
			name:            "canonical wrong version",
			legacy:          validLegacyHyprMarker,
			canonical:       `{"manager":"wahrwelt","kind":"hypr","version":1}` + "\n",
			canonicalExists: true,
		},
		{
			name:            "canonical missing kind",
			legacy:          validLegacyHyprMarker,
			canonical:       `{"manager":"wahrwelt","version":2}` + "\n",
			canonicalExists: true,
		},
		{
			name:            "canonical wrong kind",
			legacy:          validLegacyHyprMarker,
			canonical:       `{"manager":"wahrwelt","kind":"nvim","version":2}` + "\n",
			canonicalExists: true,
		},
		{
			name:            "canonical malformed",
			legacy:          validLegacyHyprMarker,
			canonical:       `{"manager":"wahrwelt",` + "\n",
			canonicalExists: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			oldMarker := filepath.Join(configHome, "hypr", ".mysetup-managed.json")
			newMarker := filepath.Join(configHome, "hypr", ".wahrwelt-managed.json")
			writeMarkerSchemaFixture(t, oldMarker, tc.legacy)
			if tc.canonicalExists {
				writeMarkerSchemaFixture(t, newMarker, tc.canonical)
			}

			output, err := runMarkerSchemaCheck(home, configHome, oldMarker, newMarker)
			if err == nil || !strings.Contains(output, "marker ownership changed") {
				t.Fatalf("invalid marker schema was accepted: err=%v\n%s", err, output)
			}
			if got := readContractFile(t, oldMarker); got != tc.legacy {
				t.Fatalf("legacy marker changed after collision: %q", got)
			}
			if tc.canonicalExists {
				if got := readContractFile(t, newMarker); got != tc.canonical {
					t.Fatalf("canonical marker changed after collision: %q", got)
				}
			} else if _, statErr := os.Lstat(newMarker); !os.IsNotExist(statErr) {
				t.Fatalf("canonical marker was published after collision: %v", statErr)
			}
		})
	}
}

func TestHomeManagerLegacyMarkerMigrationBindsKindToManagedPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		relative  string
		kind      string
		underHome bool
	}{
		{name: "hypr", relative: "hypr", kind: "hypr"},
		{name: "nvim", relative: "nvim", kind: "nvim"},
		{
			name:      "zen chrome",
			relative:  filepath.Join(".zen", "default-release", "chrome"),
			kind:      "zen-chrome",
			underHome: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			root := configHome
			if tc.underHome {
				root = home
			}
			parent := filepath.Join(root, tc.relative)
			oldMarker := filepath.Join(parent, ".mysetup-managed.json")
			newMarker := filepath.Join(parent, ".wahrwelt-managed.json")
			legacy := `{"manager":"mysetup","kind":"` + tc.kind + `","version":2}` + "\n"
			writeMarkerSchemaFixture(t, oldMarker, legacy)

			output, err := runMarkerSchemaCheck(home, configHome, oldMarker, newMarker)
			if err != nil || !strings.HasPrefix(strings.TrimSpace(output), "publish|") {
				t.Fatalf("valid %s marker rejected: err=%v\n%s", tc.kind, err, output)
			}
			if got := readContractFile(t, oldMarker); got != legacy {
				t.Fatalf("preflight changed valid marker: %q", got)
			}
		})
	}

	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	oldMarker := filepath.Join(configHome, "unowned", ".mysetup-managed.json")
	newMarker := filepath.Join(configHome, "unowned", ".wahrwelt-managed.json")
	writeMarkerSchemaFixture(t, oldMarker, validLegacyHyprMarker)
	output, err := runMarkerSchemaCheck(home, configHome, oldMarker, newMarker)
	if err == nil || !strings.Contains(output, "marker ownership changed") {
		t.Fatalf("valid schema at an unknown path was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, oldMarker); got != validLegacyHyprMarker {
		t.Fatalf("unknown-path marker changed after collision: %q", got)
	}
	if _, statErr := os.Lstat(newMarker); !os.IsNotExist(statErr) {
		t.Fatalf("unknown-path marker was canonicalized: %v", statErr)
	}

	syntheticZenOld := filepath.Join(
		configHome, "zen", "default-release", "chrome", ".mysetup-managed.json",
	)
	syntheticZenNew := filepath.Join(
		configHome, "zen", "default-release", "chrome", ".wahrwelt-managed.json",
	)
	syntheticZen := `{"manager":"mysetup","kind":"zen-chrome","version":2}` + "\n"
	writeMarkerSchemaFixture(t, syntheticZenOld, syntheticZen)
	output, err = runMarkerSchemaCheck(
		home, configHome, syntheticZenOld, syntheticZenNew,
	)
	if err == nil || !strings.Contains(output, "marker ownership changed") {
		t.Fatalf("synthetic config-home Zen marker was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, syntheticZenOld); got != syntheticZen {
		t.Fatalf("synthetic Zen marker changed after collision: %q", got)
	}
	if _, statErr := os.Lstat(syntheticZenNew); !os.IsNotExist(statErr) {
		t.Fatalf("synthetic Zen marker was canonicalized: %v", statErr)
	}
}

func TestHomeManagerLegacyMarkerMigrationRevalidatesSchemaAfterPreflight(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		mutateCanonical bool
		replacement     string
	}{
		{
			name:        "legacy version changes",
			replacement: `{"manager":"mysetup","kind":"hypr","version":1}` + "\n",
		},
		{
			name:            "canonical kind changes",
			mutateCanonical: true,
			replacement:     `{"manager":"wahrwelt","kind":"nvim","version":2}` + "\n",
		},
		{
			name:        "legacy becomes malformed",
			replacement: `{"manager":"mysetup",` + "\n",
		},
		{
			name:            "canonical becomes malformed",
			mutateCanonical: true,
			replacement:     `{"manager":"wahrwelt",` + "\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			oldMarker := filepath.Join(configHome, "hypr", ".mysetup-managed.json")
			newMarker := filepath.Join(configHome, "hypr", ".wahrwelt-managed.json")
			writeMarkerSchemaFixture(t, oldMarker, validLegacyHyprMarker)
			if tc.mutateCanonical {
				writeMarkerSchemaFixture(t, newMarker, validCanonicalHyprMarker)
			}
			tokenOutput, err := runMarkerSchemaCheck(home, configHome, oldMarker, newMarker)
			if err != nil {
				t.Fatalf("valid marker preflight failed: %v\n%s", err, tokenOutput)
			}
			target := oldMarker
			if tc.mutateCanonical {
				target = newMarker
			}
			if err := os.WriteFile(target, []byte(tc.replacement), 0o640); err != nil {
				t.Fatal(err)
			}

			output, err := exec.Command(
				"bash", homeManagerLegacyMarkerMigrate,
				"migrate", oldMarker, newMarker, configHome, home,
				strings.TrimSpace(tokenOutput),
			).CombinedOutput()
			if err == nil || !strings.Contains(string(output), "marker ownership changed") {
				t.Fatalf("post-preflight schema change was accepted: err=%v\n%s", err, output)
			}
			if got := readContractFile(t, target); got != tc.replacement {
				t.Fatalf("replacement marker changed after collision: %q", got)
			}
			if !tc.mutateCanonical {
				if _, statErr := os.Lstat(newMarker); !os.IsNotExist(statErr) {
					t.Fatalf("canonical marker was published after schema change: %v", statErr)
				}
			}
		})
	}
}
