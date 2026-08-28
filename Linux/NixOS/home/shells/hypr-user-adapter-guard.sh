#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 4 ]; then
  printf 'usage: %s check|prepare TARGET CURRENT_SOURCE OLD_GENERATION\n' "$0" >&2
  exit 2
fi

command_name="$1"
target="$2"
current_source="$3"
old_generation="$4"

ownership_collision() {
  printf 'Wahrwelt Hypr user adapter ownership collision: %s\n' "$target" >&2
  exit 1
}

digest() {
  sha256sum -- "$1" | cut -d ' ' -f 1
}

is_historical_digest() {
  case "$1" in
    cecf44b96c7afd4886d498abe0de382b2574c66281a5cf78bbac06586c1b071c | \
      e28d16bde1d68fa2fa43c755630284f00b3c6a14f75656e89cfb5514f8633263 | \
      18c3eb7f48101e0bd0b57918a683778784c74c833a215af7f7b0f1d416a0a5df | \
      24229642cd871aa3eb3d27c44b0d72357395951aec076a09d173b45ca17231a0 | \
      1d8e001faf0c6078a7d9a34e4c592fcb523afd817d2ff56099c7b2fe16407506 | \
      a547d710e9fd13ca8829e17caa378a14ee9d6a0d114426731e0ab363e9328118 | \
      3666c398dbba460e9b3dac54f396a7f53ad2093f49967c05e4588e66c41f08eb)
      return 0
      ;;
  esac
  return 1
}

