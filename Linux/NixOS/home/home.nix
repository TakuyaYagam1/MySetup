{
  config,
  pkgs,
  inputs,
  lib,
  mysetup,
  mysetupLib,
  ...
}:

let
  desktopOrMore = mysetupLib.presets.desktopOrMore mysetup;
  mysetupPkgs = pkgs.mysetup or { };
  homeLibs = import ./lib { inherit lib pkgs; };
  avatarSource =
    if builtins.pathExists ./avatar.jpg then ./avatar.jpg else ../themes/sddm-theme/icons/logo.png;
  generatedConfigFiles = [
    "foot/foot.ini"
    "btop/btop.conf"
    "gtk-3.0/gtk.css"
    "gtk-4.0/gtk.css"
    "cava/config"
    "qt5ct/qt5ct.conf"
    "qt6ct/qt6ct.conf"
  ];
  coreImports = [
    inputs.stylix.homeModules.stylix
    ./stylix.nix
    ./theming.nix
    ./programs/git.nix
    ./programs/fish.nix
    ./programs/shell-tools.nix
    ./programs/foot.nix
    ./programs/btop.nix
    ./programs/starship.nix
    ./programs/cava.nix
    ./programs/fastfetch.nix
    ./programs/mimeapps.nix
    ./programs/nightshift.nix
    ./programs/nix-index.nix
    ./programs/nixos-helper.nix
    ./programs/thunar.nix
    ./programs/uwsm.nix
    ./programs/virtualbox.nix
    ./programs/yazi.nix
    ./programs/vesktop.nix
    ./programs/packages.nix
  ];
  shellImports = [
    inputs.caelestia-shell.homeManagerModules.default
    inputs.noctalia.homeModules.default
    ./shells
    ./caelestia
    ./noctalia
    ./end4
  ];
in
{
  _module.args.homeLibs = homeLibs;

  imports = coreImports ++ lib.optionals desktopOrMore shellImports;

  home = {
    inherit (mysetup.user) username homeDirectory;
    inherit (mysetup.host) stateVersion;

    # AccountsService consumers (GNOME/KDE) read this; SDDM uses /etc/sddm/faces/ instead.
    file.".face".source = avatarSource;

    activation.copyWallpapers = lib.mkIf mysetup.wallpapers.enable (
      lib.hm.dag.entryAfter [ "writeBoundary" ] ''
        WALLS_SRC="${./../Wallpapers}"
        WALLS_DST="${config.home.homeDirectory}/Pictures/Wallpapers"

        $DRY_RUN_CMD mkdir -p "$WALLS_DST"
        if [ -d "$WALLS_SRC" ]; then
          $DRY_RUN_CMD ${pkgs.findutils}/bin/find "$WALLS_DST" -maxdepth 1 -type f -name 'preview-*' -delete
          for wall in "$WALLS_SRC"/*; do
            [ -e "$wall" ] || continue
            $DRY_RUN_CMD ${pkgs.coreutils}/bin/cp -n --no-preserve=mode "$wall" "$WALLS_DST/"
          done
          $DRY_RUN_CMD ${pkgs.coreutils}/bin/chmod -R u+w "$WALLS_DST"
        fi
      ''
    );
  };

  programs.neovim = {
    enable = true;
    package = mysetupPkgs.neovim or pkgs.neovim;
    withRuby = true;
    withPython3 = true;
    plugins = [
      (pkgs.symlinkJoin {
        name = "nvim-treesitter-parsers";
        paths = pkgs.vimPlugins.nvim-treesitter.withAllGrammars.dependencies;
      })
    ];
    initLua = ''
      require("config.lazy")
    '';
  };

  # Caelestia ships its own bar - block any upstream waybar enable.
  programs.waybar.enable = lib.mkForce false;

  # Keep GTK4 inheriting the GTK theme unless Stylix defines it explicitly.
  gtk.gtk4.theme = lib.mkDefault config.gtk.theme;

  # When HM uses the system package set, Stylix must not install package overlays
  # inside the HM evaluation as well.
  stylix.overlays.enable = false;
  stylix.targets.neovim.enable = false;

  xdg.configFile = lib.genAttrs generatedConfigFiles (_: {
    force = true;
  });
}
