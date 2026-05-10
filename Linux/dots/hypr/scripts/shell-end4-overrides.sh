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
      gsub(/[[:space:]]/, "", $2)
      print $2
      exit
    }
  ' "$file" 2>/dev/null || true)"

  printf '%s' "${value:-$fallback}"
}

hypr_variable_value() {
  local file="$1"
  local key="$2"
  local fallback="$3"
  local lhs rhs value

  if [ -f "$file" ]; then
    while IFS='=' read -r lhs rhs; do
      lhs="$(printf '%s' "$lhs" | tr -d '[:space:]')"
      if [ "$lhs" = "\$$key" ]; then
        value="$(printf '%s' "$rhs" | tr -d '[:space:]')"
        break
      fi
    done <"$file"
  fi

  printf '%s' "${value:-$fallback}"
}

end4_window_opacity() {
  local fallback="$1"
  local config_file transparency opacity

  config_file="$config_home/illogical-impulse/config.json"
  if [ -f "$config_file" ] && command -v jq >/dev/null 2>&1; then
    transparency="$(jq -r '
      if (.appearance.transparency.enable // true) == false then
        0
      else
        (.appearance.transparency.backgroundTransparency // empty)
      end
    ' "$config_file" 2>/dev/null || true)"

    opacity="$(awk -v t="$transparency" '
      BEGIN {
        if (t !~ /^-?[0-9]+([.][0-9]+)?$/) exit 1
        t += 0
        if (t < 0) t = 0
        if (t > 0.95) t = 0.95
        printf "%.3f", 1 - t
      }
    ' 2>/dev/null || true)"

    if [ -n "$opacity" ]; then
      printf '%s' "$opacity"
      return 0
    fi
  fi

  printf '%s' "$fallback"
}

end4_monitor_rule() {
  local file value

  file="$(hypr_dir)/end4/monitors.conf"
  value="$(awk -F= '
    /^[[:space:]]*monitor[[:space:]]*=/ {
      sub(/[[:space:]]*#.*/, "", $2)
      sub(/^[[:space:]]*/, "", $2)
      sub(/[[:space:]]*$/, "", $2)
      print $2
      exit
    }
  ' "$file" 2>/dev/null || true)"

  printf '%s' "${value:-eDP-1, 2560x1600@120, 0x0, 1}"
}

apply_end4_hypr_runtime_overrides() {
  local dir monitor layouts options opacity bordered_fullscreen

  [ "$profile" = "end4" ] || return 0
  command -v hyprctl >/dev/null 2>&1 || return 0
  hyprctl monitors >/dev/null 2>&1 || return 0

  dir="$(hypr_dir)"
  monitor="$(end4_monitor_rule)"
  layouts="$(hypr_config_value "$dir/end4/custom/general.conf" kb_layout "us,ru")"
  options="$(hypr_config_value "$dir/end4/custom/general.conf" kb_options "grp:alt_shift_toggle")"
  opacity="$(end4_window_opacity "$(hypr_variable_value "$dir/variables.conf" windowOpacity "0.8")")"
  bordered_fullscreen="$(hypr_variable_value "$dir/variables.conf" kbWindowBorderedFullscreen "Super+Shift, Return")"

  hyprctl keyword monitor "$monitor" >/dev/null 2>&1 || log "failed to apply end4 monitor=$monitor"
  hyprctl keyword input:kb_layout "$layouts" >/dev/null 2>&1 || log "failed to apply end4 kb_layout=$layouts"
  hyprctl keyword input:kb_options "$options" >/dev/null 2>&1 || log "failed to apply end4 kb_options=$options"
  hyprctl keyword decoration:active_opacity 1 >/dev/null 2>&1 || log "failed to apply end4 active_opacity=1"
  hyprctl keyword decoration:inactive_opacity "$opacity" >/dev/null 2>&1 || log "failed to apply end4 inactive_opacity=$opacity"
  hyprctl keyword decoration:fullscreen_opacity 1 >/dev/null 2>&1 || log "failed to apply end4 fullscreen_opacity=1"
  hyprctl keyword decoration:blur:xray true >/dev/null 2>&1 || log "failed to apply end4 blur:xray=true"
  hyprctl keyword layerrule "match:namespace .*, xray on" >/dev/null 2>&1 || log "failed to apply end4 layer xray"
  hyprctl keyword layerrule "match:namespace quickshell:.*, blur on" >/dev/null 2>&1 || log "failed to apply end4 quickshell blur"
  hyprctl keyword layerrule "match:namespace quickshell:.*, ignore_alpha 0.79" >/dev/null 2>&1 || log "failed to apply end4 quickshell ignore_alpha"
  hyprctl keyword windowrule "opacity 1 override $opacity override 1 override, match:fullscreen false" >/dev/null 2>&1 || log "failed to apply end4 opacity=$opacity"
  hyprctl keyword unbind "$bordered_fullscreen" >/dev/null 2>&1 || true
  hyprctl keyword bind "$bordered_fullscreen, fullscreen, 0" >/dev/null 2>&1 || log "failed to apply end4 fullscreen bind=$bordered_fullscreen"
}
