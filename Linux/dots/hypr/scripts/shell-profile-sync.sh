#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2154

absolute_symlink_target() {
  local link="$1"
  local target

  [ -L "$link" ] || return 1
  target="$(readlink -- "$link" 2>/dev/null || true)"
  [ -n "$target" ] || return 1
  case "$target" in
    /*) ;;
    *) target="$(dirname -- "$link")/$target" ;;
  esac
  realpath -m -s -- "$target"
}

end4_source_from_current_generation() {
  local gcroot generation source resolved

  gcroot="$HOME/.local/state/home-manager/gcroots/current-home"
  [ -L "$gcroot" ] || return 1
  generation="$(readlink -f -- "$gcroot" 2>/dev/null || true)"
  [ -n "$generation" ] || return 1
  source="$generation/home-files/.config/hypr/end4"
  [ -L "$source" ] || return 1
  resolved="$(readlink -f -- "$source" 2>/dev/null || true)"
  [ -n "$resolved" ] && [ -d "$resolved" ] && [ -f "$resolved/hyprland.lua" ] || return 1
  printf '%s' "$resolved"
}

end4_source_from_immutable_home_manager_files() {
  local target="$1"
  local source relative store_item suffix resolved

  source="$(absolute_symlink_target "$target" 2>/dev/null || true)"
  case "$source" in
    /nix/store/*) ;;
    *) return 1 ;;
  esac
  relative="${source#/nix/store/}"
  store_item="${relative%%/*}"
  suffix="${relative#*/}"
  [[ "$store_item" =~ ^[0-9a-df-np-sv-z]{32}-home-manager-files$ ]] || return 1
  [ "$suffix" = ".config/hypr/end4" ] || return 1
  resolved="$(readlink -f -- "$source" 2>/dev/null || true)"
  [[ "$resolved" =~ ^/nix/store/[0-9a-df-np-sv-z]{32}-end4-hypr-validated$ ]] || return 1
  [ "$(stat -Lc '%u:%a' -- "$resolved" 2>/dev/null || true)" = 0:555 ] || return 1
  [ "$(stat -Lc '%u:%a' -- "$resolved/hyprland.lua" 2>/dev/null || true)" = 0:444 ] || return 1
  [ "$(stat -Lc '%u:%a' -- "$resolved/.wahrwelt-runtime-contract" 2>/dev/null || true)" = 0:444 ] || return 1
  [ "$(sha256sum -- "$resolved/.wahrwelt-runtime-contract" 2>/dev/null | awk '{print $1}')" = \
    1d383cff084bb68da410e31c70fb6404630708dd10169960bd4441cfb2e81091 ] || return 1
  printf '%s' "$resolved"
}

validate_end4_profile_tree() {
  local dir target target_source current_source store_source

  [ "$(wahrwelt_shell_family "$profile")" = "end4" ] || return 0

  dir="$(hypr_dir)"
  target="$dir/end4"
  if [ ! -L "$target" ]; then
    log "end4 profile path is not a Home Manager symlink: $target"
    return 1
  fi
  target_source="$(readlink -f -- "$target" 2>/dev/null || true)"
  if [ -z "$target_source" ] || [ ! -d "$target_source" ] || [ ! -f "$target_source/hyprland.lua" ]; then
    log "end4 profile symlink is broken or incomplete: $target"
    return 1
  fi

  current_source="$(end4_source_from_current_generation 2>/dev/null || true)"
  if [ -n "$current_source" ] && [ "$target_source" = "$current_source" ]; then
    return 0
  fi

  store_source="$(end4_source_from_immutable_home_manager_files "$target" 2>/dev/null || true)"
  if [ -n "$store_source" ] && [ "$target_source" = "$store_source" ]; then
    return 0
  fi

  log "end4 profile symlink is not owned by the current Home Manager generation: $target"
  return 1
}

if ! declare -p wahrwelt_exact_path_guard_types >/dev/null 2>&1; then
  declare -A wahrwelt_exact_path_guard_types=()
  declare -A wahrwelt_exact_path_guard_identities=()
  declare -A wahrwelt_exact_path_guard_parents=()
fi

