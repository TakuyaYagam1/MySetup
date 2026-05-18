#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2154

home_manager_files_root() {
  local qs_path resolved root

  qs_path="$config_home/quickshell/ii"
  resolved="$(readlink -f "$qs_path" 2>/dev/null || true)"
  [ -n "$resolved" ] || return 1

  case "$resolved" in
    */.config/quickshell/ii*)
      root="${resolved%%/.config/quickshell/ii*}"
      ;;
    *)
      return 1
      ;;
  esac

  [ -d "$root/.config/hypr/end4" ] || return 1
  printf '%s' "$root"
}

ensure_end4_profile_link() {
  local dir target hm_root source

  [ "$profile" = "end4" ] || return 0

  dir="$(hypr_dir)"
  target="$dir/end4"

  if [ -f "$target/hyprland.lua" ]; then
    return 0
  fi

  hm_root="$(home_manager_files_root 2>/dev/null || true)"
  source="$hm_root/.config/hypr/end4"
  if [ -z "$hm_root" ] || [ ! -f "$source/hyprland.lua" ]; then
    log "unable to restore end4 profile from Home Manager generation; target=$target source=$source"
    return 1
  fi

  mkdir -p -- "$dir"
  if [ -L "$target" ]; then
    rm -f -- "$target"
  elif [ -e "$target" ]; then
    if [ -d "$target" ] && [ ! -f "$target/hyprland.lua" ]; then
      rm -rf -- "$target"
    else
      log "refusing to replace existing end4 profile path: $target"
      return 1
    fi
  fi

  ln -s -- "$source" "$target"
  log "restored end4 profile link: $target -> $source"
}

