#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 3 ] || [ "$#" -gt 5 ]; then
  printf 'usage: %s check OLD NEW | merge|verify OLD NEW RECOVERY_PARENT TOKEN\n' "$0" >&2
  exit 2
fi

command_name="$1"
old="$2"
new="$3"
recovery_parent="${4:-}"
preflight_token="${5:-}"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
temp_creator="${WAHRWELT_CACHE_TEMP_CREATOR:-$script_dir/namespace-move.py}"
temp_python="${WAHRWELT_CACHE_TEMP_PYTHON:-python3}"

collision() {
  printf 'Wahrwelt migration conflict: cache paths must be ordinary directories: %s, %s\n' "$old" "$new" >&2
  exit 1
}

parent="$(dirname -- "$old")"
[ "$parent" = "$(dirname -- "$new")" ] || collision
old_name="${old##*/}"
new_name="${new##*/}"
[ "$old_name" != "$new_name" ] || collision

pin_parent() {
  if [ -L "$parent" ] || [ ! -d "$parent" ]; then
    collision
  fi
  parent_id="$(stat -c '%d:%i' -- "$parent")"
  if ! exec {parent_fd}<"$parent/."; then
    collision
  fi
  pinned_parent="/proc/self/fd/$parent_fd"
  [ "$(stat -Lc '%d:%i' -- "$pinned_parent")" = "$parent_id" ] || collision
  [ ! -L "$parent" ] && [ -d "$parent" ] && \
    [ "$(stat -c '%d:%i' -- "$parent")" = "$parent_id" ] || collision
  pinned_old="$pinned_parent/$old_name"
  pinned_new="$pinned_parent/$new_name"
}

if [ "$command_name" = check ]; then
  if [ ! -e "$parent" ] && [ ! -L "$parent" ]; then
    "$temp_python" "$temp_creator" check "$old" "$new"
    exit 0
  fi
  pin_parent
  if [ ! -e "$pinned_old" ] && [ ! -L "$pinned_old" ]; then
    printf 'absent|%s\n' "$parent_id"
    exit 0
  fi
  [ ! -L "$pinned_old" ] && [ -d "$pinned_old" ] || collision
  old_id="$(stat -c '%d:%i' -- "$pinned_old")"
  if [ ! -e "$pinned_new" ] && [ ! -L "$pinned_new" ]; then
    printf 'move|%s|%s\n' "$parent_id" "$old_id"
    exit 0
  fi
  [ ! -L "$pinned_new" ] && [ -d "$pinned_new" ] || collision
  new_id="$(stat -c '%d:%i' -- "$pinned_new")"
  printf 'merge|%s|%s|%s\n' "$parent_id" "$old_id" "$new_id"
  exit 0
fi

if { [ "$command_name" != merge ] && [ "$command_name" != verify ]; } || \
  [ -z "$recovery_parent" ] || [ -z "$preflight_token" ]; then
  printf 'Wahrwelt legacy cache migration received an invalid command\n' >&2
  exit 2
fi
[ "$parent" = "$recovery_parent" ] || collision

IFS='|' read -r token_kind expected_parent expected_old expected_new extra <<< "$preflight_token"
[ -z "${extra:-}" ] || collision
case "$token_kind" in
  absent-parent)
    [ -n "${expected_parent:-}" ] && [ -n "${expected_old:-}" ] && \
      [ -n "${expected_new:-}" ] && [ -z "${extra:-}" ] || collision
    "$temp_python" "$temp_creator" verify "$old" "$new" "$preflight_token"
    exit 0
    ;;
  absent | move | merge) ;;
  *) collision ;;
esac

pin_parent
[ "$parent_id" = "$expected_parent" ] || collision
if [ "$command_name" = verify ]; then
  [ ! -e "$pinned_old" ] && [ ! -L "$pinned_old" ] || collision
  case "$token_kind" in
    absent)
      [ -z "${expected_old:-}${expected_new:-}" ] || collision
      ;;
    move)
      [ -n "${expected_old:-}" ] && [ -z "${expected_new:-}" ] || collision
      [ ! -L "$pinned_new" ] && [ -d "$pinned_new" ] || collision
      [ "$(stat -c '%d:%i' -- "$pinned_new")" = "$expected_old" ] || collision
      ;;
    merge)
      [ -n "${expected_old:-}" ] && [ -n "${expected_new:-}" ] || collision
      [ ! -L "$pinned_new" ] && [ -d "$pinned_new" ] || collision
      [ "$(stat -c '%d:%i' -- "$pinned_new")" = "$expected_new" ] || collision
      ;;
    *) collision ;;
  esac
  [ "$(stat -c '%d:%i' -- "$parent" 2>/dev/null || true)" = "$parent_id" ] || collision
  exit 0
fi
if [ "$token_kind" = absent ]; then
  [ -z "${expected_old:-}${expected_new:-}" ] || collision
  [ ! -e "$pinned_old" ] && [ ! -L "$pinned_old" ] || collision
  exit 0
fi

