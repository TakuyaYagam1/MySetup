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

    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    end4-dotfiles = {
      url = "git+https://github.com/end-4/dots-hyprland?submodules=1";
      flake = false;
    };

    quickshell-end4 = {
      url = "github:quickshell-mirror/quickshell/db1777c20b936a86528c1095cbcb1ebd92801402";
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

    codex = {
      url = "github:sadjow/codex-cli-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    stylix = {
      url = "github:danth/stylix";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    nixos-hardware.url = "github:NixOS/nixos-hardware/master";

    sops-nix = {
      url = "github:Mic92/sops-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-stable, nixpkgs-bleeding, home-manager, nix-snapd, zapret-discord-youtube, lanzaboote, templ, zen-browser, stylix, sops-nix, ... }@inputs:
  let
    system = "x86_64-linux";

    # Single source of truth: `hostname` flake attr name == config.var.hostname.
    # Read the literal string out of variables.nix so `nixos-rebuild switch`
    # (which derives the attr from `networking.hostName` by default) finds it.
    hostname =
      let
        vars = import ./hosts/NixOS/variables.nix {
          config = { }; lib = nixpkgs.lib;
        };
      in
        vars.config.var.hostname;

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
    qtBleedingOverlay = final: prev: {
      qt6         = pkgs-bleeding.qt6;
      qt6Packages = pkgs-bleeding.qt6Packages;
      kdePackages = pkgs-bleeding.kdePackages;
    };

    # Surface flake-input packages as canonical `pkgs.<name>` attributes so the
    # rest of the config always consumes the flake-backed package instead of the
    # nixpkgs variant.
    flakePackagesOverlay =
      final: prev:
      {
        caelestia-cli   = inputs.caelestia-cli.packages.${system}.default;
        caelestia-shell = inputs.caelestia-shell.packages.${system}.default;
        claude-code     = inputs.claude-code.packages.${system}.default;
        codex           = inputs.codex.packages.${system}.default;
        zen-browser     = inputs.zen-browser.packages.${system}.default;
        quickshell      = inputs.quickshell.packages.${system}.default.override {
          libxcb = prev.libxcb;
        };
        templ           = inputs.templ.packages.${system}.templ;
        neovim          = inputs.neovim-nightly-overlay.packages.${system}.default;
      };
  in {
    packages.${system} =
      let
        flakePkgs = import nixpkgs {
          localSystem = system;
          config.allowUnfree = true;
        };
        mysetup = flakePkgs.buildGoModule {
          pname = "mysetup";
          version = "0.1.0";
          src = if builtins.pathExists ./installer then ./installer else ../installer;
          subPackages = [ "cmd/mysetup" ];
          vendorHash = "sha256-3BLXjtDy2dsq7A12BmAkoOQbu/hYkhVm4GKCtqYglTo=";
          nativeBuildInputs = [ flakePkgs.makeWrapper ];
          ldflags = [
            "-s"
            "-w"
          ];
          postInstall = ''
            wrapProgram $out/bin/mysetup \
              --prefix PATH : ${flakePkgs.lib.makeBinPath (with flakePkgs; [
                coreutils
                findutils
                gnused
                rsync
                mkpasswd
                nix
                nixos-rebuild
                git
                curl
                jq
                hyprland
                libarchive
                unzip
                sing-box
              ])}
          '';
        };
      in
      {
        omnirouter = flakePkgs.callPackage ./packages/omnirouter.nix { };
        inherit mysetup;
        default = mysetup;
      };

    apps.${system} = {
      mysetup = {
        type = "app";
        program = "${self.packages.${system}.mysetup}/bin/mysetup";
      };
      default = self.apps.${system}.mysetup;
    };

    nixosConfigurations.${hostname} = nixpkgs.lib.nixosSystem {
      inherit system;
      specialArgs = { inherit inputs pkgs-stable pkgs-bleeding; };

      modules = [

        ./hosts/NixOS

        # nixos-hardware: enable the module matching your machine.
        # Browse available modules: https://github.com/NixOS/nixos-hardware
        # Example (uncomment + replace with your model):
        # inputs.nixos-hardware.nixosModules.common-cpu-amd
        # inputs.nixos-hardware.nixosModules.common-gpu-nvidia-nonprime
        # inputs.nixos-hardware.nixosModules.common-pc-ssd

        ({ pkgs, lib, ... }: {
          nixpkgs.overlays = [
            flakePackagesOverlay
            (final: prev: {
              valkey = prev.valkey.overrideAttrs (_: { doCheck = false; });
            })
          ] ++ lib.optional qtOverride.enable qtBleedingOverlay;

          environment.systemPackages = with pkgs; [
            templ
          ];
        })

        inputs.lanzaboote.nixosModules.lanzaboote

        zapret-discord-youtube.nixosModules.default

        inputs.nix-snapd.nixosModules.default

        inputs.stylix.nixosModules.stylix
        inputs.sops-nix.nixosModules.sops

        home-manager.nixosModules.home-manager
        ({ config, ... }: {
          home-manager = {
            useGlobalPkgs = true;
            useUserPackages = true;
            backupFileExtension = "hm-backup";
            users.${config.var.username} = import ./home/home.nix;
            extraSpecialArgs = {
              inherit inputs pkgs-stable pkgs-bleeding;
              var = config.var;
            };
          };
        })

        ({ pkgs, ... }: {
          services.flatpak.enable = true;
          services.snap.enable = true;
        })

        ./programs/xdg-portal.nix
      ];
    };
  };
}
