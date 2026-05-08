#!/usr/bin/env bash
set -euo pipefail

state_dir="${XDG_RUNTIME_DIR:-/tmp}/mysetup-recording"
pid_file="$state_dir/gpu-screen-recorder.pid"
path_file="$state_dir/gpu-screen-recorder.path"
log_file="$state_dir/gpu-screen-recorder.log"
lock_dir="$state_dir/lock"
lock_pid_file="$lock_dir/pid"
lock_owner_file="$lock_dir/owner"
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
    rm -f "$pid_file" "$path_file"
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

  rm -f "$pid_file" "$path_file"
  notify "Recording stopped" "${file:-Saved to $record_dir}"
}

mkdir -p "$state_dir" "$record_dir"

lock_owner_running() {
  local owner_pid="$1"
  local owner_name

  owner_name="$(cat "$lock_owner_file" 2>/dev/null || true)"
  [ "$owner_name" = "mysetup-record-toggle" ] || return 1
  [ -n "$owner_pid" ] || return 1
  ps -p "$owner_pid" -o args= 2>/dev/null | grep -qE '(^|[ /])record-toggle\.sh([[:space:]]|$)'
}

acquire_lock() {
  local owner_pid

  if mkdir "$lock_dir" 2>/dev/null; then
    printf '%s\n' "$$" >"$lock_pid_file"
    printf '%s\n' "mysetup-record-toggle" >"$lock_owner_file"
    return 0
  fi

  owner_pid="$(cat "$lock_pid_file" 2>/dev/null || true)"
  if lock_owner_running "$owner_pid"; then
    notify "Recording busy" "Another recorder toggle is already running"
    exit 0
  fi

  rm -rf -- "$lock_dir"
  if mkdir "$lock_dir" 2>/dev/null; then
    printf '%s\n' "$$" >"$lock_pid_file"
    printf '%s\n' "mysetup-record-toggle" >"$lock_owner_file"
    return 0
  fi

  notify "Recording busy" "Could not acquire recorder lock"
  exit 0
}

acquire_lock
trap 'rm -rf -- "$lock_dir" 2>/dev/null || true' EXIT

if [ -f "$pid_file" ]; then
  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    stop_recording "$pid"
    exit 0
  fi

  rm -f "$pid_file" "$path_file"
fi

if ! command -v gpu-screen-recorder >/dev/null 2>&1; then
  notify "Recording failed" "gpu-screen-recorder is not in PATH"
  exit 1
fi

target="${MYSETUP_RECORD_TARGET:-$(focused_monitor)}"
target="${target:-screen}"
audio="${MYSETUP_RECORD_AUDIO:-default_output}"
fps="${MYSETUP_RECORD_FPS:-60}"
file="$record_dir/recording-$(date +%Y%m%d-%H%M%S).mp4"

gpu-screen-recorder \
  -w "$target" \
  -f "$fps" \
  -a "$audio" \
  -o "$file" \
  >"$log_file" 2>&1 &

pid="$!"
printf '%s\n' "$pid" > "$pid_file"
printf '%s\n' "$file" > "$path_file"

sleep 0.2
if ! kill -0 "$pid" 2>/dev/null; then
  rm -f "$pid_file" "$path_file"
  notify "Recording failed" "$(tail -n 3 "$log_file" 2>/dev/null || true)"
  exit 1
fi

notify "Recording started" "$target + $audio -> $file"