[ ! -L "$pinned_old" ] && [ -d "$pinned_old" ] || collision
old_id="$(stat -c '%d:%i' -- "$pinned_old")"
[ "$old_id" = "$expected_old" ] || collision

if [ "$token_kind" = move ]; then
  [ -z "${expected_new:-}" ] || collision
  [ ! -e "$pinned_new" ] && [ ! -L "$pinned_new" ] || collision
elif [ "$token_kind" = merge ]; then
  [ -n "${expected_new:-}" ] || collision
  [ ! -L "$pinned_new" ] && [ -d "$pinned_new" ] || collision
  [ "$(stat -c '%d:%i' -- "$pinned_new")" = "$expected_new" ] || collision
else
  collision
fi

if [ ! -e "$pinned_new" ] && [ ! -L "$pinned_new" ]; then
  if ! mv -T --no-copy --update=none-fail -- "$pinned_old" "$pinned_new"; then
    printf 'Wahrwelt migration conflict: canonical cache appeared during migration: %s\n' "$new" >&2
    exit 1
  fi
  moved_id="$(stat -c '%d:%i' -- "$pinned_new" 2>/dev/null || true)"
  if [ "$moved_id" != "$old_id" ]; then
    restored=0
    if [ ! -e "$pinned_old" ] && [ ! -L "$pinned_old" ]; then
      if mv -T --no-copy --update=none-fail -- "$pinned_new" "$pinned_old" && \
        [ "$(stat -c '%d:%i' -- "$pinned_old" 2>/dev/null || true)" = "$moved_id" ]; then
        restored=1
      fi
    fi
    if [ "$restored" -eq 1 ]; then
      printf 'Wahrwelt legacy cache changed during migration; concurrent replacement restored: %s\n' "$old" >&2
    else
      pinned_parent_path="$(readlink -f -- "$pinned_parent" 2>/dev/null || printf '%s' "$parent")"
      printf 'Wahrwelt legacy cache changed during migration; recovery retained at %s/%s\n' "$pinned_parent_path" "$new_name" >&2
    fi
    exit 1
  fi
  if [ "$(stat -c '%d:%i' -- "$parent" 2>/dev/null || true)" != "$parent_id" ] || \
    [ "$(stat -c '%d:%i' -- "$new" 2>/dev/null || true)" != "$old_id" ]; then
    restored=0
    if [ ! -e "$pinned_old" ] && [ ! -L "$pinned_old" ] && \
      mv -T --no-copy --update=none-fail -- "$pinned_new" "$pinned_old" && \
      [ "$(stat -c '%d:%i' -- "$pinned_old" 2>/dev/null || true)" = "$moved_id" ]; then
      restored=1
    fi
    if [ "$restored" -eq 1 ]; then
      printf 'Wahrwelt cache parent changed during migration; exact source restored through pinned parent: %s\n' "$parent" >&2
    else
      pinned_parent_path="$(readlink -f -- "$pinned_parent" 2>/dev/null || printf '%s' "$parent")"
      printf 'Wahrwelt cache parent changed during migration; recovery retained at %s/%s\n' "$pinned_parent_path" "$new_name" >&2
    fi
    exit 1
  fi
  exit 0
fi

[ ! -L "$pinned_new" ] && [ -d "$pinned_new" ] || collision
new_id="$(stat -c '%d:%i' -- "$pinned_new")"

create_recovery_candidate() {
  "$temp_python" "$temp_creator" create-directory \
    "$pinned_parent" .wahrwelt-migration-recovery-cache-
}

cache_creation_barrier() {
  local ready_fd="${WAHRWELT_TEST_CACHE_RECOVERY_CREATED_READY_FD:-}"
  local continue_fd="${WAHRWELT_TEST_CACHE_RECOVERY_CREATED_CONTINUE_FD:-}"
  local release

  if [ -z "$ready_fd" ] && [ -z "$continue_fd" ]; then
    return
  fi
  if [ -z "$ready_fd" ] || [ -z "$continue_fd" ]; then
    printf 'Wahrwelt cache recovery creation barrier is incomplete\n' >&2
    exit 1
  fi
  printf 'ready\n' >&"$ready_fd"
  if ! IFS= read -r -N 1 -u "$continue_fd" release || [ "$release" != 1 ]; then
    printf 'Wahrwelt cache recovery creation barrier was not released\n' >&2
    exit 1
  fi
}

report_recovery_identity() {
  local expected="$1"
  local entry entry_id retained=

  while IFS= read -r -d '' entry; do
    entry_id="$(stat -c '%d:%i' -- "$entry" 2>/dev/null || true)"
    if [ "$entry_id" = "$expected" ]; then
      retained="$parent/${entry##*/}"
      break
    fi
  done < <(find -H "$pinned_parent" -mindepth 1 -maxdepth 1 -print0)
  if [ -n "$retained" ]; then
    printf 'Wahrwelt cache recovery candidate retained at %s\n' "$retained" >&2
  else
    printf 'Wahrwelt cache recovery candidate identity %s has no visible recovery beneath %s\n' \
      "$expected" "$parent" >&2
  fi
}

