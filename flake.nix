{
  description = "Reusable MySetup shell modules and installer entrypoint";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:NixOS/nixpkgs/nixos-26.05";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    caelestia-shell = {
      url = "github:caelestia-dots/shell";
      inputs = {
        caelestia-cli.follows = "caelestia-cli";
        nixpkgs.follows = "nixpkgs";
        quickshell.follows = "quickshell";
      };
    };
    caelestia-cli = {
      url = "github:caelestia-dots/cli";
      inputs = {
        caelestia-shell.follows = "caelestia-shell";
        nixpkgs.follows = "nixpkgs";
      };
    };
    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell/v4.7.7";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    end4-dotfiles = {
      url = "git+https://github.com/end-4/dots-hyprland?submodules=1";
      flake = false;
    };

    zen-browser = {
      url = "github:youwen5/zen-browser-flake";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    claude-code = {
      url = "github:sadjow/claude-code-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    codex = {
      url = "github:sadjow/codex-cli-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    neovim-nightly-overlay = {
      url = "github:nix-community/neovim-nightly-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    templ = {
      url = "github:a-h/templ";
      inputs = {
        nixpkgs.follows = "nixpkgs-stable";
        nixpkgs-unstable.follows = "nixpkgs";
      };
    };

    lanzaboote = {
      url = "github:nix-community/lanzaboote";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nixos-hardware.url = "github:NixOS/nixos-hardware/master";
    nix-snapd = {
      url = "github:nix-community/nix-snapd";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    sops-nix = {
      url = "github:Mic92/sops-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nix-index-database = {
      url = "github:nix-community/nix-index-database";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    zapret-discord-youtube = {
      url = "github:kartavkun/zapret-discord-youtube";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    stylix = {
      url = "github:danth/stylix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      home-manager,
      ...
    }@inputs:
    let
      inherit (nixpkgs) lib;
      systems = [ "x86_64-linux" ];
      forSystems = lib.genAttrs systems;

      layout = import ./Linux/NixOS/lib/layout.nix {
        nixosRoot = ./Linux/NixOS;
      };

      mysetupLib = import ./Linux/NixOS/lib/mysetup.nix {
        inherit lib;
      };

      shellOverlays = import ./Linux/NixOS/lib/shell-overlays.nix {
        inherit inputs;
      };

      shellHomeModule =
        {
          config,
          lib,
          mysetup,
          pkgs,
          ...
        }:
        {
          _module.args.homeLibs = import ./Linux/NixOS/home/lib {
            inherit lib pkgs;
          };

          imports = [
            inputs.caelestia-shell.homeManagerModules.default
            inputs.noctalia-shell.homeModules.default
            ./Linux/NixOS/home/shells
            ./Linux/NixOS/home/caelestia
            ./Linux/NixOS/home/noctalia
            ./Linux/NixOS/home/end4
          ];

          home = {
            username = lib.mkDefault mysetup.user.username;
            homeDirectory = lib.mkDefault mysetup.user.homeDirectory;
            stateVersion = lib.mkDefault mysetup.host.stateVersion;
          };

          programs.waybar.enable = lib.mkForce false;
        };

      shellsNixosModule =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        {
          imports = [
            ./Linux/NixOS/modules/mysetup-options.nix
            home-manager.nixosModules.home-manager
          ];

          mysetup = {
            host = {
              hostname = lib.mkDefault config.networking.hostName;
              stateVersion = lib.mkDefault config.system.stateVersion;
              configDirectory = lib.mkDefault "/etc/nixos";
            };
            locale = {
              timeZone = lib.mkDefault "UTC";
              defaultLocale = lib.mkDefault "en_US.UTF-8";
              extraLocale = lib.mkDefault "en_US.UTF-8";
              consoleKeyMap = lib.mkDefault "us";
              weatherLocation = lib.mkDefault "";
            };
            git = {
              username = lib.mkDefault config.mysetup.user.username;
              email = lib.mkDefault "";
            };
            packages.preset = lib.mkDefault "desktop";
            hardware.gpu = lib.mkDefault "other";
            features = {
              secureBoot = lib.mkDefault false;
              ctfTools = lib.mkDefault false;
              omnirouter = lib.mkDefault false;
              observability = lib.mkDefault false;
            };
            zapret = {
              enable = lib.mkDefault false;
              config = lib.mkDefault "";
            };
            hypr = {
              keyboardLayouts = lib.mkDefault "us";
              keyboardToggle = lib.mkDefault "";
              windowOpacity = lib.mkDefault "1.0";
            };
            display = {
              monitorName = lib.mkDefault "";
              monitorMode = lib.mkDefault "preferred";
              monitorPosition = lib.mkDefault "auto";
              monitorScale = lib.mkDefault "1";
            };
            wallpapers.enable = lib.mkDefault false;
          };

          nixpkgs.overlays = [
            shellOverlays.shellPackagesOverlay
            shellOverlays.valkeyNoCheckOverlay
          ];

          programs.hyprland = {
            enable = lib.mkDefault true;
            xwayland.enable = lib.mkDefault true;
          };

          services.dbus.enable = lib.mkDefault true;

          users.users.${config.mysetup.user.username} = {
            isNormalUser = lib.mkDefault true;
            description = lib.mkDefault config.mysetup.user.fullName;
            home = lib.mkDefault config.mysetup.user.homeDirectory;
          };

          xdg.portal = {
            enable = lib.mkDefault true;
            xdgOpenUsePortal = lib.mkDefault true;
            extraPortals = with pkgs; [
              xdg-desktop-portal-gtk
              xdg-desktop-portal-hyprland
            ];
            config = {
              common.default = lib.mkDefault [ "gtk" ];
              hyprland.default = lib.mkDefault [
                "hyprland"
                "gtk"
              ];
            };
          };

          home-manager = {
            useGlobalPkgs = lib.mkDefault true;
            useUserPackages = lib.mkDefault true;
            backupFileExtension = lib.mkDefault "backup";
            overwriteBackup = lib.mkDefault true;
            users.${config.mysetup.user.username} = shellHomeModule;
            extraSpecialArgs = {
              inherit inputs mysetupLib;
              inherit (config) mysetup;
            };
          };
        };

      installerOutputsFor =
        system:
        import ./Linux/NixOS/lib/flake-packages.nix {
          inherit layout nixpkgs system;
        };

      mysetupPackageFor = system: (installerOutputsFor system).packages.mysetup;
      omnirouterPackageFor = system: (installerOutputsFor system).packages.omnirouter;
      mysetupAppFor = system: {
        type = "app";
        program = "${mysetupPackageFor system}/bin/mysetup";
        meta.description = "Run the MySetup NixOS installer";
      };

      linuxNixosOutputs = (import ./Linux/NixOS/flake.nix).outputs inputs;

      shellModuleCheckFor =
        system:
        let
          nixos = lib.nixosSystem {
            inherit system;
            modules = [
              shellsNixosModule
              (
                { ... }:
                {
                  system.stateVersion = "26.05";
                  mysetup.user = {
                    username = "alice";
                    fullName = "Alice";
                    homeDirectory = "/home/alice";
                  };
                }
              )
            ];
          };
        in
        nixos.config.home-manager.users.alice.home.activationPackage;
    in
    {
      overlays = rec {
        shells = shellOverlays.shellPackagesOverlay;
        default = shells;
        valkeyNoCheck = shellOverlays.valkeyNoCheckOverlay;
      };

      homeManagerModules = rec {
        shells = shellHomeModule;
        default = shells;
      };

      nixosModules = rec {
        mysetup = linuxNixosOutputs.nixosModules.mysetup;
        workstation = mysetup;
        shells = shellsNixosModule;
        default = shells;
      };

      lib = (linuxNixosOutputs.lib or { }) // {
        inherit mysetupLib;
      };

      packages = forSystems (system: {
        mysetup = mysetupPackageFor system;
        omnirouter = omnirouterPackageFor system;
        default = self.packages.${system}.mysetup;
      });

      apps = forSystems (system: {
        mysetup = mysetupAppFor system;
        default = self.apps.${system}.mysetup;
      });

      checks = forSystems (system: {
        mysetup = self.packages.${system}.mysetup;
        "shells-module" = shellModuleCheckFor system;
      });

      formatter = forSystems (system: nixpkgs.legacyPackages.${system}.nixfmt);
    };
}
