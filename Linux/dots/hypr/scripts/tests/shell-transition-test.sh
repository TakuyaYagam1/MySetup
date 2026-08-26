#!/usr/bin/env bash
set -euo pipefail

source_scripts="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

environment_command_log="$test_root/environment-commands.log"
: >"$environment_command_log"
export WAHRWELT_TEST_ENVIRONMENT_COMMANDS="$environment_command_log"

systemctl() {
  {
    printf 'systemctl'
    printf '\t%s' "$@"
    printf '\n'
  } >>"$WAHRWELT_TEST_ENVIRONMENT_COMMANDS"
}

dbus-update-activation-environment() {
  {
    printf 'dbus-update-activation-environment'
    printf '\t%s' "$@"
    printf '\n'
  } >>"$WAHRWELT_TEST_ENVIRONMENT_COMMANDS"
}

export -f systemctl dbus-update-activation-environment

instrumented_scripts="$test_root/scripts"
early_hooks="$test_root/early-hooks.sh"
hooks="$test_root/hooks.sh"
mkdir -p "$instrumented_scripts"
for helper in shell-runtime.sh shell-runtime-env.sh shell-profile-sync.sh shell-process.sh; do
  ln -s -- "$source_scripts/$helper" "$instrumented_scripts/$helper"
done

cat >"$early_hooks" <<'EOF'
WAHRWELT_TRANSITION_EARLY_TEST_HOOKS=1

prepare_runtime_environment() {
  :
}
EOF

cat >"$hooks" <<'EOF'
WAHRWELT_TRANSITION_TEST_HOOKS=1

test_has_process() {
  case "$1" in
    caelestia | noctalia | end4 | end4-pc)
      grep -Eq -- "^${1}#[0-9]+$" "$WAHRWELT_TEST_PROCESSES" 2>/dev/null
      ;;
    *)
      grep -Fqx -- "$1" "$WAHRWELT_TEST_PROCESSES" 2>/dev/null
      ;;
  esac
}

test_add_process() {
  local generation

  test_has_process "$1" && return 0
  case "$1" in
    caelestia | noctalia | end4 | end4-pc)
      generation="$(cat "$WAHRWELT_TEST_GENERATION")"
      generation=$((generation + 1))
      printf '%s\n' "$generation" >"$WAHRWELT_TEST_GENERATION"
      printf '%s#%s\n' "$1" "$generation" >>"$WAHRWELT_TEST_PROCESSES"
      ;;
    *)
      printf '%s\n' "$1" >>"$WAHRWELT_TEST_PROCESSES"
      ;;
  esac
}

test_remove_process() {
  local process="$1"
  local next="${WAHRWELT_TEST_PROCESSES}.next"
  local stops_watcher=0

  if [ "${WAHRWELT_TEST_WATCHER_PROCESS:-}" = "$process" ] && test_has_process "$process"; then
    stops_watcher=1
  fi

  while IFS= read -r running; do
    [ "$running" = "$process" ] && continue
    case "$running" in
      "$process"#[0-9]*) continue ;;
    esac
    printf '%s\n' "$running"
  done <"$WAHRWELT_TEST_PROCESSES" >"$next"
  mv -- "$next" "$WAHRWELT_TEST_PROCESSES"

  if [ "$stops_watcher" -eq 1 ]; then
    : >"$WAHRWELT_TEST_WATCHER_OWNER"
    test_spotify_watcher_gap
    if [ -n "${WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER:-}" ]; then
      printf '%s\n' "$WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER" >"$WAHRWELT_TEST_WATCHER_OWNER"
    fi
  elif ! grep -Eq '^(caelestia|noctalia|end4|end4-pc)#[0-9]+$' "$WAHRWELT_TEST_PROCESSES"; then
    : >"$WAHRWELT_TEST_WATCHER_OWNER"
    test_spotify_watcher_gap
  fi
}

test_event() {
  printf '%s\n' "$1" >>"$WAHRWELT_TEST_EVENTS"
}

