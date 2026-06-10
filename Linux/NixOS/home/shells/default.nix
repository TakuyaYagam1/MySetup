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
  shellSelectorRoot = ./quickshell/mysetup-shell-selector;
  defaultProfile = shellProfiles.byId.${shellProfiles.defaultProfile};
  hyprDir = "${config.xdg.configHome}/hypr";
  hyprRuntimeDir = "${config.xdg.stateHome}/mysetup/hypr-runtime";
  activeShellState = "${config.xdg.stateHome}/mysetup/active-shell";
  stableEntrypoints = dotfilesLib.hyprRuntimeFiles;
  inherit (dotfilesLib) hyprScripts;
  # end4 is installed as one patched tree at hypr/end4. Adding nested
  # xdg.configFile entries under that symlink makes Home Manager reject them as
  # outside $HOME.
  standaloneShellProfiles = lib.filter (profile: profile.id != "end4") shellProfiles.ordered;

  hyprScriptFiles = lib.genAttrs (map (name: "hypr/scripts/${name}") hyprScripts) (target: {
    force = true;
    executable = true;
    source = dotsRoot + "/${target}";
  });

  mysetupHyprFiles = {
    "hypr/mysetup/hyprland.lua" = {
      force = true;
      source = dotsRoot + "/hypr/hyprland.lua";
    };
    "hypr/lib/mysetup.lua" = dotfilesLib.forcedSource (dotsRoot + "/hypr/lib/mysetup.lua");
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
    hyprScriptFiles // stableEntrypointFiles // mysetupHyprFiles // commonHyprFiles
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
    // mysetupHyprFiles
    // commonHyprFiles
    // {
      "quickshell/mysetup-shell-selector" = {
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

    pruneMySetupHyprBackups = lib.hm.dag.entryBefore [ "checkLinkTargets" ] ''
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

          $DRY_RUN_CMD mkdir -p "$runtime_dir" "$(dirname "$active_shell")"

          seed_file() {
            local target="$1"
            local source="$2"

            if [ ! -e "$target" ]; then
              $DRY_RUN_CMD install -m 644 "$source" "$target"
            fi
          }

          seed_file "$active_shell" "${pkgs.writeText "mysetup-active-shell-default" "${defaultProfile.id}\n"}"
          seed_file "$runtime_dir/hyprland.lua" "${pkgs.writeText "mysetup-hypr-runtime-default" ''
            -- Active Hyprland profile: mysetup (${defaultProfile.id})
            local home = os.getenv("HOME")
            if home == nil then
                error("HOME is not set; cannot locate MySetup Hyprland config")
            end

            local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
            local state_home = os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")
            local hypr_root = config_home .. "/hypr"
            local runtime_root = state_home .. "/mysetup/hypr-runtime"
            package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
            dofile(hypr_root .. "/mysetup/hyprland.lua")
            dofile(runtime_root .. "/shell-profile.lua")
          ''}"
          seed_file "$runtime_dir/shell-profile.lua" "${pkgs.writeText "mysetup-shell-profile-default" ''
            -- Runtime shell launcher
            hl.on("hyprland.start", function()
                hl.exec_cmd("${hyprDir}/scripts/start-shell.sh")
            end)
          ''}"
          seed_file "$runtime_dir/hyprlock.conf" "${pkgs.writeText "mysetup-empty-hyprlock" ""}"
          seed_file "$runtime_dir/hypridle.conf" "${pkgs.writeText "mysetup-empty-hypridle" ""}"
          seed_file "$runtime_dir/shell-launcher.lua" "${pkgs.writeText "mysetup-shell-launcher-default" ''
            -- Active shell launcher profile: ${defaultProfile.id}
            require("${defaultProfile.id}.launcher")
          ''}"
          seed_file "$runtime_dir/shell-keybinds.lua" "${pkgs.writeText "mysetup-shell-keybinds-default" ''
            -- Active shell keybind profile: ${defaultProfile.id}
            require("${defaultProfile.id}.keybinds")
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