wahrwelt_fs_helper_path() {
  local candidate="${WAHRWELT_FS_HELPER:-}"

  if [ -n "$candidate" ] && [ -x "$candidate" ]; then
    printf '%s' "$candidate"
    return 0
  fi
  command -v wahrwelt-fs-helper 2>/dev/null
}

wahrwelt_fs_begin() {
  local kind="$1"
  local root="$2"
  local helper
  shift 2

  helper="$(wahrwelt_fs_helper_path)" || {
    log "wahrwelt-fs-helper is unavailable"
    return 1
  }
  "$helper" runtime-begin --root "$root" --kind "$kind" "$@"
}

wahrwelt_fs_write() {
  local transaction="$1"
  local target="$2"
  local content="$3"
  local helper

  helper="$(wahrwelt_fs_helper_path)" || return 1
  printf '%s\n' "$content" |
    "$helper" runtime-write --transaction "$transaction" --target "$target" --mode 0644
}

wahrwelt_fs_remove() {
  local transaction="$1"
  local target="$2"
  local helper

  helper="$(wahrwelt_fs_helper_path)" || return 1
  "$helper" runtime-remove --transaction "$transaction" --target "$target"
}

wahrwelt_fs_rollback() {
  local helper

  [ -n "$1" ] || return 0
  helper="$(wahrwelt_fs_helper_path)" || return 1
  "$helper" runtime-rollback "$1"
}

wahrwelt_fs_commit() {
  local helper

  [ -n "$1" ] || return 0
  helper="$(wahrwelt_fs_helper_path)" || return 1
  "$helper" runtime-commit "$1"
}

wahrwelt_fs_scavenge() {
  local root="$1"
  local kind="$2"
  local helper

  helper="$(wahrwelt_fs_helper_path)" || return 1
  "$helper" runtime-scavenge --root "$root" --kind "$kind"
}

wahrwelt_path_in_list() {
  local expected="$1"
  shift
  local path

  for path in "$@"; do
    [ "$path" = "$expected" ] && return 0
  done
  return 1
}

wahrwelt_transaction_for_path() {
  local path="$1"

  if wahrwelt_path_in_list "$path" "${runtime_bundle_path_list[@]:-}"; then
    [ -n "${runtime_bundle_snapshot_dir:-}" ] || return 1
    printf '%s' "$runtime_bundle_snapshot_dir"
    return 0
  fi
  if wahrwelt_path_in_list "$path" "${state_path_list[@]:-}"; then
    [ -n "${state_snapshot_dir:-}" ] || return 1
    printf '%s' "$state_snapshot_dir"
    return 0
  fi
  return 1
}

runtime_path_kind() {
  local path="$1"

  if [ -L "$path" ]; then
    printf '%s' symlink
  elif [ -f "$path" ]; then
    printf '%s' regular
  elif [ -e "$path" ]; then
    printf '%s' other
  else
    printf '%s' absent
  fi
}

runtime_path_identity() {
  local path="$1"

  [ -e "$path" ] || [ -L "$path" ] || return 1
  stat -c '%d:%i' -- "$path" 2>/dev/null
}

runtime_regular_inode() {
  local path="$1"
  local node digest

  [ -f "$path" ] || return 1
  node="$(stat -Lc '%d:%i:%h' -- "$path" 2>/dev/null || true)"
  digest="$(sha256sum -- "$path" 2>/dev/null | awk '{print $1}' || true)"
  [ -n "$node" ] && [ -n "$digest" ] || return 1
  printf '%s:%s' "$node" "$digest"
}

runtime_regular_is_private() {
  local path="$1"

  [ -f "$path" ] &&
    [ "$(stat -Lc %h -- "$path" 2>/dev/null || true)" = 1 ]
}

runtime_path_matches_content() {
  local path="$1"
  local content="$2"
  local helper

  helper="$(wahrwelt_fs_helper_path)" || return 1
  printf '%s\n' "$content" |
    "$helper" runtime-matches --target "$path" --mode 0644 2>/dev/null
}

