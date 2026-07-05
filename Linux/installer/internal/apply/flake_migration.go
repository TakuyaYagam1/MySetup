package apply

import "strings"

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

	legacyTemplInput = `    templ = {
      url = "github:a-h/templ";
      inputs = {
        nixpkgs.follows = "nixpkgs-stable";
        nixpkgs-unstable.follows = "nixpkgs";
      };
    };

`
)

func migrateGeneratedThinFlake(text string) (string, bool) {
	updated := text
	updated = strings.Replace(updated, legacyNoctaliaInput, currentNoctaliaInput, 1)
	updated = strings.Replace(updated, legacyTemplInput, "", 1)
	updated = strings.ReplaceAll(updated, `      inputs.noctalia-shell.follows = "noctalia-shell";
`, `      inputs.noctalia.follows = "noctalia";
`)
	updated = strings.ReplaceAll(updated, `      inputs.templ.follows = "templ";
`, "")
	return updated, updated != text
}
