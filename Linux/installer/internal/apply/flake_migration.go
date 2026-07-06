package apply

import (
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

const (
	legacyNoctaliaV4Input = `    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell/v4.7.7";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`

	currentNoctaliaV5Input = `    noctalia = {
      url = "github:noctalia-dev/noctalia/main";
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

	if !strings.Contains(updated, currentNoctaliaV5Input) {
		if strings.Contains(updated, legacyNoctaliaV4Input) {
			updated = strings.Replace(updated, legacyNoctaliaV4Input, currentNoctaliaV5Input+legacyNoctaliaV4Input, 1)
		} else {
			updated = insertAfterFirst(updated, quickshellInput, currentNoctaliaV5Input)
		}
	}
	if !strings.Contains(updated, legacyNoctaliaV4Input) {
		updated = insertAfterFirst(updated, currentNoctaliaV5Input, legacyNoctaliaV4Input)
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
