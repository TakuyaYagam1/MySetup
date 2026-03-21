{ config, pkgs, inputs, ... }:

let
  quickshellPkg = inputs.quickshell.packages.${pkgs.stdenv.hostPlatform.system}.default;
in
{
  programs.caelestia = {
    enable = true;

    systemd = {
      enable = false;
      target = "graphical-session.target";
      environment = [];
    };

    cli = {
      enable = true;
      settings.theme.enableGtk = false;
    };

    settings = {
      appearance = {
        mediaGifSpeedAdjustment = 300;
        sessionGifSpeed = 0.7;
        anim = {
          durations = {
            scale = 1;
          };
        };
        font = {
          family = {
            clock = "Rubik";
            material = "Material Symbols Rounded";
            mono = "CaskaydiaCove NF";
            sans = "Rubik";
          };
          size = {
            scale = 1;
          };
        };
        padding = {
          scale = 1;
        };
        rounding = {
          scale = 1;
        };
        spacing = {
          scale = 1;
        };
        transparency = {
          enabled = true;
          base = 0.7;
          layers = 0.4;
        };
      };
      general = {
        logo = "caelestia";
        apps = {
          terminal = [ "foot" ];
          audio = [ "pavucontrol" ];
          playback = [ "mpv" ];
          explorer = [ "thunar" ];
        };
        battery = {
          warnLevels = [
            {
              level = 20;
              title = "Low battery";
              message = "You might want to plug in a charger";
              icon = "battery_android_frame_2";
            }
            {
              level = 10;
              title = "Did you see the previous message?";
              message = "You should probably plug in a charger <b>now</b>";
              icon = "battery_android_frame_1";
            }
            {
              level = 5;
              title = "Critical battery level";
              message = "PLUG THE CHARGER RIGHT NOW!!";
              icon = "battery_android_alert";
              critical = true;
            }
          ];
          criticalLevel = 3;
        };
        idle = {
          lockBeforeSleep = true;
          inhibitWhenAudio = true;
          timeouts = [
            {
              timeout = 600;
              idleAction = "lock";
            }
            {
              timeout = 1800;
              idleAction = "dpms off";
              returnAction = "dpms on";
            }
            {
              timeout = 3600;
              idleAction = [ "systemctl" "suspend-then-hibernate" ];
            }
          ];
        };
      };
      background = {
        desktopClock = {
          enabled = true;
          scale = 1.0;
          position = "top-left";
          shadow = {
            enabled = true;
            opacity = 0.7;
            blur = 0.4;
          };
          background = {
            enabled = false;
            opacity = 0.7;
            blur = true;
          };
          invertColors = false;
        };
        enabled = true;
        visualiser = {
          blur = false;
          enabled = true;
          autoHide = true;
          rounding = 1;
          spacing = 1;
        };
      };
      bar = {
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
        status = {
          showAudio = true;
          showBattery = true;
          showBluetooth = true;
          showKbLayout = true;
          showMicrophone = true;
          showNetwork = true;
          showWifi = true;
          showLockStatus = true;
        };
        tray = {
          background = true;
          compact = true;
          iconSubs = [];
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
        excludedScreens = [ "" ];
        activeWindow = {
          compact = true;
          inverted = false;
        };
      };
      border = {
        rounding = 25;
        thickness = 10;
      };
      dashboard = {
        enabled = true;
        dragThreshold = 50;
        mediaUpdateInterval = 500;
        showOnHover = true;
      };
      launcher = {
        actionPrefix = ">";
        actions = [
          {
            name = "Calculator";
            icon = "calculate";
            description = "Do simple math equations (powered by Qalc)";
            command = [ "autocomplete" "calc" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Scheme";
            icon = "palette";
            description = "Change the current colour scheme";
            command = [ "autocomplete" "scheme" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Wallpaper";
            icon = "image";
            description = "Change the current wallpaper";
            command = [ "autocomplete" "wallpaper" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Variant";
            icon = "colors";
            description = "Change the current scheme variant";
            command = [ "autocomplete" "variant" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Transparency";
            icon = "opacity";
            description = "Change shell transparency";
            command = [ "autocomplete" "transparency" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Random";
            icon = "casino";
            description = "Switch to a random wallpaper";
            command = [ "caelestia" "wallpaper" "-r" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Light";
            icon = "light_mode";
            description = "Change the scheme to light mode";
            command = [ "setMode" "light" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Dark";
            icon = "dark_mode";
            description = "Change the scheme to dark mode";
            command = [ "setMode" "dark" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Shutdown";
            icon = "power_settings_new";
            description = "Shutdown the system";
            command = [ "systemctl" "poweroff" ];
            enabled = true;
            dangerous = true;
          }
          {
            name = "Reboot";
            icon = "cached";
            description = "Reboot the system";
            command = [ "systemctl" "reboot" ];
            enabled = true;
            dangerous = true;
          }
          {
            name = "Logout";
            icon = "exit_to_app";
            description = "Log out of the current session";
            command = [ "pkill" "-KILL" "-u" "takuya" ];
            enabled = true;
            dangerous = true;
          }
          {
            name = "Lock";
            icon = "lock";
            description = "Lock the current session";
            command = [ "loginctl" "lock-session" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Sleep";
            icon = "bedtime";
            description = "Suspend then hibernate";
            command = [ "systemctl" "suspend-then-hibernate" ];
            enabled = true;
            dangerous = false;
          }
          {
            name = "Settings";
            icon = "settings";
            description = "Configure the shell";
            command = [ "caelestia" "shell" "controlCenter" "open" ];
            enabled = true;
            dangerous = false;
          }
        ];
        dragThreshold = 50;
        vimKeybinds = false;
        enableDangerousActions = true;
        maxShown = 7;
        maxWallpapers = 9;
        specialPrefix = "@";
        useFuzzy = {
          apps = false;
          actions = false;
          schemes = false;
          variants = false;
          wallpapers = false;
        };
        showOnHover = true;
        favouriteApps = [];
        hiddenApps = [];
      };
      lock = {
        recolourLogo = false;
        hideNotifs = true;
      };
      notifs = {
        actionOnClick = true;
        clearThreshold = 0.3;
        defaultExpireTimeout = 5000;
        expandThreshold = 20;
        openExpanded = true;
        expire = true;
      };
      osd = {
        enabled = true;
        enableBrightness = true;
        enableMicrophone = true;
        hideDelay = 1000;
      };
      paths = {
        mediaGif = "root:/assets/bongocat.gif";
        sessionGif = "root:/assets/kurukuru.gif";
        wallpaperDir = "~/Pictures/Wallpapers";
      };
      services = {
        audioIncrement = 0.1;
        brightnessIncrement = 0.1;
        maxVolume = 1.0;
        defaultPlayer = "Spotify";
        gpuType = "";
        playerAliases = [
          {
            from = "com.github.th_ch.youtube_music";
            to = "YT Music";
          }
        ];
        weatherLocation = "Moscow";
        useFahrenheit = false;
        useFahrenheitPerformance = false;
        useTwelveHourClock = false;
        smartScheme = true;
        visualiserBars = 45;
      };
      session = {
        dragThreshold = 30;
        enabled = true;
        vimKeybinds = false;
        icons = {
          logout = "logout";
          shutdown = "power_settings_new";
          hibernate = "downloading";
          reboot = "cached";
        };
        commands = {
          logout = [ "pkill" "-KILL" "-u" "takuya" ];
          shutdown = [ "systemctl" "poweroff" ];
          hibernate = [ "systemctl" "hibernate" ];
          reboot = [ "systemctl" "reboot" ];
        };
      };
      sidebar = {
        dragThreshold = 80;
        enabled = true;
      };
      utilities = {
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
          {
            id = "wifi";
            enabled = true;
          }
          {
            id = "bluetooth";
            enabled = true;
          }
          {
            id = "mic";
            enabled = true;
          }
          {
            id = "settings";
            enabled = true;
          }
          {
            id = "gameMode";
            enabled = true;
          }
          {
            id = "dnd";
            enabled = true;
          }
          {
            id = "vpn";
            enabled = true;
          }
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
    };
  };

  home.packages = [ quickshellPkg ];
}
