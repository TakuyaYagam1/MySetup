package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func editPackages(s *session) error {
	return newForm(
		huh.NewSelect[string]().
			Title("Package preset").
			Description("Controls optional package layers. Personal keeps Takuya's full setup; Minimal/Desktop/Developer trim app groups.").
			Options(labeledStringOptions(config.PackagePresets, packagePresetLabel)...).
			Value(&s.state.Packages.Preset),
		huh.NewConfirm().
			Title("Enable CTF tools").
			Description("Adds heavy CTF layers: reverse, pwn, web, forensics, OSINT and related tools.").
			Value(&s.state.Features.CTFTools),
		huh.NewSelect[bool]().
			Title("Secure Boot (Lanzaboote)").
			Description("Enable only on machines already prepared for Secure Boot. Leave disabled for normal installs.").
			Options(
				huh.NewOption("Disabled (normal installs)", false),
				huh.NewOption("Enabled (prepared Secure Boot only)", true),
			).
			Value(&s.state.Features.SecureBoot),
	).Run()
}

func packagePresetLabel(value string) string {
	switch value {
	case "personal":
		return "Personal (Takuya's config)"
	case "minimal":
		return "Minimal"
	case "desktop":
		return "Desktop"
	case "developer":
		return "Developer"
	default:
		return value
	}
}