stable_runtime_entrypoint_matches() {
  local path="$1"
  local content="$2"
  local gcroot generation expected resolved resolved_after

  runtime_path_matches_content "$path" "$content" && return 0
  [ -L "$path" ] || return 1

  gcroot="$HOME/.local/state/home-manager/gcroots/current-home"
  [ -L "$gcroot" ] || return 1
  generation="$(readlink -f -- "$gcroot" 2>/dev/null || true)"
  [ -n "$generation" ] || return 1
  expected="$generation/home-files/.config/hypr/hyprland.lua"
  resolved="$(readlink -f -- "$path" 2>/dev/null || true)"
  [ -n "$resolved" ] && [ -f "$resolved" ] || return 1
  [ "$resolved" = "$(readlink -f -- "$expected" 2>/dev/null || true)" ] || return 1
  printf '%s\n' "$content" | cmp -s - "$resolved" || return 1
  resolved_after="$(readlink -f -- "$path" 2>/dev/null || true)"
  [ "$resolved_after" = "$resolved" ]
}

runtime_state_identity() {
  local path="$1"

  case "$(runtime_path_kind "$path")" in
    regular) runtime_regular_inode "$path" ;;
    symlink) runtime_path_identity "$path" ;;
    absent) return 0 ;;
    *) return 1 ;;
  esac
}

runtime_parent_identity() {
  local parent

  parent="$(dirname -- "$1")"
  [ -d "$parent" ] && [ ! -L "$parent" ] || return 1
  runtime_path_identity "$parent"
}

wahrwelt_capture_exact_path_guards() {
  local path type identity parent

  wahrwelt_exact_path_guard_types=()
  wahrwelt_exact_path_guard_identities=()
  wahrwelt_exact_path_guard_parents=()
  for path in "$@"; do
    type="$(runtime_path_kind "$path")"
    case "$type" in
      absent) identity="" ;;
      regular)
        if ! runtime_regular_is_private "$path"; then
          log "refusing hardlinked state path at transaction begin: $path"
          return 1
        fi
        identity="$(runtime_state_identity "$path" 2>/dev/null || true)"
        [ -n "$identity" ] || return 1
        ;;
      symlink)
        identity="$(runtime_state_identity "$path" 2>/dev/null || true)"
        [ -n "$identity" ] || return 1
        ;;
      *)
        log "refusing non-regular state path at transaction begin: $path"
        return 1
        ;;
    esac
    parent="$(runtime_parent_identity "$path" 2>/dev/null || true)"
    if [ -z "$parent" ]; then
      log "state parent was absent or unsafe at transaction begin: $(dirname -- "$path")"
      return 1
    fi
    wahrwelt_exact_path_guard_types["$path"]="$type"
    wahrwelt_exact_path_guard_identities["$path"]="$identity"
    wahrwelt_exact_path_guard_parents["$path"]="$parent"
  done
}

wahrwelt_verify_exact_path_guards() {
  local path expected_type expected_identity expected_parent
  local current_type current_identity current_parent

  for path in "$@"; do
    if [ -z "${wahrwelt_exact_path_guard_types[$path]+x}" ]; then
      log "state path lacks a transaction-begin ownership guard: $path"
      return 1
    fi
    expected_type="${wahrwelt_exact_path_guard_types[$path]}"
    expected_identity="${wahrwelt_exact_path_guard_identities[$path]}"
    expected_parent="${wahrwelt_exact_path_guard_parents[$path]}"
    current_type="$(runtime_path_kind "$path")"
    current_identity="$(runtime_state_identity "$path" 2>/dev/null || true)"
    current_parent="$(runtime_parent_identity "$path" 2>/dev/null || true)"
    if [ "$current_type" != "$expected_type" ] ||
      [ "$current_identity" != "$expected_identity" ] ||
      [ "$current_parent" != "$expected_parent" ]; then
      log "state path changed after transaction begin; preserving concurrent winner: $path"
      return 1
    fi
  done
}

write_regular_file() {
  local path="$1"
  local content="$2"
  local transaction opened parent

  transaction="$(wahrwelt_transaction_for_path "$path" 2>/dev/null)" || {
    log "runtime publication is outside an active fd transaction: $path"
    return 1
  }
  if declare -F wahrwelt_after_runtime_preflight_hook >/dev/null 2>&1; then
    wahrwelt_after_runtime_preflight_hook "$path" "$(dirname -- "$path")" || return 1
  fi
  wahrwelt_fs_write "$transaction" "$path" "$content" || return 1
  if declare -F wahrwelt_after_runtime_publication_hook >/dev/null 2>&1; then
    opened="$(runtime_state_identity "$path" 2>/dev/null || true)"
    parent="$(runtime_parent_identity "$path" 2>/dev/null || true)"
    wahrwelt_after_runtime_publication_hook "$path" "$opened" "$parent" || return $?
  fi
}

