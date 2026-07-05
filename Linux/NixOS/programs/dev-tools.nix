{
  config,
  lib,
  mysetupLib,
  pkgs,
  ...
}:

let
  cfg = config.mysetup;
  packageSets = import ../lib/package-sets.nix { inherit lib pkgs; };
in
{
  config = mysetupLib.mkIfPresetOrMore "developer" cfg {
    environment.systemPackages =
      packageSets.development.tools
      ++ lib.optionals (mysetupLib.presets.personal cfg) packageSets.development.personalTools;

    environment.variables = {
      PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
      PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
    };
  };
}