write_regular_file() {
  local path="$1"
  local content="$2"
  local dir tmp

  if [ -L "$path" ]; then
    rm -f -- "$path"
  elif [ -e "$path" ] && [ ! -f "$path" ]; then
    log "refusing to overwrite non-regular config file: $path"
    return 1
  fi

  chmod u+w "$path" >/dev/null 2>&1 || true

  dir="$(dirname -- "$path")"
  mkdir -p -- "$dir"
  tmp="$(mktemp "$dir/.${path##*/}.tmp.XXXXXX")" || return 1
  if ! printf '%s\n' "$content" >"$tmp"; then
    rm -f -- "$tmp"
    return 1
  fi
  chmod 0644 "$tmp" >/dev/null 2>&1 || true
  if ! mv -f -- "$tmp" "$path"; then
    rm -f -- "$tmp"
    return 1
  fi
}

runtime_file() {
  mysetup_runtime_file "$1"
}

ensure_mysetup_entrypoint() {
  local dir target

  dir="$(hypr_dir)"
  target="$dir/mysetup/hyprland.lua"

  if [ -f "$target" ]; then
    return 0
  fi

  mkdir -p -- "$(dirname "$target")"
  log "mysetup hypr entrypoint missing; rebuild or rerun mysetup apply: $target"
  return 1
}

sync_shell_launcher() {
  local dir

  dir="$(hypr_dir)"
  mkdir -p -- "$hypr_runtime_dir"
  write_regular_file "$(runtime_file shell-profile.lua)" "-- Runtime shell launcher
hl.on(\"hyprland.start\", function()
    hl.exec_cmd(\"$dir/scripts/start-shell.sh\")
end)"
}

sync_shell_launcher_bindings() {
  local dir profile_launcher

  if [ "$profile" = "end4" ]; then
    write_regular_file "$(runtime_file shell-launcher.lua)" "-- Active shell launcher profile: end4
-- end4 registers launcher bindings from its own Hyprland Lua modules."
    return $?
  fi

  dir="$(hypr_dir)"
  profile_launcher="$dir/$profile/launcher.lua"

  if [ ! -f "$profile_launcher" ]; then
    log "shell launcher profile missing: $profile_launcher"
    return 1
  fi

  write_regular_file "$(runtime_file shell-launcher.lua)" "-- Active shell launcher profile: $profile
require(\"$profile.launcher\")"
}

sync_shell_keybinds() {
  local dir profile_keybinds

  if [ "$profile" = "end4" ]; then
    write_regular_file "$(runtime_file shell-keybinds.lua)" "-- Active shell keybind profile: end4
-- end4 registers keybinds from its own Hyprland Lua modules."
    return $?
  fi

  dir="$(hypr_dir)"
  profile_keybinds="$dir/$profile/keybinds.lua"

  if [ ! -f "$profile_keybinds" ]; then
    log "shell keybind profile missing: $profile_keybinds"
    return 1
  fi

  write_regular_file "$(runtime_file shell-keybinds.lua)" "-- Active shell keybind profile: $profile
require(\"$profile.keybinds\")"
}

sync_hypr_entrypoint() {
  local dir target label content

  dir="$(hypr_dir)"

  if [ "$profile" = "end4" ]; then
    target="$dir/end4/hyprland.lua"
    label="end4"
    content="-- Active Hyprland profile: $label
local home = os.getenv(\"HOME\")
if home == nil then
    error(\"HOME is not set; cannot locate end4 Hyprland config\")
end

local config_home = os.getenv(\"XDG_CONFIG_HOME\") or (home .. \"/.config\")
local hypr_root = config_home .. \"/hypr\"
local end4_root = hypr_root .. \"/end4\"
package.path = end4_root .. \"/?.lua;\" .. end4_root .. \"/?/init.lua;\" .. hypr_root .. \"/?.lua;\" .. hypr_root .. \"/?/init.lua;\" .. package.path
dofile(end4_root .. \"/hyprland.lua\")"
  else
    ensure_mysetup_entrypoint || return 1
    target="$dir/mysetup/hyprland.lua"
    label="mysetup ($profile)"
    content="-- Active Hyprland profile: $label
local home = os.getenv(\"HOME\")
if home == nil then
    error(\"HOME is not set; cannot locate MySetup Hyprland config\")
end

local config_home = os.getenv(\"XDG_CONFIG_HOME\") or (home .. \"/.config\")
local state_home = os.getenv(\"XDG_STATE_HOME\") or (home .. \"/.local/state\")
local hypr_root = config_home .. \"/hypr\"
local runtime_root = state_home .. \"/mysetup/hypr-runtime\"
package.path = hypr_root .. \"/?.lua;\" .. hypr_root .. \"/?/init.lua;\" .. package.path
dofile(hypr_root .. \"/mysetup/hyprland.lua\")
dofile(runtime_root .. \"/shell-profile.lua\")"
  fi

  if [ ! -f "$target" ]; then
    log "hypr entrypoint missing for profile=$profile path=$target"
    return 1
  fi

  write_regular_file "$(runtime_file hyprland.lua)" "$content"
}

sync_hypr_lock_stack() {
  local dir hyprlock_target hypridle_target

  dir="$(hypr_dir)"

  if [ "$profile" = "end4" ]; then
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

    write_regular_file "$(runtime_file hyprlock.conf)" "# Active Hyprlock profile: end4
source = $hyprlock_target" || return 1
    write_regular_file "$(runtime_file hypridle.conf)" "# Active Hypridle profile: end4
source = $hypridle_target"
    return $?
  fi

  write_regular_file "$(runtime_file hyprlock.conf)" "# Active Hyprlock profile: shell-managed ($profile)
# Caelestia and Noctalia use shell-native lock flows." || return 1
  write_regular_file "$(runtime_file hypridle.conf)" "# Active Hypridle profile: shell-managed ($profile)
# Caelestia and Noctalia use shell-native idle flows."
}

sync_stable_lua_entrypoint() {
  local dir target marker old_end4_marker content

  dir="$(hypr_dir)"
  target="$dir/hyprland.lua"
  marker="-- Generated by MySetup: stable Hyprland Lua runtime entrypoint"
  old_end4_marker="-- Generated by MySetup: active end4 Hyprland Lua entrypoint"

  if [ -r "$target" ] && grep -Fqx -- "$marker" "$target"; then
    return 0
  fi

  content="$marker
local home = os.getenv(\"HOME\")
if home == nil then
    error(\"HOME is not set; cannot locate MySetup Hyprland runtime\")
end

local config_home = os.getenv(\"XDG_CONFIG_HOME\") or (home .. \"/.config\")
local state_home = os.getenv(\"XDG_STATE_HOME\") or (home .. \"/.local/state\")
local hypr_root = config_home .. \"/hypr\"
local runtime_root = state_home .. \"/mysetup/hypr-runtime\"
package.path = hypr_root .. \"/?.lua;\" .. hypr_root .. \"/?/init.lua;\" .. package.path
dofile(runtime_root .. \"/hyprland.lua\")"

  if [ -e "$target" ] && ! grep -Fqx -- "$old_end4_marker" "$target" 2>/dev/null; then
    log "refreshing Hyprland Lua runtime entrypoint: $target"
  fi
  write_regular_file "$target" "$content"
}

prune_legacy_hyprland_runtime_files() {
  local dir

  dir="$(hypr_dir)"
  rm -f -- \
    "$dir/hyprland.conf" \
    "$dir/shell-profile.conf" \
    "$dir/shell-launcher.conf" \
    "$dir/shell-keybinds.conf" \
    "$dir/mysetup/hyprland.conf" \
    "$(runtime_file hyprland.conf)" \
    "$(runtime_file shell-profile.conf)" \
    "$(runtime_file shell-launcher.conf)" \
    "$(runtime_file shell-keybinds.conf)"
}

sync_end4_top_level_links() {
  local dir

  [ "$profile" = "end4" ] || return 0

  dir="$(hypr_dir)"
  rm -f -- "$dir/monitors.conf" "$dir/workspaces.conf"
  ln -sfn -- "$dir/end4/monitors.conf" "$dir/monitors.conf"
  ln -sfn -- "$dir/end4/workspaces.conf" "$dir/workspaces.conf"
}

sync_runtime_shell_files() {
  prune_legacy_hyprland_runtime_files
  sync_stable_lua_entrypoint || return 1
  sync_end4_top_level_links || return 1
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
  local dir

  dir="$(hypr_dir)"

  case "$profile" in
    caelestia)
      require_file "shell launcher profile" "$dir/caelestia/launcher.lua" || return 1
      require_file "shell keybind profile" "$dir/caelestia/keybinds.lua" || return 1
      command -v caelestia >/dev/null 2>&1 || require_command caelestia-shell || return 1
      ;;
    noctalia)
      require_file "shell launcher profile" "$dir/noctalia/launcher.lua" || return 1
      require_file "shell keybind profile" "$dir/noctalia/keybinds.lua" || return 1
      require_command noctalia-shell || return 1
      ;;
    end4)
      ensure_end4_profile_link || return 1
      require_file "end4 hypr lua entrypoint" "$dir/end4/hyprland.lua" || return 1
      require_file "end4 hyprlock entrypoint" "$dir/end4/hyprlock.conf" || return 1
      require_file "end4 hypridle entrypoint" "$dir/end4/hypridle.conf" || return 1
      require_file "end4 quickshell config" "$config_home/quickshell/ii/shell.qml" || return 1
      require_command qs-end4 || return 1
      ;;
  esac
}

prepare_profile_or_fallback() {
  validate_profile_ready && sync_runtime_shell_files
}

persist_profile() {
  mkdir -p -- "$(dirname "$persistent_state_file")"
  write_regular_file "$persistent_state_file" "$profile"
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
    hyprctl reload >/dev/null 2>&1 || log "hyprctl reload failed after profile sync"
  fi
}