write_regular_file_if_changed() {
  local path="$1"
  local content="$2"

  runtime_path_matches_content "$path" "$content" && return 0
  write_regular_file "$path" "$content"
}

runtime_file() {
  wahrwelt_runtime_file "$1"
}

runtime_shell_profile_content() {
  local dir

  dir="$(hypr_dir)"
  printf '%s' "-- Runtime shell launcher
hl.on(\"hyprland.start\", function()
    hl.exec_cmd(\"$dir/scripts/start-shell.sh\")
end)"
}

runtime_shell_launcher_content() {
  local launcher_module launcher_profile

  launcher_module="$(wahrwelt_shell_launcher_module "$profile")" || return 1
  launcher_profile="$profile"
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    launcher_profile=end4
  fi
  printf '%s' "-- Active shell launcher profile: $launcher_profile
require(\"$launcher_module\")"
}

runtime_shell_keybinds_content() {
  local adapter quickshell_path

  adapter="$(wahrwelt_shell_adapter "$profile")" || return 1
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    quickshell_path="$(wahrwelt_end4_quickshell_path "$profile")" || return 1
    printf '%s' "-- Wahrwelt shell adapter: $profile
require(\"$adapter\").load({ profile = \"$profile\", quickshell_config = \"$quickshell_path\" })"
    return 0
  fi
  printf '%s' "-- Wahrwelt shell adapter: $profile
require(\"$adapter\")"
}

runtime_hyprlock_content() {
  local dir

  dir="$(hypr_dir)"
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    printf '%s' "# Active Hyprlock profile: end4
source = $dir/end4/hyprlock.conf"
    return 0
  fi
  printf '%s' '# Active Hyprlock profile: shell-managed
# Caelestia and Noctalia use shell-native lock flows.'
}

runtime_hypridle_content() {
  local dir

  dir="$(hypr_dir)"
  if [ "$(wahrwelt_shell_family "$profile")" = end4 ]; then
    printf '%s' "# Active Hypridle profile: end4
source = $dir/end4/hypridle.conf"
    return 0
  fi
  printf '%s' '# Active Hypridle profile: shell-managed
# Caelestia and Noctalia use shell-native idle flows.'
}

ensure_wahrwelt_entrypoint() {
  local dir target

  dir="$(hypr_dir)"
  target="$dir/user/hyprland.lua"

  if [ -f "$target" ]; then
    return 0
  fi

  log "wahrwelt hypr entrypoint missing; rebuild or rerun wahrwelt apply: $target"
  return 1
}

sync_shell_launcher() {
  local content

  mkdir -p -- "$hypr_runtime_dir"
  content="$(runtime_shell_profile_content)" || return 1
  write_regular_file_if_changed "$(runtime_file shell-profile.lua)" "$content"
}

sync_shell_launcher_bindings() {
  local content dir profile_launcher launcher_module

  dir="$(hypr_dir)"
  launcher_module="$(wahrwelt_shell_launcher_module "$profile")" || return 1
  profile_launcher="$dir/${launcher_module%%.*}/launcher.lua"

  if [ ! -f "$profile_launcher" ]; then
    log "shell launcher profile missing: $profile_launcher"
    return 1
  fi

  content="$(runtime_shell_launcher_content)" || return 1
  write_regular_file_if_changed "$(runtime_file shell-launcher.lua)" "$content"
}

sync_shell_keybinds() {
  local content

  content="$(runtime_shell_keybinds_content)" || return 1
  write_regular_file_if_changed "$(runtime_file shell-keybinds.lua)" "$content"
}

sync_hypr_entrypoint() {
  local content

  content="$(wahrwelt_print_canonical_runtime_entrypoint)" || return 1

  write_regular_file_if_changed "$(runtime_file hyprland.lua)" "$content"
}

sync_hypr_lock_stack() {
  local content dir hyprlock_target hypridle_target

  dir="$(hypr_dir)"

  if [ "$(wahrwelt_shell_family "$profile")" = "end4" ]; then
    hyprlock_target="$dir/end4/hyprlock.conf"
    hypridle_target="$dir/end4/hypridle.conf"

    if [ ! -f "$hyprlock_target" ]; then
      log "hyprlock entrypoint missing for profile=$profile path=$hyprlock_target"
      return 1
    fi

    if [ ! -f "$hypridle_target" ]; then
      log "hypridle entrypoint missing for profile=$profile path=$hypridle_target"
      return 1
    fi

    content="$(runtime_hyprlock_content)" || return 1
    write_regular_file_if_changed "$(runtime_file hyprlock.conf)" "$content" || return 1
    content="$(runtime_hypridle_content)" || return 1
    write_regular_file_if_changed "$(runtime_file hypridle.conf)" "$content"
    return $?
  fi

  content="$(runtime_hyprlock_content)" || return 1
  write_regular_file_if_changed "$(runtime_file hyprlock.conf)" "$content" || return 1
  content="$(runtime_hypridle_content)" || return 1
  write_regular_file_if_changed "$(runtime_file hypridle.conf)" "$content"
}

sync_stable_lua_entrypoint() {
  local dir target content

  dir="$(hypr_dir)"
  target="$dir/hyprland.lua"
  if [ -r "$target" ] &&
    wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua" | cmp -s - "$target"; then
    return 0
  fi

  if [ -e "$target" ] || [ -L "$target" ]; then
    if [ -L "$target" ] || [ ! -f "$target" ]; then
      log "refusing unowned top-level Hyprland runtime collision: $target"
      return 1
    fi
    if ! wahrwelt_is_canonical_entrypoint "$target" &&
      ! wahrwelt_is_legacy_user_entrypoint "$target" &&
      ! wahrwelt_is_legacy_direct_end4_entrypoint "$target" "${XDG_CONFIG_HOME:-$HOME/.config}"; then
      log "refusing unowned top-level Hyprland runtime collision: $target"
      return 1
    fi
  fi

  content="$(wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua")" || return 1
  write_regular_file "$target" "$content"
}

legacy_hyprland_runtime_paths() {
  local dir

  dir="$(hypr_dir)"
  printf '%s\n' \
    "$dir/hyprland.conf" \
    "$dir/shell-profile.conf" \
    "$dir/shell-launcher.conf" \
    "$dir/shell-keybinds.conf"
  if [ -e "$dir/wahrwelt" ] || [ -L "$dir/wahrwelt" ]; then
    printf '%s\n' "$dir/wahrwelt/hyprland.conf"
  fi
  printf '%s\n' \
    "$(runtime_file hyprland.conf)" \
    "$(runtime_file shell-profile.conf)" \
    "$(runtime_file shell-launcher.conf)" \
    "$(runtime_file shell-keybinds.conf)"
}

legacy_runtime_payload_matches() {
  local path="$1"
  local name profile namespace runtime target

  [ -f "$path" ] && [ ! -L "$path" ] || return 1
  name="${path##*/}"
  runtime="$hypr_runtime_dir"
  case "$name" in
    hyprland.conf)
      wahrwelt_print_stable_runtime_entrypoint "$runtime/hyprland.conf" | cmp -s - "$path" && return 0
      for profile in caelestia noctalia end4 end4-pc; do
        for namespace in mysetup wahrwelt; do
          {
            printf '# Active Hyprland profile: %s (%s)\n' "$namespace" "$profile"
            printf 'source = %s\n' "$(hypr_dir)/$namespace/hyprland.conf"
            printf 'source = %s\n' "$runtime/shell-profile.conf"
          } | cmp -s - "$path" && return 0
        done
        case "$profile" in
          end4 | end4-pc)
            {
              printf '# Active Hyprland profile: %s\n' "$profile"
              printf 'source = %s\n' "$(hypr_dir)/end4/hyprland.conf"
              printf 'source = %s\n' "$runtime/shell-profile.conf"
            } | cmp -s - "$path" && return 0
            ;;
        esac
      done
      ;;
    shell-profile.conf)
      {
        printf '# Runtime shell launcher\n'
        printf 'exec-once = %s\n' "$(hypr_dir)/scripts/start-shell.sh"
      } | cmp -s - "$path" && return 0
      ;;
    shell-launcher.conf)
      for profile in caelestia noctalia end4 end4-pc; do
        {
          printf '# Active shell launcher profile: %s\n' "$profile"
          printf 'source = %s\n' "$(hypr_dir)/$profile/launcher.conf"
        } | cmp -s - "$path" && return 0
      done
      ;;
    shell-keybinds.conf)
      for profile in caelestia noctalia end4 end4-pc; do
        {
          printf '# Active shell keybind profile: %s\n' "$profile"
          printf 'source = %s\n' "$(hypr_dir)/$profile/keybinds.conf"
        } | cmp -s - "$path" && return 0
        case "$profile" in
          end4 | end4-pc)
            {
              printf '%s\n' "-- Active shell keybind profile: $profile"
              printf '%s\n' '-- end4 registers keybinds from its own Hyprland Lua modules.'
            } | cmp -s - "$path" && return 0
            ;;
        esac
      done
      ;;
  esac
  return 1
}