test_spotify_watcher_gap() {
  local address reveal_monitor next

  [ -n "${WAHRWELT_TEST_SPOTIFY_GAP_MODE:-}" ] || return 0
  [ ! -e "$WAHRWELT_TEST_SPOTIFY_GAP_FIRED" ] || return 0
  : >"$WAHRWELT_TEST_SPOTIFY_GAP_FIRED"
  address="$(jq -r '
    .[]
    | select((.class // "" | ascii_downcase) == "spotify")
    | select(.workspace.name == "special:music")
    | .address
  ' "$WAHRWELT_TEST_CLIENTS" | head -n 1)"
  [ -n "$address" ] || return 0

  if [ "$WAHRWELT_TEST_SPOTIFY_GAP_MODE" = respect ] &&
    grep -Fqx -- "$address=false" "$WAHRWELT_TEST_FOCUS_PROPS"; then
    test_event "spotify-activation-blocked:$address"
    return 0
  fi

  reveal_monitor="${WAHRWELT_TEST_SPOTIFY_REVEAL_MONITOR:-eDP-1}"
  next="${WAHRWELT_TEST_MONITORS}.next"
  jq --arg monitor "$reveal_monitor" '
    map(
      if .name == $monitor then
        .specialWorkspace = {"id": -98, "name": "special:music"}
      else
        .
      end
    )
  ' "$WAHRWELT_TEST_MONITORS" >"$next"
  mv -- "$next" "$WAHRWELT_TEST_MONITORS"
  if [ "${WAHRWELT_TEST_SPOTIFY_HIDE_STEALS_FOCUS:-0}" = 1 ]; then
    jq --arg monitor "$reveal_monitor" \
      'map(.focused = (.name == $monitor))' "$WAHRWELT_TEST_MONITORS" >"$next"
    mv -- "$next" "$WAHRWELT_TEST_MONITORS"
    jq --arg address "$address" '.[] | select(.address == $address)' \
      "$WAHRWELT_TEST_CLIENTS" >"$WAHRWELT_TEST_ACTIVE_WINDOW"
    test_event "focus-stolen:spotify-activation:$address"
  fi
  test_event "spotify-workspace-revealed:$address"
}

test_publish_watcher() {
  if [ -n "${WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER:-}" ]; then
    printf '%s\n' "$WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER" >"$WAHRWELT_TEST_WATCHER_OWNER"
    return 0
  fi
  printf ':1.%s\n' "$(cat "$WAHRWELT_TEST_GENERATION")" >"$WAHRWELT_TEST_WATCHER_OWNER"
}

hyprctl() {
  local command address monitor music_monitor next restore_address restore_monitor

  if { [ "${1:-}" = -j ] && [ "${2:-}" = clients ]; } ||
    { [ "${1:-}" = clients ] && [ "${2:-}" = -j ]; }; then
    [ "${WAHRWELT_TEST_FAIL_SPOTIFY_SNAPSHOT:-}" != clients ] || return 1
    cat "$WAHRWELT_TEST_CLIENTS"
    return 0
  fi
  if { [ "${1:-}" = -j ] && [ "${2:-}" = monitors ]; } ||
    { [ "${1:-}" = monitors ] && [ "${2:-}" = -j ]; }; then
    cat "$WAHRWELT_TEST_MONITORS"
    return 0
  fi
  if { [ "${1:-}" = -j ] && [ "${2:-}" = activewindow ]; } ||
    { [ "${1:-}" = activewindow ] && [ "${2:-}" = -j ]; }; then
    cat "$WAHRWELT_TEST_ACTIVE_WINDOW"
    return 0
  fi
  if [ "${1:-}" = eval ]; then
    command="${2:-}"
    case "$command" in
      *'hl.get_monitors()'*'active_special_workspace'*'set_special_workspace({})'*)
        music_monitor="$(jq -r \
          '[.[] | select(.specialWorkspace.name == "special:music")][0].name // empty' \
          "$WAHRWELT_TEST_MONITORS")"
        while IFS= read -r monitor; do
          [ -n "$monitor" ] || continue
          test_event "hide:special:music:$monitor"
        done < <(jq -r '.[] | select(.specialWorkspace.name == "special:music") | .name' \
          "$WAHRWELT_TEST_MONITORS")
        next="${WAHRWELT_TEST_MONITORS}.next"
        jq 'map(
          if .specialWorkspace.name == "special:music" then
            .specialWorkspace = {"id": 0, "name": ""}
          else
            .
          end
        )' "$WAHRWELT_TEST_MONITORS" >"$next"
        mv -- "$next" "$WAHRWELT_TEST_MONITORS"
        if [ "${WAHRWELT_TEST_SPOTIFY_HIDE_STEALS_FOCUS:-0}" = 1 ] &&
          [ -n "$music_monitor" ]; then
          jq --arg monitor "$music_monitor" \
            'map(.focused = (.name == $monitor))' "$WAHRWELT_TEST_MONITORS" >"$next"
          mv -- "$next" "$WAHRWELT_TEST_MONITORS"
          printf '%s\n' \
            '{"address":"0xfeed00","class":"foot","pid":300,"monitor":1}' \
            >"$WAHRWELT_TEST_ACTIVE_WINDOW"
          test_event "focus-stolen:special-close:$music_monitor"
        fi
        restore_monitor="$(sed -n \
          's/.*restore_monitor = hl.get_monitor("\([A-Za-z0-9_.-]*\)").*/\1/p' \
          <<<"$command")"
        restore_address="$(sed -n \
          's/.*restore_window = hl.get_window("address:\(0x[0-9a-f]*\)").*/\1/p' \
          <<<"$command")"
        if [ -n "$music_monitor" ] && [ -n "$restore_monitor" ]; then
          jq --arg monitor "$restore_monitor" \
            'map(.focused = (.name == $monitor))' "$WAHRWELT_TEST_MONITORS" >"$next"
          mv -- "$next" "$WAHRWELT_TEST_MONITORS"
          test_event "focus-restore-monitor:$restore_monitor"
        fi
        if [ -n "$music_monitor" ] && [ -n "$restore_address" ]; then
          jq --arg address "$restore_address" '.[] | select(.address == $address)' \
            "$WAHRWELT_TEST_CLIENTS" >"$WAHRWELT_TEST_ACTIVE_WINDOW"
          test_event "focus-restore-window:$restore_address"
        fi
        return 0
        ;;
    esac
    return 1
  fi

  [ "${1:-}" = dispatch ] || return 1
  command="${2:-}"

  case "$command" in
    *'hl.dsp.window.set_prop('*'prop = "focus_on_activate"'*'window = "address:'*)
      address="$(sed -n 's/.*window = "address:\([^"]*\)".*/\1/p' <<<"$command")"
      jq -e --arg address "$address" '
        any(.[];
          .address == $address and
          ((.class // "" | ascii_downcase) == "spotify")
        )
      ' "$WAHRWELT_TEST_CLIENTS" >/dev/null || return 1
      case "$command" in
        *'value = "false"'*)
          grep -Fqx -- "$address=false" "$WAHRWELT_TEST_FOCUS_PROPS" ||
            printf '%s=false\n' "$address" >>"$WAHRWELT_TEST_FOCUS_PROPS"
          test_event "spotify-focus-block:$address"
          ;;
        *'value = "unset"'*)
          next="${WAHRWELT_TEST_FOCUS_PROPS}.next"
          grep -Fvx -- "$address=false" "$WAHRWELT_TEST_FOCUS_PROPS" >"$next" || true
          mv -- "$next" "$WAHRWELT_TEST_FOCUS_PROPS"
          test_event "spotify-focus-restore:$address"
          ;;
        *) return 1 ;;
      esac
      return 0
      ;;
    'hl.dsp.workspace.toggle_special("music")')
      monitor="$(jq -r '([.[] | select(.focused == true)][0].name // .[0].name) // empty' \
        "$WAHRWELT_TEST_MONITORS")"
      next="${WAHRWELT_TEST_MONITORS}.next"
      jq --arg monitor "$monitor" 'map(
        if .name == $monitor then
          if .specialWorkspace.name == "special:music" then
            .specialWorkspace = {"id": 0, "name": ""}
          else
            .specialWorkspace = {"id": -98, "name": "special:music"}
          end
        elif .specialWorkspace.name == "special:music" then
          .specialWorkspace = {"id": 0, "name": ""}
        else
          .
        end
      )' "$WAHRWELT_TEST_MONITORS" >"$next"
      mv -- "$next" "$WAHRWELT_TEST_MONITORS"
      test_event hide:special:music
      return 0
      ;;
  esac

  return 1
}

busctl() {
  local owner

  [ "${1:-}" = --user ] || return 1
  shift
  if [ "${1:-}" = --timeout=50ms ]; then
    shift
    test_event "busctl-bounded:${1:-}"
  elif [[ "${1:-}" == --timeout=* ]]; then
    test_event "busctl-wrong-timeout:${1:-}:${2:-}"
    shift
  else
    test_event "busctl-unbounded:${1:-}"
  fi

  if [ "${1:-}" = call ] &&
    [ "${2:-}" = org.freedesktop.DBus ] &&
    [ "${5:-}" = GetNameOwner ] &&
    [ "${7:-}" = org.kde.StatusNotifierWatcher ]; then
    owner="$(tr -d '\n' <"$WAHRWELT_TEST_WATCHER_OWNER")"
    [ -n "$owner" ] || return 1
    printf 's "%s"\n' "$owner"
    return 0
  fi
  if [ "${1:-}" = get-property ] &&
    [ "${3:-}" = /StatusNotifierWatcher ] &&
    [ "${4:-}" = org.kde.StatusNotifierWatcher ] &&
    [ "${5:-}" = IsStatusNotifierHostRegistered ]; then
    owner="$(tr -d '\n' <"$WAHRWELT_TEST_WATCHER_OWNER")"
    [ -n "$owner" ] && [ "${2:-}" = "$owner" ] || return 1
    if [ "${WAHRWELT_TEST_WATCHER_UNREADY:-}" = 1 ]; then
      printf 'b false\n'
      return 0
    fi
    test_event "watcher-ready:$owner"
    printf 'b true\n'
    return 0
  fi

  return 1
}

sleep() {
  test_event "sleep:$1"
  :
}

log() {
  test_event "log:$*"
}

wait_for_session() {
  test_event wait-session
}

