{ config, inputs, lib, pkgs, ... }:

let
  dotfilesSource = inputs.end4-dotfiles;
  pythonEnv = import ./python-env.nix { inherit pkgs; };
  dotsRoot =
    let
      installedDots = ../../dots;
      repoDots = ../../../dots;
    in
      if builtins.pathExists installedDots then installedDots else repoDots;
  mysetupHyprSource = dotsRoot + "/hypr/end4";

  forcedSource = source: {
    inherit source;
    force = true;
  };

  forcedText = text: {
    inherit text;
    force = true;
  };

  patchedQuickshell = pkgs.runCommand "end4-quickshell-patched" {
    buildInputs = [
      pkgs.bash
      pythonEnv
    ];
  } ''
    cp -r ${dotfilesSource}/dots/.config/quickshell $out
    chmod -R +w $out

    find $out -name '*.py' -print0 | xargs -0 sed -i 's|^#!.*ILLOGICAL_IMPULSE_VIRTUAL_ENV.*|#!/usr/bin/env python3|'
    sed -i 's|/dev/pts/\*|/dev/pts/* 2>/dev/null|' $out/ii/scripts/colors/applycolor.sh

    patchShebangs $out
  '';

  patchedHypr = pkgs.runCommand "end4-hypr-patched" {
    buildInputs = [
      pkgs.bash
    ];
  } ''
    cp -r ${dotfilesSource}/dots/.config/hypr $out
    chmod -R +w $out
    mkdir -p $out/mysetup
    cp ${mysetupHyprSource}/keybinds.conf $out/mysetup/keybinds.conf
    cp -r ${mysetupHyprSource}/scripts $out/mysetup/scripts
    find $out/mysetup/scripts -type f -exec chmod +x {} +

    cat > $out/hyprland/env.conf <<'EOF'
${injectedHyprEnv}
EOF
    cat ${dotfilesSource}/dots/.config/hypr/hyprland/env.conf >> $out/hyprland/env.conf

    substituteInPlace $out/hyprland/general.conf \
      --replace-fail 'enable_gesture = false' '# enable_gesture = false  # Removed: obsolete hyprexpo option' \
      --replace-fail 'gesture_positive = false' '# gesture_positive = false  # Removed: obsolete hyprexpo option'

    substituteInPlace $out/hyprland.conf \
      --replace-fail 'source=hyprland/env.conf' 'source=~/.config/hypr/end4/hyprland/env.conf' \
      --replace-fail 'source=custom/env.conf' 'source=~/.config/hypr/end4/custom/env.conf' \
      --replace-fail 'source=hyprland/variables.conf' 'source=~/.config/hypr/end4/hyprland/variables.conf' \
      --replace-fail 'source=custom/variables.conf' 'source=~/.config/hypr/end4/custom/variables.conf' \
      --replace-fail 'source=hyprland/execs.conf' 'source=~/.config/hypr/end4/hyprland/execs.conf' \
      --replace-fail 'source=hyprland/general.conf' 'source=~/.config/hypr/end4/hyprland/general.conf' \
      --replace-fail 'source=hyprland/rules.conf' 'source=~/.config/hypr/end4/hyprland/rules.conf' \
      --replace-fail 'source=hyprland/colors.conf' 'source=~/.config/hypr/end4/hyprland/colors.conf' \
      --replace-fail 'source=hyprland/keybinds.conf' 'source=~/.config/hypr/end4/hyprland/keybinds.conf' \
      --replace-fail 'source=custom/execs.conf' 'source=~/.config/hypr/end4/custom/execs.conf' \
      --replace-fail 'source=custom/general.conf' 'source=~/.config/hypr/end4/custom/general.conf' \
      --replace-fail 'source=custom/rules.conf' 'source=~/.config/hypr/end4/custom/rules.conf' \
      --replace-fail 'source=custom/keybinds.conf' 'source=~/.config/hypr/end4/custom/keybinds.conf' \
      --replace-fail 'source=workspaces.conf' 'source=~/.config/hypr/end4/workspaces.conf' \
      --replace-fail 'source=monitors.conf' 'source=~/.config/hypr/end4/monitors.conf' \
      --replace-fail 'source=hyprland/shellOverrides/main.conf' 'source=~/.config/hypr/end4/hyprland/shellOverrides/main.conf'
    printf '\n# MySetup keybind overrides\nsource=~/.config/hypr/end4/mysetup/keybinds.conf\n' >> $out/hyprland.conf

    find $out -type f -exec sed -i \
      -e 's|~/.config/hypr/hyprland/|~/.config/hypr/end4/hyprland/|g' \
      -e 's|~/.config/hypr/custom/|~/.config/hypr/end4/custom/|g' \
      -e 's|~/.config/hypr/hyprlock/|~/.config/hypr/end4/hyprlock/|g' \
      -e 's|''${XDG_CONFIG_HOME:-$HOME/.config}/hypr/hyprlock/|''${XDG_CONFIG_HOME:-$HOME/.config}/hypr/end4/hyprlock/|g' \
      {} +

    patchShebangs $out
  '';

  injectedHyprEnv = ''
    env = PATH,${config.home.homeDirectory}/.nix-profile/bin:/etc/profiles/per-user/${config.home.username}/bin:$PATH
    env = XDG_DATA_DIRS,${config.home.homeDirectory}/.nix-profile/share:${config.home.homeDirectory}/.local/share:${config.home.homeDirectory}/.local/share/flatpak/exports/share:/etc/profiles/per-user/${config.home.username}/share:/run/current-system/sw/share:/var/lib/flatpak/exports/share:/usr/local/share:/usr/share:$XDG_DATA_DIRS
    env = QT_PLUGIN_PATH,${config.home.homeDirectory}/.nix-profile/lib/qt-6/plugins:${config.home.homeDirectory}/.nix-profile/lib/qt6/plugins:${config.home.homeDirectory}/.nix-profile/lib/plugins
    env = QML2_IMPORT_PATH,${config.home.homeDirectory}/.nix-profile/lib/qt-6/qml:${config.home.homeDirectory}/.nix-profile/lib/qt6/qml
    env = QT_WAYLAND_DISABLE_WINDOWDECORATION,1
    env = QT_QPA_PLATFORMTHEME,gtk3
    $qsConfig = ${config.home.homeDirectory}/.config/quickshell/ii
    env = qsConfig,${config.home.homeDirectory}/.config/quickshell/ii
    env = ILLOGICAL_IMPULSE_VIRTUAL_ENV,${config.home.homeDirectory}/.local/state/quickshell/.venv
  '';

  hyprlockEntryPoint = ''
    # MySetup end4 Hyprlock entrypoint.
    source=~/.config/hypr/end4/hyprlock.conf
  '';
