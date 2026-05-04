#!/usr/bin/env bash
set -euo pipefail

state_dir="${XDG_RUNTIME_DIR:-/tmp}/mysetup-recording"
pid_file="$state_dir/gpu-screen-recorder.pid"
path_file="$state_dir/gpu-screen-recorder.path"
log_file="$state_dir/gpu-screen-recorder.log"
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
