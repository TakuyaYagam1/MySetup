{
  config,
  homeLibs,
  lib,
  pkgs,
  ...
}:

let
  dotfilesLib = homeLibs.dotfiles;
  shellProfiles = import ./profiles.nix;
  inherit (dotfilesLib) dotsRoot;
  shellSelectorRoot = ./quickshell/wahrwelt-shell-selector;
  defaultProfile = shellProfiles.byId.${shellProfiles.defaultProfile};
  hyprDir = "${config.xdg.configHome}/hypr";
  hyprRuntimeDir = "${config.xdg.stateHome}/wahrwelt/hypr-runtime";
  activeShellState = "${config.xdg.stateHome}/wahrwelt/active-shell";
  stableEntrypoints = dotfilesLib.hyprRuntimeFiles;
  inherit (dotfilesLib) hyprScripts;
  # end4 is installed as one patched tree at hypr/end4. Adding nested
  # xdg.configFile entries under that symlink makes Home Manager reject them as
  # outside $HOME.
  standaloneShellProfiles = lib.filter (profile: profile.family != "end4") shellProfiles.ordered;

  hyprScriptFiles = lib.genAttrs (map (name: "hypr/scripts/${name}") hyprScripts) (target: {
    force = true;
    executable = true;
    source = dotsRoot + "/${target}";
  });

  wahrweltHyprFiles = {
    "hypr/wahrwelt/hyprland.lua" = {
      force = true;
      source = dotsRoot + "/hypr/hyprland.lua";
    };
    "hypr/lib/wahrwelt.lua" = dotfilesLib.forcedSource (dotsRoot + "/hypr/lib/wahrwelt.lua");
    "hypr/variables.lua" = dotfilesLib.forcedSource (dotsRoot + "/hypr/variables.lua");
    "hypr/scheme/default.lua" = dotfilesLib.forcedSource (dotsRoot + "/hypr/scheme/default.lua");
  }
  // lib.genAttrs (map (name: "hypr/hyprland/${name}.lua") [
    "animations"
    "decoration"
    "env"
    "execs"
    "general"
    "gestures"
    "group"
    "input"
    "keybinds"
    "misc"
    "rules"
    "scrolling"
  ]) (target: dotfilesLib.forcedSource (dotsRoot + "/${target}"))
  // lib.genAttrs (map (target: "hypr/${target}") (
    lib.concatMap (profile: [
      profile.launcher
      profile.keybinds
    ]) standaloneShellProfiles
  )) (target: dotfilesLib.forcedSource (dotsRoot + "/${target}"));

  commonHyprFiles = {
    "hypr/shell-common-keybinds.lua" = {
      force = true;
      source = dotsRoot + "/hypr/shell-common-keybinds.lua";
    };
    "hypr/shell-workspace-keybinds.lua" = {
      force = true;
      source = dotsRoot + "/hypr/shell-workspace-keybinds.lua";
    };
    "hypr/vm-keybinds.lua" = {
      force = true;
      source = dotsRoot + "/hypr/vm-keybinds.lua";
    };
  };

  selectorShellOptions = homeLibs.shellSelectorOptions.buildOptionsFile shellProfiles.ordered;

  shellSelectorSource = homeLibs.shellSelectorOptions.buildSelectorSource {
    selectorRoot = shellSelectorRoot;
    optionsFile = selectorShellOptions;
  };

  stableEntrypointFiles = lib.genAttrs (map (name: "hypr/${name}") stableEntrypoints) (
    target:
    let
      name = lib.removePrefix "hypr/" target;
    in
    if lib.hasSuffix ".lua" name then
      dotfilesLib.stableLuaRuntimeSourceFile "${hyprRuntimeDir}/${name}"
    else
      dotfilesLib.stableRuntimeSourceFile "${hyprRuntimeDir}/${name}"
  );

  managedHyprPaths = lib.attrNames (
    hyprScriptFiles // stableEntrypointFiles // wahrweltHyprFiles // commonHyprFiles
  );

  backupTargets = lib.concatMapStringsSep " \\\n" (
    path: ''"${config.xdg.configHome}/${path}.backup"''
  ) managedHyprPaths;

  legacyHyprlandRuntimePaths = [
    "${hyprDir}/hyprland.conf"
    "${hyprDir}/shell-profile.conf"
    "${hyprDir}/shell-launcher.conf"
    "${hyprDir}/shell-keybinds.conf"
    "${hyprDir}/mysetup/hyprland.conf"
    "${hyprDir}/mysetup/local.lua"
    "${hyprDir}/wahrwelt/hyprland.conf"
    "${hyprDir}/wahrwelt/local.lua"
    "${hyprRuntimeDir}/hyprland.conf"
    "${hyprRuntimeDir}/shell-profile.conf"
    "${hyprRuntimeDir}/shell-launcher.conf"
    "${hyprRuntimeDir}/shell-keybinds.conf"
  ];

  legacyHyprlandRuntimeTargets = lib.concatMapStringsSep " \\\n" (
    path: ''"${path}"''
  ) legacyHyprlandRuntimePaths;
