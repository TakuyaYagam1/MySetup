{ lib, ... }:

let
  inherit (lib) mkOption types;

  inherit (import ../lib/preset-names.nix) presetNames defaultPreset;

  gpuKinds = [
    "amd"
    "intel"
    "nvidia"
    "other"
  ];

  strOption = mkOption { type = types.str; };
  boolOption =
    default:
    mkOption {
      type = types.bool;
      inherit default;
    };

  mysetupType = types.submodule {
    options = {
      host = mkOption {
        type = types.submodule {
          options = {
            hostname = strOption;
            stateVersion = strOption;
            configDirectory = strOption;
            autoGarbageCollector = boolOption true;
            autoOptimiseStore = boolOption true;
          };
        };
      };
      user = mkOption {
        type = types.submodule {
          options = {
            username = strOption;
            fullName = strOption;
            homeDirectory = strOption;
          };
        };
      };
      locale = mkOption {
        type = types.submodule {
          options = {
            timeZone = strOption;
            defaultLocale = strOption;
            extraLocale = strOption;
            consoleKeyMap = strOption;
            weatherLocation = strOption;
          };
        };
      };
      git = mkOption {
        type = types.submodule {
          options = {
            username = strOption;
            email = strOption;
          };
        };
      };
      packages = mkOption {
        type = types.submodule {
          options.preset = mkOption {
            type = types.enum presetNames;
            default = defaultPreset;
          };
        };
      };
      hardware = mkOption {
        type = types.submodule {
          options.gpu = mkOption {
            type = types.enum gpuKinds;
          };
        };
      };
      features = mkOption {
        type = types.submodule {
          options = {
            secureBoot = boolOption false;
            ctfTools = boolOption false;
            omnirouter = boolOption false;
            russiaMode = boolOption false;
            observability = boolOption false;
          };
        };
      };
      zapret = mkOption {
        type = types.submodule {
          options = {
            enable = boolOption false;
            config = strOption;
          };
        };
      };
      nix = mkOption {
        type = types.submodule {
          options.gcRetention = mkOption {
            type = types.str;
            default = "14d";
            description = "Retention window passed to nix-collect-garbage --delete-older-than.";
          };
        };
        default = { };
      };
      hypr = mkOption {
        type = types.submodule {
          options = {
            keyboardLayouts = strOption;
            keyboardToggle = strOption;
            windowOpacity = strOption;
          };
        };
      };
      display = mkOption {
        type = types.submodule {
          options = {
            monitorName = strOption;
            monitorMode = strOption;
            monitorPosition = strOption;
            monitorScale = strOption;
            extraMonitors = mkOption {
              type = types.listOf (
                types.submodule {
                  options = {
                    name = strOption;
                    mode = mkOption {
                      type = types.str;
                      default = "preferred";
                    };
                    position = mkOption {
                      type = types.str;
                      default = "auto";
                    };
                    scale = mkOption {
                      type = types.str;
                      default = "1";
                    };
                  };
                }
              );
              default = [ ];
              description = "Additional Hyprland outputs beyond the primary monitor.";
            };
          };
        };
      };
      wallpapers = mkOption {
        type = types.submodule {
          options.enable = boolOption false;
        };
      };
    };
  };
in
{
  options.mysetup = mkOption {
    type = mysetupType;
    default = { };
    description = "Canonical MySetup host/user configuration.";
  };
}
