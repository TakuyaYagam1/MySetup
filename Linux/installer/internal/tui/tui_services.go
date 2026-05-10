package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func editServices(s *session) error {
	wasZapretEnabled := s.state.Zapret.Enable
	if err := newForm(serviceFields(s)...).Run(); err != nil {
		return err
	}
	if !wasZapretEnabled && s.state.Zapret.Enable {
		return newForm(zapretConfigField(&s.state.Zapret.Config)).Run()
	}
	return nil
}

func serviceFields(s *session) []huh.Field {
	fields := []huh.Field{
		huh.NewConfirm().
			Title("Enable OmniRouter").
			Description("Builds/enables the local OmniRouter package and module used by this config.").
			Value(&s.state.Features.OmniRouter),
		huh.NewConfirm().
			Title("Enable Observability").
			Description("Enables the localhost Grafana/Prometheus/Loki stack. Off by default.").
			Value(&s.state.Features.Observability),
		huh.NewConfirm().
			Title("Enable Zapret").
			Description("Enables the zapret service/config for Discord/YouTube bypass presets.").
			Value(&s.state.Zapret.Enable),
	}
	if shouldShowZapretConfig(s.state) {
		fields = append(fields, zapretConfigField(&s.state.Zapret.Config))
	}
	return fields
}

func shouldShowZapretConfig(state config.State) bool {
	return state.Zapret.Enable
}

func zapretConfigField(value *string) huh.Field {
	return huh.NewSelect[string]().
		Title("Zapret config").
		Description("Choose one of the current top-level upstream configs from the pinned flake.").
		Options(zapretConfigOptions(*value)...).
		Value(value)
}

func zapretConfigOptions(current string) []huh.Option[string] {
	configs := zapretConfigNames()
	if current != "" && !containsString(configs, current) {
		configs = append([]string{current}, configs...)
	}
	return stringOptions(configs)
}

func zapretConfigNames() []string {
	return append([]string(nil), config.ZapretConfigs...)
}
