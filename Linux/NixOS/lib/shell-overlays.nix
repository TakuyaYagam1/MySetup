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
      noctalia = inputs.noctalia.packages.${system}.default;
      quickshell = inputs.quickshell.packages.${system}.default;
    };

  shellPackagesOverlay =
    _final: prev:
    let
      shellPackages = shellPackagesFor { inherit prev; };
      wahrweltPackages = (prev.wahrwelt or (prev.mysetup or { })) // shellPackages;
    in
    shellPackages
    // {
      wahrwelt = wahrweltPackages;
      mysetup = wahrweltPackages;
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