legacy_runtime_symlink_owned() {
  local path="$1"
  local target candidate

  target="$(absolute_symlink_target "$path" 2>/dev/null || true)"
  [ -n "$target" ] || return 1
  while IFS= read -r candidate; do
    [ "$target" = "$candidate" ] || continue
    [ "$candidate" != "$path" ] || continue
    legacy_runtime_payload_matches "$candidate" && return 0
  done < <(legacy_hyprland_runtime_paths)
  return 1
}

legacy_runtime_path_owned() {
  local path="$1"

  if [ -L "$path" ]; then
    legacy_runtime_symlink_owned "$path"
    return
  fi
  legacy_runtime_payload_matches "$path"
}

prune_legacy_hyprland_runtime_files() {
  local path transaction

  while IFS= read -r path; do
    [ -e "$path" ] || [ -L "$path" ] || continue
    if ! legacy_runtime_path_owned "$path"; then
      log "refusing unowned legacy runtime collision: $path"
      return 1
    fi
    transaction="$(wahrwelt_transaction_for_path "$path" 2>/dev/null)" || {
      log "legacy runtime path is outside the active transaction: $path"
      return 1
    }
    wahrwelt_fs_remove "$transaction" "$path" || return 1
  done < <(legacy_hyprland_runtime_paths)
}

