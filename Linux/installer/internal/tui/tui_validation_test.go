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

func TestValidateUserFormReportsFieldErrors(t *testing.T) {
	state := config.Default()
	state.User.Username = "Bad User"
	state.User.FullName = ""
	state.User.HomeDirectory = "/tmp/takuya"
	state.Git.Username = ""
	state.Git.Email = "not-email"
	state.Services.PgAdminEmail = "also-not-email"

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
		"pgAdminEmail":  got.pgAdminEmail,
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
