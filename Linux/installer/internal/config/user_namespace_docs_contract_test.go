package config

import (
	"strings"
	"testing"
)

func TestPublicDocsUseCanonicalUserNamespaces(t *testing.T) {
	documents := map[string]string{
		"root README":  readContractFile(t, "../../../../README.md"),
		"Linux README": readContractFile(t, "../../../README.md"),
		"NixOS README": readContractFile(t, "../../../NixOS/README.md"),
		"keybinds":     readContractFile(t, "../../../keybinds.md"),
		"gitignore":    readContractFile(t, "../../../../.gitignore"),
	}

	for name, required := range map[string][]string{
		"root README": {
			"/etc/nixos/user/",
			"/etc/nixos/installer-state.json",
			"~/.config/hypr/user/",
			"The internal XDG runtime namespace remains\n`wahrwelt`",
		},
		"Linux README": {
			"/etc/nixos/user/",
			"/etc/nixos/installer-state.json",
			"~/.config/hypr/user/",
			"Lua namespace remains\n`wahrwelt.*`",
			"/etc/.nixos.migration.<suffix>",
		},
		"NixOS README": {
			"/etc/nixos/user/",
			"/etc/nixos/installer-state.json",
			"~/.config/hypr/user/",
			"internal `wahrwelt.*` namespace",
			"pinned same-filesystem atomic",
		},
		"keybinds":  {"~/.config/hypr/user/default.lua"},
		"gitignore": {"/etc/nixos/user"},
	} {
		for _, value := range required {
			if !strings.Contains(documents[name], value) {
				t.Fatalf("%s must document canonical path %q", name, value)
			}
		}
	}

	legacyPaths := map[string]map[string]bool{
		"/etc/nixos/private/":            {"Linux README": true, "NixOS README": true},
		"/etc/nixos/wahrwelt/state.json": {"Linux README": true, "NixOS README": true},
		"/etc/nixos/mysetup/state.json":  {"Linux README": true, "NixOS README": true},
		"~/.config/hypr/wahrwelt/":       {"Linux README": true, "NixOS README": true},
		"~/.config/hypr/mysetup/":        {"Linux README": true, "NixOS README": true},
	}
	for legacyPath, allowedDocuments := range legacyPaths {
		for name, document := range documents {
			for offset := 0; ; {
				index := strings.Index(document[offset:], legacyPath)
				if index < 0 {
					break
				}
				index += offset
				if !allowedDocuments[name] {
					t.Fatalf("%s contains stale user-facing path %q", name, legacyPath)
				}
				paragraph := strings.ToLower(namespaceParagraphAt(document, index))
				if !strings.Contains(paragraph, "migrat") && !strings.Contains(paragraph, "legacy") {
					t.Fatalf("%s uses legacy path %q outside migration prose", name, legacyPath)
				}
				offset = index + len(legacyPath)
			}
		}
	}

	if strings.Contains(documents["Linux README"], "MySetup-era backups are removed") {
		t.Fatal("Linux README must not claim that unowned recovery backups are deleted")
	}
}

func TestHomeManagerEvalForcesActivationDAG(t *testing.T) {
	makefile := readContractFile(t, "../../Makefile")
	if !strings.Contains(makefile, `home-manager.lib.hm.dag.topoSort hm.config.home.activation`) {
		t.Fatal("nix-hm-eval must force the full activation DAG and reject dependency cycles")
	}
}

func namespaceParagraphAt(text string, index int) string {
	start := strings.LastIndex(text[:index], "\n\n")
	if start < 0 {
		start = 0
	} else {
		start += 2
	}
	endOffset := strings.Index(text[index:], "\n\n")
	if endOffset < 0 {
		return text[start:]
	}
	return text[start : index+endOffset]
}
