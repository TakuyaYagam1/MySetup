package tui

import (
	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func editDisplay(s *session) error {
	var errors displayFormErrors
	for {
		if err := runDisplayForm(s, errors); err != nil {
			return err
		}
		errors = validateDisplayForm(s.state.Display)
		if errors.empty() {
			return nil
		}
	}
}

func runDisplayForm(s *session, errors displayFormErrors) error {
	return newForm(
		huh.NewSelect[string]().
			Title("GPU").
			Description("Selects driver/session defaults for the system config.").
			Options(labeledStringOptions(config.GPUProfiles, gpuLabel)...).
			Value(&s.state.Hardware.GPU),
		huh.NewInput().
			Title("Hypr keyboard layouts").
			Description("Hyprland kb_layout value, for example us,ru.").
			Value(&s.state.Locale.KeyboardLayouts),
		huh.NewInput().
			Title("Monitor name").
			Description(fieldDescription("Output name from hyprctl monitors, for example eDP-1 or DP-1.", errors.monitorName)).
			Value(&s.state.Display.MonitorName),
		huh.NewInput().
			Title("Resolution@Hz").
			Description(fieldDescription("Only the mode part, for example 2560x1600@120.", errors.monitorMode)).
			Value(&s.state.Display.MonitorMode),
		huh.NewInput().
			Title("Position").
			Description(fieldDescription("Monitor position in Hyprland syntax, for example 0x0.", errors.monitorPosition)).
			Value(&s.state.Display.MonitorPosition),
		huh.NewInput().
			Title("Scale").
			Description(fieldDescription("Monitor scale in Hyprland syntax, for example 1 or 1.25.", errors.monitorScale)).
			Value(&s.state.Display.MonitorScale),
		huh.NewSelect[string]().
			Title("Keyboard toggle").
			Description("Hyprland kb_options value for layout switching.").
			Options(labeledStringOptions(config.KeyboardToggles, keyboardToggleLabel)...).
			Value(&s.state.Locale.KeyboardToggle),
	).Run()
}

type displayFormErrors struct {
	monitorName     string
	monitorMode     string
	monitorPosition string
	monitorScale    string
}

func (e displayFormErrors) empty() bool {
	return e.monitorName == "" &&
		e.monitorMode == "" &&
		e.monitorPosition == "" &&
		e.monitorScale == ""
}

func validateDisplayForm(display config.Display) displayFormErrors {
	var errors displayFormErrors
	if !tuiMonitorNameRe.MatchString(display.MonitorName) {
		errors.monitorName = "monitor name must look like eDP-1, DP-1 or HDMI-A-1."
	}
	if !tuiMonitorModeRe.MatchString(display.MonitorMode) {
		errors.monitorMode = "resolution must look like 2560x1600@120."
	}
	if !tuiMonitorPositionRe.MatchString(display.MonitorPosition) {
		errors.monitorPosition = "position must look like 0x0 or -1920x0."
	}
	if !tuiMonitorScaleRe.MatchString(display.MonitorScale) {
		errors.monitorScale = "scale must be a number like 1 or 1.25."
	}
	return errors
}

func gpuLabel(value string) string {
	switch value {
	case "amd":
		return "AMD"
	case "intel":
		return "Intel"
	case "nvidia":
		return "NVIDIA"
	case "other":
		return "Other / VM"
	default:
		return value
	}
}

func keyboardToggleLabel(value string) string {
	switch value {
	case "grp:alt_shift_toggle":
		return "Alt+Shift"
	case "grp:win_space_toggle":
		return "Win+Space"
	case "grp:ctrl_shift_toggle":
		return "Ctrl+Shift"
	case "grp:caps_toggle":
		return "CapsLock"
	default:
		return value
	}
}
