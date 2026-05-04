{ config, lib, pkgs, var, ... }:

let
  profile = var.shellProfile or "caelestia";

  caelestiaKeybinds = ''
    # ## Shell keybinds
    # Launcher
    bindi = Super, Super_L, global, caelestia:launcher
    bindin = Super, catchall, global, caelestia:launcherInterrupt
    bindin = Super, mouse:272, global, caelestia:launcherInterrupt
    bindin = Super, mouse:273, global, caelestia:launcherInterrupt
    bindin = Super, mouse:274, global, caelestia:launcherInterrupt
    bindin = Super, mouse:275, global, caelestia:launcherInterrupt
    bindin = Super, mouse:276, global, caelestia:launcherInterrupt
    bindin = Super, mouse:277, global, caelestia:launcherInterrupt
    bindin = Super, mouse_up, global, caelestia:launcherInterrupt
    bindin = Super, mouse_down, global, caelestia:launcherInterrupt

    # Misc
    bind = $kbSession, global, caelestia:session
    bind = $kbShowSidebar, global, caelestia:sidebar
    bindl = $kbClearNotifs, global, caelestia:clearNotifs
    bind = $kbShowPanels, global, caelestia:showall
    bind = $kbLock, global, caelestia:lock

    # Restore lock
    bindl = $kbRestoreLock, exec, caelestia shell -d
    bindl = $kbRestoreLock, global, caelestia:lock

    # Brightness
    bindl = , XF86MonBrightnessUp, global, caelestia:brightnessUp
    bindl = , XF86MonBrightnessDown, global, caelestia:brightnessDown

    # Media
    bindl = Ctrl+Super, Space, global, caelestia:mediaToggle
    bindl = , XF86AudioPlay, global, caelestia:mediaToggle
    bindl = , XF86AudioPause, global, caelestia:mediaToggle
    bindl = Ctrl+Super, Equal, global, caelestia:mediaNext
    bindl = , XF86AudioNext, global, caelestia:mediaNext
    bindl = Ctrl+Super, Minus, global, caelestia:mediaPrev
    bindl = , XF86AudioPrev, global, caelestia:mediaPrev
    bindl = , XF86AudioStop, global, caelestia:mediaStop

    # Kill/restart
    bindr = Ctrl+Super+Shift, R, exec, qs -c caelestia kill
    bindr = Ctrl+Super+Alt, R, exec, qs -c caelestia kill; sleep .1; caelestia shell -d
  '';

  genericKeybinds = ''
    # ## Shell keybinds
    bind = $kbSession, exec, loginctl lock-session
    bind = $kbLock, exec, loginctl lock-session
    bindl = , XF86MonBrightnessUp, exec, brightnessctl set 5%+
    bindl = , XF86MonBrightnessDown, exec, brightnessctl set 5%-
    bindl = Ctrl+Super, Space, exec, playerctl play-pause
    bindl = , XF86AudioPlay, exec, playerctl play-pause
    bindl = , XF86AudioPause, exec, playerctl play-pause
    bindl = Ctrl+Super, Equal, exec, playerctl next
    bindl = , XF86AudioNext, exec, playerctl next
    bindl = Ctrl+Super, Minus, exec, playerctl previous
    bindl = , XF86AudioPrev, exec, playerctl previous
    bindl = , XF86AudioStop, exec, playerctl stop
  '';

  shellKeybinds =
    if profile == "caelestia" then caelestiaKeybinds
    else genericKeybinds;

  dotsRoot =
    let
      installedDots = ../../dots;
      repoDots = ../../../dots;
    in
      if builtins.pathExists installedDots then installedDots else repoDots;
in
{
  xdg.configFile."hypr/scripts/start-shell.sh" = {
    force = true;
    executable = true;
    source = dotsRoot + "/hypr/scripts/start-shell.sh";
  };

  xdg.configFile."hypr/scripts/record-toggle.sh" = {
    force = true;
    executable = true;
    source = dotsRoot + "/hypr/scripts/record-toggle.sh";
  };

  xdg.configFile."hypr/shell-profile.conf" = {
    force = true;
    text = ''
      # Active shell profile: ${profile}
      exec-once = ${config.xdg.configHome}/hypr/scripts/start-shell.sh ${profile}
    '';
  };

  xdg.configFile."hypr/shell-keybinds.conf" = {
    force = true;
    text = shellKeybinds;
  };

  home.activation.startHyprShell =
    lib.hm.dag.entryAfter [ "writeBoundary" ] ''
      if command -v hyprctl >/dev/null 2>&1 && hyprctl instances >/dev/null 2>&1; then
        $DRY_RUN_CMD ${config.xdg.configHome}/hypr/scripts/start-shell.sh ${profile} >/dev/null 2>&1 || true
      fi
    '';
}
