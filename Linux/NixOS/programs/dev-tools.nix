{
  config,
  lib,
  wahrweltLib,
  pkgs,
  ...
}:

let
  cfg = config.wahrwelt;
  packageSets = import ../lib/package-sets.nix { inherit lib pkgs; };
in
{
  config = wahrweltLib.mkIfPresetOrMore "developer" cfg {
    environment.systemPackages =
      packageSets.development.tools
      ++ lib.optionals (wahrweltLib.presets.personal cfg) packageSets.development.personalTools;

    environment.variables = {
      PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
      PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
    };
  };
}
