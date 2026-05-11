{
  host = {
    hostname = "NixOS";
    stateVersion = "25.11";
    configDirectory = "/etc/nixos";
    autoGarbageCollector = true;
    autoOptimiseStore = true;
  };

  user = {
    username = "user";
    fullName = "user";
    homeDirectory = "/home/user";
  };

  locale = {
    timeZone = "Europe/Moscow";
    defaultLocale = "en_US.UTF-8";
    extraLocale = "ru_RU.UTF-8";
    consoleKeyMap = "us";
    weatherLocation = "Moscow";
  };

  git = {
    username = "user";
    email = "user@example.com";
  };

  packages = {
    preset = "personal";
  };

  hardware = {
    gpu = "amd";
  };

  features = {
    secureBoot = false;
    ctfTools = false;
    omnirouter = false;
    russiaMode = false;
    observability = false;
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
    windowOpacity = "0.8";
  };

  display = {
    monitorName = "eDP-1";
    monitorMode = "preferred";
    monitorPosition = "0x0";
    monitorScale = "1";
    extraMonitors = [ ];
  };

  wallpapers = {
    enable = true;
  };
}