report_pinned_recovery() {
  local description="${1:-legacy recovery}"
  local retained_path visible_id

  retained_path="$(readlink -f -- "$pinned_recovery" 2>/dev/null || true)"
  if [ -n "$retained_path" ]; then
    printf 'Wahrwelt cache %s retained at %s (identity %s)\n' \
      "$description" "$retained_path" "$recovery_id" >&2
  else
    printf 'Wahrwelt cache %s retained through %s (identity %s)\n' \
      "$description" "$pinned_recovery" "$recovery_id" >&2
  fi
  visible_id="$(stat -c '%d:%i' -- "$recovery_path" 2>/dev/null || true)"
  if [ -n "$visible_id" ] && [ "$visible_id" != "$recovery_id" ]; then
    printf 'Wahrwelt cache unknown collision preserved at %s (identity %s)\n' \
      "$recovery_path" "$visible_id" >&2
  fi
}

cache_quarantine_barrier() {
  local ready_fd="${WAHRWELT_TEST_CACHE_QUARANTINED_READY_FD:-}"
  local continue_fd="${WAHRWELT_TEST_CACHE_QUARANTINED_CONTINUE_FD:-}"
  local release

  if [ -z "$ready_fd" ] && [ -z "$continue_fd" ]; then
    return
  fi
  if [ -z "$ready_fd" ] || [ -z "$continue_fd" ]; then
    printf 'Wahrwelt cache quarantine barrier is incomplete\n' >&2
    exit 1
  fi
  printf 'ready\n' >&"$ready_fd"
  if ! IFS= read -r -N 1 -u "$continue_fd" release || [ "$release" != 1 ]; then
    printf 'Wahrwelt cache quarantine barrier was not released\n' >&2
    exit 1
  fi
}

recovery_record="$(create_recovery_candidate)"
IFS='|' read -r recovery_name recovery_id recovery_extra <<< "$recovery_record"
[ -n "$recovery_name" ] && [ -n "$recovery_id" ] && [ -z "${recovery_extra:-}" ] || collision
recovery_path="$parent/$recovery_name"
created_recovery="$pinned_parent/$recovery_name"
cache_creation_barrier
if ! exec {recovery_fd}<"$created_recovery/."; then
  report_recovery_identity "$recovery_id"
  printf 'Wahrwelt could not pin cache recovery: %s\n' "$recovery_path" >&2
  exit 1
fi
pinned_recovery="/proc/self/fd/$recovery_fd"
visible_recovery_id="$(stat -c '%d:%i' -- "$created_recovery" 2>/dev/null || true)"
if [ "$(stat -Lc '%d:%i' -- "$pinned_recovery" 2>/dev/null || true)" != "$recovery_id" ] || \
  [ "$visible_recovery_id" != "$recovery_id" ] || [ ! -d "$pinned_recovery" ]; then
  report_recovery_identity "$recovery_id"
  if [ -n "$visible_recovery_id" ] && [ "$visible_recovery_id" != "$recovery_id" ]; then
    printf 'Wahrwelt cache unknown collision preserved at %s\n' "$recovery_path" >&2
  fi
  exit 1
fi
chmod 700 -- "$pinned_recovery"
backup="$pinned_recovery/legacy-original"

if ! mv -T --no-copy --update=none-fail -- "$pinned_old" "$backup"; then
  printf 'Wahrwelt legacy cache changed before quarantine: %s\n' "$old" >&2
  exit 1
fi
moved_id="$(stat -c '%d:%i' -- "$backup" 2>/dev/null || true)"
if [ "$moved_id" != "$old_id" ]; then
  if [ ! -e "$pinned_old" ] && [ ! -L "$pinned_old" ] && \
    mv -T --no-copy --update=none-fail -- "$backup" "$pinned_old"; then
    pinned_parent_path="$(readlink -f -- "$pinned_parent" 2>/dev/null || true)"
    printf 'Wahrwelt legacy cache changed during quarantine and the replacement was restored at %s/%s\n' \
      "${pinned_parent_path:-$pinned_parent}" "$old_name" >&2
  else
    printf 'Wahrwelt legacy cache changed during quarantine\n' >&2
    report_pinned_recovery "quarantined replacement"
  fi
  exit 1
fi

cache_quarantine_barrier

if [ "$(stat -Lc '%d:%i' -- "$pinned_recovery")" != "$recovery_id" ] || \
  [ "$(stat -c '%d:%i' -- "$recovery_path" 2>/dev/null || true)" != "$recovery_id" ] || \
  [ "$(stat -c '%d:%i' -- "$parent" 2>/dev/null || true)" != "$parent_id" ] || \
  [ "$(stat -c '%d:%i' -- "$new" 2>/dev/null || true)" != "$new_id" ]; then
  printf 'Wahrwelt cache identity changed during merge\n' >&2
  report_pinned_recovery
  exit 1
fi

printf 'Wahrwelt migration preserved canonical cache unchanged; legacy cache recovery retained at %s\n' "$recovery_path" >&2
printf '%s\n' "$recovery_path"