runtime_bundle_fast_path_ready() {
  local dir path stable_content launcher_content hypr_content

  dir="$(hypr_dir)"
  stable_content="$(wahrwelt_print_stable_runtime_entrypoint "$hypr_runtime_dir/hyprland.lua")" || return 1
  launcher_content="$(runtime_shell_profile_content)" || return 1
  hypr_content="$(wahrwelt_print_canonical_runtime_entrypoint)" || return 1

  stable_runtime_entrypoint_matches "$dir/hyprland.lua" "$stable_content" || return 1
  runtime_path_matches_content "$(runtime_file shell-profile.lua)" "$launcher_content" || return 1
  runtime_path_matches_content "$(runtime_file hyprland.lua)" "$hypr_content" || return 1
  while IFS= read -r path; do
    if [ -e "$path" ] || [ -L "$path" ]; then
      return 1
    fi
  done < <(legacy_hyprland_runtime_paths)
}

runtime_profile_mutation_paths() {
  local content path

  path="$(runtime_file shell-launcher.lua)"
  content="$(runtime_shell_launcher_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"

  path="$(runtime_file shell-keybinds.lua)"
  content="$(runtime_shell_keybinds_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"

  path="$(runtime_file hyprlock.conf)"
  content="$(runtime_hyprlock_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"

  path="$(runtime_file hypridle.conf)"
  content="$(runtime_hypridle_content)" || return 1
  runtime_path_matches_content "$path" "$content" || printf '%s\n' "$path"
}

runtime_full_bundle_paths() {
  local dir

  dir="$(hypr_dir)"
  printf '%s\n' \
    "$dir/hyprland.lua" \
    "$(runtime_file shell-profile.lua)" \
    "$(runtime_file shell-launcher.lua)" \
    "$(runtime_file shell-keybinds.lua)" \
    "$(runtime_file hyprland.lua)" \
    "$(runtime_file hyprlock.conf)" \
    "$(runtime_file hypridle.conf)"
  legacy_hyprland_runtime_paths
}

