package tui

import (
	"github.com/charmbracelet/huh"
)

func editRegion(s *session) error {
	return newForm(
		newBottomFilterSelect().
			Title("Timezone").
			Description("Nix time.timeZone value. Press / to search like archinstall, Esc clears search.").
			Options(timeZoneOptions(s.state.Locale.TimeZone)...).
			Height(14).
			Value(&s.state.Locale.TimeZone).
			OnChange(func(timeZone string) {
				s.state.Locale.WeatherLocation = weatherLocationFromTimeZone(timeZone)
			}),
		newBottomFilterSelect().
			Title("Default locale").
			Description("Primary glibc locale. Keep the .UTF-8 suffix.").
			Options(localeOptions(s.state.Locale.DefaultLocale)...).
			Height(12).
			Value(&s.state.Locale.DefaultLocale),
		newBottomFilterSelect().
			Title("Extra locale").
			Description("Additional generated locale, usually ru_RU.UTF-8 for Russian input/tools.").
			Options(localeOptions(s.state.Locale.ExtraLocale)...).
			Height(12).
			Value(&s.state.Locale.ExtraLocale),
		newBottomFilterSelect().
			Title("Console keymap").
			Description("TTY keyboard map only. Default is us; Hypr graphical layouts are configured in Display.").
			Options(consoleKeymapOptions(s.state.Locale.ConsoleKeyMap)...).
			Height(12).
			Value(&s.state.Locale.ConsoleKeyMap),
		newLiveInput().
			Title("Weather location").
			Description("City name used by shell/weather widgets if enabled.").
			Value(&s.state.Locale.WeatherLocation),
		huh.NewConfirm().
			Title("Russia mode").
			Description("Enables region-specific defaults in this config. VPN/proxy services are separate.").
			Value(&s.state.Features.RussiaMode),
	).Run()
}
