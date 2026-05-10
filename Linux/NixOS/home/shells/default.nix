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

  hyprScriptFiles = lib.genAttrs (map (name: "hypr/scripts/${name}") hyprScripts) (target: {
    force = true;
    executable = true;
    source = dotsRoot + "/${target}";
  });

  commonHyprFiles = {
    "hypr/shell-common-keybinds.conf" = {
      force = true;
      source = dotsRoot + "/hypr/shell-common-keybinds.conf";
    };
    "hypr/shell-workspace-keybinds.conf" = {
      force = true;
      source = dotsRoot + "/hypr/shell-workspace-keybinds.conf";
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
    dotfilesLib.stableRuntimeSourceFile "${hyprRuntimeDir}/${name}"
  );

  managedHyprPaths =
    (map (name: "hypr/${name}") stableEntrypoints)
    ++ [
      "hypr/shell-common-keybinds.conf"
      "hypr/shell-workspace-keybinds.conf"
    ]
    ++ (map (name: "hypr/scripts/${name}") hyprScripts);

  backupTargets = lib.concatMapStringsSep " \\\n" (
    path: ''"${config.xdg.configHome}/${path}.hm-backup"''
  ) managedHyprPaths;
in
{
  xdg.configFile =
    hyprScriptFiles
    // stableEntrypointFiles
    // commonHyprFiles
    // {
      "quickshell/mysetup-shell-selector" = {
        force = true;
        source = shellSelectorSource;
      };
    };

  home.activation = {
    seedMySetupHyprConfig = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      target="${config.xdg.configHome}/hypr/mysetup/hyprland.conf"
      src="${dotsRoot}/hypr/hyprland.conf"

      $DRY_RUN_CMD mkdir -p "$(dirname "$target")"

      if [ -L "$target" ]; then
        $DRY_RUN_CMD rm -f "$target"
      fi

      if [ ! -e "$target" ]; then
        $DRY_RUN_CMD install -m 644 "$src" "$target"
      fi
    '';

    seedHyprShellRuntime =
      lib.hm.dag.entryAfter
        [
          "caelestiaSeedShellJson"
          "noctaliaSeedConfig"
          "end4SeedConfig"
          "end4RepairRuntime"
          "end4SeedAppConfig"
          "seedMySetupHyprConfig"
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
          seed_file "$runtime_dir/hyprland.conf" "${pkgs.writeText "mysetup-hypr-runtime-default" ''
            source = ${hyprDir}/mysetup/hyprland.conf
            source = ${hyprRuntimeDir}/shell-profile.conf
          ''}"
          seed_file "$runtime_dir/shell-profile.conf" "${pkgs.writeText "mysetup-shell-profile-default" ''
            exec-once = ${hyprDir}/scripts/start-shell.sh
          ''}"
          seed_file "$runtime_dir/hyprlock.conf" "${pkgs.writeText "mysetup-empty-hyprlock" ""}"
          seed_file "$runtime_dir/hypridle.conf" "${pkgs.writeText "mysetup-empty-hypridle" ""}"
          seed_file "$runtime_dir/shell-launcher.conf" "${dotsRoot}/hypr/${defaultProfile.launcher}"
          seed_file "$runtime_dir/shell-keybinds.conf" "${dotsRoot}/hypr/${defaultProfile.keybinds}"
        '';

    liveSyncHyprShell = lib.hm.dag.entryAfter [ "seedHyprShellRuntime" ] ''
      if command -v hyprctl >/dev/null 2>&1 && hyprctl instances >/dev/null 2>&1; then
        $DRY_RUN_CMD hyprctl reload >/dev/null 2>&1 || true
        $DRY_RUN_CMD ${config.xdg.configHome}/hypr/scripts/start-shell.sh >/dev/null 2>&1 || true
      fi
    '';

    pruneMySetupHyprBackups = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      for backup in \
        ${backupTargets}
      do
        if [ -e "$backup" ]; then
          $DRY_RUN_CMD rm -f "$backup"
        fi
      done
    '';
  };
}