in
{
  home.file = {
    ".local/state/quickshell/.venv/bin/activate" = {
      force = true;
      text = ''
        export VIRTUAL_ENV="${pythonEnv}"
        export PATH="${pythonEnv}/bin:$PATH"
        deactivate () {
          return 0
        }
      '';
    };

    ".local/state/quickshell/.venv/bin/python" = {
      force = true;
      source = "${pythonEnv}/bin/python";
    };

    ".local/state/quickshell/.venv/bin/python3" = {
      force = true;
      source = "${pythonEnv}/bin/python3";
    };

    ".local/state/quickshell/.venv/pyvenv.cfg" = {
      force = true;
      text = ''
        home = ${pythonEnv}/bin
        include-system-site-packages = false
        version = ${pkgs.python3.pythonVersion}
      '';
    };
  };

  xdg.configFile = {
    "chrome-flags.conf" = forcedSource "${dotfilesSource}/dots/.config/chrome-flags.conf";
    "code-flags.conf" = forcedSource "${dotfilesSource}/dots/.config/code-flags.conf";
    "darklyrc" = forcedSource "${dotfilesSource}/dots/.config/darklyrc";
    "dolphinrc" = forcedSource "${dotfilesSource}/dots/.config/dolphinrc";
    "fuzzel" = forcedSource "${dotfilesSource}/dots/.config/fuzzel";
    "hypr/hyprlock.conf" = forcedText hyprlockEntryPoint;
    "hypr/end4" = forcedSource patchedHypr;
    "kde-material-you-colors" = forcedSource "${dotfilesSource}/dots/.config/kde-material-you-colors";
    "kitty" = forcedSource "${dotfilesSource}/dots/.config/kitty";
    "Kvantum" = forcedSource "${dotfilesSource}/dots/.config/Kvantum";
    "matugen" = forcedSource "${dotfilesSource}/dots/.config/matugen";
    "mpv" = forcedSource "${dotfilesSource}/dots/.config/mpv";
    "quickshell/ii" = forcedSource "${patchedQuickshell}/ii";
    "wlogout" = forcedSource "${dotfilesSource}/dots/.config/wlogout";
    "xdg-desktop-portal" = forcedSource "${dotfilesSource}/dots/.config/xdg-desktop-portal";
  };

  xdg.dataFile = {
    "icons/hicolor/scalable/apps/illogical-impulse.svg" = forcedSource "${dotfilesSource}/dots/.local/share/icons/illogical-impulse.svg";
    "konsole/Profile 1.profile" = forcedSource "${dotfilesSource}/dots/.local/share/konsole/Profile 1.profile";
  };

  home.activation.end4PrepareRuntime =
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      config_dir="$HOME/.config/illogical-impulse"
      hypr_dir="$HOME/.config/hypr"
      end4_dir="$hypr_dir/end4"
      kdeglobals_target="$HOME/.config/kdeglobals"
      konsole_target="$HOME/.local/share/konsole"

      $DRY_RUN_CMD mkdir -p "$config_dir"
      $DRY_RUN_CMD mkdir -p "$hypr_dir"
      $DRY_RUN_CMD mkdir -p "$konsole_target"
      $DRY_RUN_CMD rm -f "$hypr_dir/hypridle.conf" "$hypr_dir/monitors.conf" "$hypr_dir/workspaces.conf"
      $DRY_RUN_CMD ln -sfn "$end4_dir/hypridle.conf" "$hypr_dir/hypridle.conf"
      $DRY_RUN_CMD ln -sfn "$end4_dir/monitors.conf" "$hypr_dir/monitors.conf"
      $DRY_RUN_CMD ln -sfn "$end4_dir/workspaces.conf" "$hypr_dir/workspaces.conf"

      if [ -L "$kdeglobals_target" ]; then
        $DRY_RUN_CMD rm -f "$kdeglobals_target"
      fi

      if [ ! -e "$kdeglobals_target" ]; then
        $DRY_RUN_CMD install -m 644 "${dotfilesSource}/dots/.config/kdeglobals" "$kdeglobals_target"
      fi

      if [ -e "$kdeglobals_target" ]; then
        $DRY_RUN_CMD chmod u+w "$kdeglobals_target"
      fi

      if [ ! -e "$konsole_target/Profile 1.profile" ]; then
        $DRY_RUN_CMD install -m 644 "${dotfilesSource}/dots/.local/share/konsole/Profile 1.profile" "$konsole_target/Profile 1.profile"
      fi
    '';
}
