package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	SchemaVersion          = 7
	MaxGeneratedStateBytes = 1024 * 1024
)

type State struct {
	SchemaVersion int      `json:"schemaVersion"`
	Host          Host     `json:"host"`
	Source        Source   `json:"source"`
	User          User     `json:"user"`
	Locale        Locale   `json:"locale"`
	Git           Git      `json:"git"`
	Packages      Packages `json:"packages"`
	Display       Display  `json:"display"`
	Hardware      Hardware `json:"hardware"`
	Features      Features `json:"features"`
	Dots          Dots     `json:"dots"`
	Noctalia      Noctalia `json:"noctalia"`
}
type Host struct {
	Hostname     string `json:"hostname"`
	StateVersion string `json:"stateVersion"`
}
type Source struct {
	Channel string `json:"channel"`
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
	MonitorName     string    `json:"monitorName"`
	MonitorMode     string    `json:"monitorMode"`
	MonitorPosition string    `json:"monitorPosition"`
	MonitorScale    string    `json:"monitorScale"`
	ExtraMonitors   []Monitor `json:"extraMonitors,omitempty"`
}

type Monitor struct {
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	Position string `json:"position"`
	Scale    string `json:"scale"`
}

func (m Monitor) MonitorLine() string {
	mode := strings.TrimSpace(m.Mode)
	switch mode {
	case "preferred", "auto", "highres", "highrr", "":
		if mode == "" {
			mode = "preferred"
		}
		return fmt.Sprintf(",%s,auto,%s", mode, m.Scale)
	default:
		return fmt.Sprintf("%s, %s, %s, %s", m.Name, m.Mode, m.Position, m.Scale)
	}
}

func (d Display) MonitorLine() string {
	return Monitor{
		Name:     d.MonitorName,
		Mode:     d.MonitorMode,
		Position: d.MonitorPosition,
		Scale:    d.MonitorScale,
	}.MonitorLine()
}

func (d Display) MonitorLines() []string {
	lines := make([]string, 0, 1+len(d.ExtraMonitors))
	lines = append(lines, "monitor = "+d.MonitorLine())
	for _, extra := range d.ExtraMonitors {
		lines = append(lines, "monitor = "+extra.MonitorLine())
	}
	return lines
}

type Hardware struct {
	GPU string `json:"gpu"`
}
type Features struct {
	SecureBoot    bool `json:"secureBoot"`
	CTFTools      bool `json:"ctfTools"`
	OmniRouter    bool `json:"omniRouter"`
	Portainer     bool `json:"portainer"`
	Observability bool `json:"observability"`
}
type Dots struct {
	Hypr       bool `json:"hypr"`
	ZenTheme   bool `json:"zenTheme"`
	Sine       bool `json:"sine"`
	Neovim     bool `json:"neovim"`
	V2rayN     bool `json:"v2rayN"`
	Wallpapers bool `json:"wallpapers"`
}
type Noctalia struct {
	Version string `json:"version"`
}
type Secrets struct {
	UserPassword string
}

