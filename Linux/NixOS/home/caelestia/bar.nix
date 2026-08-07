_:

{
  caelestiaShellSettings.bar = {
    clock = {
      background = true;
      showDate = true;
      showIcon = true;
    };
    dragThreshold = 20;
    entries = [
      {
        id = "logo";
        enabled = true;
      }
      {
        id = "workspaces";
        enabled = true;
      }
      {
        id = "spacer";
        enabled = true;
      }
      {
        id = "activeWindow";
        enabled = true;
      }
      {
        id = "spacer";
        enabled = true;
      }
      {
        id = "tray";
        enabled = true;
      }
      {
        id = "clock";
        enabled = true;
      }
      {
        id = "statusIcons";
        enabled = true;
      }
      {
        id = "power";
        enabled = true;
      }
    ];
    persistent = true;
    popouts = {
      activeWindow = true;
      statusIcons = true;
      tray = true;
    };
    scrollActions = {
      brightness = true;
      workspaces = true;
      volume = true;
    };
    showOnHover = true;
    statusIcons = [
      {
        id = "lockStatus";
        enabled = true;
      }
      {
        id = "audio";
        enabled = true;
      }
      {
        id = "microphone";
        enabled = true;
      }
      {
        id = "kbLayout";
        enabled = true;
      }
      {
        id = "network";
        enabled = true;
      }
      {
        id = "bluetooth";
        enabled = true;
      }
      {
        id = "battery";
        enabled = true;
      }
    ];
    tray = {
      background = true;
      compact = true;
      iconSubs = [ ];
      recolour = true;
    };
    workspaces = {
      activeIndicator = true;
      activeLabel = "󰮯";
      activeTrail = true;
      label = "  ";
      occupiedBg = true;
      occupiedLabel = "󰮯";
      perMonitorWorkspaces = true;
      showWindows = true;
      shown = 5;
      specialWorkspaceIcons = [
        {
          name = "steam";
          icon = "sports_esports";
        }
      ];
      windowIcons = [
        {
          regex = "steam(_app_(default|[0-9]+))?";
          icon = "sports_esports";
        }
      ];
    };
    excludedScreens = [ ];
    activeWindow = {
      compact = true;
      inverted = false;
      showOnHover = true;
    };
  };
}
