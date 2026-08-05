{
  description = "Wahrwelt minimal NixOS preset";

  # Keep this literal for Nix flake input discovery. The canonical definitions
  # live in ../../lib/preset-inputs.nix and are checked for drift.
  inputs = {
    # Temporary compatibility pin: Hyprland 0.56.1 requires glaze < 8.
    nixpkgs.url = "github:NixOS/nixpkgs/643809054d65fdd466a63e3155b8c498cb483c04";
    nixpkgs-stable.url = "github:NixOS/nixpkgs/nixos-26.05";
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    neovim-nightly-overlay = {
      # Temporary pin: the next nightly fails its upstream functional tests.
      url = "github:nix-community/neovim-nightly-overlay/5522fc3be8969569a980f3d14b86600a55e713fc";
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
    stylix = {
      url = "github:danth/stylix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs:
    import ../../lib/preset-flake.nix {
      inherit inputs;
      preset = "minimal";
      nixosRoot = ../..;
    };
}