test_mutate_bundle() {
  local mutation_profile="$1"
  local path index=0 owned_identity parent_identity owned_copy
  local paths=()

  mapfile -t paths < <(runtime_bundle_paths)
  for path in "${paths[@]}"; do
    index=$((index + 1))
    if [ "$index" -ge 8 ]; then
      rm -f -- "$path"
      parent_identity="$(runtime_parent_identity "$path" 2>/dev/null || true)"
      record_exact_snapshot_mutation "$path" absent "" "$parent_identity" || return 1
    else
      mkdir -p -- "$(dirname -- "$path")"
      rm -f -- "$path"
      printf 'prepared profile=%s path=%s\n' "$mutation_profile" "$index" >"$path"
      chmod 0644 "$path"
      owned_identity="$(runtime_regular_inode "$path")"
      parent_identity="$(runtime_parent_identity "$path")"
      if [ "${WAHRWELT_TEST_REPLACE_AFTER_PUBLISH_INDEX:-}" = "$index" ]; then
        owned_copy="${path}.transaction-owned"
        mv -- "$path" "$owned_copy"
        printf 'concurrent bundle winner path=%s\n' "$index" >"$path"
        chmod 0600 "$path"
      fi
      record_exact_snapshot_mutation "$path" regular "$owned_identity" "$parent_identity" || return 1
    fi
    if [ "${WAHRWELT_TEST_FAIL_PREPARE_INDEX:-}" = "$index" ]; then
      return 1
    fi
  done
}

prepare_profile_or_fallback() {
  test_event "prepare:$profile"
  if [ "${WAHRWELT_TEST_FAIL_PREPARE:-}" = "$profile" ] &&
    [ -z "${WAHRWELT_TEST_FAIL_PREPARE_INDEX:-}" ]; then
    WAHRWELT_TEST_FAIL_PREPARE_INDEX=4 test_mutate_bundle "$profile"
    return 1
  fi
  test_mutate_bundle "$profile" || return 1
  if [ "${WAHRWELT_TEST_ABORT_PREPARE:-}" = "$profile" ]; then
    kill -TERM "$$"
  fi
}

stop_shell_selector() {
  test_event stop:selector
  test_remove_process selector
}

stop_caelestia() {
  test_event stop:caelestia
  test_remove_process caelestia
  stop_caelestia_resizer
}

stop_caelestia_resizer() {
  test_event stop:caelestia-resizer
  test_remove_process caelestia-resizer
}

stop_noctalia() {
  test_event stop:noctalia
  test_remove_process noctalia
}

stop_end4() {
  test_event stop:end4-family
  test_remove_process end4
  test_remove_process end4-pc
}

stop_end4_idle() {
  test_event stop:end4-idle
  test_remove_process end4-idle
}

start_profile_shell() {
  test_event "start:$profile"
  case "$profile" in
    end4 | end4-pc)
      test_add_process end4-idle
      ;;
  esac

  if [ -n "${WAHRWELT_TEST_FAIL_START:-}" ]; then
    case ",${WAHRWELT_TEST_FAIL_START}," in
      *",$profile,"*)
        test_add_process "$profile"
        test_publish_watcher
        return 1
        ;;
    esac
  fi

  case "$profile" in
    end4 | end4-pc)
      if test_has_process "$profile"; then
        return 0
      fi
      test_remove_process end4
      test_remove_process end4-pc
      ;;
  esac
  test_add_process "$profile"
  test_publish_watcher
  if [ "${WAHRWELT_TEST_ABORT_AFTER_START:-}" = "$profile" ]; then
    test_event "abort-after-start:$profile"
    kill -TERM "$$"
  fi
}

reload_hypr() {
  test_event reload
  if [ "${WAHRWELT_TEST_FAIL_RELOAD:-}" = "$profile" ]; then
    return 1
  fi
}

propagate_runtime_environment() {
  test_event propagate
  if [ "${WAHRWELT_TEST_ABORT_AFTER_PROPAGATE:-}" = "$profile" ]; then
    test_event "abort-after-propagate:$profile"
    kill -TERM "$$"
  fi
}

eval "$(declare -f persist_profile | sed '1s/^persist_profile /test_real_persist_profile /')"
persist_profile() {
  test_event "persist:$profile"
  if [ "${WAHRWELT_TEST_FAIL_PERSIST:-}" = "$profile" ]; then
    return 1
  fi
  test_real_persist_profile "$@" || return 1
  if [ "${WAHRWELT_TEST_ABORT_AFTER_PERSIST:-}" = "$profile" ]; then
    test_event "abort-after-persist:$profile"
    kill -TERM "$$"
  fi
}
EOF

awk -v early_hooks="$early_hooks" -v hooks="$hooks" '
  $0 == "prepare_runtime_environment" {
    while ((getline hook_line < early_hooks) > 0) {
      print hook_line
    }
    close(early_hooks)
  }
  $0 == "log \"requested profile=$profile input=${requested_profile:-auto}\"" {
    while ((getline hook_line < hooks) > 0) {
      print hook_line
    }
    close(hooks)
  }
  { print }
' "$source_scripts/start-shell.sh" >"$instrumented_scripts/start-shell.sh"
early_hook_count="$(grep -Fc 'WAHRWELT_TRANSITION_EARLY_TEST_HOOKS=1' "$instrumented_scripts/start-shell.sh")"
if [ "$early_hook_count" -ne 1 ]; then
  printf 'FAIL: could not inject early transition test hooks into start-shell.sh\n' >&2
  exit 1
fi
late_hook_count="$(grep -Fc 'WAHRWELT_TRANSITION_TEST_HOOKS=1' "$instrumented_scripts/start-shell.sh")"
if [ "$late_hook_count" -ne 1 ]; then
  printf 'FAIL: could not inject transition test hooks into start-shell.sh\n' >&2
  exit 1
fi
chmod 0755 "$instrumented_scripts/start-shell.sh"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local expected="$1"
  local actual="$2"
  local message="$3"

  if [ "$actual" != "$expected" ]; then
    printf 'FAIL: %s: got %q, want %q\n' "$message" "$actual" "$expected" >&2
    exit 1
  fi
}

assert_process_set() {
  local process_file="$1"
  local profile="$2"
  local expected actual

  expected="unmanaged-hypridle
$profile"
  case "$profile" in
    end4 | end4-pc) expected="$expected
end4-idle" ;;
  esac
  expected="$(printf '%s\n' "$expected" | sort -u)"
  actual="$(sed -E 's/#[0-9]+$//' "$process_file" | sort -u)"
  assert_eq "$expected" "$actual" "process set for $profile"

  if grep -Eq '^end4#[0-9]+$' "$process_file" &&
    grep -Eq '^end4-pc#[0-9]+$' "$process_file"; then
    fail "End4 Official and pC coexist"
  fi
}

profile_instance() {
  local process_file="$1"
  local profile="$2"

  grep -E -- "^${profile}#[0-9]+$" "$process_file"
}

assert_success_order() {
  local event_file="$1"
  local profile="$2"
  local context="$3"
  local prepare_line action_line persist_line

  prepare_line="$(grep -n -m1 -F "prepare:$profile" "$event_file" | cut -d: -f1 || true)"
  action_line="$(grep -n -m1 -E '^(stop:|start:)' "$event_file" | cut -d: -f1 || true)"
  persist_line="$(grep -n -m1 -F "persist:$profile" "$event_file" | cut -d: -f1 || true)"

  [ -n "$prepare_line" ] || fail "$context has no successful prepare event"
  [ -n "$action_line" ] || fail "$context has no stop/start event"
  [ -n "$persist_line" ] || fail "$context has no persist event"
  if [ "$prepare_line" -ge "$action_line" ] || [ "$action_line" -ge "$persist_line" ]; then
    fail "$context event order is not prepare < stop/start < persist"
  fi
}

