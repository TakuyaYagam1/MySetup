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
  luaString = value: builtins.toJSON (toString value);
  luaEnvLines = lib.concatStringsSep "\n" (
    lib.mapAttrsToList (
      name: value: "hl.env(${luaString name}, ${luaString value})"
    ) runtimeEnv.hyprVariables
  );
  isAutoMonitor =
    monitor: monitor.mode == "preferred" || monitor.mode == "auto" || monitor.mode == "";
  renderLuaMonitor =
    monitor:
    let
      auto = isAutoMonitor monitor;
    in
    ''
      hl.monitor({
          output = ${luaString (if auto then "" else monitor.name)},
          mode = ${luaString (if monitor.mode == "" then "preferred" else monitor.mode)},
          position = ${luaString (if auto then "auto" else monitor.position)},
          scale = ${luaString monitor.scale}
      })
    '';
  luaMonitorLines = lib.concatMapStringsSep "\n" renderLuaMonitor settings.monitor.definitions;

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
            cp ${mysetupHyprSource}/keybinds.lua $out/mysetup/keybinds.lua
            cp ${mysetupHyprSource}/launcher.lua $out/launcher.lua
            chmod -R u+w $out/mysetup

            cat > $out/hyprland/env.lua <<'EOF'
        ${luaEnvLines}
        EOF
            cat ${dotfilesSource}/dots/.config/hypr/hyprland/env.lua >> $out/hyprland/env.lua
            optional_patch_line "$out/hyprland/env.lua" \
              'local xdg_data_dirs_old = os.getenv("XDG_DATA_DIRS") or ""' \
              '/^local xdg_data_dirs_old = os.getenv("XDG_DATA_DIRS") or ""$/d'
            if ! grep -Eq '^hl\.env\("XDG_DATA_DIRS",' "$out/hyprland/env.lua"; then
              echo "missing XDG_DATA_DIRS patch target (NixOS session environment owns XDG_DATA_DIRS): $out/hyprland/env.lua" >&2
              exit 1
            fi
            sed -i '/^hl\.env("XDG_DATA_DIRS",/d' "$out/hyprland/env.lua"

            substituteInPlace "$out/hyprland/variables.lua" \
              --replace-fail 'hl.env("qsConfig", "ii")' 'hl.env("qsConfig", ${luaString runtimeEnv.sessionVariables.qsConfig})'

            # Disabling upstream exec-once entries here avoids duplicate sessions.
            strict_patch_line "$out/hyprland/execs.lua" \
              '    hl.exec_cmd("qs -c $qsConfig")' \
              '/^    hl\.exec_cmd("qs -c \$qsConfig")$/d' \
              'MySetup start-shell owns end4 QuickShell lifecycle'
            strict_patch_line "$out/hyprland/execs.lua" \
              '    hl.exec_cmd("hypridle")' \
              '/^    hl\.exec_cmd("hypridle")$/d' \
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

            cat > "$out/custom/execs.lua" <<'EOF'
        hl.on("hyprland.start", function()
            local home = os.getenv("HOME")
            local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
            hl.exec_cmd(config_home .. "/hypr/scripts/start-shell.sh")
        end)
        EOF

            cat > "$out/custom/general.lua" <<'EOF'
        ${luaMonitorLines}

        hl.config({
            input = {
                kb_layout = ${luaString settings.keyboard.layouts},
                kb_options = ${luaString settings.keyboard.toggle}
            },
            general = {
                border_size = 3,
                col = {
                    active_border = "rgba(c2c1ffe6)",
                    inactive_border = "rgba(c8c5d111)"
                },
                gaps_in = 10,
                gaps_out = 40
            },
            misc = {
                vrr = 2
            },
            render = {
                direct_scanout = 1
            },
            cursor = {
                no_hardware_cursors = false
            }
        })

        hl.curve("mysetupStandard", {
            type = "bezier",
            points = { { 0.2, 0 }, { 0, 1 } }
        })
        hl.animation({
            leaf = "workspaces",
            enabled = true,
            speed = 5,
            bezier = "mysetupStandard",
            style = "slide"
        })
        EOF

            cat > "$out/custom/rules.lua" <<'EOF'
        hl.window_rule({ match = { fullscreen = 0 }, opacity = "${end4WindowOpacity} override" })
        hl.window_rule({ match = { class = ".*" }, no_blur = false })
        hl.layer_rule({ match = { namespace = "quickshell:.*" }, ignore_alpha = 0.79 })
        EOF

            cat > "$out/custom/keybinds.lua" <<'EOF'
        local home = os.getenv("HOME")
        local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
        dofile(config_home .. "/hypr/end4/mysetup/keybinds.lua")
        EOF

            # Rewrite legacy ~/.config/hypr/* paths to the end4 namespace so end-4
            # configs co-exist with our own Hyprland config in $XDG_CONFIG_HOME.
            find $out -type f -exec sed -i \
              -e 's|~/.config/hypr/hyprland/|~/.config/hypr/end4/hyprland/|g' \
              -e 's|~/.config/hypr/custom/|~/.config/hypr/end4/custom/|g' \
              -e 's|$HOME/.config/hypr/hyprland/|$HOME/.config/hypr/end4/hyprland/|g' \
              -e 's|$HOME/.config/hypr/custom/|$HOME/.config/hypr/end4/custom/|g' \
              -e 's|HOME \.\. "/.config/hypr/custom/|HOME .. "/.config/hypr/end4/custom/|g' \
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
