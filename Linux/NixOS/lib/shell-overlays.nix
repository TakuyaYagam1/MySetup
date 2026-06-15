{ inputs }:

let
  shellPackagesFor =
    {
      prev,
      system ? prev.stdenv.hostPlatform.system,
    }:
    {
      caelestia-cli = inputs.caelestia-cli.packages.${system}.default;
      caelestia-shell = inputs.caelestia-shell.packages.${system}.with-cli;
      noctalia-shell = inputs.noctalia-shell.packages.${system}.default;
      quickshell = inputs.quickshell.packages.${system}.default;
    };

  shellPackagesOverlay =
    _final: prev:
    let
      shellPackages = shellPackagesFor { inherit prev; };
    in
    shellPackages
    // {
      mysetup = (prev.mysetup or { }) // shellPackages;
    };

  valkeyNoCheckOverlay = _final: prev: {
    valkey = prev.valkey.overrideAttrs (_: {
      # Noctalia pulls Valkey through its graph; upstream checks are flaky on this pinned nixpkgs.
      doCheck = false;
    });
  };
in
{
  inherit
    shellPackagesFor
    shellPackagesOverlay
    valkeyNoCheckOverlay
    ;
}
