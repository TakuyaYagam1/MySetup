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

optional_patch_line() {
    local file="$1"
    local from="$2"
    local sed_expr="$3"

    if grep -Fqx "$from" "$file"; then
        sed -i "$sed_expr" "$file"
    fi
}
