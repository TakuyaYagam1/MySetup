package tui

import (
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func TestValidateGeneralFormReportsHostnameError(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "-bad"

	got := validateGeneralForm(state)
	if got.hostname == "" {
		t.Fatal("expected hostname validation error")
	}
}

func TestValidateGeneralFormReportsSourceChannelError(t *testing.T) {
	state := config.Default()
	state.Source.Channel = "nightly"

	got := validateGeneralForm(state)
	if got.sourceChannel == "" {
		t.Fatal("expected source channel validation error")
	}
}

func TestValidateUserFormReportsFieldErrors(t *testing.T) {
	state := config.Default()
	state.User.Username = "Bad User"
	state.User.FullName = ""
	state.User.HomeDirectory = "/tmp/takuya"
	state.Git.Username = ""
	state.Git.Email = "not-email"

	got := validateUserForm(state)
	if got.empty() {
		t.Fatal("expected user form validation errors")
	}
	for name, value := range map[string]string{
		"username":      got.username,
		"fullName":      got.fullName,
		"homeDirectory": got.homeDirectory,
		"gitUsername":   got.gitUsername,
		"gitEmail":      got.gitEmail,
	} {
		if value == "" {
			t.Fatalf("expected %s validation error, got %#v", name, got)
		}
	}
}

func TestValidateDisplayFormReportsFieldErrors(t *testing.T) {
	got := validateDisplayForm(config.Display{
		MonitorName:     "bad monitor",
		MonitorMode:     "2560x1600",
		MonitorPosition: "zero",
		MonitorScale:    "big",
	})

	if got.empty() {
		t.Fatal("expected display form validation errors")
	}
	for name, value := range map[string]string{
		"monitorName":     got.monitorName,
		"monitorMode":     got.monitorMode,
		"monitorPosition": got.monitorPosition,
		"monitorScale":    got.monitorScale,
	} {
		if value == "" {
			t.Fatalf("expected %s validation error, got %#v", name, got)
		}
	}
}

func TestValidateMultiMonitorStateClean(t *testing.T) {
	state := config.Default()
	state.Display.ExtraMonitors = []config.Monitor{
		{Name: "HDMI-A-1", Mode: "preferred", Position: "auto", Scale: "1"},
		{Name: "DP-2", Mode: "1920x1080@144", Position: "2560x0", Scale: "1.25"},
	}
	if err := config.Validate(state); err != nil {
		t.Fatalf("multi-monitor state with valid extras must validate: %v", err)
	}
}

func TestValidateMultiMonitorStateSurfacesIndexedError(t *testing.T) {
	state := config.Default()
	state.Display.ExtraMonitors = []config.Monitor{
		{Name: "HDMI-A-1", Mode: "preferred", Position: "auto", Scale: "1"},
		{Name: "DP-2", Mode: "not-a-mode", Position: "auto", Scale: "1"},
	}
	errs := config.ValidateDetailed(state)
	if errs.ExtraMonitors == nil || len(errs.ExtraMonitors) != 2 {
		t.Fatalf("expected 2 extra-monitor slots, got %#v", errs.ExtraMonitors)
	}
	if errs.ExtraMonitors[0] != "" {
		t.Fatalf("first extra must be clean, got %q", errs.ExtraMonitors[0])
	}
	if errs.ExtraMonitors[1] == "" {
		t.Fatal("second extra must surface mode error")
	}
}

func TestMonitorRowErrorsAcceptsPreferredSentinel(t *testing.T) {
	row := config.Monitor{Name: "HDMI-A-1", Mode: "preferred", Position: "auto", Scale: "1"}
	errs := monitorRowErrors(row)
	if !errs.empty() {
		t.Fatalf("preferred sentinel must validate cleanly, got %#v", errs)
	}
}