assert_no_target_stop() {
  local event_file="$1"
  local profile="$2"
  local forbidden

  case "$profile" in
    caelestia) forbidden=stop:caelestia ;;
    noctalia) forbidden=stop:noctalia ;;
    end4 | end4-pc) forbidden=stop:end4-family ;;
  esac
  if grep -Fqx -- "$forbidden" "$event_file"; then
    fail "$profile same-profile reuse performed target-specific stop $forbidden"
  fi
}

assert_event_before() {
  local event_file="$1"
  local earlier="$2"
  local later="$3"
  local message="$4"
  local earlier_line later_line

  earlier_line="$(grep -n -m1 -F -- "$earlier" "$event_file" | cut -d: -f1 || true)"
  later_line="$(grep -n -m1 -F -- "$later" "$event_file" | cut -d: -f1 || true)"
  [ -n "$earlier_line" ] || fail "$message has no $earlier event"
  [ -n "$later_line" ] || fail "$message has no $later event"
  [ "$earlier_line" -lt "$later_line" ] || fail "$message event order is not $earlier before $later"
}

bundle_paths_for_root() {
  local root="$1"
  local hypr="$root/home/.config/hypr"
  local runtime="$root/home/.local/state/wahrwelt/hypr-runtime"

  printf '%s\n' \
    "$hypr/hyprland.lua" \
    "$runtime/shell-profile.lua" \
    "$runtime/shell-launcher.lua" \
    "$runtime/shell-keybinds.lua" \
    "$runtime/hyprland.lua" \
    "$runtime/hyprlock.conf" \
    "$runtime/hypridle.conf" \
    "$hypr/hyprland.conf" \
    "$hypr/shell-profile.conf" \
    "$hypr/shell-launcher.conf" \
    "$hypr/shell-keybinds.conf" \
    "$hypr/wahrwelt/hyprland.conf" \
    "$runtime/hyprland.conf" \
    "$runtime/shell-profile.conf" \
    "$runtime/shell-launcher.conf" \
    "$runtime/shell-keybinds.conf"
}

seed_bundle() {
  local root="$1"
  local path link_source index=0

  while IFS= read -r path; do
    index=$((index + 1))
    mkdir -p -- "$(dirname -- "$path")"
    case $((index % 3)) in
      0)
        printf 'prior bundle path=%s\nsecond line\n' "$index" >"$path"
        chmod 0600 "$path"
        ;;
      1)
        link_source="$root/bundle-source-$index"
        printf 'prior symlink source=%s\n' "$index" >"$link_source"
        ln -s -- "$link_source" "$path"
        ;;
      2)
        ;;
    esac
  done < <(bundle_paths_for_root "$root")
}

describe_paths() {
  local path mode digest

  for path in "$@"; do
    if [ -L "$path" ]; then
      printf 'symlink %s\n' "$(readlink -- "$path")"
    elif [ -f "$path" ]; then
      mode="$(stat -c %a "$path")"
      digest="$(sha256sum "$path" | awk '{ print $1 }')"
      printf 'regular %s %s\n' "$mode" "$digest"
    elif [ -e "$path" ]; then
      printf 'other\n'
    else
      printf 'absent\n'
    fi
  done
}

capture_case_state() {
  local root="$1"
  local paths=()

  mapfile -t paths < <(bundle_paths_for_root "$root")
  paths+=(
    "$root/home/.local/state/wahrwelt/active-shell"
    "$root/home/.local/state/wahrwelt/end4-variant"
  )
  describe_paths "${paths[@]}"
}

new_case_root() {
  local name="$1"
  local root="$test_root/cases/$name"

  mkdir -p "$root/home/.local/state/wahrwelt" "$root/runtime"
  chmod 0700 "$root/runtime"
  printf '%s' "$root"
}

seed_case() {
  local root="$1"
  local previous="$2"
  local state="$root/home/.local/state/wahrwelt"

  printf '%s\n' "$previous" >"$state/active-shell"
  case "$previous" in
    end4 | end4-pc) printf '%s\n' "$previous" >"$state/end4-variant" ;;
    *) printf '%s\n' end4-pc >"$state/end4-variant" ;;
  esac
  chmod 0600 "$state/active-shell"
  chmod 0640 "$state/end4-variant"

  seed_bundle "$root"

  printf '%s\n' 1 >"$root/generation"
  printf '%s\n' unmanaged-hypridle "$previous#1" >"$root/processes"
  case "$previous" in
    end4 | end4-pc) printf '%s\n' end4-idle >>"$root/processes" ;;
  esac
  : >"$root/events"
  : >"$root/clients.json"
  printf '%s\n' '[]' >"$root/clients.json"
  printf '%s\n' '{}' >"$root/activewindow.json"
  printf '%s\n' '[{"name":"eDP-1","focused":true,"specialWorkspace":{"id":0,"name":""}}]' \
    >"$root/monitors.json"
  printf '%s\n' ':1.1' >"$root/watcher-owner"
  : >"$root/focus-props"
}

seed_spotify() {
  local root="$1"
  local visible="${2:-hidden}"

  printf '%s\n' '[
    {"address":"0xabc123","class":"sPoTiFy","pid":100,"monitor":0,"mapped":true,"workspace":{"id":-98,"name":"special:music"}},
    {"address":"0xdef456","class":"foot","pid":200,"monitor":0,"mapped":true,"workspace":{"id":1,"name":"1"}}
  ]' >"$root/clients.json"
  printf '%s\n' \
    '{"address":"0xdef456","class":"foot","pid":200,"monitor":0,"workspace":{"id":1,"name":"1"}}' \
    >"$root/activewindow.json"
  if [ "$visible" = visible ]; then
    printf '%s\n' '[{"name":"eDP-1","specialWorkspace":{"id":-98,"name":"special:music"}}]' \
      >"$root/monitors.json"
  fi
}

