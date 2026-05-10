package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const SchemaVersion = 4

type State struct {
	SchemaVersion int      `json:"schemaVersion"`
	Host          Host     `json:"host"`
	User          User     `json:"user"`
	Locale        Locale   `json:"locale"`
	Git           Git      `json:"git"`
	Packages      Packages `json:"packages"`
	Display       Display  `json:"display"`
	Hardware      Hardware `json:"hardware"`
	Features      Features `json:"features"`
	Zapret        Zapret   `json:"zapret"`
	Services      Services `json:"services"`
	Dots          Dots     `json:"dots"`
}
type Host struct {
	Hostname     string `json:"hostname"`
	StateVersion string `json:"stateVersion"`
}
type User struct {
	Username      string `json:"username"`
	FullName      string `json:"fullName"`
	HomeDirectory string `json:"homeDirectory"`
}
type Locale struct {
	TimeZone        string `json:"timeZone"`
	DefaultLocale   string `json:"defaultLocale"`
	ExtraLocale     string `json:"extraLocale"`
	ConsoleKeyMap   string `json:"consoleKeyMap"`
	WeatherLocation string `json:"weatherLocation"`
	KeyboardLayouts string `json:"keyboardLayouts"`
	KeyboardToggle  string `json:"keyboardToggle"`
}
type Git struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}
type Packages struct {
	Preset string `json:"preset"`
}
type Display struct {
	MonitorName     string `json:"monitorName"`
	MonitorMode     string `json:"monitorMode"`
	MonitorPosition string `json:"monitorPosition"`
	MonitorScale    string `json:"monitorScale"`
}
type Hardware struct {
	GPU string `json:"gpu"`
}
type Features struct {
	SecureBoot    bool `json:"secureBoot"`
	CTFTools      bool `json:"ctfTools"`
	OmniRouter    bool `json:"omniRouter"`
	RussiaMode    bool `json:"russiaMode"`
	Observability bool `json:"observability"`
}
type Zapret struct {
	Enable bool   `json:"enable"`
	Config string `json:"config"`
}
type Services struct {
	PgAdminEmail string `json:"pgAdminEmail"`
}
type Dots struct {
	Hypr             bool `json:"hypr"`
	ZenTheme         bool `json:"zenTheme"`
	Sine             bool `json:"sine"`
	Neovim           bool `json:"neovim"`
	NeovimCleanState bool `json:"neovimCleanState"`
	V2rayN           bool `json:"v2rayN"`
	Wallpapers       bool `json:"wallpapers"`
}
type Secrets struct {
	UserPassword    string
	PgAdminPassword string
}

func Default() State {
	return State{
		SchemaVersion: SchemaVersion,
		Host: Host{
			Hostname:     "NixOS",
			StateVersion: "25.11",
		},
		User: User{
			Username:      currentUser(),
			FullName:      currentUser(),
			HomeDirectory: filepath.Join("/home", currentUser()),
		},
		Locale: Locale{
			TimeZone:        "Europe/Moscow",
			DefaultLocale:   "en_US.UTF-8",
			ExtraLocale:     "ru_RU.UTF-8",
			ConsoleKeyMap:   "us",
			WeatherLocation: "Moscow",
			KeyboardLayouts: "us,ru",
			KeyboardToggle:  "grp:alt_shift_toggle",
		},
		Git: Git{
			Username: currentUser(),
			Email:    "user@example.com",
		},
		Packages: Packages{
			Preset: "personal",
		},
		Display: Display{
			MonitorName:     "eDP-1",
			MonitorMode:     "2560x1600@120",
			MonitorPosition: "0x0",
			MonitorScale:    "1",
		},
		Hardware: Hardware{
			GPU: "amd",
		},
		Features: Features{},
		Zapret: Zapret{
			Config: "general (FAKE_TLS_AUTO_ALT3)",
		},
		Services: Services{
			PgAdminEmail: "admin@localhost.local",
		},
		Dots: Dots{
			Hypr:             true,
			ZenTheme:         true,
			Sine:             true,
			Neovim:           true,
			NeovimCleanState: true,
			V2rayN:           true,
			Wallpapers:       true,
		},
	}
}

func currentUser() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		if user := os.Getenv("USER"); user == "" || user == "root" {
			return sudoUser
		}
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "user"
}

