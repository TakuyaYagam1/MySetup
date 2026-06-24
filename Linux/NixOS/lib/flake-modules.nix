{
  inputs,
  mysetupLib,
  overlays,
  pkgs-stable,
}:

{
  mysetupModule = import ../modules/mysetup-stack.nix;

  overlaysModule = _: {
    nixpkgs.overlays = [
      overlays.flakePackagesOverlay
      overlays.valkeyNoCheckOverlay
      overlays.omnirouterFromMySetupOverlay
      overlays.pipxTestCompatibilityOverlay
      overlays.openblasI686NoCheckOverlay
    ];
  };

  homeManagerModule = import ./flake-home-module.nix {
    inherit
      inputs
      mysetupLib
      pkgs-stable
      ;
  };
}
