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
  end4Profile = shellProfiles.byId.end4;
  end4PCProfile = shellProfiles.byId.end4-pc;
  hyprDir = "${config.xdg.configHome}/hypr";
  hyprRuntimeDir = "${config.xdg.stateHome}/wahrwelt/hypr-runtime";
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
    "hypr/user/hyprland.lua" = {
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
    "hypr/end4-adapter.lua" = {
      force = true;
      source = dotsRoot + "/hypr/end4-adapter.lua";
    };
    "hypr/shell-common-keybinds.lua" = {
      force = true;
      source = dotsRoot + "/hypr/shell-common-keybinds.lua";
    };
    "hypr/shell-common-rules.lua" = {
      force = true;
      source = dotsRoot + "/hypr/shell-common-rules.lua";
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

  runtimeActivation = pkgs.writeShellApplication {
    name = "wahrwelt-runtime-activation";
    runtimeInputs = [
      pkgs.bash
      pkgs.coreutils
      pkgs.python3
      pkgs.util-linux
    ];
    text = builtins.readFile ./runtime-activation.sh;
  };

  canonicalHyprRuntime = pkgs.writeText "wahrwelt-hypr-runtime-default" ''
    -- Wahrwelt canonical Hyprland runtime entrypoint
    local home = os.getenv("HOME")
    if home == nil then
        error("HOME is not set; cannot locate Wahrwelt Hyprland config")
    end

    local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
    local hypr_root = config_home .. "/hypr"
    package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
    dofile(hypr_root .. "/user/hyprland.lua")
  '';

  legacyWahrweltRuntime = pkgs.writeText "wahrwelt-hypr-runtime-legacy-namespace" ''
    -- Wahrwelt canonical Hyprland runtime entrypoint
    local home = os.getenv("HOME")
    if home == nil then
        error("HOME is not set; cannot locate Wahrwelt Hyprland config")
    end

    local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
    local hypr_root = config_home .. "/hypr"
    package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
    dofile(hypr_root .. "/wahrwelt/hyprland.lua")
  '';

  legacyHomeManagerWahrweltRuntime = pkgs.writeText "wahrwelt-hypr-runtime-legacy-home-manager-namespace" ''
    -- Active Hyprland profile: wahrwelt (${defaultProfile.id})
    local home = os.getenv("HOME")
    if home == nil then
        error("HOME is not set; cannot locate Wahrwelt Hyprland config")
    end

    local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
    local hypr_root = config_home .. "/hypr"
    package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
    dofile(hypr_root .. "/wahrwelt/hyprland.lua")
  '';

  mkLegacySeededHyprRuntime =
    namespace:
    pkgs.writeText "wahrwelt-hypr-runtime-legacy-seeded-${namespace}" ''
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
      dofile(hypr_root .. "/${namespace}/hyprland.lua")
      dofile(runtime_root .. "/shell-profile.lua")
    '';

  legacySeededWahrweltRuntime = mkLegacySeededHyprRuntime "wahrwelt";
  legacySeededUserRuntime = mkLegacySeededHyprRuntime "user";

  defaultHyprUserConfig = pkgs.writeText "wahrwelt-hypr-default" ''
    local wahrwelt = require("lib.wahrwelt")

    wahrwelt.optional_require("wahrwelt.execs")
    wahrwelt.optional_require("wahrwelt.general")
    wahrwelt.optional_require("wahrwelt.rules")
    wahrwelt.optional_require("wahrwelt.keybinds")
  '';

  defaultShellProfileRuntime = pkgs.writeText "wahrwelt-shell-profile-default" ''
    -- Runtime shell launcher
    hl.on("hyprland.start", function()
        hl.exec_cmd("${hyprDir}/scripts/start-shell.sh")
    end)
  '';

  defaultHyprlockRuntime = pkgs.writeText "wahrwelt-empty-hyprlock" ''
    # Active Hyprlock profile: shell-managed
    # Caelestia and Noctalia use shell-native lock flows.
  '';
  defaultHypridleRuntime = pkgs.writeText "wahrwelt-empty-hypridle" ''
    # Active Hypridle profile: shell-managed
    # Caelestia and Noctalia use shell-native idle flows.
  '';

  defaultShellLauncherRuntime = pkgs.writeText "wahrwelt-shell-launcher-default" ''
    -- Active shell launcher profile: ${defaultProfile.id}
    require("${defaultProfile.id}.launcher")
  '';

  defaultShellKeybindRuntime = pkgs.writeText "wahrwelt-shell-keybinds-default" ''
    -- Wahrwelt shell adapter: ${defaultProfile.id}
    require("${defaultProfile.adapter}")
  '';

  end4RuntimeContract = pkgs.writeText "end4-runtime-contract" "end4-adapter-v1\n";

  mkEnd4RuntimeBundle = profile: {
    profile = defaultShellProfileRuntime;
    lock = pkgs.writeText "wahrwelt-hyprlock-${profile.id}" ''
      # Active Hyprlock profile: ${profile.id}
      source = ${hyprDir}/end4/hyprlock.conf
    '';
    idle = pkgs.writeText "wahrwelt-hypridle-${profile.id}" ''
      # Active Hypridle profile: ${profile.id}
      source = ${hyprDir}/end4/hypridle.conf
    '';
    launcher = pkgs.writeText "wahrwelt-shell-launcher-${profile.id}" ''
      -- Active shell launcher profile: ${profile.id}
      require("end4.launcher")
    '';
    keybinds = pkgs.writeText "wahrwelt-shell-keybinds-${profile.id}" ''
      -- Wahrwelt shell adapter: ${profile.id}
      require("${profile.adapter}").load({ profile = "${profile.id}", quickshell_config = "${config.xdg.configHome}/quickshell/${profile.quickshellConfig}" })
    '';
    legacyLauncher = pkgs.writeText "wahrwelt-shell-launcher-legacy-${profile.id}" ''
      -- Active shell launcher profile: ${profile.id}
      -- end4 registers launcher bindings from its own Hyprland Lua modules.
    '';
    legacyKeybinds = pkgs.writeText "wahrwelt-shell-keybinds-legacy-${profile.id}" ''
      -- Active shell keybind profile: ${profile.id}
      -- end4 registers keybinds from its own Hyprland Lua modules.
    '';
  };

  end4Runtime = mkEnd4RuntimeBundle end4Profile;
  end4PCRuntime = mkEnd4RuntimeBundle end4PCProfile;

in
{
  home.packages = [ pkgs.python3 ];

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
    prepareWahrweltHyprDirectory =
      lib.hm.dag.entryBetween [ "checkLinkTargets" ] [ "migrateWahrweltUserPaths" ]
        ''
          user_dir="${hyprDir}/user"
          $DRY_RUN_CMD "${runtimeActivation}/bin/wahrwelt-runtime-activation" \
            activate-user-dir \
            "$user_dir" \
            "${dotsRoot}/hypr/hyprland.lua" \
            "''${oldGenPath:-}" \
            "${defaultHyprUserConfig}"
        '';

    seedHyprShellRuntime =
      lib.hm.dag.entryAfter
        [
          "caelestiaSeedShellJson"
          "noctaliaSeedConfig"
          "end4SeedConfig"
          "end4SeedAppConfig"
          "linkGeneration"
        ]
        ''
          runtime_dir="${hyprRuntimeDir}"
          activation_helper="${runtimeActivation}/bin/wahrwelt-runtime-activation"
          hypr_runtime_source="${canonicalHyprRuntime}"
          wahrwelt_direct_end4_process_upgrade=""
          wahrwelt_direct_end4_process_runtime_hex=""
          wahrwelt_direct_end4_process_runtime_id=""

          direct_end4_bundle_args=(
            "$runtime_dir" \
            "${hyprDir}/hyprland.lua" \
            "${config.xdg.stateHome}/wahrwelt/end4-variant" \
            "$hypr_runtime_source" \
            "${./legacy-hypr-runtime/end4.lua}" \
            "${./legacy-hypr-runtime/end4-pc.lua}" \
            "${end4Runtime.profile}" \
            "${end4Runtime.lock}" \
            "${end4Runtime.idle}" \
            "${end4Runtime.launcher}" \
            "${end4Runtime.keybinds}" \
            "${end4Runtime.legacyLauncher}" \
            "${end4Runtime.legacyKeybinds}" \
            "${end4PCRuntime.profile}" \
            "${end4PCRuntime.lock}" \
            "${end4PCRuntime.idle}" \
            "${end4PCRuntime.launcher}" \
            "${end4PCRuntime.keybinds}" \
            "${end4PCRuntime.legacyLauncher}" \
            "${end4PCRuntime.legacyKeybinds}" \
            "${dotsRoot}/hypr/hyprland.lua" \
            "${dotsRoot}/hypr/end4-adapter.lua" \
            "${dotsRoot}/hypr/end4/launcher.lua" \
            "${end4RuntimeContract}" \
            "${hyprDir}/user/hyprland.lua" \
            "${hyprDir}/end4-adapter.lua" \
            "${hyprDir}/end4/launcher.lua" \
            "${hyprDir}/end4/hyprland.lua" \
            "${hyprDir}/end4/.wahrwelt-runtime-contract" \
            "${config.xdg.configHome}/quickshell/${end4Profile.quickshellConfig}/shell.qml" \
            "${config.xdg.configHome}/quickshell/${end4PCProfile.quickshellConfig}/shell.qml" \
            "${dotsRoot}/hypr/scripts/start-shell.sh"
          )

          if [ -n "''${DRY_RUN_CMD:-}" ]; then
            $DRY_RUN_CMD "$activation_helper" migrate-direct-end4-bundle \
              "''${direct_end4_bundle_args[@]}"
          else
            migration_result="$(
              "$activation_helper" migrate-direct-end4-bundle \
                "''${direct_end4_bundle_args[@]}"
            )"
            case "$migration_result" in
              current | legacy-upgrade=)
                ;;
              legacy-upgrade=*)
                migration_payload="''${migration_result#legacy-upgrade=}"
                case "$migration_payload" in
                  *';runtime-hex='*';runtime-id='*)
                    wahrwelt_direct_end4_process_upgrade="''${migration_payload%%;runtime-hex=*}"
                    migration_runtime="''${migration_payload#*;runtime-hex=}"
                    wahrwelt_direct_end4_process_runtime_hex="''${migration_runtime%%;runtime-id=*}"
                    wahrwelt_direct_end4_process_runtime_id="''${migration_runtime#*;runtime-id=}"
                    ;;
                  *)
                    echo "Direct End4 upgrade result has no runtime proof" >&2
                    exit 1
                    ;;
                esac
                ;;
              resume-upgrade-runtime-hex=*)
                migration_runtime="''${migration_result#resume-upgrade-runtime-hex=}"
                case "$migration_runtime" in
                  *';runtime-id='*)
                    wahrwelt_direct_end4_process_runtime_hex="''${migration_runtime%%;runtime-id=*}"
                    wahrwelt_direct_end4_process_runtime_id="''${migration_runtime#*;runtime-id=}"
                    ;;
                  *)
                    echo "Direct End4 resume result has no runtime identity" >&2
                    exit 1
                    ;;
                esac
                ;;
              *)
                echo "Invalid direct End4 runtime migration result" >&2
                exit 1
                ;;
            esac
            case "$wahrwelt_direct_end4_process_runtime_hex" in
              "")
                if [ -n "$wahrwelt_direct_end4_process_runtime_id" ] || \
                  [ -n "$wahrwelt_direct_end4_process_upgrade" ]; then
                  echo "Incomplete direct End4 runtime proof" >&2
                  exit 1
                fi
                ;;
              *[!0-9a-f]*)
                echo "Invalid direct End4 runtime proof" >&2
                exit 1
                ;;
              *)
                if [ "$(( ''${#wahrwelt_direct_end4_process_runtime_hex} % 2 ))" -ne 0 ]; then
                  echo "Invalid direct End4 runtime proof length" >&2
                  exit 1
                fi
                ;;
            esac
            if [ -n "$wahrwelt_direct_end4_process_runtime_hex" ]; then
              runtime_dev=""
              runtime_ino=""
              runtime_uid=""
              runtime_mode=""
              runtime_extra=""
              IFS=: read -r runtime_dev runtime_ino runtime_uid runtime_mode runtime_extra \
                <<<"$wahrwelt_direct_end4_process_runtime_id"
              case "$runtime_dev:$runtime_ino:$runtime_uid:$runtime_mode" in
                *[!0-9:]* | *::*)
                  echo "Invalid direct End4 runtime identity" >&2
                  exit 1
                  ;;
              esac
              if [ -z "$runtime_dev" ] || [ -z "$runtime_ino" ] || \
                [ -z "$runtime_uid" ] || [ -z "$runtime_mode" ] || \
                [ -n "$runtime_extra" ] || [ "$runtime_mode" != 700 ]; then
                echo "Invalid direct End4 runtime identity shape" >&2
                exit 1
              fi
            fi
          fi

          $DRY_RUN_CMD "$activation_helper" activate-runtime-dir \
            "$runtime_dir" \
            "$hypr_runtime_source" \
            "${legacyWahrweltRuntime}" \
            "${legacyHomeManagerWahrweltRuntime}" \
            "${legacySeededWahrweltRuntime}" \
            "${legacySeededUserRuntime}" \
            "${./legacy-hypr-runtime/end4.lua}" \
            "${./legacy-hypr-runtime/end4-pc.lua}" \
            "${./legacy-hypr-runtime/user-namespace-transition.lua}" \
            "${defaultShellProfileRuntime}" \
            "${defaultHyprlockRuntime}" \
            "${defaultHypridleRuntime}" \
            "${defaultShellLauncherRuntime}" \
            "${defaultShellKeybindRuntime}"
        '';

    liveSyncHyprShell =
      lib.hm.dag.entryAfter
        [
          "seedHyprShellRuntime"
          "linkGeneration"
        ]
        ''
          activation_helper="${runtimeActivation}/bin/wahrwelt-runtime-activation"
          hyprctl_path="$(command -v hyprctl 2>/dev/null || true)"
          run_live_shell_command() {
            if [ -n "''${wahrwelt_direct_end4_process_runtime_hex:-}" ]; then
              $DRY_RUN_CMD "$activation_helper" run-with-runtime-hex \
                "$wahrwelt_direct_end4_process_runtime_hex" \
                "$wahrwelt_direct_end4_process_runtime_id" \
                "$@"
            else
              $DRY_RUN_CMD "$@"
            fi
          }
          if [ -n "$hyprctl_path" ] && [ "''${hyprctl_path#/}" != "$hyprctl_path" ] && \
            [ -x "$hyprctl_path" ] && run_live_shell_command "$hyprctl_path" instances >/dev/null 2>&1; then
            hypr_version="$(run_live_shell_command "$hyprctl_path" version 2>/dev/null | awk 'NR == 1 { print $2 }')"
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
              if ! run_live_shell_command "$hyprctl_path" reload; then
                echo "Failed to reload the active Hyprland configuration" >&2
                exit 1
              fi
              if run_live_shell_command "${config.xdg.configHome}/hypr/scripts/start-shell.sh"; then
                start_shell_status=0
              else
                start_shell_status=$?
              fi
              if [ "$start_shell_status" -ne 0 ]; then
                echo "Failed to start the configured Wahrwelt shell profile" >&2
                exit 1
              fi
              wahrwelt_direct_end4_process_upgrade=""
              wahrwelt_direct_end4_process_runtime_hex=""
              wahrwelt_direct_end4_process_runtime_id=""
            else
              echo "Skipping live Hyprland reload; running Hyprland $hypr_version cannot load Lua runtime. Logout or reboot after switch." >&2
            fi
          fi
        '';
  };
}
