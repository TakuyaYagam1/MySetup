package tui

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/defaults"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type monitorListAction string

const (
	monitorActionDone    monitorListAction = "done"
	monitorActionPrimary monitorListAction = "primary"
	monitorActionAdd     monitorListAction = "add"
	monitorActionDetect  monitorListAction = "detect"
	monitorActionEditFmt                   = "edit:%d"
	monitorActionRmFmt                     = "remove:%d"
)

func editDisplay(ctx context.Context, s *session) error {
	if err := editPrimaryDisplay(s); err != nil {
		return err
	}
	return editMonitorList(ctx, s)
}

func editPrimaryDisplay(s *session) error {
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

func editMonitorList(ctx context.Context, s *session) error {
	for {
		action, err := chooseMonitorAction(s.state.Display)
		if err != nil {
			return err
		}
		switch action {
		case monitorActionDone:
			return nil
		case monitorActionPrimary:
			if err := editPrimaryDisplay(s); err != nil {
				return err
			}
		case monitorActionAdd:
			if err := addExtraMonitor(s); err != nil {
				return err
			}
		case monitorActionDetect:
			if err := detectMonitors(ctx, s); err != nil {
				return err
			}
		default:
			if err := dispatchExtraMonitorAction(s, action); err != nil {
				return err
			}
		}
	}
}

func dispatchExtraMonitorAction(s *session, action monitorListAction) error {
	var index int
	if _, err := fmt.Sscanf(string(action), monitorActionEditFmt, &index); err == nil {
		return editExtraMonitor(s, index)
	}
	if _, err := fmt.Sscanf(string(action), monitorActionRmFmt, &index); err == nil {
		removeExtraMonitor(s, index)
		return nil
	}
	return fmt.Errorf("unknown monitor action %q", action)
}

func chooseMonitorAction(display config.Display) (monitorListAction, error) {
	options := make([]huh.Option[string], 0, 4+2*len(display.ExtraMonitors))
	options = append(options,
		huh.NewOption("Done", string(monitorActionDone)),
		huh.NewOption(
			fmt.Sprintf("Edit primary: %s (%s)", display.MonitorName, display.MonitorMode),
			string(monitorActionPrimary),
		),
		huh.NewOption("Add monitor", string(monitorActionAdd)),
		huh.NewOption("Detect from hyprctl", string(monitorActionDetect)),
	)
	for i, monitor := range display.ExtraMonitors {
		options = append(options,
			huh.NewOption(
				fmt.Sprintf("Edit #%d: %s (%s)", i+1, monitor.Name, monitor.Mode),
				fmt.Sprintf(monitorActionEditFmt, i),
			),
			huh.NewOption(
				fmt.Sprintf("Remove #%d: %s (%s)", i+1, monitor.Name, monitor.Mode),
				fmt.Sprintf(monitorActionRmFmt, i),
			),
		)
	}

	var selected string
	err := newForm(
		huh.NewSelect[string]().
			Title("Monitors").
			Description("Manage additional outputs; primary is edited via the first option.").
			Options(options...).
			Value(&selected),
	).Run()
	if err != nil {
		return monitorActionDone, err
	}
	return monitorListAction(selected), nil
}

func addExtraMonitor(s *session) error {
	monitor := config.Monitor{
		Name:     "",
		Mode:     "preferred",
		Position: "auto",
		Scale:    "1",
	}
	if err := runMonitorRowForm(&monitor, "Add monitor", displayFormErrors{}); err != nil {
		return err
	}
	if err := validateNewMonitor(&monitor); err != nil {
		return err
	}
	s.state.Display.ExtraMonitors = append(s.state.Display.ExtraMonitors, monitor)
	return nil
}

func editExtraMonitor(s *session, index int) error {
	if index < 0 || index >= len(s.state.Display.ExtraMonitors) {
		return fmt.Errorf("extra monitor index %d out of range", index)
	}
	monitor := s.state.Display.ExtraMonitors[index]
	if err := runMonitorRowForm(&monitor, fmt.Sprintf("Edit monitor #%d", index+1), displayFormErrors{}); err != nil {
		return err
	}
	if err := validateNewMonitor(&monitor); err != nil {
		return err
	}
	s.state.Display.ExtraMonitors[index] = monitor
	return nil
}

func removeExtraMonitor(s *session, index int) {
	if index < 0 || index >= len(s.state.Display.ExtraMonitors) {
		return
	}
	s.state.Display.ExtraMonitors = append(
		s.state.Display.ExtraMonitors[:index],
		s.state.Display.ExtraMonitors[index+1:]...,
	)
}

func validateNewMonitor(monitor *config.Monitor) error {
	for {
		errs := monitorRowErrors(*monitor)
		if errs.empty() {
			return nil
		}
		if err := runMonitorRowForm(monitor, "Fix monitor entry", errs); err != nil {
			return err
		}
	}
}

func monitorRowErrors(monitor config.Monitor) displayFormErrors {
	var errs displayFormErrors
	if !tuiMonitorNameRe.MatchString(monitor.Name) {
		errs.monitorName = "monitor name must look like eDP-1, DP-1 or HDMI-A-1."
	}
	if !tuiMonitorModeRe.MatchString(monitor.Mode) {
		errs.monitorMode = "mode must look like 2560x1600@120, or use 'preferred'."
	}
	if !tuiMonitorPositionRe.MatchString(monitor.Position) {
		errs.monitorPosition = "position must look like 0x0, -1920x0, or 'auto'."
	}
	if !tuiMonitorScaleRe.MatchString(monitor.Scale) {
		errs.monitorScale = "scale must be a number like 1 or 1.25."
	}
	return errs
}

func runMonitorRowForm(monitor *config.Monitor, title string, errors displayFormErrors) error {
	return newForm(
		huh.NewNote().Title(title),
		huh.NewInput().
			Title("Monitor name").
			Description(fieldDescription("Output name from hyprctl monitors, for example HDMI-A-1 or DP-2.", errors.monitorName)).
			Value(&monitor.Name),
		huh.NewInput().
			Title("Resolution@Hz").
			Description(fieldDescription("Mode part like 1920x1080@144, or 'preferred' to auto-pick highest.", errors.monitorMode)).
			Value(&monitor.Mode),
		huh.NewInput().
			Title("Position").
			Description(fieldDescription("Position in Hyprland syntax, for example 2560x0 or 'auto'.", errors.monitorPosition)).
			Value(&monitor.Position),
		huh.NewInput().
			Title("Scale").
			Description(fieldDescription("Scale in Hyprland syntax, for example 1 or 1.25.", errors.monitorScale)).
			Value(&monitor.Scale),
	).Run()
}

func detectMonitors(ctx context.Context, s *session) error {
	runner := quietRunner()
	monitors, err := defaults.DetectMonitors(ctx, runner)
	if err != nil {
		return err
	}
	if len(monitors) == 0 {
		return newForm(
			huh.NewNote().
				Title("Detect from hyprctl").
				Description("No Hyprland session reachable. Either hyprctl isn't on PATH or no outputs are active."),
		).Run()
	}

	preview := buildDetectedMonitorsPreview(monitors)
	var confirm bool
	err = newForm(
		huh.NewConfirm().
			Title("Replace monitors with detected layout?").
			Description(preview).
			Affirmative("Yes, overwrite").
			Negative("Cancel").
			Value(&confirm),
	).Run()
	if err != nil || !confirm {
		return err
	}

	primary := monitors[0]
	s.state.Display.MonitorName = primary.Name
	s.state.Display.MonitorMode = primary.Mode
	s.state.Display.MonitorPosition = primary.Position
	s.state.Display.MonitorScale = primary.Scale
	if len(monitors) > 1 {
		s.state.Display.ExtraMonitors = append([]config.Monitor(nil), monitors[1:]...)
	} else {
		s.state.Display.ExtraMonitors = nil
	}
	return nil
}

func buildDetectedMonitorsPreview(monitors []config.Monitor) string {
	var b builder
	b.line("Detected (primary first):")
	for i, monitor := range monitors {
		role := "extra"
		if i == 0 {
			role = "primary"
		}
		b.line(fmt.Sprintf("  %s: %s @ %s, scale %s (%s)", role, monitor.Name, monitor.Position, monitor.Scale, monitor.Mode))
	}
	return b.s
}

func quietRunner() run.Runner {
	return run.Runner{Stdout: io.Discard, Stderr: io.Discard}
}

func runDisplayForm(s *session, errors displayFormErrors) error {
	return newForm(
		huh.NewSelect[string]().
			Title("GPU").
			Description("Selects driver/session defaults for the system config.").
			Options(labeledStringOptions(config.GPUProfiles, gpuLabel)...).
			Value(&s.state.Hardware.GPU),
		huh.NewInput().
			Title("Monitor name").
			Description(fieldDescription("Output name from hyprctl monitors, for example eDP-1 or DP-1.", errors.monitorName)).
			Value(&s.state.Display.MonitorName),
		huh.NewInput().
			Title("Resolution@Hz").
			Description(fieldDescription("Mode part like 2560x1600@120, or 'preferred' to auto-pick the panel's highest mode.", errors.monitorMode)).
			Value(&s.state.Display.MonitorMode),
		huh.NewInput().
			Title("Position").
			Description(fieldDescription("Monitor position in Hyprland syntax, for example 0x0 or 'auto'.", errors.monitorPosition)).
			Value(&s.state.Display.MonitorPosition),
		huh.NewInput().
			Title("Scale").
			Description(fieldDescription("Monitor scale in Hyprland syntax, for example 1 or 1.25.", errors.monitorScale)).
			Value(&s.state.Display.MonitorScale),
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
		errors.monitorMode = "mode must look like 2560x1600@120, or use 'preferred'."
	}
	if !tuiMonitorPositionRe.MatchString(display.MonitorPosition) {
		errors.monitorPosition = "position must look like 0x0, -1920x0, or 'auto'."
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
