{
  end4Lib,
  inputs,
  lib,
  pkgs,
  ...
}:

let
  dotfilesSource = inputs.end4-dotfiles;
  inherit (end4Lib) dotfilesLib runtimeEnv settings;
  mysetupHyprSource = dotfilesLib.dotsRoot + "/hypr/end4";
  end4WindowOpacity = settings.window.opacity;
  hyprSourceReplacements = map (path: {
    from = "source=${path}";
    to = "source=~/.config/hypr/end4/${path}";
  }) settings.hyprSourceFiles;
  substituteHyprSources = lib.concatMapStringsSep " \\\n" (
    replacement:
    "      --replace-fail ${lib.escapeShellArg replacement.from} ${lib.escapeShellArg replacement.to}"
  ) hyprSourceReplacements;

  patchedHypr =
    pkgs.runCommand "end4-hypr-patched"
      {
        buildInputs = [
          pkgs.bash
        ];
      }
      ''
            source ${./hypr-helpers.sh}

            cp -r ${dotfilesSource}/dots/.config/hypr $out
            chmod -R +w $out
            find $out -type f -name '*.sh' -exec chmod +x {} +
            mkdir -p $out/mysetup
            cp ${mysetupHyprSource}/keybinds.conf $out/mysetup/keybinds.conf
            cp ${mysetupHyprSource}/launcher.conf $out/launcher.conf
            chmod -R u+w $out/mysetup

            cat > $out/hyprland/env.conf <<'EOF'
        ${runtimeEnv.hyprEnv}
        EOF
            cat ${dotfilesSource}/dots/.config/hypr/hyprland/env.conf >> $out/hyprland/env.conf
            sed -i '/^env *= *XDG_DATA_DIRS,/d' "$out/hyprland/env.conf"

            optional_patch_line "$out/hyprland/general.conf" \
              'enable_gesture = false' \
              '/^enable_gesture = false$/d'
            optional_patch_line "$out/hyprland/general.conf" \
              'gesture_positive = false' \
              '/^gesture_positive = false$/d'

            # Disabling upstream exec-once entries here avoids duplicate sessions.
            strict_patch_line "$out/hyprland/execs.conf" \
              'exec-once = qs -c $qsConfig &' \
              '/^exec-once = qs -c \$qsConfig &$/d' \
              'MySetup start-shell owns end4 QuickShell lifecycle'
            strict_patch_line "$out/hyprland/execs.conf" \
              'exec-once = hypridle' \
              '/^exec-once = hypridle$/d' \
              'MySetup start-shell owns end4 hypridle lifecycle'

            cat > "$out/monitors.conf" <<'EOF'
        ${lib.concatMapStringsSep "\n" (r: "monitor = ${r}") settings.monitor.rules}
        EOF

            cat > "$out/hypridle.conf" <<'EOF'
        $lock_cmd = hyprctl dispatch global quickshell:lock & pidof qs quickshell hyprlock || hyprlock
        $hibernate_cmd = systemctl hibernate || loginctl hibernate

        general {
            lock_cmd = $lock_cmd
            before_sleep_cmd = loginctl lock-session
            after_sleep_cmd = hyprctl dispatch global quickshell:lockFocus
            inhibit_sleep = 3
        }

        listener {
            timeout = ${toString settings.idle.lockTimeout}
            on-timeout = loginctl lock-session
        }

        listener {
            timeout = ${toString settings.idle.screenOffTimeout}
            on-timeout = hyprctl dispatch dpms off
            on-resume = hyprctl dispatch dpms on
        }

        listener {
            timeout = ${toString settings.idle.hibernateTimeout}
            on-timeout = $hibernate_cmd
        }
        EOF

            cat >> "$out/custom/general.conf" <<'EOF'

        input {
            kb_layout = ${settings.keyboard.layouts}
            kb_options = ${settings.keyboard.toggle}
        }

        general {
            border_size = 3
            col.active_border = rgba(c2c1ffe6)
            col.inactive_border = rgba(c8c5d111)
            gaps_in = 10
            gaps_out = 40
        }

        animations {
            bezier = mysetupStandard, 0.2, 0, 0, 1
            animation = workspaces, 1, 5, mysetupStandard, slide
        }

        misc {
            vrr = 2
            vfr = true
        }

        render {
            direct_scanout = 1
        }

        cursor {
            no_hardware_cursors = false
        }
        EOF

            cat >> "$out/custom/rules.conf" <<'EOF'

        windowrule = opacity ${end4WindowOpacity} override, match:fullscreen false
        windowrule = no_blur off, match:class .*

        layerrule = match:namespace quickshell:.*, ignore_alpha 0.79
        EOF

            substituteInPlace $out/hyprland.conf \
        ${substituteHyprSources}
            printf '\nsource=~/.config/hypr/variables.conf\nsource=~/.config/hypr/end4/mysetup/keybinds.conf\n' >> $out/hyprland.conf

            # Rewrite legacy ~/.config/hypr/* paths to the end4 namespace so end-4
            # configs co-exist with our own Hyprland config in $XDG_CONFIG_HOME.
            find $out -type f -exec sed -i \
              -e 's|~/.config/hypr/hyprland/|~/.config/hypr/end4/hyprland/|g' \
              -e 's|~/.config/hypr/custom/|~/.config/hypr/end4/custom/|g' \
              -e 's|~/.config/hypr/hyprlock/|~/.config/hypr/end4/hyprlock/|g' \
              -e 's|''${XDG_CONFIG_HOME:-$HOME/.config}/hypr/hyprlock/|''${XDG_CONFIG_HOME:-$HOME/.config}/hypr/end4/hyprlock/|g' \
              {} +

            patchShebangs $out
      '';

in
{
  xdg.configFile."hypr/end4" = {
    force = true;
    source = patchedHypr;
  };
}