run_switch() {
  local root="$1"
  local target="${2:-}"
  local fail_prepare="${3:-}"
  local fail_start="${4:-}"
  local fail_persist="${5:-}"
  local fail_prepare_index="${6:-}"
  local abort_prepare="${7:-}"
  local abort_after_start="${8:-}"
  local abort_after_persist="${9:-}"
  local abort_after_propagate="${10:-}"
  local replace_after_publish_index="${11:-}"
  local fail_reload="${12:-}"
  local switch_status command_summary
  local args=()

  [ -z "$target" ] || args+=("$target")
  HOME="$root/home" \
    XDG_CONFIG_HOME="$root/home/.config" \
    XDG_STATE_HOME="$root/home/.local/state" \
    XDG_RUNTIME_DIR="$root/runtime" \
    WAHRWELT_TEST_PROCESSES="$root/processes" \
    WAHRWELT_TEST_GENERATION="$root/generation" \
    WAHRWELT_TEST_EVENTS="$root/events" \
    WAHRWELT_TEST_CLIENTS="$root/clients.json" \
    WAHRWELT_TEST_ACTIVE_WINDOW="$root/activewindow.json" \
    WAHRWELT_TEST_MONITORS="$root/monitors.json" \
    WAHRWELT_TEST_WATCHER_OWNER="$root/watcher-owner" \
    WAHRWELT_TEST_WATCHER_PROCESS="${WAHRWELT_TEST_WATCHER_PROCESS:-}" \
    WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER="${WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER:-}" \
    WAHRWELT_TEST_FOCUS_PROPS="$root/focus-props" \
    WAHRWELT_TEST_SPOTIFY_GAP_FIRED="$root/spotify-gap-fired" \
    WAHRWELT_TEST_SPOTIFY_GAP_MODE="${WAHRWELT_TEST_SPOTIFY_GAP_MODE:-}" \
    WAHRWELT_TEST_SPOTIFY_REVEAL_MONITOR="${WAHRWELT_TEST_SPOTIFY_REVEAL_MONITOR:-}" \
    WAHRWELT_TEST_SPOTIFY_HIDE_STEALS_FOCUS="${WAHRWELT_TEST_SPOTIFY_HIDE_STEALS_FOCUS:-}" \
    WAHRWELT_TEST_FAIL_SPOTIFY_SNAPSHOT="${WAHRWELT_TEST_FAIL_SPOTIFY_SNAPSHOT:-}" \
    WAHRWELT_TEST_WATCHER_UNREADY="${WAHRWELT_TEST_WATCHER_UNREADY:-}" \
    WAHRWELT_TEST_FAIL_PREPARE="$fail_prepare" \
    WAHRWELT_TEST_FAIL_START="$fail_start" \
    WAHRWELT_TEST_FAIL_PERSIST="$fail_persist" \
    WAHRWELT_TEST_FAIL_PREPARE_INDEX="$fail_prepare_index" \
    WAHRWELT_TEST_ABORT_PREPARE="$abort_prepare" \
    WAHRWELT_TEST_ABORT_AFTER_START="$abort_after_start" \
    WAHRWELT_TEST_ABORT_AFTER_PERSIST="$abort_after_persist" \
    WAHRWELT_TEST_ABORT_AFTER_PROPAGATE="$abort_after_propagate" \
    WAHRWELT_TEST_REPLACE_AFTER_PUBLISH_INDEX="$replace_after_publish_index" \
    WAHRWELT_TEST_FAIL_RELOAD="$fail_reload" \
    "$instrumented_scripts/start-shell.sh" "${args[@]}"
  switch_status=$?
  if [ -s "$environment_command_log" ]; then
    command_summary="$(tr '\n' ';' <"$environment_command_log")"
    fail "instrumented start-shell reached live environment commands: $command_summary"
  fi
  return "$switch_status"
}

root="$(new_case_root spotify-same-profile-surviving-watcher)"
seed_case "$root" noctalia
seed_spotify "$root"
before_instance="$(profile_instance "$root/processes" noctalia)"
run_switch "$root"
after_instance="$(profile_instance "$root/processes" noctalia)"
assert_eq "$before_instance" "$after_instance" \
  "same-profile surviving watcher keeps the existing shell process"
assert_eq '' "$(cat "$root/focus-props")" \
  "same-profile surviving watcher restores Spotify focus-on-activate"
assert_eq 1 "$(grep -Fc 'spotify-focus-block:0xabc123' "$root/events")" \
  "same-profile surviving watcher guards exact hidden Spotify once"
assert_eq 1 "$(grep -Fc 'spotify-focus-restore:0xabc123' "$root/events")" \
  "same-profile surviving watcher restores exact hidden Spotify once"
grep -Fqx 'watcher-ready::1.1' "$root/events" ||
  fail "same-profile surviving watcher did not accept the same ready owner"
if grep -Fqx 'log:StatusNotifierWatcher readiness timeout' "$root/events"; then
  fail "same-profile surviving watcher waited for an unnecessary replacement owner"
fi
if grep -Fqx 'sleep:0.05' "$root/events"; then
  fail "same-profile surviving watcher added a polling cooldown"
fi
if grep -Eq '^busctl-(unbounded|wrong-timeout):' "$root/events"; then
  fail "same-profile surviving watcher used the wrong D-Bus timeout contract"
fi

if [ "${WAHRWELT_TEST_SPOTIFY_FAST_SAME_OWNER_ONLY:-0}" = 1 ]; then
  printf 'OK shell transition same-owner Spotify regression\n'
  exit 0
fi

root="$(new_case_root spotify-guarded-window-active)"
seed_case "$root" noctalia
seed_spotify "$root"
jq '.[] | select(.address == "0xabc123")' "$root/clients.json" >"$root/activewindow.json"
WAHRWELT_TEST_SPOTIFY_GAP_MODE=respect run_switch "$root" end4
assert_process_set "$root/processes" end4
if grep -Eq '^spotify-focus-(block|restore):' "$root/events"; then
  fail "active guarded Spotify was retained as a focus recovery target"
fi
grep -Fqx \
  'log:Spotify activation snapshot focused the guarded window; continuing without focus guard' \
  "$root/events" || fail "active guarded Spotify snapshot did not fail open"

if [ "${WAHRWELT_TEST_SPOTIFY_ACTIVE_GUARDED_ONLY:-0}" = 1 ]; then
  printf 'OK shell transition active-guarded Spotify regression\n'
  exit 0
fi

root="$(new_case_root spotify-hidden-success)"
seed_case "$root" noctalia
seed_spotify "$root"
WAHRWELT_TEST_SPOTIFY_GAP_MODE=respect run_switch "$root" end4
assert_eq '' "$(cat "$root/focus-props")" "successful switch restores Spotify focus-on-activate"
assert_eq '' "$(jq -r '.[] | select(.specialWorkspace.name == "special:music") | .name' "$root/monitors.json")" \
  "successful switch keeps hidden Spotify workspace hidden"
assert_eq 1 "$(grep -Fc 'spotify-focus-block:0xabc123' "$root/events")" \
  "successful switch guards exact hidden Spotify once"
assert_eq 1 "$(grep -Fc 'spotify-focus-restore:0xabc123' "$root/events")" \
  "successful switch restores exact hidden Spotify once"
grep -Fqx 'spotify-activation-blocked:0xabc123' "$root/events" ||
  fail "hidden Spotify activation was not blocked during the watcher gap"
if grep -Fqx 'hide:special:music' "$root/events"; then
  fail "blocked Spotify activation redundantly toggled the hidden workspace"
fi
assert_event_before "$root/events" 'spotify-focus-block:0xabc123' 'stop:selector' \
  "successful hidden Spotify guard"
assert_event_before "$root/events" 'start:end4' 'watcher-ready::1.2' \
  "successful hidden Spotify watcher replacement"
assert_event_before "$root/events" 'reload' 'spotify-focus-restore:0xabc123' \
  "successful hidden Spotify reload"
assert_event_before "$root/events" 'watcher-ready::1.2' 'spotify-focus-restore:0xabc123' \
  "successful hidden Spotify watcher readiness"
if grep -Eq '^busctl-(unbounded|wrong-timeout):' "$root/events"; then
  fail "successful hidden Spotify guard used the wrong D-Bus timeout contract"
fi
grep -Fqx 'busctl-bounded:call' "$root/events" ||
  fail "Spotify guard did not bound the watcher owner call"
grep -Fqx 'busctl-bounded:get-property' "$root/events" ||
  fail "Spotify guard did not bound the watcher readiness call"

