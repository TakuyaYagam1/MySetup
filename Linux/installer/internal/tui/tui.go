package tui

import (
	"github.com/charmbracelet/huh"
)

func editDots(s *session) error {
	return newForm(
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
			Title("Clean Neovim runtime data").
			Description("Off by default. Enable only when plugins/parsers misbehave: wipes ~/.local/share/nvim, ~/.local/state/nvim and ~/.cache/nvim so Mason/Lazy re-downloads everything. This resets to off after a successful apply.").
			Value(&s.state.Dots.NeovimCleanState),
		huh.NewConfirm().
			Title("Seed v2rayN sing-box").
			Description("Copies sing-box support into v2rayN when the target directory is detected.").
			Value(&s.state.Dots.V2rayN),
	).Run()
}

func labeledStringOptions(values []string, label func(string) string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(values))
	for _, value := range values {
		options = append(options, huh.NewOption(label(value), value))
	}
	return options
}
