package apply

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func HostVarsNix(s config.State) (string, error) {
	var out bytes.Buffer
	if err := hostVarsTemplate.Execute(&out, s); err != nil {
		return "", fmt.Errorf("render host-vars.nix: %w", err)
	}
	return out.String(), nil
}

var hostVarsTemplate = template.Must(template.New("host-vars.nix").Funcs(template.FuncMap{
	"nixBool":        nixBool,
	"nixString":      nixString,
	"nixMonitorList": nixMonitorList,
}).Parse(`{
  host = {
    hostname = {{ nixString .Host.Hostname }};
    stateVersion = {{ nixString .Host.StateVersion }};
    configDirectory = "/etc/nixos";
    autoGarbageCollector = true;
    autoOptimiseStore = true;
  };

  user = {
    username = {{ nixString .User.Username }};
    fullName = {{ nixString .User.FullName }};
    homeDirectory = {{ nixString .User.HomeDirectory }};
  };

  locale = {
    timeZone = {{ nixString .Locale.TimeZone }};
    defaultLocale = {{ nixString .Locale.DefaultLocale }};
    extraLocale = {{ nixString .Locale.ExtraLocale }};
    consoleKeyMap = {{ nixString .Locale.ConsoleKeyMap }};
    weatherLocation = {{ nixString .Locale.WeatherLocation }};
  };

  git = {
    username = {{ nixString .Git.Username }};
    email = {{ nixString .Git.Email }};
  };

  packages = {
    preset = {{ nixString .Packages.Preset }};
  };

  hardware = {
    gpu = {{ nixString .Hardware.GPU }};
  };

  features = {
    secureBoot = {{ nixBool .Features.SecureBoot }};
    ctfTools = {{ nixBool .Features.CTFTools }};
    omnirouter = {{ nixBool .Features.OmniRouter }};
    russiaMode = {{ nixBool .Features.RussiaMode }};
    observability = {{ nixBool .Features.Observability }};
  };

  zapret = {
    enable = {{ nixBool .Zapret.Enable }};
    config = {{ nixString .Zapret.Config }};
  };

  services = {
    pgadminEmail = {{ nixString .Services.PgAdminEmail }};
  };

  hypr = {
    keyboardLayouts = {{ nixString .Locale.KeyboardLayouts }};
    keyboardToggle = {{ nixString .Locale.KeyboardToggle }};
    windowOpacity = "0.8";
  };

  display = {
    monitorName = {{ nixString .Display.MonitorName }};
    monitorMode = {{ nixString .Display.MonitorMode }};
    monitorPosition = {{ nixString .Display.MonitorPosition }};
    monitorScale = {{ nixString .Display.MonitorScale }};
    extraMonitors = {{ nixMonitorList .Display.ExtraMonitors }};
  };

  wallpapers = {
    enable = {{ nixBool .Dots.Wallpapers }};
  };
}
`))

func HashedPasswordNix(hash string) string {
	return fmt.Sprintf(`{ config, ... }:

{
  users.users.${config.mysetup.user.username}.initialHashedPassword = %s;
}
`, nixString(hash))
}

func nixBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func nixString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, `${`, `''${`)
	return `"` + escaped + `"`
}

func nixMonitorList(monitors []config.Monitor) string {
	if len(monitors) == 0 {
		return "[ ]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for _, monitor := range monitors {
		b.WriteString("      {\n")
		fmt.Fprintf(&b, "        name = %s;\n", nixString(monitor.Name))
		fmt.Fprintf(&b, "        mode = %s;\n", nixString(monitor.Mode))
		fmt.Fprintf(&b, "        position = %s;\n", nixString(monitor.Position))
		fmt.Fprintf(&b, "        scale = %s;\n", nixString(monitor.Scale))
		b.WriteString("      }\n")
	}
	b.WriteString("    ]")
	return b.String()
}