root="$(new_case_root spotify-snapshot-unavailable)"
seed_case "$root" noctalia
seed_spotify "$root"
if ! WAHRWELT_TEST_FAIL_SPOTIFY_SNAPSHOT=clients run_switch "$root" end4; then
  fail "unavailable Spotify snapshot blocked the shell switch"
fi
assert_process_set "$root/processes" end4
if grep -Eq '^spotify-focus-(block|restore):' "$root/events"; then
  fail "unavailable Spotify snapshot left a partial focus guard"
fi

root="$(new_case_root spotify-watcher-timeout)"
seed_case "$root" noctalia
seed_spotify "$root"
if ! WAHRWELT_TEST_SPOTIFY_GAP_MODE=respect WAHRWELT_TEST_WATCHER_UNREADY=1 \
  run_switch "$root" end4; then
  fail "StatusNotifierWatcher timeout rolled back a successful shell switch"
fi
assert_process_set "$root/processes" end4
assert_eq '' "$(cat "$root/focus-props")" "watcher timeout restores Spotify focus-on-activate"
assert_eq 1 "$(grep -Fc 'spotify-focus-restore:0xabc123' "$root/events")" \
  "watcher timeout restores exact hidden Spotify once"
grep -Fqx 'log:StatusNotifierWatcher readiness timeout' "$root/events" ||
  fail "StatusNotifierWatcher timeout was not logged"
assert_event_before "$root/events" 'reload' 'spotify-focus-restore:0xabc123' \
  "watcher timeout cleanup"
bounded_busctl_calls="$(grep -c '^busctl-bounded:' "$root/events" || true)"
watcher_poll_sleeps="$(grep -c '^sleep:0.05$' "$root/events" || true)"
watcher_timeout_budget_ms=$((bounded_busctl_calls * 50 + watcher_poll_sleeps * 50))
if [ "$watcher_timeout_budget_ms" -gt 2000 ]; then
  fail "StatusNotifierWatcher exceptional timeout budget is ${watcher_timeout_budget_ms}ms, want <=2000ms"
fi

root="$(new_case_root spotify-reveal-recovery)"
seed_case "$root" noctalia
seed_spotify "$root"
WAHRWELT_TEST_SPOTIFY_GAP_MODE=force run_switch "$root" end4
assert_eq '' "$(cat "$root/focus-props")" "revealed Spotify cleanup restores focus-on-activate"
assert_eq '' "$(jq -r '.[] | select(.specialWorkspace.name == "special:music") | .name' "$root/monitors.json")" \
  "revealed Spotify workspace is hidden again"
assert_eq 1 "$(grep -Fc 'hide:special:music' "$root/events")" \
  "revealed Spotify workspace is hidden exactly once"
assert_event_before "$root/events" 'watcher-ready::1.2' 'hide:special:music' \
  "revealed Spotify recovery waits for the replacement watcher"

root="$(new_case_root spotify-multi-monitor-reveal-recovery)"
seed_case "$root" noctalia
seed_spotify "$root"
printf '%s\n' '[
  {"name":"eDP-1","focused":true,"specialWorkspace":{"id":0,"name":""}},
  {"name":"HDMI-A-1","focused":false,"specialWorkspace":{"id":0,"name":""}}
]' >"$root/monitors.json"
WAHRWELT_TEST_SPOTIFY_GAP_MODE=force \
  WAHRWELT_TEST_SPOTIFY_REVEAL_MONITOR=HDMI-A-1 \
  WAHRWELT_TEST_SPOTIFY_HIDE_STEALS_FOCUS=1 \
  run_switch "$root" end4
assert_eq '' "$(jq -r '.[] | select(.specialWorkspace.name == "special:music") | .name' \
  "$root/monitors.json")" "multi-monitor recovery hides Spotify on its actual monitor"
assert_eq eDP-1 "$(jq -r '.[] | select(.focused == true) | .name' "$root/monitors.json")" \
  "multi-monitor recovery preserves the focused monitor"
assert_eq 0xdef456 "$(jq -r '.address // empty' "$root/activewindow.json")" \
  "multi-monitor recovery preserves the exact active window"
assert_eq 1 "$(grep -Fc 'hide:special:music:HDMI-A-1' "$root/events")" \
  "multi-monitor recovery hides the exact monitor once"
if grep -Fqx 'hide:special:music:eDP-1' "$root/events"; then
  fail "multi-monitor recovery hid Spotify on the focused monitor"
fi

root="$(new_case_root spotify-visible-before-switch)"
seed_case "$root" noctalia
seed_spotify "$root" visible
WAHRWELT_TEST_SPOTIFY_GAP_MODE=force run_switch "$root" end4
if grep -Eq '^spotify-focus-(block|restore):' "$root/events"; then
  fail "visible Spotify received a hidden-workspace focus guard"
fi
if grep -Fqx 'hide:special:music' "$root/events"; then
  fail "workspace visible before the switch was hidden"
fi
assert_eq eDP-1 "$(jq -r '.[] | select(.specialWorkspace.name == "special:music") | .name' "$root/monitors.json")" \
  "workspace visible before the switch remains visible"

root="$(new_case_root spotify-same-profile-stale-watcher)"
seed_case "$root" noctalia
seed_spotify "$root"
printf '%s\n' 'caelestia#2' >>"$root/processes"
WAHRWELT_TEST_SPOTIFY_GAP_MODE=respect \
  WAHRWELT_TEST_WATCHER_PROCESS=caelestia \
  WAHRWELT_TEST_REPLACEMENT_WATCHER_OWNER=:1.2 \
  run_switch "$root"
assert_process_set "$root/processes" noctalia
assert_eq '' "$(jq -r '.[] | select(.specialWorkspace.name == "special:music") | .name' \
  "$root/monitors.json")" "same-profile stale watcher cleanup keeps Spotify hidden"
assert_eq '' "$(cat "$root/focus-props")" \
  "same-profile stale watcher cleanup restores Spotify focus-on-activate"
grep -Fqx 'spotify-activation-blocked:0xabc123' "$root/events" ||
  fail "same-profile stale watcher cleanup did not block Spotify activation"
assert_event_before "$root/events" 'spotify-focus-block:0xabc123' 'stop:caelestia' \
  "same-profile stale watcher guard"
assert_event_before "$root/events" 'watcher-ready::1.2' 'spotify-focus-restore:0xabc123' \
  "same-profile stale watcher readiness"

root="$(new_case_root spotify-start-failure-rollback)"
seed_case "$root" noctalia
seed_spotify "$root"
if WAHRWELT_TEST_SPOTIFY_GAP_MODE=respect run_switch "$root" end4 "" end4; then
  fail "Spotify rollback fixture hid the requested start failure"
fi
assert_eq '' "$(cat "$root/focus-props")" "fallback rollback restores Spotify focus-on-activate"
assert_process_set "$root/processes" noctalia
assert_eq 1 "$(grep -Fc 'spotify-focus-block:0xabc123' "$root/events")" \
  "fallback rollback guards exact hidden Spotify once"
assert_eq 1 "$(grep -Fc 'spotify-focus-restore:0xabc123' "$root/events")" \
  "fallback rollback restores exact hidden Spotify once"
assert_event_before "$root/events" 'start:noctalia' 'watcher-ready::1.3' \
  "fallback rollback watcher replacement"
