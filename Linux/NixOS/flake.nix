{
  description = "NixOS + Caelestia-dots + meowrch themes + Flatpak + Snap + Zapret (Kartavkun)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    
    nixpkgs-stable.url = "github:NixOS/nixpkgs/nixos-25.11";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    caelestia-shell = {
      url = "github:caelestia-dots/shell";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    caelestia-cli = {
      url = "github:caelestia-dots/cli";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    antigravity = {
      url = "github:jacopone/antigravity-nix";
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
  };

  outputs = { self, nixpkgs, nixpkgs-stable, home-manager, nix-snapd, zapret-discord-youtube, lanzaboote, templ, ... }@inputs:
  let
    system = "x86_64-linux";
    
    pkgs-stable = import nixpkgs-stable {
      localSystem = system;
      config.allowUnfree = true;
    };
  in {
    nixosConfigurations.NixOS = nixpkgs.lib.nixosSystem {
      specialArgs = { inherit inputs pkgs-stable; };

      modules = [
        { nixpkgs.hostPlatform = system; }

        ./configuration.nix

        ({ pkgs, ... }: {
          nixpkgs.overlays = [ 
            inputs.antigravity.overlays.default
            
            (final: prev: {
              vmware-workstation = pkgs-stable.vmware-workstation;
            })
          ];

          environment.systemPackages = with pkgs; [
            inputs.templ.packages.${system}.templ
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
            users.takuya = import ./home/home.nix;
            extraSpecialArgs = { inherit inputs system pkgs-stable; };
          };
        }

        ({ pkgs, ... }: {
          services.flatpak.enable = true;   
          services.snap.enable = true;

          xdg.portal = {
            enable = true;
            extraPortals = with pkgs; [ xdg-desktop-portal-gtk ];
            config.common.default = "*";
          };
        })
      ];
    };
  };
}
