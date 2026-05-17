#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2154

hypr_config_value() {
  local file="$1"
  local key="$2"
  local fallback="$3"
  local value

  value="$(awk -F= -v key="$key" '
    $1 ~ "^[[:space:]]*" key "[[:space:]]*$" {
      gsub(/[[:space:]\",]/, "", $2)
      print $2
      exit
    }
  ' "$file" 2>/dev/null || true)"

  printf '%s' "${value:-$fallback}"
}

end4_monitor_rules() {
  local file
  file="$(hypr_dir)/end4/monitors.conf"

  awk -F= '
    /^[[:space:]]*monitor[[:space:]]*=/ {
      sub(/[[:space:]]*#.*/, "", $2)
      sub(/^[[:space:]]*/, "", $2)
      sub(/[[:space:]]*$/, "", $2)
      print $2
    }
  ' "$file" 2>/dev/null || true
}

apply_end4_hypr_runtime_overrides() {
  local dir rules layouts options rule applied=0

  [ "$profile" = "end4" ] || return 0
  command -v hyprctl >/dev/null 2>&1 || return 0
  hyprctl monitors >/dev/null 2>&1 || return 0

  dir="$(hypr_dir)"
  rules="$(end4_monitor_rules)"
  layouts="$(hypr_config_value "$dir/end4/custom/general.lua" kb_layout "us,ru")"
  options="$(hypr_config_value "$dir/end4/custom/general.lua" kb_options "grp:alt_shift_toggle")"

  if [ -n "$rules" ]; then
    while IFS= read -r rule; do
      [ -n "$rule" ] || continue
      hyprctl keyword monitor "$rule" >/dev/null 2>&1 || log "failed to apply end4 monitor=$rule"
      applied=$((applied + 1))
    done <<<"$rules"
  fi
  if [ "$applied" -eq 0 ]; then
    hyprctl keyword monitor ",preferred,auto,1" >/dev/null 2>&1 || log "failed to apply end4 monitor fallback"
  fi
  hyprctl keyword input:kb_layout "$layouts" >/dev/null 2>&1 || log "failed to apply end4 kb_layout=$layouts"
  hyprctl keyword input:kb_options "$options" >/dev/null 2>&1 || log "failed to apply end4 kb_options=$options"
}