runtime_bundle_paths() {
  if runtime_bundle_fast_path_ready; then
    runtime_profile_mutation_paths
    return $?
  fi
  runtime_full_bundle_paths
}

runtime_union_path_if_needed() {
  local path="$1"
  local requested_content="$2"
  local fallback_content="$3"

  if [ "$requested_content" != "$fallback_content" ] ||
    ! runtime_path_matches_content "$path" "$requested_content"; then
    printf '%s\n' "$path"
  fi
  return 0
}

runtime_profile_union_mutation_paths() {
  local requested_profile="$1"
  local fallback_profile="$2"
  local path requested_content fallback_content
  local profile="$requested_profile"

  path="$(runtime_file shell-launcher.lua)"
  requested_content="$(runtime_shell_launcher_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_shell_launcher_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"

  profile="$requested_profile"
  path="$(runtime_file shell-keybinds.lua)"
  requested_content="$(runtime_shell_keybinds_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_shell_keybinds_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"

  profile="$requested_profile"
  path="$(runtime_file hyprlock.conf)"
  requested_content="$(runtime_hyprlock_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_hyprlock_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"

  profile="$requested_profile"
  path="$(runtime_file hypridle.conf)"
  requested_content="$(runtime_hypridle_content)" || return 1
  profile="$fallback_profile"
  fallback_content="$(runtime_hypridle_content)" || return 1
  runtime_union_path_if_needed "$path" "$requested_content" "$fallback_content"
}

runtime_switch_bundle_paths() {
  local requested_profile="$profile"

  if ! runtime_bundle_fast_path_ready; then
    runtime_full_bundle_paths
    return $?
  fi
  if wahrwelt_valid_shell_profile "${previous:-}" && [ "$previous" != "$requested_profile" ]; then
    runtime_profile_union_mutation_paths "$requested_profile" "$previous"
    return $?
  fi
  runtime_profile_mutation_paths
}

state_bundle_paths() {
  if ! runtime_path_matches_content "$persistent_state_file" "$profile"; then
    printf '%s\n' "$persistent_state_file"
  fi
  if wahrwelt_valid_end4_variant "$profile" &&
    ! runtime_path_matches_content "$wahrwelt_end4_variant_state" "$profile"; then
    printf '%s\n' "$wahrwelt_end4_variant_state"
  fi
}

sync_runtime_shell_files() {
  local path content

  if [ "${runtime_transaction_fast_path:-0}" -eq 1 ]; then
    for path in "${runtime_bundle_path_list[@]}"; do
      case "$path" in
        "$(runtime_file shell-launcher.lua)") content="$(runtime_shell_launcher_content)" || return 1 ;;
        "$(runtime_file shell-keybinds.lua)") content="$(runtime_shell_keybinds_content)" || return 1 ;;
        "$(runtime_file hyprlock.conf)") content="$(runtime_hyprlock_content)" || return 1 ;;
        "$(runtime_file hypridle.conf)") content="$(runtime_hypridle_content)" || return 1 ;;
        *)
          log "fast runtime plan contained an unexpected path: $path"
          return 1
          ;;
      esac
      write_regular_file "$path" "$content" || return 1
    done
    return 0
  fi
  prune_legacy_hyprland_runtime_files || return 1
  sync_stable_lua_entrypoint || return 1
  sync_shell_launcher || return 1
  sync_shell_launcher_bindings || return 1
  sync_shell_keybinds || return 1
  sync_hypr_entrypoint || return 1
  sync_hypr_lock_stack
}

require_file() {
  local label="$1"
  local path="$2"

  if [ -f "$path" ]; then
    return 0
  fi

  log "$label missing for profile=$profile path=$path"
  return 1
}

require_command() {
  local command_name="$1"

  if command -v "$command_name" >/dev/null 2>&1; then
    return 0
  fi

  log "$command_name command not found for profile=$profile"
  return 1
}