in
{
  xdg.configFile =
    hyprScriptFiles
    // stableEntrypointFiles
    // wahrweltHyprFiles
    // commonHyprFiles
    // {
      "quickshell/wahrwelt-shell-selector" = {
        force = true;
        source = shellSelectorSource;
      };
    };

  home.activation = {
    pruneLegacyHyprlandRuntime = lib.hm.dag.entryBefore [ "checkLinkTargets" ] ''
      for target in \
        ${legacyHyprlandRuntimeTargets}
      do
        if [ -e "$target" ] || [ -L "$target" ]; then
          $DRY_RUN_CMD rm -f -- "$target"
        fi
      done
    '';

    pruneWahrweltHyprBackups = lib.hm.dag.entryBefore [ "checkLinkTargets" ] ''
      for backup in \
        ${backupTargets}
      do
        if [ -e "$backup" ]; then
          $DRY_RUN_CMD rm -f "$backup"
        fi
      done
    '';

    seedHyprShellRuntime =
      lib.hm.dag.entryAfter
        [
          "caelestiaSeedShellJson"
          "noctaliaSeedConfig"
          "end4SeedConfig"
          "end4RepairRuntime"
          "end4SeedAppConfig"
          "pruneLegacyHyprlandRuntime"
        ]
        ''
          runtime_dir="${hyprRuntimeDir}"
          active_shell="${activeShellState}"

          $DRY_RUN_CMD mkdir -p "$runtime_dir" "$(dirname "$active_shell")" "${hyprDir}/wahrwelt"

          seed_file() {
            local target="$1"
            local source="$2"

            if [ ! -e "$target" ]; then
              $DRY_RUN_CMD install -m 644 "$source" "$target"
            fi
          }

          seed_file "$active_shell" "${pkgs.writeText "wahrwelt-active-shell-default" "${defaultProfile.id}\n"}"
          seed_file "$runtime_dir/hyprland.lua" "${pkgs.writeText "wahrwelt-hypr-runtime-default" ''
            -- Active Hyprland profile: wahrwelt (${defaultProfile.id})
            local home = os.getenv("HOME")
            if home == nil then
                error("HOME is not set; cannot locate Wahrwelt Hyprland config")
            end

            local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
            local state_home = os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")
            local hypr_root = config_home .. "/hypr"
            local runtime_root = state_home .. "/wahrwelt/hypr-runtime"
            package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
            dofile(hypr_root .. "/wahrwelt/hyprland.lua")
            dofile(runtime_root .. "/shell-profile.lua")
          ''}"
          seed_file "$runtime_dir/shell-profile.lua" "${pkgs.writeText "wahrwelt-shell-profile-default" ''
            -- Runtime shell launcher
            hl.on("hyprland.start", function()
                hl.exec_cmd("${hyprDir}/scripts/start-shell.sh")
            end)
          ''}"
          seed_file "$runtime_dir/hyprlock.conf" "${pkgs.writeText "wahrwelt-empty-hyprlock" ""}"
          seed_file "$runtime_dir/hypridle.conf" "${pkgs.writeText "wahrwelt-empty-hypridle" ""}"
          seed_file "$runtime_dir/shell-launcher.lua" "${pkgs.writeText "wahrwelt-shell-launcher-default" ''
            -- Active shell launcher profile: ${defaultProfile.id}
            require("${defaultProfile.id}.launcher")
          ''}"
          seed_file "$runtime_dir/shell-keybinds.lua" "${pkgs.writeText "wahrwelt-shell-keybinds-default" ''
            -- Active shell keybind profile: ${defaultProfile.id}
            require("${defaultProfile.id}.keybinds")
          ''}"

          seed_file "${hyprDir}/wahrwelt/keybinds.lua" "${pkgs.writeText "wahrwelt-hypr-local-keybinds-example" ''
            -- Your own Hyprland keybind overrides go here. This file is NOT
            -- managed by Nix/Home Manager - edit it freely, it survives rebuilds
            -- and `wahrwelt update`.
            --
            -- It loads LAST, after every bind in this setup is already
            -- registered, so hl.unbind() here actually works. Hyprland does not
            -- auto-replace a duplicate bind - skip unbind() and both the old and
            -- the new action fire on the same key.
            -- Docs: https://wiki.hypr.land/Configuring/Basics/Binds/
            --
            -- Example: replace the default AmneziaVPN launcher (SUPER + SHIFT +
            -- Q) with something else, e.g. OpenVPN:
            --
            -- hl.unbind("SUPER + SHIFT + Q")
            -- hl.bind("SUPER + SHIFT + Q", hl.dsp.exec_cmd("openvpn --config ~/my.ovpn"))
          ''}"
        '';

    liveSyncHyprShell = lib.hm.dag.entryAfter [ "seedHyprShellRuntime" ] ''
      if command -v hyprctl >/dev/null 2>&1 && hyprctl instances >/dev/null 2>&1; then
        hypr_version="$(hyprctl version 2>/dev/null | awk 'NR == 1 { print $2 }')"
        hypr_version="''${hypr_version#v}"
        hypr_major="''${hypr_version%%.*}"
        hypr_rest="''${hypr_version#*.}"
        hypr_minor="''${hypr_rest%%.*}"

        case "$hypr_major" in
          "" | *[!0-9]*)
            hypr_major=0
            ;;
        esac
        case "$hypr_minor" in
          "" | *[!0-9]*)
            hypr_minor=0
            ;;
        esac

        if [ "$hypr_major" -gt 0 ] || [ "$hypr_minor" -ge 55 ]; then
          $DRY_RUN_CMD hyprctl reload >/dev/null 2>&1 || true
          $DRY_RUN_CMD ${config.xdg.configHome}/hypr/scripts/start-shell.sh >/dev/null 2>&1 || true
        else
          echo "Skipping live Hyprland reload; running Hyprland $hypr_version cannot load Lua runtime. Logout or reboot after switch." >&2
        fi
      fi
    '';
  };
}
