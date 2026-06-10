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

func FlakeNix(s config.State) (string, error) {
	var out bytes.Buffer
	if err := flakeTemplate.Execute(&out, s); err != nil {
		return "", fmt.Errorf("render flake.nix: %w", err)
	}
	return out.String(), nil
}

func ConfigurationNix() string {
	return `{ pkgs, ... }:

{
  imports = [
    ./private
  ];

  environment.systemPackages = with pkgs; [
    # Add system packages here.
  ];
}
`
}

func PrivateDefaultNix() string {
	return `# Host-local private NixOS modules. This file is preserved by MySetup.
# Put local-only modules and payloads in this directory, then uncomment or add
# explicit imports below.
{ ... }:

{
  imports = [
    # ./ida-pro.nix
    # ./ida-mcp.nix
    # ./ida-plugins.nix
  ];
}
`
}

func HomeNix() string {
	return `# Host-local Home Manager overrides. This file is preserved by MySetup.
{ pkgs, ... }:

{
  home.packages = with pkgs; [
    # Add user packages here.
  ];
}
`
}

var flakeTemplate = template.Must(template.New("flake.nix").Funcs(template.FuncMap{
	"nixString": nixString,
}).Parse(`{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    mysetup.url = "github:TakuyaYagam1/MySetup?dir=Linux/NixOS";
  };

  outputs = { mysetup, ... }:
    let
      system = "x86_64-linux";
      hostname = {{ nixString .Host.Hostname }};
    in
    {
      nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {
        inherit system hostname;

        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        hashedPassword =
          if builtins.pathExists ./hashed-password.nix then ./hashed-password.nix else null;
        secretsDir =
          if builtins.pathExists ./secrets then ./secrets else null;

        extraModules = [ ./configuration.nix ];
        homeExtraModules =
          if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`))

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
    observability = {{ nixBool .Features.Observability }};
  };

  zapret = {
    enable = {{ nixBool .Zapret.Enable }};
    config = {{ nixString .Zapret.Config }};
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
