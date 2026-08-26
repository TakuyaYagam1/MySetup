{
  config,
  lib,
  pkgs,
  ...
}:

let
  home = config.home.homeDirectory;
  configHome = config.xdg.configHome;
  stateHome = config.xdg.stateHome;
  cacheHome = config.xdg.cacheHome;
  shellProfiles = import ../shells/profiles.nix;
  defaultProfile = shellProfiles.byId.${shellProfiles.defaultProfile};
  end4Profile = shellProfiles.byId.end4;
  end4PCProfile = shellProfiles.byId.end4-pc;
  hyprDir = "${configHome}/hypr";
  hyprRuntimeTarget = "${stateHome}/wahrwelt/hypr-runtime/hyprland.lua";
  hyprTopLevelTarget = "${configHome}/hypr/hyprland.lua";
  managedHyprUserAdapter = ../../../dots/hypr/hyprland.lua;
  hyprRuntimeActivation = pkgs.writeShellApplication {
    name = "wahrwelt-runtime-activation";
    runtimeInputs = [
      pkgs.coreutils
      pkgs.python3
    ];
    text = builtins.readFile ../shells/runtime-activation.sh;
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
  transitionHyprRuntime = ../shells/legacy-hypr-runtime/user-namespace-transition.lua;
  end4RuntimeContract = pkgs.writeText "end4-runtime-contract" "end4-adapter-v1\n";
  end4ShellProfileRuntime = pkgs.writeText "wahrwelt-shell-profile-default" ''
    -- Runtime shell launcher
    hl.on("hyprland.start", function()
        hl.exec_cmd("${hyprDir}/scripts/start-shell.sh")
    end)
  '';
  mkEnd4RuntimeBundle = profile: {
    profile = end4ShellProfileRuntime;
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
      require("${profile.adapter}").load({ profile = "${profile.id}", quickshell_config = "${configHome}/quickshell/${profile.quickshellConfig}" })
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
  stableHyprEntrypoint = pkgs.writeText "wahrwelt-stable-hypr-runtime-entrypoint" ''
    -- Generated by Wahrwelt: stable Hyprland Lua runtime entrypoint
    local home = os.getenv("HOME")
    if home == nil then
        error("HOME is not set; cannot locate Wahrwelt Hyprland runtime")
    end

    local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
    local hypr_root = config_home .. "/hypr"
    package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
    dofile(${builtins.toJSON hyprRuntimeTarget})
  '';
  hyprUserAdapterGuard = pkgs.writeShellApplication {
    name = "hypr-user-adapter-guard";
    runtimeInputs = [
      pkgs.coreutils
      pkgs.python3
    ];
    text = builtins.readFile ../shells/hypr-user-adapter-guard.sh;
  };
  legacyLinkGuard = pkgs.writeShellApplication {
    name = "legacy-link-guard";
    runtimeInputs = [ pkgs.python3 ];
    text = ''
      exec python3 ${./legacy-link-guard.py} "$@"
    '';
  };
  legacyCacheMerge = pkgs.writeShellApplication {
    name = "legacy-cache-merge";
    runtimeInputs = [
      pkgs.coreutils
      pkgs.findutils
      pkgs.python3
    ];
    text = ''
      export WAHRWELT_CACHE_TEMP_CREATOR=${./legacy-namespace-move.py}
      export WAHRWELT_CACHE_TEMP_PYTHON=${pkgs.python3}/bin/python3
    ''
    + builtins.readFile ./legacy-cache-merge.sh;
  };
  legacyNamespaceMove = pkgs.writeShellApplication {
    name = "legacy-namespace-move";
    runtimeInputs = [ pkgs.python3 ];
    text = ''
      exec python3 ${./legacy-namespace-move.py} "$@"
    '';
  };
  legacyMarkerMigrate = pkgs.writeShellApplication {
    name = "legacy-marker-migrate";
    runtimeInputs = [ pkgs.python3 ];
    text = ''
      exec python3 ${./legacy-marker-migrate.py} "$@"
    '';
  };
in
{
  home.activation.migrateWahrweltUserPaths = lib.hm.dag.entryBefore [ "checkLinkTargets" ] ''
    preflight_hypr_user_tree() {
      user="${configHome}/hypr/user"
      old_wahrwelt="${configHome}/hypr/wahrwelt"
      old_mysetup="${configHome}/hypr/mysetup"
      hypr_user_source=""
      hypr_namespace_token=""
      hypr_wahrwelt_namespace_token=""
      hypr_mysetup_namespace_token=""

      for path in "$user" "$old_wahrwelt" "$old_mysetup"; do
        if [ -L "$path" ] || { [ -e "$path" ] && [ ! -d "$path" ]; }; then
          echo "Wahrwelt migration conflict: unsupported Hypr user path: $path" >&2
          return 1
        fi
      done
      if [ -d "$old_wahrwelt" ] && [ -d "$old_mysetup" ]; then
        echo "Wahrwelt migration conflict: both legacy Hypr user directories exist" >&2
        return 1
      fi
      if [ -d "$old_wahrwelt" ]; then hypr_user_source="$old_wahrwelt"; fi
      if [ -d "$old_mysetup" ]; then hypr_user_source="$old_mysetup"; fi
      if [ -n "$hypr_user_source" ] && [ -e "$user" ]; then
        echo "Wahrwelt migration conflict: legacy and canonical Hypr user directories coexist" >&2
        return 1
      fi
      for tree in "$user" "$hypr_user_source"; do
        [ -n "$tree" ] && [ -d "$tree" ] || continue
        leaf_path="$tree/default.lua"
        if [ ! -L "$leaf_path" ] && [ -e "$leaf_path" ] && [ ! -f "$leaf_path" ]; then
          echo "Refusing non-regular Wahrwelt user config collision: $leaf_path" >&2
          return 1
        fi
        leaf_path="$tree/hyprland.lua"
        "${hyprUserAdapterGuard}/bin/hypr-user-adapter-guard" \
          "check" "$leaf_path" "${managedHyprUserAdapter}" "''${oldGenPath:-}"
      done
      hypr_wahrwelt_namespace_token="$("${legacyNamespaceMove}/bin/legacy-namespace-move" \
        check "$old_wahrwelt" "$user")"
      hypr_mysetup_namespace_token="$("${legacyNamespaceMove}/bin/legacy-namespace-move" \
        check "$old_mysetup" "$user")"
      if [ -n "$hypr_user_source" ]; then
        if [ "$hypr_user_source" = "$old_wahrwelt" ]; then
          hypr_namespace_token="$hypr_wahrwelt_namespace_token"
        else
          hypr_namespace_token="$hypr_mysetup_namespace_token"
        fi
      fi
    }

    preflight_move_tree() {
      old="$1"
      new="$2"
      token_variable="$3"

      namespace_token="$("${legacyNamespaceMove}/bin/legacy-namespace-move" check "$old" "$new")"
      printf -v "$token_variable" '%s' "$namespace_token"
    }

    preflight_merge_cache() {
      old="$1"
      new="$2"
      token_variable="$3"

      cache_token="$("${legacyCacheMerge}/bin/legacy-cache-merge" "check" "$old" "$new")"
      printf -v "$token_variable" '%s' "$cache_token"
    }

    preflight_old_links() {
      current_home_generation="${home}/.local/state/home-manager/gcroots/current-home"
      for old_link_spec in \
        "${configHome}/hypr/lib/mysetup.lua|.config/hypr/lib/mysetup.lua" \
        "${configHome}/quickshell/mysetup-shell-selector|.config/quickshell/mysetup-shell-selector" \
        "${configHome}/hypr/monitors.conf|.config/hypr/monitors.conf" \
        "${configHome}/hypr/workspaces.conf|.config/hypr/workspaces.conf"
      do
        old_link="''${old_link_spec%%|*}"
        expected_relative="''${old_link_spec#*|}"
        link_tokens+=("$("${legacyLinkGuard}/bin/legacy-link-guard" \
          "check" "$old_link" "$expected_relative" \
          "''${oldGenPath:-}" "$current_home_generation" "${configHome}")")
      done
    }

    quarantine_old_links() {
      current_home_generation="${home}/.local/state/home-manager/gcroots/current-home"

      link_index=0
      for old_link_spec in \
        "${configHome}/hypr/lib/mysetup.lua|.config/hypr/lib/mysetup.lua" \
        "${configHome}/quickshell/mysetup-shell-selector|.config/quickshell/mysetup-shell-selector" \
        "${configHome}/hypr/monitors.conf|.config/hypr/monitors.conf" \
        "${configHome}/hypr/workspaces.conf|.config/hypr/workspaces.conf"
      do
        old_link="''${old_link_spec%%|*}"
        expected_relative="''${old_link_spec#*|}"
        link_token="''${link_tokens[$link_index]}"
        link_index=$((link_index + 1))
        if [ -n "''${DRY_RUN_CMD:-}" ]; then
          $DRY_RUN_CMD "${legacyLinkGuard}/bin/legacy-link-guard" \
            "quarantine" "$old_link" "$expected_relative" \
            "''${oldGenPath:-}" "$current_home_generation" "${configHome}" "$link_token"
          continue
        fi
        link_recovery="$("${legacyLinkGuard}/bin/legacy-link-guard" \
          "quarantine" "$old_link" "$expected_relative" \
          "''${oldGenPath:-}" "$current_home_generation" "${configHome}" "$link_token")"
        if [ -n "$link_recovery" ]; then
          echo "Wahrwelt migration link recovery retained at $link_recovery"
        fi
      done
    }

    preflight_markers() {
      while IFS= read -r -d "" old_marker; do
        case "$old_marker" in
          "$old_wahrwelt"/* | "$old_mysetup"/* | "$user"/*)
            continue
            ;;
        esac
        marker_dir="$(${pkgs.coreutils}/bin/dirname "$old_marker")"
        new_marker="$marker_dir/.wahrwelt-managed.json"
        marker_paths+=("$old_marker")
        marker_tokens+=("$("${legacyMarkerMigrate}/bin/legacy-marker-migrate" \
          check "$old_marker" "$new_marker" "${configHome}" "${home}")")
      done < <(
        ${pkgs.findutils}/bin/find "${configHome}" \
          -name .mysetup-managed.json -print0 2>/dev/null
        if [ -d "${home}/.zen" ]; then
          ${pkgs.findutils}/bin/find "${home}/.zen" \
            -mindepth 3 -maxdepth 3 \
            -path '*/chrome/.mysetup-managed.json' \
            -print0 2>/dev/null
        fi
      )
    }

    migrate_markers() {
      for marker_index in "''${!marker_paths[@]}"; do
        old_marker="''${marker_paths[$marker_index]}"
        marker_dir="$(${pkgs.coreutils}/bin/dirname "$old_marker")"
        new_marker="$marker_dir/.wahrwelt-managed.json"
        if [ -n "''${DRY_RUN_CMD:-}" ]; then
          echo "Would migrate marker $old_marker -> $new_marker"
          continue
        fi
        "${legacyMarkerMigrate}/bin/legacy-marker-migrate" \
          migrate "$old_marker" "$new_marker" "${configHome}" "${home}" \
          "''${marker_tokens[$marker_index]}"
      done
    }

    verify_tree() {
      old="$1"
      new="$2"
      namespace_token="$3"

      "${legacyNamespaceMove}/bin/legacy-namespace-move" \
        verify "$old" "$new" "$namespace_token"
    }

    verify_hypr_user_tree() {
      user="${configHome}/hypr/user"
      "${legacyNamespaceMove}/bin/legacy-namespace-move" \
        verify "${configHome}/hypr/wahrwelt" "$user" "$hypr_wahrwelt_namespace_token"
      "${legacyNamespaceMove}/bin/legacy-namespace-move" \
        verify "${configHome}/hypr/mysetup" "$user" "$hypr_mysetup_namespace_token"
    }

    verify_cache() {
      "${legacyCacheMerge}/bin/legacy-cache-merge" \
        verify "${cacheHome}/mysetup" "${cacheHome}/wahrwelt" \
        "${cacheHome}" "$cache_token"
    }

    verify_old_links() {
      current_home_generation="${home}/.local/state/home-manager/gcroots/current-home"

      link_index=0
      for old_link_spec in \
        "${configHome}/hypr/lib/mysetup.lua|.config/hypr/lib/mysetup.lua" \
        "${configHome}/quickshell/mysetup-shell-selector|.config/quickshell/mysetup-shell-selector" \
        "${configHome}/hypr/monitors.conf|.config/hypr/monitors.conf" \
        "${configHome}/hypr/workspaces.conf|.config/hypr/workspaces.conf"
      do
        old_link="''${old_link_spec%%|*}"
        expected_relative="''${old_link_spec#*|}"
        original_token="''${link_tokens[$link_index]}"
        link_index=$((link_index + 1))
        current_token="$("${legacyLinkGuard}/bin/legacy-link-guard" \
          check "$old_link" "$expected_relative" \
          "''${oldGenPath:-}" "$current_home_generation" "${configHome}")"
        case "$original_token" in
          present\|*)
            IFS='|' read -r _ expected_root expected_parent _ _ <<< "$original_token"
            expected_token="absent|$expected_root|$expected_parent"
            [ "$current_token" = "$expected_token" ] || {
              echo "Wahrwelt migration conflict: legacy link appeared after quarantine: $old_link" >&2
              return 1
            }
            ;;
          absent\|*)
            [ "$current_token" = "$original_token" ] || {
              echo "Wahrwelt migration conflict: legacy link changed after preflight: $old_link" >&2
              return 1
            }
            ;;
          absent-parent\|*)
            original_root="''${original_token#*|}"
            current_root="''${current_token#*|}"
            current_root="''${current_root%%|*}"
            case "$current_token" in
              absent-parent\|* | absent\|*) ;;
              *)
                echo "Wahrwelt migration conflict: legacy link appeared after absent preflight: $old_link" >&2
                return 1
                ;;
            esac
            [ "$current_root" = "$original_root" ] || {
              echo "Wahrwelt migration conflict: legacy link root changed after preflight: ${configHome}" >&2
              return 1
            }
            ;;
          absent-root\|*)
            [ "$current_token" = "$original_token" ] || {
              echo "Wahrwelt migration conflict: legacy link root changed after preflight: ${configHome}" >&2
              return 1
            }
            ;;
          *)
            echo "Wahrwelt migration conflict: invalid legacy link preflight token" >&2
            return 1
            ;;
        esac
      done
    }

    verify_no_legacy_markers() {
      "${legacyNamespaceMove}/bin/legacy-namespace-move" \
        verify-markers "${configHome}" "$old_wahrwelt" "$old_mysetup" "$user"
      if [ -d "${home}/.zen" ]; then
        while IFS= read -r -d "" old_marker; do
          echo "Wahrwelt migration conflict: legacy marker appeared after preflight: $old_marker" >&2
          return 1
        done < <(
          ${pkgs.findutils}/bin/find "${home}/.zen" \
            -mindepth 3 -maxdepth 3 \
            -path '*/chrome/.mysetup-managed.json' \
            -print0 2>/dev/null
        )
      fi
    }

    commit_hypr_user_tree() {
      user="${configHome}/hypr/user"

      [ -n "''${hypr_user_source:-}" ] || return 0
      if [ -e "$user" ] || [ -L "$user" ]; then
        echo "Wahrwelt migration conflict: legacy and canonical Hypr user directories coexist" >&2
        return 1
      fi
      "${hyprUserAdapterGuard}/bin/hypr-user-adapter-guard" \
        "check" "$hypr_user_source/hyprland.lua" "${managedHyprUserAdapter}" "''${oldGenPath:-}"
      if [ -n "''${DRY_RUN_CMD:-}" ]; then
        echo "Would migrate Hypr user namespace $hypr_user_source -> $user"
        return 0
      fi
      revalidate_prepared_hypr_runtimes
      revalidate_static_hypr_top_level_runtime
      "${legacyNamespaceMove}/bin/legacy-namespace-move" \
        move "$hypr_user_source" "$user" "$hypr_namespace_token"
    }

    classify_hypr_top_level_runtime() {
      hypr_top_runtime_source_index=
      if [ ! -e "${hyprTopLevelTarget}" ] && [ ! -L "${hyprTopLevelTarget}" ]; then
        hypr_top_runtime_token="absent"
        hypr_top_runtime_kind="absent"
        return 0
      fi
      hypr_top_runtime_token="$(
        "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
          classify-top-level-runtime \
          "${hyprTopLevelTarget}" \
          "''${oldGenPath:-}" \
          ".config/hypr/hyprland.lua" \
          "${stableHyprEntrypoint}" \
          "${canonicalHyprRuntime}" \
          "${transitionHyprRuntime}" \
          "${legacyWahrweltRuntime}" \
          "${legacyHomeManagerWahrweltRuntime}" \
          "${legacySeededWahrweltRuntime}" \
          "${legacySeededUserRuntime}" \
          "${../shells/legacy-hypr-runtime/end4.lua}" \
          "${../shells/legacy-hypr-runtime/end4-pc.lua}"
      )"
      hypr_top_runtime_kind="''${hypr_top_runtime_token%%|*}"
      case "$hypr_top_runtime_kind" in
        direct-regular)
          IFS='|' read -r _ hypr_top_runtime_source_index _ <<< "$hypr_top_runtime_token"
          case "$hypr_top_runtime_source_index" in
            1 | 2 | 3 | 4 | 5 | 6 | 7 | 8) ;;
            *)
              echo "Wahrwelt top-level Hyprland runtime ownership collision: invalid source index" >&2
              return 1
              ;;
          esac
          ;;
        absent | stable-link | stable-regular) ;;
        *)
          echo "Wahrwelt top-level Hyprland runtime ownership collision: invalid classifier result" >&2
          return 1
          ;;
      esac
    }

    classify_hypr_state_runtime() {
      hypr_state_runtime_token=absent
      hypr_state_runtime_kind=absent
      hypr_state_runtime_source_index=
      if [ ! -e "${hyprRuntimeTarget}" ] && [ ! -L "${hyprRuntimeTarget}" ]; then
        return 0
      fi
      hypr_state_runtime_token="$(
        "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
          classify-direct-runtime \
          "${hyprRuntimeTarget}" \
          "${canonicalHyprRuntime}" \
          "${transitionHyprRuntime}" \
          "${legacyWahrweltRuntime}" \
          "${legacyHomeManagerWahrweltRuntime}" \
          "${legacySeededWahrweltRuntime}" \
          "${legacySeededUserRuntime}" \
          "${../shells/legacy-hypr-runtime/end4.lua}" \
          "${../shells/legacy-hypr-runtime/end4-pc.lua}"
      )"
      hypr_state_runtime_kind="''${hypr_state_runtime_token%%|*}"
      case "$hypr_state_runtime_kind" in
        direct-regular)
          IFS='|' read -r _ hypr_state_runtime_source_index _ <<< "$hypr_state_runtime_token"
          case "$hypr_state_runtime_source_index" in
            1 | 2 | 3 | 4 | 5 | 6 | 7 | 8) ;;
            *)
              echo "Wahrwelt Hyprland runtime ownership collision: invalid source index" >&2
              return 1
              ;;
          esac
          ;;
        absent) ;;
        *)
          echo "Wahrwelt Hyprland runtime ownership collision: invalid classifier result" >&2
          return 1
          ;;
      esac
    }

    is_direct_end4_runtime_index() {
      case "$1" in
        7 | 8) return 0 ;;
        *) return 1 ;;
      esac
    }

    preflight_hypr_end4_runtime_bundle() {
      "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
        preflight-direct-end4-bundle \
        "${stateHome}/wahrwelt/hypr-runtime" \
        "${hyprTopLevelTarget}" \
        "${stateHome}/wahrwelt/end4-variant" \
        "${canonicalHyprRuntime}" \
        "${../shells/legacy-hypr-runtime/end4.lua}" \
        "${../shells/legacy-hypr-runtime/end4-pc.lua}" \
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
        "${managedHyprUserAdapter}" \
        "${../../../dots/hypr/end4-adapter.lua}" \
        "${../../../dots/hypr/end4/launcher.lua}" \
        "${end4RuntimeContract}"
    }

    direct_end4_runtime_source() {
      case "$1" in
        7) printf '%s\n' "${../shells/legacy-hypr-runtime/end4.lua}" ;;
        8) printf '%s\n' "${../shells/legacy-hypr-runtime/end4-pc.lua}" ;;
        *) return 1 ;;
      esac
    }

    stage_top_direct_end4_state_runtime() {
      is_direct_end4_runtime_index "''${hypr_top_runtime_source_index:-}" || return 0
      is_direct_end4_runtime_index "''${hypr_state_runtime_source_index:-}" && return 0

      staged_source="$(direct_end4_runtime_source "$hypr_top_runtime_source_index")"
      if [ -n "''${DRY_RUN_CMD:-}" ]; then
        $DRY_RUN_CMD "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
          stage-known-runtime \
          "${hyprRuntimeTarget}" \
          "$staged_source" \
          "${canonicalHyprRuntime}" \
          "${transitionHyprRuntime}" \
          "${legacyWahrweltRuntime}" \
          "${legacyHomeManagerWahrweltRuntime}" \
          "${legacySeededWahrweltRuntime}" \
          "${legacySeededUserRuntime}" \
          "${../shells/legacy-hypr-runtime/end4.lua}" \
          "${../shells/legacy-hypr-runtime/end4-pc.lua}"
        return 0
      fi
      "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
        stage-known-runtime \
        "${hyprRuntimeTarget}" \
        "$staged_source" \
        "${canonicalHyprRuntime}" \
        "${transitionHyprRuntime}" \
        "${legacyWahrweltRuntime}" \
        "${legacyHomeManagerWahrweltRuntime}" \
        "${legacySeededWahrweltRuntime}" \
        "${legacySeededUserRuntime}" \
        "${../shells/legacy-hypr-runtime/end4.lua}" \
        "${../shells/legacy-hypr-runtime/end4-pc.lua}"
      classify_hypr_state_runtime
      [ "$hypr_state_runtime_source_index" = "$hypr_top_runtime_source_index" ] || {
        echo "Wahrwelt Hyprland runtime ownership collision: staged End4 provenance changed" >&2
        return 1
      }
    }

    revalidate_static_hypr_top_level_runtime() {
      [ "$hypr_top_runtime_kind" != "direct-regular" ] || return 0
      previous_token="$hypr_top_runtime_token"
      classify_hypr_top_level_runtime
      [ "$hypr_top_runtime_token" = "$previous_token" ] || {
        echo "Wahrwelt top-level Hyprland runtime ownership collision: entrypoint changed after preflight" >&2
        return 1
      }
    }

    exact_hypr_runtime_token() {
      runtime_target="$1"
      runtime_source="$2"
      if [ ! -e "$runtime_target" ] && [ ! -L "$runtime_target" ]; then
        echo "absent"
        return 0
      fi
      "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
        token-exact-runtime "$runtime_target" "$runtime_source"
    }

    capture_prepared_hypr_runtime_tokens() {
      prepared_state_runtime_token=
      prepared_state_runtime_source=
      prepared_top_runtime_token=
      prepared_top_runtime_source=
      [ -z "''${DRY_RUN_CMD:-}" ] || return 0
      [ "''${hypr_user_source:-}" = "${configHome}/hypr/wahrwelt" ] || return 0
      prepared_state_runtime_source="${transitionHyprRuntime}"
      if is_direct_end4_runtime_index "''${hypr_state_runtime_source_index:-}"; then
        prepared_state_runtime_source="$(direct_end4_runtime_source "$hypr_state_runtime_source_index")"
      fi
      prepared_state_runtime_token="$(
        exact_hypr_runtime_token "${hyprRuntimeTarget}" "$prepared_state_runtime_source"
      )"
      if [ "$hypr_top_runtime_kind" = "direct-regular" ]; then
        prepared_top_runtime_source="${transitionHyprRuntime}"
        if is_direct_end4_runtime_index "''${hypr_top_runtime_source_index:-}"; then
          prepared_top_runtime_source="$(direct_end4_runtime_source "$hypr_top_runtime_source_index")"
        fi
        prepared_top_runtime_token="$(
          exact_hypr_runtime_token "${hyprTopLevelTarget}" "$prepared_top_runtime_source"
        )"
      fi
    }

    revalidate_prepared_hypr_runtimes() {
      [ -n "''${prepared_state_runtime_token:-}" ] || return 0
      current_state_runtime_token="$(
        exact_hypr_runtime_token "${hyprRuntimeTarget}" "$prepared_state_runtime_source"
      )" || {
        echo "Wahrwelt Hyprland runtime ownership collision: state runtime changed after transition preparation" >&2
        return 1
      }
      [ "$current_state_runtime_token" = "$prepared_state_runtime_token" ] || {
        echo "Wahrwelt Hyprland runtime ownership collision: state runtime changed after transition preparation" >&2
        return 1
      }
      if [ -n "''${prepared_top_runtime_token:-}" ]; then
        current_top_runtime_token="$(
          exact_hypr_runtime_token "${hyprTopLevelTarget}" "$prepared_top_runtime_source"
        )" || {
          echo "Wahrwelt top-level Hyprland runtime ownership collision: runtime changed after transition preparation" >&2
          return 1
        }
        [ "$current_top_runtime_token" = "$prepared_top_runtime_token" ] || {
          echo "Wahrwelt top-level Hyprland runtime ownership collision: runtime changed after transition preparation" >&2
          return 1
        }
      fi
    }

    migrate_hypr_user_runtime() {
      runtime_target="$1"
      desired_runtime="$2"
      shift 2

      if [ ! -e "$runtime_target" ] && [ ! -L "$runtime_target" ]; then
        return 0
      fi
      if [ -n "''${DRY_RUN_CMD:-}" ]; then
        $DRY_RUN_CMD "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
          migrate-known-runtime "$runtime_target" "$desired_runtime" "$@"
        return 0
      fi
      "${hyprRuntimeActivation}/bin/wahrwelt-runtime-activation" \
        migrate-known-runtime "$runtime_target" "$desired_runtime" "$@"
    }

    migrate_hypr_runtime_targets() {
      desired_runtime="$1"
      shift
      if ! is_direct_end4_runtime_index "''${hypr_state_runtime_source_index:-}"; then
        migrate_hypr_user_runtime "${hyprRuntimeTarget}" "$desired_runtime" "$@"
      fi
      if [ "$hypr_top_runtime_kind" = "direct-regular" ] \
        && ! is_direct_end4_runtime_index "''${hypr_top_runtime_source_index:-}"; then
        migrate_hypr_user_runtime "${hyprTopLevelTarget}" "$desired_runtime" "$@"
      fi
    }

    prepare_hypr_user_runtime_transition() {
      if [ "''${hypr_user_source:-}" = "${configHome}/hypr/wahrwelt" ]; then
        migrate_hypr_runtime_targets \
          "${transitionHyprRuntime}" \
          "${canonicalHyprRuntime}" \
          "${legacyWahrweltRuntime}" \
          "${legacyHomeManagerWahrweltRuntime}" \
          "${legacySeededWahrweltRuntime}" \
          "${legacySeededUserRuntime}" \
          "${../shells/legacy-hypr-runtime/end4.lua}" \
          "${../shells/legacy-hypr-runtime/end4-pc.lua}"
        return 0
      fi
      if [ -z "''${hypr_user_source:-}" ] && [ -d "${configHome}/hypr/user" ]; then
        finalize_hypr_user_runtime
      fi
    }

    finalize_hypr_user_runtime() {
      [ -z "''${DRY_RUN_CMD:-}" ] || return 0
      [ -d "${configHome}/hypr/user" ] || return 0
      [ ! -e "${configHome}/hypr/wahrwelt" ] && [ ! -L "${configHome}/hypr/wahrwelt" ] || return 0
      migrate_hypr_runtime_targets \
        "${canonicalHyprRuntime}" \
        "${transitionHyprRuntime}" \
        "${legacyWahrweltRuntime}" \
        "${legacyHomeManagerWahrweltRuntime}" \
        "${legacySeededWahrweltRuntime}" \
        "${legacySeededUserRuntime}" \
        "${../shells/legacy-hypr-runtime/end4.lua}" \
        "${../shells/legacy-hypr-runtime/end4-pc.lua}"
    }

    move_tree() {
      old="$1"
      new="$2"
      namespace_token="$3"

      if [ -n "''${DRY_RUN_CMD:-}" ]; then
        echo "Would migrate namespace $old -> $new"
        return 0
      fi
      "${legacyNamespaceMove}/bin/legacy-namespace-move" move "$old" "$new" "$namespace_token"
    }

    merge_cache() {
      old="$1"
      new="$2"
      cache_token="$3"

      if [ -n "''${DRY_RUN_CMD:-}" ]; then
        $DRY_RUN_CMD "${legacyCacheMerge}/bin/legacy-cache-merge" \
          "merge" "$old" "$new" "${cacheHome}" "$cache_token"
        return 0
      fi
      cache_recovery="$("${legacyCacheMerge}/bin/legacy-cache-merge" \
        "merge" "$old" "$new" "${cacheHome}" "$cache_token")"
      if [ -n "$cache_recovery" ]; then
        echo "Wahrwelt migration cache recovery retained at $cache_recovery"
      fi
    }

    preflight_hypr_user_tree
    config_namespace_token=
    state_namespace_token=
    cache_token=
    marker_paths=()
    marker_tokens=()
    link_tokens=()
    hypr_top_runtime_token=
    hypr_top_runtime_kind=
    hypr_top_runtime_source_index=
    hypr_state_runtime_token=
    hypr_state_runtime_kind=
    hypr_state_runtime_source_index=
    prepared_state_runtime_token=
    prepared_state_runtime_source=
    prepared_top_runtime_token=
    prepared_top_runtime_source=
    preflight_move_tree "${configHome}/mysetup" "${configHome}/wahrwelt" config_namespace_token
    preflight_move_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt" state_namespace_token
    preflight_merge_cache "${cacheHome}/mysetup" "${cacheHome}/wahrwelt" cache_token
    preflight_old_links
    preflight_markers
    preflight_hypr_end4_runtime_bundle
    classify_hypr_state_runtime
    classify_hypr_top_level_runtime
    stage_top_direct_end4_state_runtime
    prepare_hypr_user_runtime_transition
    capture_prepared_hypr_runtime_tokens
    quarantine_old_links
    commit_hypr_user_tree
    finalize_hypr_user_runtime
    revalidate_static_hypr_top_level_runtime

    move_tree "${configHome}/mysetup" "${configHome}/wahrwelt" "$config_namespace_token"
    move_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt" "$state_namespace_token"
    merge_cache "${cacheHome}/mysetup" "${cacheHome}/wahrwelt" "$cache_token"
    migrate_markers
    if [ -z "''${DRY_RUN_CMD:-}" ]; then
      verify_hypr_user_tree
      verify_tree "${configHome}/mysetup" "${configHome}/wahrwelt" "$config_namespace_token"
      verify_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt" "$state_namespace_token"
      verify_cache
      verify_no_legacy_markers
      verify_old_links
    fi

  '';
}