assert_event_before "$root/events" 'reload' 'spotify-focus-restore:0xabc123' \
  "fallback rollback reload"
assert_event_before "$root/events" 'watcher-ready::1.3' 'spotify-focus-restore:0xabc123' \
  "fallback rollback watcher readiness"

if [ "${WAHRWELT_TEST_SPOTIFY_ONLY:-0}" = 1 ]; then
  printf 'OK shell transition Spotify regressions\n'
  exit 0
fi

profiles=(caelestia noctalia end4 end4-pc)
case_index=0
for previous in "${profiles[@]}"; do
  for target in "${profiles[@]}"; do
    case_index=$((case_index + 1))
    root="$(new_case_root "matrix-$case_index")"
    seed_case "$root" "$previous"
    user_hypr_conf="$root/home/.config/hypr/user/hyprland.conf"
    mkdir -p -- "$(dirname -- "$user_hypr_conf")"
    printf '%s\n' 'user-owned legacy-format config' >"$user_hypr_conf"
    run_switch "$root" "$target"

    if grep -Fq 'new start-shell lock changed before ownership record' "$root/runtime/wahrwelt-shell.log" 2>/dev/null; then
      fail "$previous -> $target continued after rejecting its own start-shell lock"
    fi

    assert_eq 'user-owned legacy-format config' "$(tr -d '\n' <"$user_hypr_conf")" \
      "$previous -> $target preserves user/hyprland.conf"
    if [ -e "$root/home/.config/hypr/wahrwelt/hyprland.conf" ] ||
      [ -L "$root/home/.config/hypr/wahrwelt/hyprland.conf" ]; then
      fail "$previous -> $target retained known legacy wahrwelt/hyprland.conf"
    fi

    state_dir="$root/home/.local/state/wahrwelt"
    assert_eq "$target" "$(tr -d '[:space:]' <"$state_dir/active-shell")" \
      "$previous -> $target active state"
    if [[ "$target" == end4* ]]; then
      expected_variant="$target"
    elif [[ "$previous" == end4* ]]; then
      expected_variant="$previous"
    else
      expected_variant=end4-pc
    fi
    assert_eq "$expected_variant" "$(tr -d '[:space:]' <"$state_dir/end4-variant")" \
      "$previous -> $target variant state"
    assert_process_set "$root/processes" "$target"
    assert_success_order "$root/events" "$target" "$previous -> $target"

    before_instance="$(profile_instance "$root/processes" "$target")"
    : >"$root/events"
    run_switch "$root"
    assert_eq "$target" "$(tr -d '[:space:]' <"$state_dir/active-shell")" \
      "$target same-profile active state"
    assert_eq "$expected_variant" "$(tr -d '[:space:]' <"$state_dir/end4-variant")" \
      "$target same-profile variant state"
    assert_process_set "$root/processes" "$target"
    after_instance="$(profile_instance "$root/processes" "$target")"
    assert_eq "$before_instance" "$after_instance" "$target same-profile process instance reuse"
    assert_no_target_stop "$root/events" "$target"
    assert_success_order "$root/events" "$target" "$target same-profile reuse"
  done
done

root="$(new_case_root successful-transition-residue)"
seed_case "$root" noctalia
run_switch "$root" end4
run_switch "$root" noctalia
if residue="$(find "$root" \( -name '.runtime-rollback.*' -o -name '.state-rollback.*' -o -name '.state-switch-rollback.*' -o -name '.wahrwelt-runtime-recovery-*' -o -name '.wahrwelt-lock-recovery-*' -o -name '.wahrwelt-runtime-rollback-*' -o -name '.*.stage.*' \) -print -quit)" && [ -n "$residue" ]; then
  fail "successful repeated transitions retained transaction residue at $residue"
fi
while IFS= read -r -d '' retained_stage; do
  [ -f "$retained_stage" ] && [ ! -L "$retained_stage" ] ||
    fail "retained runtime stage is not a regular file: $retained_stage"
  IFS=: read -r retained_links retained_uid retained_mode < <(stat -c '%h:%u:%a' -- "$retained_stage")
  [ "$retained_links" = 1 ] || fail "retained runtime stage is hardlinked: $retained_stage"
  [ "$retained_uid" = "$UID" ] || fail "retained runtime stage has an unexpected owner: $retained_stage"
  case "$retained_mode" in
    600 | 640 | 644) ;;
    *) fail "retained runtime stage has an unsafe mode $retained_mode: $retained_stage" ;;
  esac
done < <(find "$root" -name '.wahrwelt-runtime-stage-*' -print0)

for fail_index in $(seq 1 16); do
  root="$(new_case_root "prepare-failure-$fail_index")"
  seed_case "$root" noctalia
  before_state="$(capture_case_state "$root")"
  if run_switch "$root" end4 "" "" "" "$fail_index"; then
    fail "prepare failure at bundle path $fail_index returned success"
  fi
  after_state="$(capture_case_state "$root")"
  assert_eq "$before_state" "$after_state" "prepare failure $fail_index restores exact bundle and state"
  assert_process_set "$root/processes" noctalia
  if grep -Eq '^(stop:|start:)' "$root/events"; then
    fail "prepare failure $fail_index stopped or started a shell"
  fi
done

root="$(new_case_root bundle-concurrent-winner)"
seed_case "$root" noctalia
winner_path="$root/home/.local/state/wahrwelt/hypr-runtime/shell-profile.lua"
if run_switch "$root" end4 "" "" "" 3 "" "" "" "" 2; then
  fail "bundle concurrent-winner preparation failure returned success"
fi
assert_eq 'concurrent bundle winner path=2' "$(tr -d '\n' <"$winner_path")" \
  "rollback preserves a winner swapped between publication and ownership record"
if ! find "$root/runtime" -maxdepth 1 -type d -name '.runtime-rollback-*' -print -quit | grep -q .; then
  fail "concurrent bundle winner did not retain transaction recovery"
fi
assert_process_set "$root/processes" noctalia

root="$(new_case_root bundle-collision)"
seed_case "$root" noctalia
collision_path="$root/home/.local/state/wahrwelt/hypr-runtime/hyprland.lua"
rm -f -- "$collision_path"
mkdir -p -- "$collision_path"
printf '%s\n' untouched >"$collision_path/user-data"
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4; then
  fail "non-regular bundle collision returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "non-regular bundle collision remains untouched"
assert_eq untouched "$(tr -d '\n' <"$collision_path/user-data")" "bundle collision contents remain untouched"
assert_process_set "$root/processes" noctalia
if grep -q '^prepare:' "$root/events"; then
  fail "bundle collision reached preparation"
fi

root="$(new_case_root start-failure-no-fallback)"
seed_case "$root" noctalia
printf '%s\n' invalid >"$root/home/.local/state/wahrwelt/active-shell"
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" end4; then
  fail "start failure without fallback returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "start failure without fallback restores exact bundle and state"
assert_eq unmanaged-hypridle "$(sort -u "$root/processes")" "failed End4 start without fallback cleans managed processes"

root="$(new_case_root start-failure-fallback)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" end4; then
  fail "successful fallback hid the requested start failure"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "successful fallback restores exact prior bundle and state"
assert_eq noctalia "$(tr -d '[:space:]' <"$root/home/.local/state/wahrwelt/active-shell")" \
  "fallback restores previous active state"