func Default() State {
	return State{
		SchemaVersion: SchemaVersion,
		Host: Host{
			Hostname:     "NixOS",
			StateVersion: "26.05",
		},
		Source: Source{
			Channel: SourceChannelStable,
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
			MonitorMode:     "preferred",
			MonitorPosition: "0x0",
			MonitorScale:    "1",
		},
		Hardware: Hardware{
			GPU: "amd",
		},
		Features: Features{},
		Dots: Dots{
			Hypr:       true,
			ZenTheme:   true,
			Sine:       true,
			Neovim:     true,
			V2rayN:     true,
			Wallpapers: true,
		},
		Noctalia: Noctalia{
			Version: NoctaliaVersionV5,
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
	state, err := load(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	return state, err
}

func LoadExisting(path string) (State, error) {
	state, err := load(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return state, fmt.Errorf("state file does not exist %s: %w", path, err)
	}
	return state, err
}

// LoadGeneratedExisting accepts only complete state shapes emitted by released
// installer schemas. Draft and partial state remain supported by LoadExisting.
func LoadGeneratedExisting(path string) (State, error) {
	state, err := load(path, true)
	if errors.Is(err, os.ErrNotExist) {
		return state, fmt.Errorf("state file does not exist %s: %w", path, err)
	}
	return state, err
}

type generatedStateShapeError struct {
	err error
}

func (e *generatedStateShapeError) Error() string {
	return e.err.Error()
}

func (e *generatedStateShapeError) Unwrap() error {
	return e.err
}

// IsGeneratedStateShapeError reports a stable schema/content mismatch. Read,
// topology, and concurrent-mutation failures are deliberately excluded.
func IsGeneratedStateShapeError(err error) bool {
	var shapeError *generatedStateShapeError
	return errors.As(err, &shapeError)
}

func load(path string, requireGenerated bool) (State, error) {
	var state State
	data, err := readStateData(path)
	if err != nil {
		return state, fmt.Errorf("read state %s: %w", path, err)
	}
	if !utf8.Valid(data) {
		return state, fmt.Errorf("parse state %s: installer state must be valid UTF-8 JSON", path)
	}
	if requireGenerated {
		if err := validateGeneratedStateJSON(data); err != nil {
			return state, fmt.Errorf("parse generated state %s: %w", path, &generatedStateShapeError{err: err})
		}
	}
	data, err = stateJSONWithoutAllowlistedHistoricalFields(data)
	if err != nil {
		return state, fmt.Errorf("parse state %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, fmt.Errorf("parse state %s: %w", path, err)
	}
	if err := validateStateSchema(path, state.SchemaVersion); err != nil {
		return State{}, err
	}
	state = Migrate(state)
	return state, nil
}

func readStateData(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	data, err := io.ReadAll(io.LimitReader(file, MaxGeneratedStateBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxGeneratedStateBytes {
		return nil, fmt.Errorf("installer state exceeds the %d byte limit", MaxGeneratedStateBytes)
	}
	return data, nil
}

type historicalJSONKind uint8

const (
	historicalBool historicalJSONKind = iota
	historicalString
)

func stateJSONWithoutAllowlistedHistoricalFields(data []byte) ([]byte, error) {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return nil, err
	}
	if err := validateExactStateJSONFields(data); err != nil {
		return nil, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("state must be a JSON object")
	}
	version := 0
	if rawVersion, ok := root["schemaVersion"]; ok {
		if err := json.Unmarshal(rawVersion, &version); err != nil {
			return nil, err
		}
	}
	if err := validateStateSchema("state payload", version); err != nil {
		return nil, err
	}
	if err := stripHistoricalObject(root, "shell", version, 1, 3, map[string]historicalJSONKind{
		"profile": historicalString,
	}); err != nil {
		return nil, err
	}
	if err := stripHistoricalObject(root, "services", version, 1, 5, map[string]historicalJSONKind{
		"pgAdminEmail": historicalString,
	}); err != nil {
		return nil, err
	}
	if err := stripHistoricalObject(root, "zapret", version, 1, 7, map[string]historicalJSONKind{
		"enable": historicalBool,
		"config": historicalString,
	}); err != nil {
		return nil, err
	}
	if err := stripHistoricalNestedField(root, "features", "russiaMode", version, 1, 5, historicalBool); err != nil {
		return nil, err
	}
	if err := stripHistoricalNestedField(root, "dots", "neovimCleanState", version, 4, 6, historicalBool); err != nil {
		return nil, err
	}
	return json.Marshal(root)
}

//nolint:gocyclo // Exact field/type validation stays centralized so schema aliases cannot bypass a split validator.
func validateExactStateJSONFields(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	if err := validateExactJSONObjectFields(root, "state",
		"schemaVersion", "host", "source", "user", "locale", "git", "packages",
		"display", "hardware", "features", "dots", "noctalia",
		"shell", "services", "zapret",
	); err != nil {
		return err
	}
	if rawVersion, ok := root["schemaVersion"]; ok {
		if err := validateExactJSONScalar(rawVersion, "state.schemaVersion", "integer"); err != nil {
			return err
		}
	}
	objectFields := []struct {
		name         string
		stringFields []string
		boolFields   []string
		otherFields  []string
	}{
		{name: "host", stringFields: []string{"hostname", "stateVersion"}},
		{name: "source", stringFields: []string{"channel"}},
		{name: "user", stringFields: []string{"username", "fullName", "homeDirectory"}},
		{name: "locale", stringFields: []string{
			"timeZone", "defaultLocale", "extraLocale", "consoleKeyMap",
			"weatherLocation", "keyboardLayouts", "keyboardToggle",
		}},
		{name: "git", stringFields: []string{"username", "email"}},
		{name: "packages", stringFields: []string{"preset"}},
		{name: "display", stringFields: []string{
			"monitorName", "monitorMode", "monitorPosition", "monitorScale",
		}, otherFields: []string{"extraMonitors"}},
		{name: "hardware", stringFields: []string{"gpu"}},
		{name: "features", boolFields: []string{
			"secureBoot", "ctfTools", "omniRouter", "portainer", "observability", "russiaMode",
		}},
		{name: "dots", boolFields: []string{
			"hypr", "zenTheme", "sine", "neovim", "v2rayN", "wallpapers", "neovimCleanState",
		}},
		{name: "noctalia", stringFields: []string{"version"}},
		{name: "shell", stringFields: []string{"profile"}},
		{name: "services", stringFields: []string{"pgAdminEmail"}},
		{name: "zapret", stringFields: []string{"config"}, boolFields: []string{"enable"}},
	}
	objects := make(map[string]map[string]json.RawMessage, len(objectFields))
	for _, objectField := range objectFields {
		object, present, err := decodeExactJSONObjectField(root, objectField.name)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		fields := make([]string, 0, len(objectField.stringFields)+len(objectField.boolFields)+len(objectField.otherFields))
		fields = append(fields, objectField.stringFields...)
		fields = append(fields, objectField.boolFields...)
		fields = append(fields, objectField.otherFields...)
		if err := validateExactJSONObjectFields(object, "state."+objectField.name, fields...); err != nil {
			return err
		}
		for _, field := range objectField.stringFields {
			if raw, ok := object[field]; ok {
				if err := validateExactJSONScalar(raw, "state."+objectField.name+"."+field, "string"); err != nil {
					return err
				}
			}
		}
		for _, field := range objectField.boolFields {
			if raw, ok := object[field]; ok {
				if err := validateExactJSONScalar(raw, "state."+objectField.name+"."+field, "boolean"); err != nil {
					return err
				}
			}
		}
		objects[objectField.name] = object
	}

	display, ok := objects["display"]
	if !ok {
		return nil
	}
	rawMonitors, ok := display["extraMonitors"]
	if !ok {
		return nil
	}
	var monitors []json.RawMessage
	if err := json.Unmarshal(rawMonitors, &monitors); err != nil || monitors == nil {
		return fmt.Errorf("state field %q must be a non-null JSON array", "state.display.extraMonitors")
	}
	for index, rawMonitor := range monitors {
		var monitor map[string]json.RawMessage
		if err := json.Unmarshal(rawMonitor, &monitor); err != nil || monitor == nil {
			return fmt.Errorf(
				"state field %q must be a non-null JSON object",
				fmt.Sprintf("state.display.extraMonitors[%d]", index),
			)
		}
		monitorPath := fmt.Sprintf("state.display.extraMonitors[%d]", index)
		if err := validateExactJSONObjectFields(
			monitor,
			monitorPath,
			"name", "mode", "position", "scale",
		); err != nil {
			return err
		}
		for _, field := range []string{"name", "mode", "position", "scale"} {
			if raw, ok := monitor[field]; ok {
				if err := validateExactJSONScalar(raw, monitorPath+"."+field, "string"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func decodeExactJSONObjectField(root map[string]json.RawMessage, name string) (map[string]json.RawMessage, bool, error) {
	raw, ok := root[name]
	if !ok {
		return nil, false, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, true, fmt.Errorf(
			"unknown or invalid state field %q must be a non-null JSON object",
			"state."+name,
		)
	}
	return object, true, nil
}

func validateExactJSONScalar(raw json.RawMessage, path, kind string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	valid := false
	switch kind {
	case "string":
		_, valid = value.(string)
	case "boolean":
		_, valid = value.(bool)
	case "integer":
		if number, ok := value.(json.Number); ok {
			_, err := number.Int64()
			valid = err == nil
		}
	default:
		return fmt.Errorf("unsupported state JSON scalar kind %q", kind)
	}
	if !valid {
		return fmt.Errorf("state field %q must be a non-null JSON %s", path, kind)
	}
	return nil
}

func validateExactJSONObjectFields(object map[string]json.RawMessage, path string, fields ...string) error {
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := allowed[key]; ok {
			continue
		}
		for _, canonical := range fields {
			if strings.EqualFold(key, canonical) {
				return fmt.Errorf(
					"exact canonical JSON field %q is required at %s; got %q",
					canonical,
					path,
					key,
				)
			}
		}
		return fmt.Errorf("unknown state field %q", path+"."+key)
	}
	return nil
}

//nolint:gocyclo // Versioned schema invariants are intentionally evaluated together as one exact compatibility matrix.
func validateGeneratedStateJSON(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("generated state must be valid UTF-8 JSON")
	}
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
	if err := validateExactStateJSONFields(data); err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("generated state must be a JSON object")
	}
	version := 0
	if raw, ok := root["schemaVersion"]; ok {
		if err := json.Unmarshal(raw, &version); err != nil {
			return err
		}
	}
	if err := validateStateSchema("generated state payload", version); err != nil {
		return err
	}

	required := []string{"host", "user", "locale", "git", "packages", "display", "hardware", "features", "dots"}
	optional := []string{}
	if version == 0 {
		optional = append(optional, "schemaVersion")
	} else {
		required = append(required, "schemaVersion")
	}
	if version <= 2 {
		required = append(required, "shell")
	} else if version == 3 {
		optional = append(optional, "shell")
	}
	if version <= 4 {
		required = append(required, "services")
	} else if version == 5 {
		optional = append(optional, "services")
	}
	if version <= 6 {
		required = append(required, "zapret")
	} else {
		optional = append(optional, "zapret")
	}
	if version >= 6 {
		required = append(required, "source")
	}
	if version >= 7 {
		required = append(required, "noctalia")
	}
	if err := validateGeneratedJSONObjectShape(root, "state", required, optional); err != nil {
		return err
	}

	shapes := []struct {
		name     string
		required []string
		optional []string
	}{
		{name: "host", required: []string{"hostname", "stateVersion"}},
		{name: "user", required: []string{"username", "fullName", "homeDirectory"}},
		{name: "locale", required: []string{
			"timeZone", "defaultLocale", "extraLocale", "consoleKeyMap",
			"weatherLocation", "keyboardLayouts", "keyboardToggle",
		}},
		{name: "git", required: []string{"username", "email"}},
		{name: "packages", required: []string{"preset"}},
		{name: "display", required: []string{
			"monitorName", "monitorMode", "monitorPosition", "monitorScale",
		}},
		{name: "hardware", required: []string{"gpu"}},
		{name: "shell", required: []string{"profile"}},
		{name: "services", required: []string{"pgAdminEmail"}},
		{name: "zapret", required: []string{"enable", "config"}},
		{name: "source", required: []string{"channel"}},
		{name: "noctalia", required: []string{"version"}},
	}
	for _, shape := range shapes {
		if shape.name == "display" && version >= 5 {
			shape.optional = []string{"extraMonitors"}
		}
		object, present, err := decodeExactJSONObjectField(root, shape.name)
		if err != nil {
			return err
		}
		if !present {
			continue
		}
		if err := validateGeneratedJSONObjectShape(object, "state."+shape.name, shape.required, shape.optional); err != nil {
			return err
		}
	}
	display, _, err := decodeExactJSONObjectField(root, "display")
	if err != nil {
		return err
	}
	if rawMonitors, ok := display["extraMonitors"]; ok {
		var monitors []json.RawMessage
		if err := json.Unmarshal(rawMonitors, &monitors); err != nil {
			return err
		}
		for index, rawMonitor := range monitors {
			var monitor map[string]json.RawMessage
			if err := json.Unmarshal(rawMonitor, &monitor); err != nil {
				return err
			}
			if err := validateGeneratedJSONObjectShape(
				monitor,
				fmt.Sprintf("state.display.extraMonitors[%d]", index),
				[]string{"name", "mode", "position", "scale"},
				nil,
			); err != nil {
				return err
			}
		}
	}

	features, _, err := decodeExactJSONObjectField(root, "features")
	if err != nil {
		return err
	}
	featureRequired := []string{"secureBoot", "ctfTools", "omniRouter"}
	featureOptional := []string{}
	if version >= 4 {
		featureRequired = append(featureRequired, "observability")
	}
	if version <= 4 {
		featureRequired = append(featureRequired, "russiaMode")
	} else if version == 5 {
		featureOptional = append(featureOptional, "russiaMode")
	}
	if version == 7 {
		featureOptional = append(featureOptional, "portainer")
	}
	if err := validateGeneratedJSONObjectShape(features, "state.features", featureRequired, featureOptional); err != nil {
		return err
	}

	dots, _, err := decodeExactJSONObjectField(root, "dots")
	if err != nil {
		return err
	}
	dotRequired := []string{"hypr", "zenTheme", "sine", "neovim", "v2rayN", "wallpapers"}
	dotOptional := []string{}
	if version >= 4 && version <= 5 {
		dotRequired = append(dotRequired, "neovimCleanState")
	} else if version == 6 {
		dotOptional = append(dotOptional, "neovimCleanState")
	}
	if err := validateGeneratedJSONObjectShape(dots, "state.dots", dotRequired, dotOptional); err != nil {
		return err
	}
	if version == 5 {
		_, hasServices := root["services"]
		_, hasRussia := features["russiaMode"]
		if hasServices && !hasRussia {
			return fmt.Errorf("generated schema 5 has an unknown services/Russia mode combination")
		}
	}
	if version == 7 {
		_, hasZapret := root["zapret"]
		_, hasPortainer := features["portainer"]
		if hasZapret && hasPortainer {
			return fmt.Errorf("generated schema 7 has an unknown Zapret/Portainer combination")
		}
	}
	return nil
}

func validateGeneratedJSONObjectShape(
	object map[string]json.RawMessage,
	path string,
	required []string,
	optional []string,
) error {
	allowed := make(map[string]struct{}, len(required)+len(optional))
	for _, field := range required {
		allowed[field] = struct{}{}
		if _, ok := object[field]; !ok {
			return fmt.Errorf("generated state object %q is missing required field %q", path, field)
		}
	}
	for _, field := range optional {
		allowed[field] = struct{}{}
	}
	for field := range object {
		if _, ok := allowed[field]; !ok {
			return fmt.Errorf("generated state object %q has unsupported field %q", path, field)
		}
	}
	return nil
}

func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, "state"); err != nil {
		return err
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected trailing JSON value %v", token)
	}
	return nil
}

//nolint:gocyclo // Recursive object and array token validation is clearer as one small JSON grammar walker.
func scanJSONValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("JSON object key at %s is not a string", path)
			}
			fieldPath := path + "." + key
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON field %q", fieldPath)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, fieldPath); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("invalid JSON object terminator at %s", path)
		}
		return nil
	case '[':
		for index := 0; decoder.More(); index++ {
			if err := scanJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("invalid JSON array terminator at %s", path)
		}
		return nil
	default:
		return fmt.Errorf("unexpected JSON delimiter %q at %s", delimiter, path)
	}
}

func stripHistoricalObject(root map[string]json.RawMessage, name string, version, minVersion, maxVersion int, fields map[string]historicalJSONKind) error {
	raw, ok := root[name]
	if !ok {
		return nil
	}
	if !historicalSchemaAllowed(version, minVersion, maxVersion) {
		return fmt.Errorf("unknown state field %q for schemaVersion %d", name, version)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return fmt.Errorf("unknown or invalid historical state field %q", name)
	}
	if len(object) != len(fields) {
		return fmt.Errorf("unknown or incomplete historical state field %q", name)
	}
	for field, kind := range fields {
		value, exists := object[field]
		if !exists || !validHistoricalJSONValue(value, kind) {
			return fmt.Errorf("unknown or invalid historical state field %q", name+"."+field)
		}
	}
	delete(root, name)
	return nil
}

func stripHistoricalNestedField(root map[string]json.RawMessage, parent, name string, version, minVersion, maxVersion int, kind historicalJSONKind) error {
	rawParent, ok := root[parent]
	if !ok {
		return nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(rawParent, &object); err != nil {
		return err
	}
	if object == nil {
		return fmt.Errorf("unknown or invalid historical state field %q", parent)
	}
	value, ok := object[name]
	if !ok {
		return nil
	}
	field := parent + "." + name
	if !historicalSchemaAllowed(version, minVersion, maxVersion) {
		return fmt.Errorf("unknown state field %q for schemaVersion %d", field, version)
	}
	if !validHistoricalJSONValue(value, kind) {
		return fmt.Errorf("unknown or invalid historical state field %q", field)
	}
	delete(object, name)
	updated, err := json.Marshal(object)
	if err != nil {
		return err
	}
	root[parent] = updated
	return nil
}

func historicalSchemaAllowed(version, minVersion, maxVersion int) bool {
	return version == 0 || version >= minVersion && version <= maxVersion
}

func validHistoricalJSONValue(raw json.RawMessage, kind historicalJSONKind) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	switch kind {
	case historicalBool:
		var value bool
		return json.Unmarshal(raw, &value) == nil
	case historicalString:
		var value string
		return json.Unmarshal(raw, &value) == nil
	default:
		return false
	}
}

func validateStateSchema(path string, version int) error {
	if version >= 0 && version <= SchemaVersion {
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
	state = migrateSource(state, def)
	state = migrateUser(state, def)
	state = migrateLocale(state, def, oldVersion)
	state = migrateIdentity(state, def)
	state = migrateDisplay(state, def)
	state = migrateNoctalia(state, def)
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

func migrateSource(state State, def State) State {
	if state.Source.Channel == "" {
		state.Source.Channel = def.Source.Channel
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

func migrateNoctalia(state State, def State) State {
	if state.Noctalia.Version == "" {
		state.Noctalia.Version = def.Noctalia.Version
	}
	return state
}

func migrateFeatures(state State, def State, legacy bool, oldVersion int) State {
	if state.Hardware.GPU == "" {
		state.Hardware.GPU = def.Hardware.GPU
	}
	if legacy {
		state.Dots = def.Dots
	}
	if oldVersion < 3 {
		state.Dots.Sine = true
		state.Dots.Neovim = true
	}
	return state
}
