{ lib }:

let
  presets = import ./presets.nix { inherit lib; };
  ports = import ./service-ports.nix;
  bootTheme = import ./boot-theme.nix;
in
{
  inherit presets ports bootTheme;

  mkIfPresetOrMore =
    preset: cfg: body:
    lib.mkIf (presets.atLeast preset cfg) body;

  defaults = {
    dns = [
      "8.8.8.8"
      "1.1.1.1"
      "77.88.8.8"
    ];
  };
}
