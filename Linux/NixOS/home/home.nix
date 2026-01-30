{ config, pkgs, inputs, lib, ... }:

{
  imports = [
    inputs.caelestia-shell.homeManagerModules.default
    ./shell.nix
    ./desktop.nix
    ./apps.nix
    ./dev-packages.nix
  ];

  home.username = "takuya";
  home.homeDirectory = "/home/takuya";
  home.stateVersion = "25.11";

  programs.neovim = {
    enable = true;
    package = inputs.neovim-nightly-overlay.packages.${pkgs.stdenv.hostPlatform.system}.default;
  };

  programs.waybar.enable = lib.mkForce false;

  home.activation.copyWallpapers = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    WALLS_SRC="${./../Wallpapers}"
    WALLS_DST="${config.home.homeDirectory}/Pictures/Wallpapers"

    $DRY_RUN_CMD mkdir -p "$WALLS_DST"
    if [ -d "$WALLS_SRC" ]; then
      cp -n "$WALLS_SRC"/* "$WALLS_DST" 2>/dev/null || true
    fi
  '';
}
