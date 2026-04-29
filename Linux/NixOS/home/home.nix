{ config, pkgs, inputs, lib, var, ... }:

{
  imports = [
    inputs.caelestia-shell.homeManagerModules.default
    inputs.stylix.homeModules.stylix
    ../themes/active.nix
    ./caelestia
    # ./secrets       # HM-level sops; bootstrap: see home/secrets/default.nix
    ./theming.nix
    ./apps.nix
    ./dev-packages.nix
    ./programs/git.nix
    ./programs/fish.nix
    ./programs/foot.nix
    ./programs/btop.nix
    ./programs/starship.nix
    ./programs/cava.nix
    ./programs/fastfetch.nix
    ./programs/nixos-helper.nix
    ./programs/thunar.nix
    ./programs/uwsm.nix
    ./programs/vesktop.nix

    # Application package groups (categorised home.packages lists).
    ./programs/user-apps/desktop.nix
    ./programs/user-apps/dev.nix
    ./programs/user-apps/api-tools.nix
    ./programs/user-apps/wayland.nix
    ./programs/user-apps/media.nix
    ./programs/user-apps/games.nix
    ./programs/user-apps/containers.nix
    ./programs/user-apps/misc.nix
  ];

  home.username = var.username;
  home.homeDirectory = var.homeDirectory;
  home.stateVersion = var.stateVersion;

  programs.neovim = {
    enable = true;
    package = pkgs.neovim;
    withRuby = true;
    withPython3 = true;
  };

  # Caelestia ships its own bar - block any upstream waybar enable.
  programs.waybar.enable = lib.mkForce false;

  # Silence HM 26.05 warning: keep legacy behaviour (gtk4 inherits gtk theme).
  gtk.gtk4.theme = config.gtk.theme;

  # When HM uses the system package set, Stylix must not install package overlays
  # inside the HM evaluation as well.
  stylix.overlays.enable = false;

  # AccountsService consumers (GNOME/KDE) read this; SDDM uses /etc/sddm/faces/ instead.
  home.file.".face".source = ./avatar.gif;

  home.activation.copyWallpapers = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    WALLS_SRC="${./../Wallpapers}"
    WALLS_DST="${config.home.homeDirectory}/Pictures/Wallpapers"

    $DRY_RUN_CMD mkdir -p "$WALLS_DST"
    if [ -d "$WALLS_SRC" ]; then
      cp -n "$WALLS_SRC"/* "$WALLS_DST" 2>/dev/null || true
    fi
  '';
}
