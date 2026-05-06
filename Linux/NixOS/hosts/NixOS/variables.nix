{ config, lib, ... }:

{
  options.var = lib.mkOption {
    type = lib.types.attrs;
    default = { };
    description = "Centralised host/user variables consumed across the config.";
  };

  config.var = {
    # Host
    hostname = "NixOS";
    stateVersion = "25.11";

    # User
    username = "user";
    fullName = "user";

    # Paths
    configDirectory = "/etc/nixos";
    homeDirectory = "/home/user";

    # Locale / region
    timeZone = "Europe/Moscow";
    defaultLocale = "en_US.UTF-8";
    extraLocale = "ru_RU.UTF-8";
    consoleKeyMap = "us";
    weatherLocation = "Moscow";

    # Git identity (used by home-manager programs.git)
    git = {
      username = "user";
      email = "user@example.com";
    };

    # Behaviour flags
    shellProfile = "caelestia"; # caelestia | noctalia
    packagePreset = "personal"; # minimal | desktop | developer | personal
    hardware = {
      gpu = "amd"; # amd | intel | nvidia | other
    };
    features = {
      secureBoot = false;
      ctfTools = false;
      omnirouter = false;
      russiaMode = false;
    };
    zapret = {
      enable = false;
      config = "general (FAKE_TLS_AUTO_ALT3)";
    };
    services = {
      pgadminEmail = "admin@localhost.local";
    };
    hypr = {
      keyboardLayouts = "us,ru";
      keyboardToggle = "grp:alt_shift_toggle";
    };
    wallpapers = {
      enable = true;
    };
    autoGarbageCollector = true;
    autoOptimiseStore = true;
  };
}
