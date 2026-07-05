package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func editDots(s *session) error {
	var errors dotsFormErrors
	for {
		if err := runDotsForm(s, errors); err != nil {
			return err
		}
		errors = validateDotsForm(s.state)
		if errors.empty() {
			return nil
		}
	}
}

func runDotsForm(s *session, errors dotsFormErrors) error {
	return newForm(
		huh.NewSelect[string]().
			Title("Noctalia version").
			Description(fieldDescription("Pick the current v5 integration or keep the legacy v4 JSON-seeded profile.", errors.noctalia)).
			Options(noctaliaVersionOptions()...).
			Value(&s.state.Noctalia.Version),
		huh.NewConfirm().
			Title("Sync Hypr dots").
			Description("Mirrors Linux/dots/hypr into ~/.config/hypr, chmods scripts and reloads Hypr if running.").
			Value(&s.state.Dots.Hypr),
		huh.NewConfirm().
			Title("Copy wallpapers").
			Description("Copies Linux/NixOS/Wallpapers into ~/Pictures/Wallpapers and removes preview-* files.").
			Value(&s.state.Dots.Wallpapers),
		huh.NewConfirm().
			Title("Install Zen Catppuccin chrome").
			Description("Finds a Zen Browser profile and installs the Catppuccin chrome theme.").
			Value(&s.state.Dots.ZenTheme),
		huh.NewConfirm().
			Title("Install Sine profile files").
			Description("Best-effort Zen Sine files install; pinned URLs are checked and skipped with a warning if missing.").
			Value(&s.state.Dots.Sine),
		huh.NewConfirm().
			Title("Sync Neovim config").
			Description("Backs up/syncs the repo Neovim config into ~/.config/nvim when enabled.").
			Value(&s.state.Dots.Neovim),
		huh.NewConfirm().
			Title("Seed v2rayN sing-box").
			Description("Copies sing-box support into v2rayN when the target directory is detected.").
			Value(&s.state.Dots.V2rayN),
	).Run()
}

type dotsFormErrors struct {
	noctalia string
}

func (e dotsFormErrors) empty() bool {
	return e.noctalia == ""
}

func validateDotsForm(state config.State) dotsFormErrors {
	var errors dotsFormErrors
	fieldErrors := config.ValidateDetailed(state)
	if fieldErrors.Noctalia != "" {
		errors.noctalia = "choose v5 for the current upstream shell, or v4 for the preserved legacy config."
	}
	return errors
}

func noctaliaVersionOptions() []huh.Option[string] {
	return labeledStringOptions(config.NoctaliaVersions, noctaliaVersionLabel)
}

func noctaliaVersionLabel(version string) string {
	switch version {
	case config.NoctaliaVersionV4:
		return "v4 (legacy seeded config)"
	case config.NoctaliaVersionV5:
		return "v5 (current shell integration)"
	default:
		return version
	}
}

func labeledStringOptions(values []string, label func(string) string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(values))
	for _, value := range values {
		options = append(options, huh.NewOption(label(value), value))
	}
	return options
}
