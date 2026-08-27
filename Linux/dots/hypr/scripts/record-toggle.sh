#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=Linux/dots/hypr/scripts/shell-runtime.sh
. "$script_dir/shell-runtime.sh"

wahrwelt_enter_runtime_lock_v2 \
  wahrwelt-record-toggle-v2.lock 40 0 "$0" "$@"

if ! wahrwelt_adopt_legacy_private_state_directory wahrwelt-recording record-toggle-state; then
  printf 'Wahrwelt recorder ownership collision: unsafe pre-marker state preserved\n' >&2
  exit 1
fi
if ! wahrwelt_open_private_state_directory wahrwelt-recording record-toggle-state; then
  printf 'Wahrwelt recorder ownership collision: %s/wahrwelt-recording\n' \
    "$wahrwelt_runtime_session_public_dir" >&2
  exit 1
fi
state_dir="$wahrwelt_private_state_directory_path"
state_dir_fd="$wahrwelt_private_state_directory_fd"
if ! wahrwelt_open_managed_regular_file "$state_dir_fd" "$state_dir" gpu-screen-recorder.pid recorder-pid; then
  printf 'Wahrwelt recorder ownership collision: PID state preserved\n' >&2
  exit 1
fi
exec {pid_fd}<&"$wahrwelt_managed_regular_fd"
pid_file="/proc/${BASHPID:-$$}/fd/$pid_fd"
exec {wahrwelt_managed_regular_fd}<&-
wahrwelt_managed_regular_fd=""
if ! wahrwelt_open_managed_regular_file "$state_dir_fd" "$state_dir" gpu-screen-recorder.path recorder-path; then
  printf 'Wahrwelt recorder ownership collision: path state preserved\n' >&2
  exit 1
fi
exec {path_fd}<&"$wahrwelt_managed_regular_fd"
path_file="/proc/${BASHPID:-$$}/fd/$path_fd"
exec {wahrwelt_managed_regular_fd}<&-
wahrwelt_managed_regular_fd=""
if ! wahrwelt_open_managed_regular_file "$state_dir_fd" "$state_dir" gpu-screen-recorder.log recorder-log; then
  printf 'Wahrwelt recorder ownership collision: log state preserved\n' >&2
  exit 1
fi
exec {recorder_log_fd}<&"$wahrwelt_managed_regular_fd"
log_file="/proc/${BASHPID:-$$}/fd/$recorder_log_fd"
exec {wahrwelt_managed_regular_fd}<&-
wahrwelt_managed_regular_fd=""
record_dir="${XDG_VIDEOS_DIR:-$HOME/Videos}/Recordings"

notify() {
  if command -v notify-send >/dev/null 2>&1; then
    notify-send -a "Recorder" "$@"
  fi
}

focused_monitor() {
  if command -v hyprctl >/dev/null 2>&1; then
    hyprctl monitors 2>/dev/null | awk '
      /^Monitor / { monitor = $2 }
      /focused: yes/ { print monitor; exit }
    '
  fi
}

stop_recording() {
  local pid="$1"
  local file=""

  if ! ps -p "$pid" -o args= 2>/dev/null | grep -qE '(^|/)gpu-screen-recorder([[:space:]]|$)'; then
    : >"$pid_file"
    : >"$path_file"
    notify "Recording state cleared" "Stored PID no longer belongs to gpu-screen-recorder"
    return 0
  fi

  if [ -f "$path_file" ]; then
    file="$(cat "$path_file" 2>/dev/null || true)"
  fi

  kill -INT "$pid" 2>/dev/null || true

  for _ in $(seq 1 50); do
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    sleep 0.1
  done

  if kill -0 "$pid" 2>/dev/null; then
    kill -TERM "$pid" 2>/dev/null || true
  fi

  : >"$pid_file"
  : >"$path_file"
  notify "Recording stopped" "${file:-Saved to $record_dir}"
}

mkdir -p -- "$record_dir"

if [ -f "$pid_file" ]; then
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    stop_recording "$pid"
    exit 0
  fi

  : >"$pid_file"
  : >"$path_file"
fi

if ! command -v gpu-screen-recorder >/dev/null 2>&1; then
  notify "Recording failed" "gpu-screen-recorder is not in PATH"
  exit 1
fi

target="${WAHRWELT_RECORD_TARGET:-${WAHRWELT_RECORD_TARGET:-$(focused_monitor)}}"
target="${target:-screen}"
audio="${WAHRWELT_RECORD_AUDIO:-${WAHRWELT_RECORD_AUDIO:-default_output}}"
fps="${WAHRWELT_RECORD_FPS:-${WAHRWELT_RECORD_FPS:-60}}"
file="$record_dir/recording-$(date +%Y%m%d-%H%M%S).mp4"

gpu-screen-recorder \
  -w "$target" \
  -f "$fps" \
  -a "$audio" \
  -o "$file" \
  >"$log_file" 2>&1 &

pid="$!"
printf '%s\n' "$pid" >"$pid_file"
printf '%s\n' "$file" >"$path_file"

sleep 0.2
if ! kill -0 "$pid" 2>/dev/null; then
  : >"$pid_file"
  : >"$path_file"
  notify "Recording failed" "$(tail -n 3 "$log_file" 2>/dev/null || true)"
  exit 1
fi

notify "Recording started" "$target + $audio -> $file"
