package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	usernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	hostnameRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	emailRe    = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	localeRe   = regexp.MustCompile(`^[a-z]{2,3}_[A-Z]{2}\.UTF-8$`)
	monitorRe  = regexp.MustCompile(`^[A-Za-z0-9_.-]+,\s*[0-9]+x[0-9]+@[0-9]+(\.[0-9]+)?,\s*-?[0-9]+x-?[0-9]+,\s*[0-9]+(\.[0-9]+)?$`)
)

func Validate(state State) error {
	var errs []string
	errs = append(errs, validateUser(state)...)
	errs = append(errs, validateLocale(state)...)
	errs = append(errs, validateEnums(state)...)
	if err := validateMonitor(state.Display); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateUser(state State) []string {
	var errs []string
	if !usernameRe.MatchString(state.User.Username) {
		errs = append(errs, "username must match ^[a-z_][a-z0-9_-]{0,31}$")
	}
	if !hostnameRe.MatchString(state.Host.Hostname) {
		errs = append(errs, "hostname must be a single RFC1123 label")
	}
	if state.User.FullName == "" {
		errs = append(errs, "full name cannot be empty")
	}
	if err := validateHomeDirectory(state.User.HomeDirectory, state.User.Username); err != nil {
		errs = append(errs, err.Error())
	}
	if !emailRe.MatchString(state.Git.Email) {
		errs = append(errs, "git email is invalid")
	}
	if state.Services.PgAdminEmail != "" && !emailRe.MatchString(state.Services.PgAdminEmail) {
		errs = append(errs, "pgAdmin email is invalid")
	}
	return errs
}

func validateHomeDirectory(homeDirectory, username string) error {
	clean := filepath.Clean(homeDirectory)
	if homeDirectory == "" || !filepath.IsAbs(homeDirectory) || clean != homeDirectory {
		return fmt.Errorf("home directory must be a clean absolute /home/<username> path")
	}
	if !strings.HasPrefix(clean, "/home/") || clean == "/home" {
		return fmt.Errorf("home directory must be a clean absolute /home/<username> path")
	}
	if usernameRe.MatchString(username) && clean != filepath.Join("/home", username) {
		return fmt.Errorf("home directory must match /home/%s", username)
	}
	return nil
}

func validateLocale(state State) []string {
	var errs []string
	if state.Locale.TimeZone == "" || !strings.Contains(state.Locale.TimeZone, "/") {
		errs = append(errs, "timezone must look like Europe/Moscow")
	}
	if !localeRe.MatchString(state.Locale.DefaultLocale) {
		errs = append(errs, "default locale must look like en_US.UTF-8")
	}
	if !localeRe.MatchString(state.Locale.ExtraLocale) {
		errs = append(errs, "extra locale must look like ru_RU.UTF-8")
	}
	return errs
}

func validateEnums(state State) []string {
	var errs []string
	if !oneOf(state.Shell.Profile, "caelestia", "noctalia") {
		errs = append(errs, "shell profile must be caelestia or noctalia")
	}
	if !oneOf(state.Packages.Preset, "minimal", "desktop", "developer", "personal") {
		errs = append(errs, "package preset must be minimal, desktop, developer, or personal")
	}
	if !oneOf(state.Hardware.GPU, "amd", "intel", "nvidia", "other") {
		errs = append(errs, "gpu must be amd, intel, nvidia, or other")
	}
	return errs
}

func validateMonitor(display Display) error {
	monitorLine := fmt.Sprintf("%s, %s, %s, %s", display.MonitorName, display.MonitorMode, display.MonitorPosition, display.MonitorScale)
	return ValidateMonitorLine(monitorLine)
}

func ValidateMonitorLine(line string) error {
	if monitorRe.MatchString(line) {
		return nil
	}
	return fmt.Errorf("monitor line must look like eDP-1, 2560x1600@120, 0x0, 1")
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
