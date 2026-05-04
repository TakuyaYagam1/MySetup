package config

import "testing"

func TestValidateDefault(t *testing.T) {
	if err := Validate(Default()); err != nil {
		t.Fatalf("default state should validate: %v", err)
	}
}

func TestDefaultPrefersSudoUserOverRoot(t *testing.T) {
	t.Setenv("USER", "root")
	t.Setenv("SUDO_USER", "takuya")

	state := Default()
	if state.User.Username != "takuya" {
		t.Fatalf("expected sudo user as default username, got %q", state.User.Username)
	}
	if state.User.HomeDirectory != "/home/takuya" {
		t.Fatalf("expected sudo user home directory, got %q", state.User.HomeDirectory)
	}
}

func TestValidateRejectsBadUsername(t *testing.T) {
	state := Default()
	state.User.Username = "Bad User"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid username error")
	}
}

func TestValidateRejectsBadShell(t *testing.T) {
	state := Default()
	state.Shell.Profile = "zenities"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid shell error")
	}
}

func TestValidateRejectsBadPackagePreset(t *testing.T) {
	state := Default()
	state.Packages.Preset = "everything"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid package preset error")
	}
}

func TestValidateMonitorLine(t *testing.T) {
	if err := ValidateMonitorLine("eDP-1, 2560x1600@120, 0x0, 1"); err != nil {
		t.Fatalf("expected monitor line to validate: %v", err)
	}
	if err := ValidateMonitorLine("eDP-1, lol, 0x0, 1"); err == nil {
		t.Fatal("expected invalid monitor line")
	}
}
