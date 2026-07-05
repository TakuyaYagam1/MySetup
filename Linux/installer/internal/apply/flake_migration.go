package apply

import (
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

const (
	legacyNoctaliaInput = `    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell/v4.7.7";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`

	currentNoctaliaInput = `    noctalia = {
      url = "github:noctalia-dev/noctalia/v5.0.0-beta1";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`

	quickshellInput = `    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`

	legacyTemplInput = `    templ = {
      url = "github:a-h/templ";
      inputs = {
        nixpkgs.follows = "nixpkgs-stable";
        nixpkgs-unstable.follows = "nixpkgs";
      };
    };
`
)

func insertAfterFirst(text, anchor, insert string) string {
	index := strings.Index(text, anchor)
	if index == -1 {
		return text
	}
	index += len(anchor)
	return text[:index] + insert + text[index:]
}

func migrateGeneratedThinFlake(text, channel string) (string, bool) {
	updated := text
	updated = strings.Replace(updated, legacyTemplInput, "", 1)
	updated = strings.ReplaceAll(updated, `      inputs.templ.follows = "templ";
`, "")

	if !strings.Contains(updated, currentNoctaliaInput) {
		if strings.Contains(updated, legacyNoctaliaInput) {
			updated = strings.Replace(updated, legacyNoctaliaInput, currentNoctaliaInput+legacyNoctaliaInput, 1)
		} else {
			updated = insertAfterFirst(updated, quickshellInput, currentNoctaliaInput)
		}
	}
	if !strings.Contains(updated, legacyNoctaliaInput) {
		updated = insertAfterFirst(updated, currentNoctaliaInput, legacyNoctaliaInput)
	}

	if !strings.Contains(updated, `      inputs.noctalia.follows = "noctalia";`) {
		if strings.Contains(updated, `      inputs.noctalia-shell.follows = "noctalia-shell";`) {
			updated = strings.Replace(
				updated,
				`      inputs.noctalia-shell.follows = "noctalia-shell";`,
				`      inputs.noctalia.follows = "noctalia";
      inputs.noctalia-shell.follows = "noctalia-shell";`,
				1,
			)
		} else {
			updated = insertAfterFirst(updated, `      inputs.quickshell.follows = "quickshell";
`, `      inputs.noctalia.follows = "noctalia";
`)
		}
	}
	if !strings.Contains(updated, `      inputs.noctalia-shell.follows = "noctalia-shell";`) {
		updated = strings.Replace(
			updated,
			`      inputs.noctalia.follows = "noctalia";`,
			`      inputs.noctalia.follows = "noctalia";
      inputs.noctalia-shell.follows = "noctalia-shell";`,
			1,
		)
	}

	desiredMySetupURL := config.MySetupFlakeURL(channel)
	for _, flakeURL := range config.KnownMySetupFlakeURLs() {
		updated = strings.ReplaceAll(updated, flakeURL, desiredMySetupURL)
	}

	return updated, updated != text
}
