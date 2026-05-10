import ../../../lib/mk-home-packages-module.nix {
  preset = "minimalOrMore";
  customSelector = { packageSets, ... }: packageSets.runtime.waylandTools;
}