validate_profile_ready() {
  local dir adapter launcher_module

  dir="$(hypr_dir)"
  ensure_wahrwelt_entrypoint || return 1
  require_file "shell lifecycle launcher" "$dir/scripts/start-shell.sh" || return 1
  launcher_module="$(wahrwelt_shell_launcher_module "$profile")" || return 1
  require_file "shell launcher profile" "$dir/${launcher_module%%.*}/launcher.lua" || return 1
  adapter="$(wahrwelt_shell_adapter "$profile")" || return 1

  case "$profile" in
    caelestia)
      require_file "shell keybind profile" "$dir/caelestia/keybinds.lua" || return 1
      command -v caelestia >/dev/null 2>&1 || require_command caelestia-shell || return 1
      ;;
    noctalia)
      require_file "shell keybind profile" "$dir/noctalia/keybinds.lua" || return 1
      if ! wahrwelt_noctalia_command >/dev/null; then
        log "noctalia command not found for profile=$profile"
        return 1
      fi
      ;;
    end4 | end4-pc)
      validate_end4_profile_tree || return 1
      require_file "end4 shell adapter" "$dir/$adapter.lua" || return 1
      require_file "end4 hypr lua entrypoint" "$dir/end4/hyprland.lua" || return 1
      require_file "end4 hyprlock entrypoint" "$dir/end4/hyprlock.conf" || return 1
      require_file "end4 hypridle entrypoint" "$dir/end4/hypridle.conf" || return 1
      require_file "end4 quickshell config" "$(wahrwelt_end4_quickshell_path "$profile")/shell.qml" || return 1
      require_command qs-end4 || return 1
      ;;
  esac
}

prepare_profile_or_fallback() {
  validate_profile_ready && sync_runtime_shell_files
}

persist_profile() {
  local planned_paths status=0 owns_transaction=0

  if [ "${switch_transaction_active:-0}" -eq 1 ]; then
    wahrwelt_verify_exact_path_guards "$persistent_state_file" "$wahrwelt_end4_variant_state" || return 1
  fi
  if [ -z "${state_snapshot_dir:-}" ]; then
    state_path_list=()
    planned_paths="$(state_bundle_paths)" || return 1
    [ -n "$planned_paths" ] || return 0
    mapfile -t state_path_list <<<"$planned_paths"
    state_snapshot_dir="$(
      wahrwelt_fs_begin state "$wahrwelt_runtime_session_public_dir" "${state_path_list[@]}"
    )" || {
      state_snapshot_dir=""
      state_path_list=()
      return 1
    }
    if [ "${switch_transaction_active:-0}" -ne 1 ]; then
      owns_transaction=1
    fi
  fi

  if [ "$status" -eq 0 ] && wahrwelt_valid_end4_variant "$profile"; then
    write_regular_file_if_changed "$wahrwelt_end4_variant_state" "$profile" || status=1
  fi
  if [ "$status" -eq 0 ]; then
    write_regular_file_if_changed "$persistent_state_file" "$profile" || status=1
  fi

  if [ "$status" -ne 0 ]; then
    if [ "$owns_transaction" -eq 1 ] && ! wahrwelt_fs_rollback "$state_snapshot_dir"; then
      log "failed to restore shell state transaction after persistence error; preserving private transaction: $state_snapshot_dir"
      return 1
    fi
  fi
  if [ "$owns_transaction" -eq 1 ]; then
    if ! wahrwelt_fs_commit "$state_snapshot_dir"; then
      log "state persistence transaction cleanup refused; exact recovery retained: $state_snapshot_dir"
      return 1
    fi
    state_snapshot_dir=""
    state_path_list=()
  fi
  return "$status"
}

hypr_supports_lua_runtime() {
  local version major minor rest

  version="$(hyprctl version 2>/dev/null | awk 'NR == 1 { print $2 }')"
  version="${version#v}"
  major="${version%%.*}"
  rest="${version#*.}"
  minor="${rest%%.*}"

  case "$major" in
    "" | *[!0-9]*) return 1 ;;
  esac
  case "$minor" in
    "" | *[!0-9]*) minor=0 ;;
  esac

  [ "$major" -gt 0 ] || [ "$minor" -ge 55 ]
}

reload_hypr() {
  if command -v hyprctl >/dev/null 2>&1 && hyprctl monitors >/dev/null 2>&1; then
    if ! hypr_supports_lua_runtime; then
      log "skipping hyprctl reload; running Hyprland is older than 0.55 and cannot load Lua runtime"
      return 0
    fi
    if ! hyprctl reload >/dev/null 2>&1; then
      log "hyprctl reload failed after profile sync"
      return 1
    fi
  fi
}
