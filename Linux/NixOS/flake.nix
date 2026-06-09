{
  description = "NixOS + Caelestia-dots + meowrch themes + Flatpak + Snap + Zapret (Kartavkun)";

  inputs = {
    # Core
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    nixpkgs-stable.url = "github:NixOS/nixpkgs/nixos-26.05";
    nixpkgs-bleeding.url = "github:NixOS/nixpkgs/nixos-unstable-small";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # Shells
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
      inputs.nixpkgs.follows = "nixpkgs";
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

    # Apps & editors
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
    burpsuitepro = {
      type = "github";
      owner = "xiv3r";
      repo = "Burpsuite-Professional";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    templ = {
      url = "github:a-h/templ";
      inputs = {
        nixpkgs.follows = "nixpkgs-stable";
        nixpkgs-unstable.follows = "nixpkgs";
      };
    };

    # System & security
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
    zapret-discord-youtube = {
      url = "github:kartavkun/zapret-discord-youtube";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # Theming
    stylix = {
      url = "github:danth/stylix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-stable,
      nixpkgs-bleeding,
      home-manager,
      zapret-discord-youtube,
      ...
    }@inputs:
    let
      system = "x86_64-linux";
      layout = import ./lib/layout.nix {
        nixosRoot = ./.;
      };

      # Toggle to true when shells need Qt newer than nixos-unstable provides.
      enableQtBleeding = false;

      inputsForModules = inputs // {
        mysetup = self;
      };

      mysetupLib = import ./lib/mysetup.nix {
        inherit (nixpkgs) lib;
      };

      mkSystemContext =
        system:
        let
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

          overlays = import ./lib/flake-overlays.nix {
            inputs = inputsForModules;
            inherit pkgs-bleeding system;
          };

          flakeModules = import ./lib/flake-modules.nix {
            inherit
              enableQtBleeding
              mysetupLib
              overlays
              pkgs-bleeding
              pkgs-stable
              ;
            inputs = inputsForModules;
          };
        in
        {
          inherit
            flakeModules
            overlays
            pkgs-bleeding
            pkgs-stable
            ;
        };

      defaultContext = mkSystemContext system;

      mkMySetupHost = import ./lib/mk-host.nix {
        inherit
          enableQtBleeding
          home-manager
          mysetupLib
          nixpkgs
          nixpkgs-bleeding
          nixpkgs-stable
          zapret-discord-youtube
          ;
        inputs = inputsForModules;
      };

      flakeOutputs = import ./lib/flake-packages.nix {
        inherit layout nixpkgs system;
      };

      hosts = import ./lib/hosts.nix {
        inherit mkMySetupHost system;
      };
    in
    {
      lib = {
        inherit mkMySetupHost;
        mysetup = mysetupLib;
      };

      nixosModules = rec {
        mysetup = defaultContext.flakeModules.mysetupModule;
        default = mysetup;
      };

      packages.${system} = flakeOutputs.packages;
      checks.${system} = flakeOutputs.checks;
      apps.${system} = flakeOutputs.apps;
      formatter.${system} = nixpkgs.legacyPackages.${system}.nixfmt;
    }
    // hosts;
}
