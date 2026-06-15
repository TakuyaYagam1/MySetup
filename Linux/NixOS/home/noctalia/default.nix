{
  config,
  homeLibs,
  inputs,
  lib,
  options,
  pkgs,
  ...
}:

let
  dotfilesLib = homeLibs.dotfiles;
  trans = homeLibs.transparency;
  noctaliaShellPackage = inputs.noctalia-shell.packages.${pkgs.stdenv.hostPlatform.system}.default;
  settingsJson = ./config/settings.json;
  colorsJson = ./config/colors.json;
  pluginsJson = ./config/plugins.json;
  colorSchemesDir = ./config/colorschemes;
  wallpapersJson = pkgs.writeText "noctalia-wallpapers.json" (
    builtins.toJSON {
      wallpapers = { };
      defaultWallpaper = "@HOME@/Pictures/Wallpapers/1.jpg";
      usedRandomWallpapers = { };
    }
  );

  colorSchemeFiles =
    if builtins.pathExists colorSchemesDir then
      lib.filesystem.listFilesRecursive colorSchemesDir
    else
      [ ];

  noctaliaConfigDir = "$HOME/.config/noctalia";

  noctaliaProgramOption = if options.programs ? noctalia then "noctalia" else "noctalia-shell";

  noctaliaProgramConfig = {
    enable = true;
    package = noctaliaShellPackage;
    systemd.enable = false;
    settings = lib.mkForce { };
  }
  // (
    if noctaliaProgramOption == "noctalia" then
      { customPalettes = lib.mkForce { }; }
    else
      { colors = lib.mkForce { }; }
  );

  seedColorSchemeFiles = lib.concatMapStringsSep "\n" (
    file:
    let
      relPath = lib.removePrefix "${toString colorSchemesDir}/" (toString file);
      relDir = builtins.dirOf relPath;
      storeFile = builtins.path {
        path = file;
        name = lib.replaceStrings [ " " ] [ "-" ] (builtins.baseNameOf file);
      };
      targetDirExpr =
        if relDir == "." then
          "${noctaliaConfigDir}/colorschemes"
        else
          "${noctaliaConfigDir}/colorschemes/${relDir}";
      targetExpr = "${noctaliaConfigDir}/colorschemes/${relPath}";
    in
    ''
      $DRY_RUN_CMD ${pkgs.coreutils}/bin/mkdir -p "${targetDirExpr}"
      if [ ! -e "${targetExpr}" ]; then
        $DRY_RUN_CMD ${pkgs.coreutils}/bin/install -m 644 "${storeFile}" "${targetExpr}"
      fi
    ''
  ) colorSchemeFiles;
in
{
  programs.${noctaliaProgramOption} = noctaliaProgramConfig;

  # The upstream Stylix target writes this as a Nix store symlink, but Noctalia
  # uses it as mutable runtime state for selected wallpapers.
  home.file.".cache/noctalia/wallpapers.json".enable = lib.mkForce false;

  home.activation.noctaliaSeedConfig = homeLibs.shellSeed.mkSeedActivation {
    dirs = [
      "$HOME/.config/noctalia"
      "$HOME/.cache/noctalia"
    ];
    body = ''
      seed_json_object "$HOME/.cache/noctalia/wallpapers.json" "${wallpapersJson}" "${config.home.homeDirectory}"
      seed_json_object "$HOME/.config/noctalia/settings.json" "${settingsJson}" "${config.home.homeDirectory}" '
          .bar //= {} |
          ${dotfilesLib.mkOpacityDefault trans ".bar.backgroundOpacity"} |
          ${dotfilesLib.mkOpacityDefault trans ".bar.capsuleOpacity"} |
          .dock //= {} |
          ${dotfilesLib.mkOpacityDefault trans ".dock.backgroundOpacity"} |
          .general //= {} |
          .general.dimmerOpacity //= ${toString trans.content} |
          .notifications //= {} |
          ${dotfilesLib.mkOpacityDefault trans ".notifications.backgroundOpacity"} |
          .osd //= {} |
          ${dotfilesLib.mkOpacityDefault trans ".osd.backgroundOpacity"} |
          .ui //= {} |
          ${dotfilesLib.mkOpacityDefault trans ".ui.panelBackgroundOpacity"}
      '

      seed_json_object "$HOME/.config/noctalia/colors.json" "${colorsJson}"
      seed_json_object "$HOME/.config/noctalia/plugins.json" "${pluginsJson}"

      ${seedColorSchemeFiles}
    '';
  };
}
