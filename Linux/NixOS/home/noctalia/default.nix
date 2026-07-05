{
  inputs,
  lib,
  pkgs,
  ...
}:

let
  noctaliaPackage = inputs.noctalia.packages.${pkgs.stdenv.hostPlatform.system}.default;
  noctaliaProgramConfig = {
    enable = true;
    package = noctaliaPackage;
    systemd.enable = false;
    validateConfig = true;
    settings = {
      shell = {
        setup_wizard_enabled = false;
        telemetry_enabled = false;
        panel = {
          transparency_mode = "soft";
        };
      };
      theme = {
        mode = "dark";
      };
    };
    customPalettes = lib.mkForce { };
  };
in
{
  programs.noctalia = noctaliaProgramConfig;
}
