#!/usr/bin/env bash
set -uo pipefail

repo_root="$(CDPATH='' cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../../.." && pwd)"
process_lib="$repo_root/Linux/dots/hypr/scripts/shell-process.sh"
fixture_dir="$(mktemp -d)"
daemon_pid=""
setsid_bin="$(command -v setsid)"
original_path="$PATH"

fail() {
  printf 'shell process lifecycle test failed: %s\n' "$*" >&2
  exit 1
}

process_is_live() {
  local pid="$1"
  local state

  [ -r "/proc/$pid/stat" ] || return 1
  state="$(awk '{ print $3 }' "/proc/$pid/stat" 2>/dev/null || true)"
  [ -n "$state" ] && [ "$state" != Z ]
}

wait_for_pid_file() {
  local path="$1"

  for _ in $(seq 1 100); do
    [ -s "$path" ] && return 0
    sleep 0.01
  done
  return 1
}

wait_for_exit() {
  local pid="$1"

  for _ in $(seq 1 100); do
    process_is_live "$pid" || return 0
    sleep 0.01
  done
  return 1
}

cleanup() {
  if [ -n "$daemon_pid" ] && process_is_live "$daemon_pid"; then
    kill -TERM "$daemon_pid" >/dev/null 2>&1 || true
    wait_for_exit "$daemon_pid" || kill -KILL "$daemon_pid" >/dev/null 2>&1 || true
  fi
  rm -rf -- "$fixture_dir"
}
trap cleanup EXIT HUP INT TERM

wrapper="$fixture_dir/wrapper.sh"
daemonizer="$fixture_dir/daemonize.sh"

cat >"$daemonizer" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
pid_file="$1"
(
  trap 'exit 0' HUP INT TERM
  printf '%s\n' "$BASHPID" >"$pid_file"
  while :; do
    sleep 1 &
    wait "$!" || true
  done
) </dev/null >/dev/null 2>&1 &
exit 0
EOF
chmod 0755 "$daemonizer"

cat >"$wrapper" <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
process_lib="$1"
daemonizer="$2"
daemon_pid_file="$3"
wrapper_ready_file="$4"
log_file="$5"

log() {
  printf '%s\n' "$*" >>"$log_file"
}

# shellcheck source=Linux/dots/hypr/scripts/shell-process.sh
. "$process_lib"

# shell-process.sh owns matching semantics in production. This fixture narrows
# readiness to the exact daemon PID so the test exercises only launch lifetime.
matching_pids() {
  local pid=""
  [ -s "$daemon_pid_file" ] && read -r pid <"$daemon_pid_file"
  [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1 && printf '%s\n' "$pid"
}

start_with_retry daemon test-handle "$daemonizer" "$daemon_pid_file" || exit 36
: >"$wrapper_ready_file"
exit 37
EOF
chmod 0755 "$wrapper"

run_lifecycle_case() {
  local name="$1"
  local launch_path="$2"
  local case_dir="$fixture_dir/$name"
  local daemon_pid_file="$case_dir/daemon.pid"
  local wrapper_ready_file="$case_dir/wrapper.ready"
  local wrapper_pid wrapper_status daemon_pgid daemon_sid

  mkdir -p -- "$case_dir"
  env PATH="$launch_path" "$setsid_bin" "$wrapper" \
    "$process_lib" \
    "$daemonizer" \
    "$daemon_pid_file" \
    "$wrapper_ready_file" \
    "$case_dir/launch.log" &
  wrapper_pid=$!

  wrapper_status=0
  wait "$wrapper_pid" || wrapper_status=$?
  [ "$wrapper_status" -eq 37 ] || fail "$name wrapper exit=$wrapper_status, want 37"
  [ -e "$wrapper_ready_file" ] || fail "$name wrapper did not observe the daemon start"
  wait_for_pid_file "$daemon_pid_file" || fail "$name daemon PID was not published"
  read -r daemon_pid <"$daemon_pid_file"
  [[ "$daemon_pid" =~ ^[1-9][0-9]*$ ]] || fail "$name invalid daemon PID: $daemon_pid"
  process_is_live "$daemon_pid" || fail "$name daemon exited before runtime-lock cleanup"

  # This is the runtime-lock watchdog's nonzero-exit cleanup. A real shell must
  # not remain in the transaction PGID or this exact kill removes the fallback.
  kill -KILL -- "-$wrapper_pid" >/dev/null 2>&1 || true
  sleep 0.05
  process_is_live "$daemon_pid" || fail "$name daemon was killed with the failed wrapper process group"

  daemon_pgid="$(ps -o pgid= -p "$daemon_pid" | tr -d '[:space:]')"
  daemon_sid="$(ps -o sid= -p "$daemon_pid" | tr -d '[:space:]')"
  [ "$daemon_pgid" != "$wrapper_pid" ] || fail "$name daemon retained the runtime-lock process group"
  [ "$daemon_sid" != "$wrapper_pid" ] || fail "$name daemon retained the runtime-lock session"

  kill -TERM "$daemon_pid" >/dev/null 2>&1 || fail "$name failed to stop detached daemon"
  wait_for_exit "$daemon_pid" || fail "$name detached daemon did not stop cleanly"
  daemon_pid=""
}

run_lifecycle_case normal "$original_path"

fallback_bin="$fixture_dir/fallback-bin"
mkdir -p -- "$fallback_bin"
for command_name in bash grep seq setsid sleep sort; do
  ln -s -- "$(command -v "$command_name")" "$fallback_bin/$command_name"
done
[ ! -e "$fallback_bin/systemctl" ] || fail 'fallback fixture unexpectedly contains systemctl'
[ ! -e "$fallback_bin/systemd-run" ] || fail 'fallback fixture unexpectedly contains systemd-run'
run_lifecycle_case without-systemd "$fallback_bin"

printf 'shell process lifecycle test passed\n'
