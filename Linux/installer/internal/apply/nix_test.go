package apply

import (
	"os"
	"regexp"
	"testing"
)

func TestNixCachePolicyMatchesMakefile(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for name, want := range map[string]string{
		"NIX_CACHE_SUB" + "STITUTERS": nixCacheSubstituters,
		"NIX_CACHE_KEYS":              nixCacheTrustedKeys,
	} {
		got := makeVariable(t, text, name)
		if got != want {
			t.Fatalf("%s drifted from Go nix cache policy\nMakefile: %q\nGo:       %q", name, got, want)
		}
	}
}

func makeVariable(t *testing.T, text, name string) string {
	t.Helper()

	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + ` := (.+)$`)
	match := re.FindStringSubmatch(text)
	if match == nil {
		t.Fatalf("Makefile variable %s missing\n%s", name, text)
	}
	return match[1]
}
