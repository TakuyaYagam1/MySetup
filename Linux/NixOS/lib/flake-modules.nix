{
  enableQtBleeding,
  inputs,
  mysetupLib,
  overlays,
  pkgs-bleeding,
  pkgs-stable,
}:

{
  overlaysModule =
    { lib, ... }:
    {
      nixpkgs.overlays = [
        overlays.flakePackagesOverlay
        overlays.valkeyNoCheckOverlay
      ]
      ++ lib.optional enableQtBleeding overlays.qtBleedingOverlay;
    };

  homeManagerModule = import ./flake-home-module.nix {
    inherit
      inputs
      mysetupLib
      pkgs-bleeding
      pkgs-stable
      ;
  };
}
