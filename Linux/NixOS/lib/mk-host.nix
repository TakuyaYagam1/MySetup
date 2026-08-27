{
  home-manager,
  inputs,
  wahrweltLib,
  nixpkgs,
  nixpkgs-stable,
  preset ? "full",
}:

{
  system ? "x86_64-linux",
  hostname ? "NixOS",
  hostVars,
  hardware ? null,
  hashedPasswordFile ? null,
  secretsDir ? null,
  extraModules ? [ ],
  homeExtraModules ? [ ],
  extraOverlays ? [ ],
  hostInputs ? { },
}:

let
  inherit (nixpkgs) lib;

  supportedHostInputNames = [ "lanzaboote" ];
  unsupportedHostInputNames = builtins.filter (name: !(builtins.elem name supportedHostInputNames)) (
    builtins.attrNames hostInputs
  );
  effectiveInputs =
    assert lib.assertMsg (unsupportedHostInputNames == [ ])
      "Wahrwelt received unsupported host inputs: ${builtins.concatStringsSep ", " unsupportedHostInputNames}";
    inputs // lib.optionalAttrs (hostInputs ? lanzaboote) { inherit (hostInputs) lanzaboote; };

  pkgs-stable = import nixpkgs-stable {
    localSystem = system;
    config = {
      allowUnfree = true;
      permittedInsecurePackages = import ./permitted-insecure-packages.nix;
    };
  };

  overlays = import ./flake-overlays.nix {
    inputs = effectiveInputs;
    inherit preset system;
  };

  flakeModules = import ./flake-modules.nix {
    inherit
      wahrweltLib
      overlays
      pkgs-stable
      ;
    inputs = effectiveInputs;
  };

  hostVarsValue = if builtins.isAttrs hostVars then hostVars else import hostVars;
  secureBoot = hostVarsValue.features.secureBoot or false;
  personalOrFull = builtins.elem preset [
    "personal"
    "full"
  ];
  optionalPath = path: lib.optional (path != null && builtins.pathExists path) path;
  secretsFile = if secretsDir == null then null else secretsDir + "/secrets.yaml";
  lanzabooteModules =
    if !secureBoot then
      [ ]
    else if effectiveInputs ? lanzaboote then
      [
        effectiveInputs.lanzaboote.nixosModules.lanzaboote
        ../system/boot/secure.nix
      ]
    else
      throw "Wahrwelt Secure Boot requires the host-owned lanzaboote input";

  happModules = lib.optionals personalOrFull [
    (effectiveInputs.happ-nix + "/happ-module.nix")
    (
      { pkgs, ... }:
      {
        programs.happ = {
          enable = true;
          package = pkgs.happ;
          tunMode.enable = true;
        };
      }
    )
  ];

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

  passwordModule =
    { config, ... }:
    lib.mkIf (hashedPasswordFile != null) {
      users.users.${config.wahrwelt.user.username}.hashedPasswordFile = hashedPasswordFile;
    };
in
nixpkgs.lib.nixosSystem {
  inherit system;

  specialArgs = {
    inherit
      wahrweltLib
      pkgs-stable
      ;
    inputs = effectiveInputs;
    mysetupLib = wahrweltLib;
  };

  modules = [
    flakeModules.wahrweltModule
    hostModule
  ]
  ++ optionalPath hardware
  ++ lib.optional (hashedPasswordFile != null) passwordModule
  ++ [
    flakeModules.overlaysModule
    extraOverlaysModule

  ]
  ++ lanzabooteModules
  ++ happModules
  ++ [
    effectiveInputs.nix-snapd.nixosModules.default
    effectiveInputs.stylix.nixosModules.stylix
    effectiveInputs.sops-nix.nixosModules.sops
    sopsModule

    home-manager.nixosModules.home-manager
    flakeModules.homeManagerModule
    homeExtraModule
  ]
  ++ extraModules;
}
