{
  home-manager,
  inputs,
  mysetupLib,
  nixpkgs,
  nixpkgs-stable,
  zapret-discord-youtube,
}:

{
  system ? "x86_64-linux",
  hostname ? "NixOS",
  hostVars,
  hardware ? null,
  hashedPassword ? null,
  secretsDir ? null,
  extraModules ? [ ],
  homeExtraModules ? [ ],
  extraOverlays ? [ ],
}:

let
  inherit (nixpkgs) lib;

  overlays = import ./flake-overlays.nix {
    inherit inputs system;
  };

  pkgs-stable = import nixpkgs-stable {
    localSystem = system;
    config.allowUnfree = true;
    overlays = [ overlays.openblasI686NoCheckOverlay ];
  };

  flakeModules = import ./flake-modules.nix {
    inherit
      inputs
      mysetupLib
      overlays
      pkgs-stable
      ;
  };

  hostVarsValue = if builtins.isAttrs hostVars then hostVars else import hostVars;
  optionalPath = path: lib.optional (path != null && builtins.pathExists path) path;
  secretsFile = if secretsDir == null then null else secretsDir + "/secrets.yaml";

  hostModule =
    { lib, ... }:
    {
      networking.hostName = lib.mkDefault hostname;
      mysetup = hostVarsValue;
    };

  extraOverlaysModule = {
    nixpkgs.overlays = extraOverlays;
  };

  sopsModule =
    { lib, ... }:
    lib.mkIf (secretsFile != null && builtins.pathExists secretsFile) {
      sops = {
        defaultSopsFile = secretsFile;
        defaultSopsFormat = "yaml";
        age.sshKeyPaths = [ "/etc/ssh/ssh_host_ed25519_key" ];
      };
    };

  homeExtraModule =
    { config, lib, ... }:
    lib.mkIf (homeExtraModules != [ ]) {
      home-manager.users.${config.mysetup.user.username}.imports = homeExtraModules;
    };
in
nixpkgs.lib.nixosSystem {
  inherit system;

  specialArgs = {
    inherit
      inputs
      mysetupLib
      pkgs-stable
      ;
  };

  modules = [
    flakeModules.mysetupModule
    hostModule
  ]
  ++ optionalPath hardware
  ++ optionalPath hashedPassword
  ++ [
    flakeModules.overlaysModule
    extraOverlaysModule

    inputs.lanzaboote.nixosModules.lanzaboote
    zapret-discord-youtube.nixosModules.default
    inputs.nix-snapd.nixosModules.default
    inputs.stylix.nixosModules.stylix
    inputs.sops-nix.nixosModules.sops
    sopsModule

    home-manager.nixosModules.home-manager
    flakeModules.homeManagerModule
    homeExtraModule
  ]
  ++ extraModules;
}
