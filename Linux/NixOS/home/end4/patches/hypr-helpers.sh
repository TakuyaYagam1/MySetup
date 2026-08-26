#!/usr/bin/env bash
# shellcheck shell=bash
#
# Helpers used by patch-hypr.nix to massage upstream end-4 dotfiles.
# strict_patch_line: fail the build if the source line is missing - guards us
# against silent drift when end-4 renames its conf entries.
# optional_patch_line: best-effort delete; safe to run on configs where the
# line was already removed upstream.

strict_patch_line() {
    local file="$1"
    local from="$2"
    local sed_expr="$3"
    local reason="$4"

    if ! grep -Fqx "$from" "$file"; then
        echo "missing strict patch target ($reason): $file :: $from" >&2
        exit 1
    fi

    sed -i "$sed_expr" "$file"
}

strict_patch_two_lines() {
    local file="$1"
    local first="$2"
    local second="$3"
    local reason="$4"
    local line next
    local -a matches

    mapfile -t matches < <(grep -nFx -- "$first" "$file" || true)
    if [ "${#matches[@]}" -ne 1 ]; then
        echo "missing or ambiguous strict patch target ($reason): $file :: $first" >&2
        exit 1
    fi

    line="${matches[0]%%:*}"
    next="$(sed -n "$((line + 1))p" "$file")"
    if [ "$next" != "$second" ]; then
        echo "changed strict patch continuation ($reason): $file :: $next" >&2
        exit 1
    fi

    sed -i "${line},$((line + 1))d" "$file"
}

strict_replace_line_from_files() {
    local file="$1"
    local from_file="$2"
    local to_file="$3"
    local reason="$4"
    local from line
    local -a matches

    IFS= read -r from <"$from_file"
    mapfile -t matches < <(grep -nFx -- "$from" "$file" || true)
    if [ "${#matches[@]}" -ne 1 ]; then
        echo "missing or ambiguous strict patch target ($reason): $file :: $from" >&2
        exit 1
    fi
    line="${matches[0]%%:*}"
    sed -i "${line}r $to_file" "$file"
    sed -i "${line}d" "$file"
}

optional_patch_line() {
    local file="$1"
    local from="$2"
    local sed_expr="$3"

    if grep -Fqx "$from" "$file"; then
        sed -i "$sed_expr" "$file"
    fi
}
