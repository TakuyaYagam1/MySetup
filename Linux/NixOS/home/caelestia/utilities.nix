{ ... }:

{
  caelestiaShellSettings.notifs = {
    actionOnClick = true;
    clearThreshold = 0.3;
    defaultExpireTimeout = 5000;
    expandThreshold = 20;
    openExpanded = true;
    expire = true;
  };

  caelestiaShellSettings.osd = {
    enabled = true;
    enableBrightness = true;
    enableMicrophone = true;
    hideDelay = 1000;
  };

  caelestiaShellSettings.dashboard = {
    enabled = true;
    dragThreshold = 50;
    mediaUpdateInterval = 500;
    showOnHover = true;
  };

  caelestiaShellSettings.sidebar = {
    dragThreshold = 80;
    enabled = true;
  };

  caelestiaShellSettings.utilities = {
    enabled = true;
    maxToasts = 1;
    toasts = {
      audioInputChanged = true;
      audioOutputChanged = true;
      capsLockChanged = true;
      chargingChanged = true;
      configLoaded = true;
      dndChanged = true;
      gameModeChanged = true;
      kbLayoutChanged = true;
      kbLimit = true;
      numLockChanged = true;
      vpnChanged = true;
      nowPlaying = true;
    };
    quickToggles = [
      { id = "wifi";      enabled = true; }
      { id = "bluetooth"; enabled = true; }
      { id = "mic";       enabled = true; }
      { id = "settings";  enabled = true; }
      { id = "gameMode";  enabled = true; }
      { id = "dnd";       enabled = true; }
      { id = "vpn";       enabled = true; }
    ];
    vpn = {
      enabled = true;
      provider = [
        {
          name = "wireguard";
          interface = "your-connection-name";
          displayName = "Wireguard (Your VPN)";
          enabled = false;
        }
      ];
    };
  };
}
