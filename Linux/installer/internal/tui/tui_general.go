package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
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
		huh.NewInput().
			Title("State version").
			Description("NixOS compatibility baseline. Do not bump casually on an existing machine.").
			Value(&s.state.Host.StateVersion),
	).Run()
}

type generalFormErrors struct {
	hostname string
}

func (e generalFormErrors) empty() bool {
	return e.hostname == ""
}

func validateGeneralForm(state config.State) generalFormErrors {
	var errors generalFormErrors
	if config.ValidateDetailed(state).Hostname != "" {
		errors.hostname = "hostname must be a single DNS label, for example NixOS or laptop-01."
	}
	return errors
}
