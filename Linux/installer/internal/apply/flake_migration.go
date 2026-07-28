package apply

import (
	"regexp"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

const (
	legacyTemplInput = `    templ = {
      url = "github:a-h/templ";
      inputs = {
        nixpkgs.follows = "nixpkgs-stable";
        nixpkgs-unstable.follows = "nixpkgs";
      };
    };
`

	legacyZapretDiscordYoutubeInput = `    zapret-discord-youtube = {
      url = "github:kartavkun/zapret-discord-youtube";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`
	legacyZapretDiscordYoutubeFollows = `      inputs.zapret-discord-youtube.follows = "zapret-discord-youtube";
`

	claudeCodeInput = `    claude-code = {
      url = "github:sadjow/claude-code-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`
	codexInput = `    codex = {
      url = "github:sadjow/codex-cli-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`
	lanzabooteInput = `    lanzaboote = {
      url = "github:nix-community/lanzaboote";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`

	claudeCodeFollows = `      inputs.claude-code.follows = "claude-code";
`
	codexFollows = `      inputs.codex.follows = "codex";
`
	lanzabooteFollows = `      inputs.lanzaboote.follows = "lanzaboote";
`

	zenBrowserInputAnchor = `    zen-browser = {
      url = "github:youwen5/zen-browser-flake";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`
	neovimNightlyOverlayInputAnchor = `    neovim-nightly-overlay = {
      url = "github:nix-community/neovim-nightly-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
`
	zenBrowserFollowsAnchor = `      inputs.zen-browser.follows = "zen-browser";
`
	neovimNightlyOverlayFollowsAnchor = `      inputs.neovim-nightly-overlay.follows = "neovim-nightly-overlay";
`
)

var noctaliaV5InputPattern = regexp.MustCompile(`    noctalia = \{\n      url = "[^"\n]*";\n      inputs\.nixpkgs\.follows = "nixpkgs";\n    \};\n`)
var noctaliaV4InputPattern = regexp.MustCompile(`    noctalia-shell = \{\n      url = "[^"\n]*";\n      inputs\.nixpkgs\.follows = "nixpkgs";\n    \};\n`)

var caelestiaShellInputPattern = regexp.MustCompile(`    caelestia-shell = \{\n      url = "[^"\n]*";\n      inputs = \{\n        caelestia-cli\.follows = "caelestia-cli";\n        nixpkgs\.follows = "nixpkgs";\n        quickshell\.follows = "quickshell";\n      \};\n    \};\n`)
var caelestiaCliInputPattern = regexp.MustCompile(`    caelestia-cli = \{\n      url = "[^"\n]*";\n      inputs\.nixpkgs\.follows = "nixpkgs";\n    \};\n`)

func insertAfterFirst(text, anchor, insert string) string {
	index := strings.Index(text, anchor)
	if index == -1 {
		return text
	}
	index += len(anchor)
	return text[:index] + insert + text[index:]
}

type desiredFlakeInputs struct {
	Personal   bool
	SecureBoot bool
}

func desiredFlakeInputsFromState(s config.State) desiredFlakeInputs {
	return desiredFlakeInputs{
		Personal:   s.Packages.Preset == "personal",
		SecureBoot: s.Features.SecureBoot,
	}
}

func syncOptionalInput(text string, want bool, inputBlock, afterInputAnchor, followsLine, afterFollowsAnchor string) string {
	switch hasInput := strings.Contains(text, inputBlock); {
	case want && !hasInput:
		text = insertAfterFirst(text, afterInputAnchor, inputBlock)
	case !want && hasInput:
		text = strings.Replace(text, inputBlock, "", 1)
	}
	switch hasFollows := strings.Contains(text, followsLine); {
	case want && !hasFollows:
		text = insertAfterFirst(text, afterFollowsAnchor, followsLine)
	case !want && hasFollows:
		text = strings.Replace(text, followsLine, "", 1)
	}
	return text
}

func migrateGeneratedThinFlake(text string, state config.State) (string, bool) {
	updated := text
	updated = strings.Replace(updated, legacyTemplInput, "", 1)
	updated = strings.ReplaceAll(updated, `      inputs.templ.follows = "templ";
`, "")
	updated = strings.Replace(updated, legacyZapretDiscordYoutubeInput, "", 1)
	updated = strings.Replace(updated, legacyZapretDiscordYoutubeFollows, "", 1)

	for _, pattern := range []*regexp.Regexp{
		caelestiaShellInputPattern,
		caelestiaCliInputPattern,
		noctaliaV5InputPattern,
		noctaliaV4InputPattern,
	} {
		updated = pattern.ReplaceAllString(updated, "")
	}
	for _, follows := range []string{
		`      inputs.caelestia-shell.follows = "caelestia-shell";
`,
		`      inputs.caelestia-cli.follows = "caelestia-cli";
`,
		`      inputs.noctalia.follows = "noctalia";
`,
		`      inputs.noctalia-shell.follows = "noctalia-shell";
`,
	} {
		updated = strings.ReplaceAll(updated, follows, "")
	}

	desired := desiredFlakeInputsFromState(state)
	updated = syncOptionalInput(updated, desired.Personal, claudeCodeInput, zenBrowserInputAnchor, claudeCodeFollows, zenBrowserFollowsAnchor)
	updated = syncOptionalInput(updated, desired.Personal, codexInput, claudeCodeInput, codexFollows, claudeCodeFollows)
	updated = syncOptionalInput(updated, desired.SecureBoot, lanzabooteInput, neovimNightlyOverlayInputAnchor, lanzabooteFollows, neovimNightlyOverlayFollowsAnchor)

	desiredWahrweltURL := config.WahrweltFlakeURL(state.Source.Channel)
	for _, flakeURL := range config.KnownWahrweltFlakeURLs() {
		updated = strings.ReplaceAll(updated, flakeURL, desiredWahrweltURL)
	}
	updated = strings.Replace(updated, "    mysetup = {", "    wahrwelt = {", 1)
	updated = strings.Replace(updated, "    mysetup.url =", "    wahrwelt.url =", 1)
	updated = strings.Replace(updated, "outputs = { mysetup, ... }:", "outputs = { wahrwelt, ... }:", 1)
	updated = strings.ReplaceAll(updated, "mysetup.lib.mkMySetupHost", "wahrwelt.lib.mkWahrweltHost")

	return updated, updated != text
}