is_root_owned_readonly_directory() {
  local path="$1"
  local identity owner mode

  [ -d "$path" ] && [ ! -L "$path" ] || return 1
  identity="$(stat -Lc '%u:%a' -- "$path" 2>/dev/null)" || return 1
  owner="${identity%%:*}"
  mode="${identity#*:}"
  [ "$owner" = 0 ] && [[ "$mode" =~ ^[0-7]{3,4}$ ]] || return 1
  (( (8#$mode & 8#022) == 0 ))
}

is_root_owned_store_leaf() {
  local path="$1"
  local allow_symlink="$2"
  local owner mode

  owner="$(stat -c '%u' -- "$path" 2>/dev/null)" || return 1
  [ "$owner" = 0 ] || return 1
  if [ -L "$path" ]; then
    [ "$allow_symlink" -eq 1 ]
    return
  fi
  [ -f "$path" ] || return 1
  mode="$(stat -c '%a' -- "$path" 2>/dev/null)" || return 1
  [[ "$mode" =~ ^[0-7]{3,4}$ ]] || return 1
  (( (8#$mode & 8#222) == 0 ))
}

is_immutable_nix_store_leaf() {
  local path="$1"
  local allow_symlink="$2"
  local object_path suffix current leaf index
  local components=()

  [[ "$path" =~ ^(/nix/store/[0-9abcdfghijklmnpqrsvwxyz]{32}-[^/]+)(/.*)?$ ]] || return 1
  object_path="${BASH_REMATCH[1]}"
  suffix="${BASH_REMATCH[2]:-}"
  leaf="$object_path"
  if [ -n "$suffix" ]; then
    is_root_owned_readonly_directory "$object_path" || return 1
    IFS=/ read -r -a components <<<"${suffix#/}"
    [ "${#components[@]}" -gt 0 ] || return 1
    current="$object_path"
    for ((index = 0; index + 1 < ${#components[@]}; index++)); do
      current="$current/${components[index]}"
      is_root_owned_readonly_directory "$current" || return 1
    done
    leaf="$current/${components[${#components[@]} - 1]}"
  fi
  is_root_owned_store_leaf "$leaf" "$allow_symlink"
}

if [ -L "$current_source" ] || [ ! -f "$current_source" ]; then
  printf 'Wahrwelt managed Hypr user adapter source is not a regular file: %s\n' "$current_source" >&2
  exit 1
fi
current_digest="$(digest "$current_source")"

classification=
target_digest=
classify_target() {
  classification=
  target_digest=

  if [ ! -e "$target" ] && [ ! -L "$target" ]; then
    classification=absent
    return 0
  fi

  if [ -L "$target" ]; then
    raw_target="$(readlink -- "$target" 2>/dev/null || true)"
    resolved_target="$(readlink -e -- "$target" 2>/dev/null || true)"
    [ -n "$raw_target" ] && [ -n "$resolved_target" ] || ownership_collision
    [ -f "$resolved_target" ] && [ ! -L "$resolved_target" ] || ownership_collision

    if [[ "$raw_target" =~ ^/nix/store/[0-9abcdfghijklmnpqrsvwxyz]{32}-home-manager-files/\.config/hypr/(user|wahrwelt|mysetup)/hyprland\.lua$ ]] &&
      is_immutable_nix_store_leaf "$raw_target" 1 &&
      is_immutable_nix_store_leaf "$resolved_target" 0; then
      target_digest="$(digest "$resolved_target")"
      if [ "$target_digest" = "$current_digest" ] || is_historical_digest "$target_digest"; then
        linked_identity="$(stat -Lc '%d:%i' -- "$target" 2>/dev/null || true)"
        raw_identity="$(stat -Lc '%d:%i' -- "$raw_target" 2>/dev/null || true)"
        resolved_identity="$(stat -Lc '%d:%i' -- "$resolved_target" 2>/dev/null || true)"
        if [ -n "$linked_identity" ] && [ "$linked_identity" = "$raw_identity" ] &&
          [ "$linked_identity" = "$resolved_identity" ] &&
          [ "$(readlink -- "$target" 2>/dev/null || true)" = "$raw_target" ] &&
          [ "$(readlink -e -- "$target" 2>/dev/null || true)" = "$resolved_target" ] &&
          [ "$(digest "$resolved_target")" = "$target_digest" ]; then
          classification=nixos-home-manager-link
          return 0
        fi
      fi
    fi

    [ -n "$old_generation" ] || ownership_collision
    old_home_files="$(readlink -e -- "$old_generation/home-files" 2>/dev/null || true)"
    [ -n "$old_home_files" ] || ownership_collision

    for namespace in user wahrwelt mysetup; do
      expected="$old_home_files/.config/hypr/$namespace/hyprland.lua"
      if [ "$raw_target" != "$expected" ] || [ ! -f "$expected" ]; then
        continue
      fi
      expected_resolved="$(readlink -e -- "$expected" 2>/dev/null || true)"
      if [ -n "$expected_resolved" ] && [ "$resolved_target" = "$expected_resolved" ]; then
        classification=home-manager-link
        return 0
      fi
    done
    ownership_collision
  fi

  [ -f "$target" ] || ownership_collision
  target_digest="$(digest "$target")"
  if [ "$target_digest" = "$current_digest" ]; then
    classification="current-regular"
    return 0
  fi
  if is_historical_digest "$target_digest"; then
    classification=historical-regular
    return 0
  fi
  ownership_collision
}

prepare_historical_regular() {
  local target_dir target_name target_dir_id target_dir_fd pinned_dir pinned_target
  local before_id before_digest pinned_recovery_dir recovery_name recovery_path
  local recovery_dir_id recovery_dir_fd pinned_recovery backup publication_error restored
  local recovery_record recovery_ready_fd recovery_continue_fd published_id

  target_dir="$(dirname -- "$target")"
  target_name="${target##*/}"
  if [ -L "$target_dir" ] || [ ! -d "$target_dir" ]; then
    ownership_collision
  fi
  target_dir_id="$(stat -c '%d:%i' -- "$target_dir")"
  if ! exec {target_dir_fd}<"$target_dir/."; then
    ownership_collision
  fi
  pinned_dir="/proc/self/fd/$target_dir_fd"
  if [ "$(stat -Lc '%d:%i' -- "$pinned_dir")" != "$target_dir_id" ]; then
    printf 'Wahrwelt Hypr user adapter parent changed before preparation: %s\n' "$target_dir" >&2
    exit 1
  fi
  pinned_target="$pinned_dir/$target_name"
  before_id="$(stat -c '%d:%i' -- "$pinned_target")"
  before_digest="$target_digest"
  if [ -L "$pinned_target" ] || [ ! -f "$pinned_target" ] || \
    [ "$before_id" != "$(stat -c '%d:%i' -- "$target")" ] || \
    [ "$(digest "$pinned_target")" != "$before_digest" ]; then
    printf 'Wahrwelt Hypr user adapter changed before guarded preparation: %s\n' "$target" >&2
    exit 1
  fi

  recovery_record="$({ python3 -I -S - "$pinned_dir/." <<'PY'
import os
import secrets
import stat
import sys


parent_path = sys.argv[1]
parent_fd = os.open(
    parent_path,
    os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
)
try:
    for _ in range(128):
        name = ".wahrwelt-hyprland-recovery." + secrets.token_hex(8)
        try:
            os.mkdir(name, 0o700, dir_fd=parent_fd)
        except FileExistsError:
            continue
        created = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
        if not stat.S_ISDIR(created.st_mode):
            raise RuntimeError("created recovery is not a directory")
        os.fsync(parent_fd)
        print(f"{name} {created.st_dev}:{created.st_ino}")
        break
    else:
        raise RuntimeError("could not allocate unique recovery directory")
finally:
    os.close(parent_fd)
PY
  } 2>/dev/null)" || {
    printf 'Wahrwelt could not create pinned Hypr user adapter recovery beneath %s\n' "$target_dir" >&2
    exit 1
  }
  recovery_name="${recovery_record%% *}"
  recovery_dir_id="${recovery_record#* }"
  if [ -z "$recovery_name" ] || [ -z "$recovery_dir_id" ] || [ "$recovery_name" = "$recovery_record" ]; then
    printf 'Wahrwelt received invalid Hypr user adapter recovery identity beneath %s\n' "$target_dir" >&2
    exit 1
  fi
  pinned_recovery_dir="$pinned_dir/$recovery_name"
  recovery_path="$target_dir/$recovery_name"

  recovery_ready_fd="${WAHRWELT_TEST_ADAPTER_RECOVERY_READY_FD:-}"
  recovery_continue_fd="${WAHRWELT_TEST_ADAPTER_RECOVERY_CONTINUE_FD:-}"
  if [ -n "$recovery_ready_fd" ] || [ -n "$recovery_continue_fd" ]; then
    if [ -z "$recovery_ready_fd" ] || [ -z "$recovery_continue_fd" ]; then
      printf 'incomplete Wahrwelt adapter recovery test barrier\n' >&2
      exit 1
    fi
    printf 'ready\n' >&"$recovery_ready_fd"
    IFS= read -r -n 1 recovery_proceed <&"$recovery_continue_fd" || true
    if [ "${recovery_proceed:-}" != 1 ]; then
      printf 'closed Wahrwelt adapter recovery test barrier\n' >&2
      exit 1
    fi
  fi

  if ! exec {recovery_dir_fd}<"$pinned_recovery_dir/."; then
    printf 'Wahrwelt could not pin Hypr user adapter recovery: %s\n' "$recovery_path" >&2
    exit 1
  fi
  pinned_recovery="/proc/self/fd/$recovery_dir_fd"
  if [ "$(stat -Lc '%d:%i' -- "$pinned_recovery")" != "$recovery_dir_id" ]; then
    printf 'Wahrwelt Hypr user adapter recovery changed before preparation: %s\n' "$recovery_path" >&2
    exit 1
  fi
  backup="$pinned_recovery/previous.lua"

  mv -T --no-copy --update=none-fail -- "$pinned_target" "$backup"
  moved_id="$(stat -c '%d:%i' -- "$backup" 2>/dev/null || true)"
  if [ -L "$backup" ] || [ ! -f "$backup" ] || \
    [ "$moved_id" != "$before_id" ] || \
    [ "$(digest "$backup")" != "$before_digest" ]; then
    if [ -n "$moved_id" ] && [ ! -e "$pinned_target" ] && [ ! -L "$pinned_target" ] && \
      mv -T --no-copy --update=none-fail -- "$backup" "$pinned_target" && \
      [ "$(stat -c '%d:%i' -- "$pinned_target" 2>/dev/null || true)" = "$moved_id" ]; then
      printf 'Wahrwelt Hypr user adapter changed during guarded preparation; concurrent replacement restored: %s\n' "$target" >&2
    else
      printf 'Wahrwelt Hypr user adapter changed during guarded preparation; recovery retained at %s/previous.lua\n' "$recovery_path" >&2
    fi
    exit 1
  fi

  publication_error=
  if ! (umask 022; cp -T --update=none-fail --no-preserve=mode -- "$current_source" "$pinned_target"); then
    publication_error="Wahrwelt Hypr user adapter appeared before publication: $target"
  elif [ -L "$pinned_target" ] || [ ! -f "$pinned_target" ] || [ "$(digest "$pinned_target")" != "$current_digest" ]; then
    publication_error="Wahrwelt Hypr user adapter changed during publication: $target"
  else
    published_id="$(stat -c '%d:%i' -- "$pinned_target")"
  fi
  if [ -n "$publication_error" ]; then
    restored=0
    if [ ! -e "$pinned_target" ] && [ ! -L "$pinned_target" ] && \
      [ ! -L "$backup" ] && [ -f "$backup" ] && \
      [ "$(stat -c '%d:%i' -- "$backup")" = "$before_id" ] && \
      [ "$(digest "$backup")" = "$before_digest" ] && \
      mv -T --no-copy --update=none-fail -- "$backup" "$pinned_target" && \
      [ ! -L "$pinned_target" ] && [ -f "$pinned_target" ] && \
      [ "$(stat -c '%d:%i' -- "$pinned_target")" = "$before_id" ] && \
      [ "$(digest "$pinned_target")" = "$before_digest" ]; then
      restored=1
    fi
    printf '%s\n' "$publication_error" >&2
    if [ "$restored" -eq 1 ]; then
      if [ ! -L "$target_dir" ] && [ -d "$target_dir" ] && \
        [ "$(stat -c '%d:%i' -- "$target_dir")" = "$target_dir_id" ]; then
        printf 'Wahrwelt restored the previous Hypr user adapter at %s\n' "$target" >&2
      else
        printf 'Wahrwelt restored the previous Hypr user adapter in pinned directory inode %s\n' "$target_dir_id" >&2
      fi
    elif [ ! -L "$target_dir" ] && [ -d "$target_dir" ] && \
      [ "$(stat -c '%d:%i' -- "$target_dir")" = "$target_dir_id" ] && \
      [ ! -L "$pinned_recovery_dir" ] && [ -d "$pinned_recovery_dir" ] && \
      [ "$(stat -c '%d:%i' -- "$pinned_recovery_dir")" = "$recovery_dir_id" ]; then
      printf 'Wahrwelt retained the previous Hypr user adapter recovery at %s/previous.lua\n' "$recovery_path" >&2
    else
      printf 'Wahrwelt retained uncertain Hypr user adapter recovery in pinned directory inode %s\n' "$recovery_dir_id" >&2
    fi
    exit 1
  fi

  ready_fd="${WAHRWELT_TEST_ADAPTER_READY_FD:-}"
  continue_fd="${WAHRWELT_TEST_ADAPTER_CONTINUE_FD:-}"
  if [ -n "$ready_fd" ] || [ -n "$continue_fd" ]; then
    if [ -z "$ready_fd" ] || [ -z "$continue_fd" ]; then
      printf 'incomplete Wahrwelt adapter test barrier\n' >&2
      exit 1
    fi
    printf 'ready\n' >&"$ready_fd"
    IFS= read -r -n 1 proceed <&"$continue_fd" || true
    if [ "${proceed:-}" != 1 ]; then
      printf 'closed Wahrwelt adapter test barrier\n' >&2
      exit 1
    fi
  fi

  if [ -L "$backup" ] || [ ! -f "$backup" ] || \
    [ "$(stat -c '%d:%i' -- "$backup")" != "$before_id" ] || \
    [ "$(digest "$backup")" != "$before_digest" ] || \
    [ "$(stat -c '%d:%i' -- "$backup")" != "$before_id" ]; then
    printf 'Wahrwelt retained uncertain Hypr user adapter recovery in pinned directory inode %s\n' "$recovery_dir_id" >&2
    exit 1
  fi
  if [ -z "$published_id" ] || [ -L "$pinned_target" ] || [ ! -f "$pinned_target" ] || \
    [ "$(stat -c '%d:%i' -- "$pinned_target")" != "$published_id" ] || \
    [ "$(digest "$pinned_target")" != "$current_digest" ] || \
    [ "$(stat -c '%d:%i' -- "$pinned_target")" != "$published_id" ]; then
    printf 'Wahrwelt Hypr user adapter changed after guarded publication: %s\n' "$target" >&2
    printf 'Wahrwelt retained the previous Hypr user adapter at %s/previous.lua\n' "$recovery_path" >&2
    exit 1
  fi
  if [ -L "$target_dir" ] || [ ! -d "$target_dir" ] || \
    [ "$(stat -c '%d:%i' -- "$target_dir")" != "$target_dir_id" ] || \
    [ -L "$pinned_recovery_dir" ] || [ ! -d "$pinned_recovery_dir" ] || \
    [ "$(stat -c '%d:%i' -- "$pinned_recovery_dir")" != "$recovery_dir_id" ]; then
    printf 'Wahrwelt retained uncertain Hypr user adapter recovery in pinned directory inode %s\n' "$recovery_dir_id" >&2
    exit 1
  fi
  printf 'Wahrwelt retained the previous Hypr user adapter at %s/previous.lua\n' "$recovery_path" >&2
}

case "$command_name" in
  check)
    classify_target
    ;;
  prepare)
    classify_target
    if [ "$classification" = historical-regular ]; then
      prepare_historical_regular
    fi
    ;;
  *)
    printf 'unknown Wahrwelt Hypr user adapter guard command: %s\n' "$command_name" >&2
    exit 2
    ;;
esac
