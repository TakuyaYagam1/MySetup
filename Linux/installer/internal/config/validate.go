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
	versionRe  = regexp.MustCompile(`^[0-9]{2}\.[0-9]{2}$`)
	keymapRe   = regexp.MustCompile(`^[A-Za-z0-9_.+-]+$`)
	layoutsRe  = regexp.MustCompile(`^[A-Za-z0-9_-]+(,[A-Za-z0-9_-]+)*$`)
)

type FieldErrors struct {
	Username      string
	Hostname      string
	StateVersion  string
	FullName      string
	HomeDirectory string
	GitUsername   string
	GitEmail      string
	PgAdminEmail  string
	TimeZone      string
	DefaultLocale string
	ExtraLocale   string
	ConsoleKeyMap string
	Keyboard      string
	PackagePreset string
	GPU           string
	ZapretConfig  string
	Monitor       string
}

func (e FieldErrors) Messages() []string {
	var messages []string
	for _, message := range []string{
		e.Username,
		e.Hostname,
		e.StateVersion,
		e.FullName,
		e.HomeDirectory,
		e.GitUsername,
		e.GitEmail,
		e.PgAdminEmail,
		e.TimeZone,
		e.DefaultLocale,
		e.ExtraLocale,
		e.ConsoleKeyMap,
		e.Keyboard,
		e.PackagePreset,
		e.GPU,
		e.ZapretConfig,
		e.Monitor,
	} {
		if message != "" {
			messages = append(messages, message)
		}
	}
	return messages
}

func (e FieldErrors) Empty() bool {
	return len(e.Messages()) == 0
}

func Validate(state State) error {
	errs := ValidateDetailed(state).Messages()
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func ValidateDetailed(state State) FieldErrors {
	var errs FieldErrors
	validateUserFields(state, &errs)
	validateLocaleFields(state, &errs)
	validateEnumFields(state, &errs)
	if err := validateMonitor(state.Display); err != nil {
		errs.Monitor = err.Error()
	}
	return errs
}

func validateUserFields(state State, errs *FieldErrors) {
	if !usernameRe.MatchString(state.User.Username) {
		errs.Username = "username must match ^[a-z_][a-z0-9_-]{0,31}$"
	}
	if !hostnameRe.MatchString(state.Host.Hostname) {
		errs.Hostname = "hostname must be a single RFC1123 label"
	}
	if !versionRe.MatchString(state.Host.StateVersion) {
		errs.StateVersion = "state version must look like 25.11"
	}
	if state.User.FullName == "" {
		errs.FullName = "full name cannot be empty"
	}
	if err := validateHomeDirectory(state.User.HomeDirectory, state.User.Username); err != nil {
		errs.HomeDirectory = err.Error()
	}
	if strings.TrimSpace(state.Git.Username) == "" {
		errs.GitUsername = "git user.name cannot be empty"
	}
	if !emailRe.MatchString(state.Git.Email) {
		errs.GitEmail = "git email is invalid"
	}
	if state.Services.PgAdminEmail != "" && !emailRe.MatchString(state.Services.PgAdminEmail) {
		errs.PgAdminEmail = "pgAdmin email is invalid"
	}
}

func validateLocaleFields(state State, errs *FieldErrors) {
	if state.Locale.TimeZone == "" || !strings.Contains(state.Locale.TimeZone, "/") {
		errs.TimeZone = "timezone must look like Europe/Moscow"
	}
	if !localeRe.MatchString(state.Locale.DefaultLocale) {
		errs.DefaultLocale = "default locale must look like en_US.UTF-8"
	}
	if !localeRe.MatchString(state.Locale.ExtraLocale) {
		errs.ExtraLocale = "extra locale must look like ru_RU.UTF-8"
	}
	if !keymapRe.MatchString(state.Locale.ConsoleKeyMap) {
		errs.ConsoleKeyMap = "console keymap must be a valid keymap name such as us"
	}
	if !layoutsRe.MatchString(state.Locale.KeyboardLayouts) {
		errs.Keyboard = "keyboard layouts must be comma-separated XKB layout names such as us,ru"
	}
	if !IsKeyboardToggle(state.Locale.KeyboardToggle) {
		errs.Keyboard = "keyboard toggle must be one of the supported XKB toggle options"
	}
}

func validateEnumFields(state State, errs *FieldErrors) {
	if !IsPackagePreset(state.Packages.Preset) {
		errs.PackagePreset = "package preset must be minimal, desktop, developer, or personal"
	}
	if !IsGPUProfile(state.Hardware.GPU) {
		errs.GPU = "gpu must be amd, intel, nvidia, or other"
	}
	if !IsZapretConfig(state.Zapret.Config) {
		errs.ZapretConfig = "zapret config must be one of the supported upstream presets"
	}
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
