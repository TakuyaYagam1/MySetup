{
  description = "NixOS + Caelestia-dots + meowrch themes + Flatpak + Snap + Zapret (Kartavkun)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    nixpkgs-stable.url = "github:NixOS/nixpkgs/nixos-25.11";

    # Qt escape-hatch: tracks nixos-unstable-small (Hydra-tested, days ahead of
    # nixos-unstable). When upstream requires a newer Qt than unstable provides,
    # flip qtOverride.enable = true and run: nix flake update nixpkgs-bleeding
    # Flip back to false once nixos-unstable catches up.
    nixpkgs-bleeding.url = "github:NixOS/nixpkgs/nixos-unstable-small";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    caelestia-shell = {
      url = "github:caelestia-dots/shell";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.quickshell.follows = "quickshell";
    };

    caelestia-cli = {
      url = "github:caelestia-dots/cli";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    templ.url = "github:a-h/templ";

    nix-snapd = {
      url = "github:nix-community/nix-snapd";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    neovim-nightly-overlay = {
      url = "github:nix-community/neovim-nightly-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # Zapret for russian users
    # Based on
    # https://github.com/kartavkun/zapret-discord-youtube
    # https://github.com/bol-van/zapret
    zapret-discord-youtube = {
      url = "github:kartavkun/zapret-discord-youtube";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # For users who using secure boot
    lanzaboote = {
      url = "github:nix-community/lanzaboote";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    zen-browser = {
      url = "github:youwen5/zen-browser-flake";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    claude-code = {
      url = "github:sadjow/claude-code-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-stable, nixpkgs-bleeding, home-manager, nix-snapd, zapret-discord-youtube, lanzaboote, templ, zen-browser, ... }@inputs:
  let
    system = "x86_64-linux";

    # Set to true ONLY when upstream projects require a newer Qt than
    # nixos-unstable currently provides. Flip back to false once unstable
    # catches up - no other changes needed.
    qtOverride.enable = false;

    pkgs-stable = import nixpkgs-stable {
      localSystem = system;
      config.allowUnfree = true;
      config.permittedInsecurePackages = [
        "python3.12-pypdf2-3.0.1"
      ];
    };

    pkgs-bleeding = import nixpkgs-bleeding {
      localSystem = system;
      config.allowUnfree = true;
    };

    # Atomically swaps the entire Qt scope when the hatch is active.
    # Always replace full scopes, never individual packages - partial
    # overrides cause ABI mismatches in QML plugins and kdePackages.
    qtBleedingOverlay = final: prev:
      if !qtOverride.enable then { } else {
        qt6         = pkgs-bleeding.qt6;
        qt6Packages = pkgs-bleeding.qt6Packages;
        kdePackages = pkgs-bleeding.kdePackages;
      };
  in {
    nixosConfigurations.NixOS = nixpkgs.lib.nixosSystem {
      inherit system;
      specialArgs = { inherit inputs pkgs-stable pkgs-bleeding; };

      modules = [

        ./configuration.nix

        ({ pkgs, ... }: {
          nixpkgs.overlays = [
            (final: prev: {
              valkey = prev.valkey.overrideAttrs (_: { doCheck = false; });
            })
            qtBleedingOverlay
          ];

          environment.systemPackages = with pkgs; [
            inputs.templ.packages.${pkgs.stdenv.hostPlatform.system}.templ
          ];
        })

        inputs.lanzaboote.nixosModules.lanzaboote

        zapret-discord-youtube.nixosModules.default

        inputs.nix-snapd.nixosModules.default

        home-manager.nixosModules.home-manager
        {
          home-manager = {
            useGlobalPkgs = true;
            useUserPackages = true;
            backupFileExtension = "hm-backup";
            users.takuya = import ./home/home.nix;
            extraSpecialArgs = { inherit inputs pkgs-stable pkgs-bleeding; };
          };
        }

        ({ pkgs, ... }: {
          services.flatpak.enable = true;
          services.snap.enable = true;
        })

        ./programs/xdg-portal.nix
      ];
    };
  };
}
