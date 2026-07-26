package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

func editGeneral(s *session) error {
	var errors generalFormErrors
	for {
		if err := runGeneralForm(s, errors); err != nil {
			return err
		}
		errors = validateGeneralForm(s.state)
		if errors.empty() {
			return nil
		}
	}
}

func runGeneralForm(s *session, errors generalFormErrors) error {
	return newForm(
		huh.NewInput().
			Title("Hostname").
			Description(fieldDescription("NixOS host name and flake target, for example NixOS.", errors.hostname)).
			Value(&s.state.Host.Hostname),
		huh.NewSelect[string]().
			Title("Wahrwelt channel").
			Description(fieldDescription("Which Wahrwelt branch the installed /etc/nixos wrapper follows.", errors.sourceChannel)).
			Options(sourceChannelOptions()...).
			Value(&s.state.Source.Channel),
		huh.NewInput().
			Title("State version").
			Description("NixOS compatibility baseline. Do not bump casually on an existing machine.").
			Value(&s.state.Host.StateVersion),
	).Run()
}

type generalFormErrors struct {
	hostname      string
	sourceChannel string
}

func (e generalFormErrors) empty() bool {
	return e.hostname == "" && e.sourceChannel == ""
}

func validateGeneralForm(state config.State) generalFormErrors {
	var errors generalFormErrors
	fieldErrors := config.ValidateDetailed(state)
	if fieldErrors.Hostname != "" {
		errors.hostname = "hostname must be a single DNS label, for example NixOS or laptop-01."
	}
	if fieldErrors.SourceChannel != "" {
		errors.sourceChannel = "choose stable (main) or development (dev)."
	}
	return errors
}

func sourceChannelOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption(sourceChannelLabel(config.SourceChannelStable), config.SourceChannelStable),
		huh.NewOption(sourceChannelLabel(config.SourceChannelDevelopment), config.SourceChannelDevelopment),
	}
}

func sourceChannelLabel(channel string) string {
	switch channel {
	case config.SourceChannelDevelopment:
		return "development (dev branch)"
	case config.SourceChannelStable:
		return "stable (main branch)"
	default:
		return channel
	}
}
