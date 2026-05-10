{
  config,
  lib,
  mysetupLib,
  pkgs,
  ...
}:

let
  packageSets = import ../lib/package-sets.nix { inherit lib pkgs; };
in
{
  config = mysetupLib.mkIfPresetOrMore "developer" config.mysetup {
    environment.systemPackages = packageSets.development.tools;

    environment.variables = {
      PLAYWRIGHT_BROWSERS_PATH = "${pkgs.playwright-driver.browsers}";
      PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS = "true";
    };
  };
}
