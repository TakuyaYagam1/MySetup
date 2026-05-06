package apply

import (
	"fmt"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func VariablesNix(s config.State) string {
	return fmt.Sprintf(`{ config, lib, ... }:

{
  options.var = lib.mkOption {
    type = lib.types.attrs;
    default = { };
    description = "Centralised host/user variables consumed across the config.";
  };

  config.var = {
    hostname = %s;
    stateVersion = %s;

    username = %s;
    fullName = %s;

    configDirectory = "/etc/nixos";
    homeDirectory = %s;

    timeZone = %s;
    defaultLocale = %s;
    extraLocale = %s;
    consoleKeyMap = %s;
    weatherLocation = %s;

    git = {
      username = %s;
      email = %s;
    };

    shellProfile = %s;
    packagePreset = %s; # minimal | desktop | developer | personal

    hardware = {
      gpu = %s;
    };

    features = {
      secureBoot = %s;
      ctfTools = %s;
      omnirouter = %s;
      russiaMode = %s;
    };

    zapret = {
      enable = %s;
      config = %s;
    };

    services = {
      pgadminEmail = %s;
    };

    hypr = {
      keyboardLayouts = %s;
      keyboardToggle = %s;
    };

    wallpapers = {
      enable = %s;
    };

    autoGarbageCollector = true;
    autoOptimiseStore = true;
  };
}
`,
		nixString(s.Host.Hostname),
		nixString(s.Host.StateVersion),
		nixString(s.User.Username),
		nixString(s.User.FullName),
		nixString(s.User.HomeDirectory),
		nixString(s.Locale.TimeZone),
		nixString(s.Locale.DefaultLocale),
		nixString(s.Locale.ExtraLocale),
		nixString(s.Locale.ConsoleKeyMap),
		nixString(s.Locale.WeatherLocation),
		nixString(s.Git.Username),
		nixString(s.Git.Email),
		nixString(s.Shell.Profile),
		nixString(s.Packages.Preset),
		nixString(s.Hardware.GPU),
		nixBool(s.Features.SecureBoot),
		nixBool(s.Features.CTFTools),
		nixBool(s.Features.OmniRouter),
		nixBool(s.Features.RussiaMode),
		nixBool(s.Zapret.Enable),
		nixString(s.Zapret.Config),
		nixString(s.Services.PgAdminEmail),
		nixString(s.Locale.KeyboardLayouts),
		nixString(s.Locale.KeyboardToggle),
		nixBool(s.Dots.Wallpapers),
	)
}

func HostDefaultNix() string {
	return `{ config, pkgs, lib, inputs, ... }:

{
  imports = [
    ./variables.nix

    ../../system/boot/grub.nix
    ../../system/boot/plymouth.nix
    ../../system/boot/secure.nix
    ../../system/locale.nix
    ../../system/networking.nix
    ../../system/security.nix
    ../../system/nvidia-drivers.nix
    ../../system/power.nix
    ../../system/hardware.nix
    ../../system/settings.nix

    ../../services/sddm.nix
    ../../services/databases.nix
    ../../services/virtualization.nix
    ../../services/system-services.nix
    ../../services/zapret.nix

    ../../programs/hyprland.nix
    ../../programs/thunar.nix
    ../../programs/gaming.nix
    ../../programs/fish.nix
    ../../programs/development.nix
    ../../programs/system-tools.nix

    ../../packages/system-packages.nix
    ../../packages/dev-tools.nix
    ../../packages/fonts.nix
    ../../packages/zen-browser.nix
    ../../packages/ctf
    # ../../packages/ida-mcp.nix
    # ../../packages/ida-plugins.nix
    # ../../packages/ida-pro.nix
    ../../users/user.nix

    ../../modules/omnirouter.nix
  ]
  ++ lib.optional (builtins.pathExists ./hardware-configuration.nix) ./hardware-configuration.nix
  ++ lib.optional (builtins.pathExists ./hashed-password.nix) ./hashed-password.nix;

  boot.kernelPackages = lib.mkForce pkgs.linuxPackages_6_18;

  environment.sessionVariables = {
    NIXOS_OZONE_WL = "1";
    ELECTRON_OZONE_PLATFORM_HINT = "wayland";
  };

  system.stateVersion = config.var.stateVersion;
}
`
}

func HashedPasswordNix(hash string) string {
	return fmt.Sprintf(`{ config, ... }:

{
  users.users.${config.var.username}.initialHashedPassword = %s;
}
`, nixString(hash))
}

func ManagedMarker(kind string) string {
	return fmt.Sprintf(`{
  "manager": "mysetup",
  "kind": %s,
  "version": 1
}
`, nixString(kind))
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
