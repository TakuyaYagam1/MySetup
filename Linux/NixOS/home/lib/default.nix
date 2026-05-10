{ lib, pkgs }:

let
  dotfiles = import ./dotfiles.nix { inherit lib pkgs; };
in
{
  inherit dotfiles;
  qtRuntime.mkSearchPath =
    paths: packages: lib.concatStringsSep ":" (map (path: lib.makeSearchPath path packages) paths);
  qtDefaults.platformTheme = "qt6ct";
  transparency = import ./transparency-defaults.nix;
  shellSeed = import ./shell-seed.nix {
    inherit lib pkgs;
    dotfilesLib = dotfiles;
  };
  shellSelectorOptions = import ./shell-selector-options.nix { inherit lib pkgs; };
}