assert_process_set "$root/processes" noctalia
grep -Fqx 'start:end4' "$root/events" || fail "failed End4 start was not attempted"
grep -Fqx 'start:noctalia' "$root/events" || fail "previous profile fallback was not attempted"

root="$(new_case_root exact-variant-fallback)"
seed_case "$root" end4-pc
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" end4; then
  fail "End4 variant fallback hid the requested start failure"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "End4 variant fallback restores exact bundle and state"
assert_eq end4-pc "$(tr -d '[:space:]' <"$root/home/.local/state/wahrwelt/active-shell")" \
  "End4 fallback restores exact profile"
assert_eq end4-pc "$(tr -d '[:space:]' <"$root/home/.local/state/wahrwelt/end4-variant")" \
  "End4 fallback persists exact variant"
assert_process_set "$root/processes" end4-pc

root="$(new_case_root fallback-prepare-failure)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 noctalia end4; then
  fail "fallback prepare failure returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "fallback prepare failure restores exact bundle and state"
assert_process_set "$root/processes" noctalia

root="$(new_case_root fallback-start-failure)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" end4,noctalia; then
  fail "fallback start failure returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "fallback start failure restores exact bundle and state"
assert_eq unmanaged-hypridle "$(sort -u "$root/processes")" "fallback start failure cleans every failed managed process"

root="$(new_case_root primary-persistence-failure)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" "" end4; then
  fail "primary persistence failure returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "primary persistence failure and fallback restore exact bundle and state"
assert_process_set "$root/processes" noctalia
assert_eq 1 "$(grep -c '^reload$' "$root/events")" "successful fallback reloads exactly once after persistence"

root="$(new_case_root post-sync-reload-failure)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" "" "" "" "" "" "" "" "" end4; then
  fail "post-sync reload failure returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "post-sync reload failure restores exact runtime and state"
assert_process_set "$root/processes" noctalia
grep -Fqx 'reload' "$root/events" || fail "post-sync reload failure did not reach strict reload"

root="$(new_case_root fallback-persistence-failure)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" end4 noctalia; then
  fail "fallback persistence failure returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "fallback persistence failure restores exact bundle and state"
assert_process_set "$root/processes" noctalia
if grep -Eq '^(reload|propagate)$' "$root/events"; then
  fail "fallback persistence failure reloaded or reported runtime propagation"
fi

root="$(new_case_root persistence-failure-no-fallback)"
seed_case "$root" noctalia
printf '%s\n' invalid >"$root/home/.local/state/wahrwelt/active-shell"
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" "" end4; then
  fail "persistence failure without fallback returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "persistence failure without fallback restores exact bundle and state"
assert_eq unmanaged-hypridle "$(sort -u "$root/processes")" "persistence failure without fallback cleans requested End4"

root="$(new_case_root absent-state-persistence-failure)"
seed_case "$root" noctalia
rm -f -- \
  "$root/home/.local/state/wahrwelt/active-shell" \
  "$root/home/.local/state/wahrwelt/end4-variant"
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" "" end4; then
  fail "persistence failure with absent prior state returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "persistence failure restores exact prior state absence"
assert_eq unmanaged-hypridle "$(sort -u "$root/processes")" "absent-state persistence failure cleans requested End4"

for failed_profile in caelestia noctalia; do
  if [ "$failed_profile" = caelestia ]; then
    previous_profile=noctalia
  else
    previous_profile=caelestia
  fi
  root="$(new_case_root "failed-family-cleanup-$failed_profile")"
  seed_case "$root" "$previous_profile"
  before_state="$(capture_case_state "$root")"
  if run_switch "$root" "$failed_profile" "" "$failed_profile"; then
    fail "$failed_profile failure with fallback returned success"
  fi
  after_state="$(capture_case_state "$root")"
  assert_eq "$before_state" "$after_state" "$failed_profile failure restores exact bundle and state"
  assert_process_set "$root/processes" "$previous_profile"
done

root="$(new_case_root unexpected-exit-rollback)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
if run_switch "$root" end4 "" "" "" "" end4 2>"$root/abort.stderr"; then
  fail "unexpected exit injection returned success"
fi
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "EXIT trap restores exact bundle and state"
assert_process_set "$root/processes" noctalia

root="$(new_case_root signal-after-start-rollback)"
seed_case "$root" noctalia
seed_spotify "$root"
before_state="$(capture_case_state "$root")"
set +e
WAHRWELT_TEST_SPOTIFY_GAP_MODE=respect \
  run_switch "$root" end4 "" "" "" "" "" end4 2>"$root/abort-after-start.stderr"
signal_status=$?
set -e
assert_eq 143 "$signal_status" "TERM after target start preserves signal status"
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "TERM after target start restores exact bundle and state"
assert_process_set "$root/processes" noctalia
grep -Fqx 'abort-after-start:end4' "$root/events" || fail "post-start TERM hook did not run"
grep -Fqx 'start:noctalia' "$root/events" || fail "post-start TERM did not restart previous shell"
assert_eq '' "$(cat "$root/focus-props")" "TERM rollback restores Spotify focus-on-activate"
assert_eq 1 "$(grep -Fc 'spotify-focus-restore:0xabc123' "$root/events")" \
  "TERM rollback restores exact hidden Spotify once"
assert_event_before "$root/events" 'watcher-ready::1.3' 'spotify-focus-restore:0xabc123' \
  "TERM rollback watcher readiness"
if grep -Eq '^(reload|propagate)$' "$root/events"; then
  fail "post-start TERM reloaded an uncommitted runtime"
fi

root="$(new_case_root signal-after-persist-rollback)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
set +e
run_switch "$root" end4 "" "" "" "" "" "" end4 2>"$root/abort-after-persist.stderr"
signal_status=$?
set -e
assert_eq 143 "$signal_status" "TERM after state persistence preserves signal status"
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "TERM after state persistence restores exact bundle and state"
assert_process_set "$root/processes" noctalia
grep -Fqx 'abort-after-persist:end4' "$root/events" || fail "post-persist TERM hook did not run"
grep -Fqx 'start:noctalia' "$root/events" || fail "post-persist TERM did not restart previous shell"
if grep -Eq '^(reload|propagate)$' "$root/events"; then
  fail "post-persist TERM reloaded an uncommitted runtime"
fi

root="$(new_case_root signal-after-propagate-rollback)"
seed_case "$root" noctalia
before_state="$(capture_case_state "$root")"
set +e
run_switch "$root" end4 "" "" "" "" "" "" "" end4 2>"$root/abort-after-propagate.stderr"
signal_status=$?
set -e
assert_eq 143 "$signal_status" "TERM before transaction commit preserves signal status"
after_state="$(capture_case_state "$root")"
assert_eq "$before_state" "$after_state" "TERM before transaction commit restores exact bundle and state"
assert_process_set "$root/processes" noctalia
grep -Fqx 'abort-after-propagate:end4' "$root/events" || fail "pre-commit TERM hook did not run"
grep -Fqx 'start:noctalia' "$root/events" || fail "pre-commit TERM did not restart previous shell"
assert_eq 2 "$(grep -c '^reload$' "$root/events")" "pre-commit TERM reloads target then restored runtime"

printf 'OK shell transition matrix\n'
