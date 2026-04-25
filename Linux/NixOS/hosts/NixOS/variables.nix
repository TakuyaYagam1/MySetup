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
    consoleKeyMap = "ruwin_alt_sh-UTF-8";
    weatherLocation = "Moscow";

    # Git identity (used by home-manager programs.git)
    git = {
      username = "user";
      email = "user@example.com";
    };

    # Behaviour flags
    autoGarbageCollector = true;
    autoOptimiseStore = true;
  };
}
