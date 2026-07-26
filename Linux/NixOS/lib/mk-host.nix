{
  home-manager,
  inputs,
  wahrweltLib,
  nixpkgs,
  nixpkgs-stable,
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

  pkgs-stable = import nixpkgs-stable {
    localSystem = system;
    config = {
      allowUnfree = true;
      allowInsecurePredicate = _: true;
    };
  };

  overlays = import ./flake-overlays.nix {
    inherit inputs system;
  };

  flakeModules = import ./flake-modules.nix {
    inherit
      inputs
      wahrweltLib
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
      wahrwelt = hostVarsValue;
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
      home-manager.users.${config.wahrwelt.user.username}.imports = homeExtraModules;
    };
in
nixpkgs.lib.nixosSystem {
  inherit system;

  specialArgs = {
    inherit
      inputs
      wahrweltLib
      pkgs-stable
      ;
    mysetupLib = wahrweltLib;
  };

  modules = [
    flakeModules.wahrweltModule
    hostModule
  ]
  ++ optionalPath hardware
  ++ optionalPath hashedPassword
  ++ [
    flakeModules.overlaysModule
    extraOverlaysModule

    inputs.lanzaboote.nixosModules.lanzaboote
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