func Load(path string) (State, error) {
	state, err := load(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	return state, err
}

func LoadExisting(path string) (State, error) {
	state, err := load(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, fmt.Errorf("state file does not exist %s: %w", path, err)
	}
	return state, err
}

func load(path string) (State, error) {
	var state State
	data, err := os.ReadFile(path)
	if err != nil {
		return state, fmt.Errorf("read state %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("parse state %s: %w", path, err)
	}
	if err := validateStateSchema(path, state.SchemaVersion); err != nil {
		return State{}, err
	}
	state = Migrate(state)
	return state, nil
}

func validateStateSchema(path string, version int) error {
	if version <= SchemaVersion {
		return nil
	}
	return fmt.Errorf("state %s schemaVersion %d is unsupported; expected %d; rerun TUI or regenerate state", path, version, SchemaVersion)
}

func Save(path string, state State) error {
	state.SchemaVersion = SchemaVersion
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state %s: %w", path, err)
	}
	return nil
}

func Migrate(state State) State {
	def := Default()
	oldVersion := state.SchemaVersion
	legacy := state.SchemaVersion == 0
	state = migrateHost(state, def)
	state = migrateUser(state, def)
	state = migrateLocale(state, def, oldVersion)
	state = migrateIdentity(state, def)
	state = migrateDisplay(state, def)
	return migrateFeatures(state, def, legacy, oldVersion)
}

func migrateHost(state State, def State) State {
	if state.SchemaVersion == 0 {
		state.SchemaVersion = SchemaVersion
	}
	if state.Host.Hostname == "" {
		state.Host.Hostname = def.Host.Hostname
	}
	if state.Host.StateVersion == "" {
		state.Host.StateVersion = def.Host.StateVersion
	}
	return state
}

func migrateUser(state State, def State) State {
	if state.User.Username == "" {
		state.User.Username = def.User.Username
	}
	if state.User.FullName == "" {
		state.User.FullName = state.User.Username
	}
	if state.User.HomeDirectory == "" {
		state.User.HomeDirectory = filepath.Join("/home", state.User.Username)
	}
	return state
}

func migrateLocale(state State, def State, oldVersion int) State {
	if state.Locale.TimeZone == "" {
		state.Locale.TimeZone = def.Locale.TimeZone
	}
	if state.Locale.DefaultLocale == "" {
		state.Locale.DefaultLocale = def.Locale.DefaultLocale
	}
	if state.Locale.ExtraLocale == "" {
		state.Locale.ExtraLocale = def.Locale.ExtraLocale
	}
	if state.Locale.ConsoleKeyMap == "" {
		state.Locale.ConsoleKeyMap = def.Locale.ConsoleKeyMap
	}
	if oldVersion < 2 && state.Locale.ConsoleKeyMap == "ruwin_alt_sh-UTF-8" {
		state.Locale.ConsoleKeyMap = def.Locale.ConsoleKeyMap
	}
	if state.Locale.WeatherLocation == "" {
		state.Locale.WeatherLocation = def.Locale.WeatherLocation
	}
	if state.Locale.KeyboardLayouts == "" {
		state.Locale.KeyboardLayouts = def.Locale.KeyboardLayouts
	}
	if state.Locale.KeyboardToggle == "" {
		state.Locale.KeyboardToggle = def.Locale.KeyboardToggle
	}
	return state
}

func migrateIdentity(state State, def State) State {
	if state.Git.Username == "" {
		state.Git.Username = def.Git.Username
	}
	if state.Git.Email == "" {
		state.Git.Email = def.Git.Email
	}
	if state.Packages.Preset == "" {
		state.Packages.Preset = def.Packages.Preset
	}
	return state
}

func migrateDisplay(state State, def State) State {
	if state.Display.MonitorName == "" {
		state.Display.MonitorName = def.Display.MonitorName
	}
	if state.Display.MonitorMode == "" {
		state.Display.MonitorMode = def.Display.MonitorMode
	}
	if state.Display.MonitorPosition == "" {
		state.Display.MonitorPosition = def.Display.MonitorPosition
	}
	if state.Display.MonitorScale == "" {
		state.Display.MonitorScale = def.Display.MonitorScale
	}
	return state
}

func migrateFeatures(state State, def State, legacy bool, oldVersion int) State {
	if state.Hardware.GPU == "" {
		state.Hardware.GPU = def.Hardware.GPU
	}
	if state.Zapret.Config == "" {
		state.Zapret.Config = def.Zapret.Config
	}
	if state.Services.PgAdminEmail == "" {
		state.Services.PgAdminEmail = def.Services.PgAdminEmail
	}
	if legacy {
		state.Dots = def.Dots
	}
	if oldVersion < 3 {
		state.Dots.Sine = true
		state.Dots.Neovim = true
	}
	if oldVersion < 4 {
		state.Dots.NeovimCleanState = true
	}
	return state
}
