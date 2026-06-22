package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func editRegion(s *session) error {
	primaryLayout, additionalLayouts := splitPrimaryKeyboardLayouts(s.state.Locale.KeyboardLayouts)
	err := newForm(
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
			Description("Additional generated locale for regional input/tools.").
			Options(localeOptions(s.state.Locale.ExtraLocale)...).
			Height(12).
			Value(&s.state.Locale.ExtraLocale),
		newBottomFilterSelect().
			Title("Console keymap").
			Description("TTY keyboard map only. Hypr graphical layouts use the XKB field below.").
			Options(consoleKeymapOptions(s.state.Locale.ConsoleKeyMap)...).
			Height(12).
			Value(&s.state.Locale.ConsoleKeyMap),
		newBottomFilterSelect().
			Title("Primary Hypr/XKB keyboard layout").
			Description("First graphical layout from xkeyboard-config. English UK is gb, not console uk.").
			Options(xkbLayoutOptions([]string{primaryLayout})...).
			Height(12).
			Value(&primaryLayout).
			Validate(config.ValidateXKBLayout),
		huh.NewMultiSelect[string]().
			Title("Additional Hypr/XKB keyboard layouts").
			Description("Optional secondary graphical layouts. Use Space to toggle.").
			Options(xkbLayoutOptions(additionalLayouts)...).
			Height(12).
			Value(&additionalLayouts).
			Validate(config.ValidateOptionalXKBLayoutSelection),
		huh.NewSelect[string]().
			Title("Keyboard toggle").
			Description("Hyprland kb_options value for layout switching.").
			Options(labeledStringOptions(config.KeyboardToggles, keyboardToggleLabel)...).
			Value(&s.state.Locale.KeyboardToggle),
		newLiveInput().
			Title("Weather location").
			Description("City name used by shell/weather widgets if enabled.").
			Value(&s.state.Locale.WeatherLocation),
	).Run()
	if err != nil {
		return err
	}
	s.state.Locale.KeyboardLayouts = joinPrimaryKeyboardLayouts(primaryLayout, additionalLayouts)
	return nil
}

func splitPrimaryKeyboardLayouts(value string) (string, []string) {
	layouts := config.SplitKeyboardLayouts(value)
	if len(layouts) == 0 {
		return "us", nil
	}
	return layouts[0], append([]string{}, layouts[1:]...)
}

func joinPrimaryKeyboardLayouts(primary string, additional []string) string {
	layouts := append([]string{primary}, additional...)
	return config.JoinKeyboardLayouts(layouts)
}
