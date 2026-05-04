{ config, pkgs, inputs, lib, var, ... }:

{
  imports = [
    inputs.caelestia-shell.homeManagerModules.default
    inputs.noctalia-shell.homeModules.default
    inputs.stylix.homeModules.stylix
    ../themes/active.nix
    ./shells
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
  ]
  ++ lib.optional (var.shellProfile == "caelestia") ./caelestia
  ++ lib.optional (var.shellProfile == "noctalia") ./noctalia;

  home.username = var.username;
  home.homeDirectory = var.homeDirectory;
  home.stateVersion = var.stateVersion;

  assertions = [
    {
      assertion = lib.elem var.shellProfile [ "caelestia" "noctalia" ];
      message = "var.shellProfile must be one of: caelestia, noctalia";
    }
    {
      assertion = lib.elem (var.packagePreset or "personal") [ "minimal" "desktop" "developer" "personal" ];
      message = "var.packagePreset must be one of: minimal, desktop, developer, personal";
    }
  ];

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

  # These files are fully generated from Nix options. Force them into place so
  # stale manual copies or old *.hm-backup files do not block HM activation.
  xdg.configFile."foot/foot.ini".force = true;
  xdg.configFile."btop/btop.conf".force = true;
  xdg.configFile."gtk-3.0/gtk.css".force = true;
  xdg.configFile."gtk-4.0/gtk.css".force = true;
  xdg.configFile."cava/config".force = true;
  xdg.configFile."qt5ct/qt5ct.conf".force = true;
  xdg.configFile."qt6ct/qt6ct.conf".force = true;

  # AccountsService consumers (GNOME/KDE) read this; SDDM uses /etc/sddm/faces/ instead.
  home.file.".face".source = ./avatar.gif;

  home.activation.copyWallpapers = lib.mkIf (var.wallpapers.enable or true) (
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      WALLS_SRC="${./../Wallpapers}"
      WALLS_DST="${config.home.homeDirectory}/Pictures/Wallpapers"

      $DRY_RUN_CMD mkdir -p "$WALLS_DST"
      if [ -d "$WALLS_SRC" ]; then
        $DRY_RUN_CMD ${pkgs.findutils}/bin/find "$WALLS_DST" -maxdepth 1 -type f -name 'preview-*' -delete
        for wall in "$WALLS_SRC"/*; do
          [ -e "$wall" ] || continue
          $DRY_RUN_CMD ${pkgs.coreutils}/bin/cp -n "$wall" "$WALLS_DST/"
        done
      fi
    ''
  );
}
