{ ... }:

{
  xdg.configFile."uwsm/env".text = ''
    export QT_QPA_PLATFORMTHEME='qt6ct'
    export QT_WAYLAND_DISABLE_WINDOWDECORATION='1'
    export QT_AUTO_SCREEN_SCALE_FACTOR='1'

    export APP2UNIT_SLICES='a=app-graphical.slice b=background-graphical.slice s=session-graphical.slice'
  '';

  xdg.configFile."uwsm/env-hyprland".text = ''
    export GDK_BACKEND='wayland,x11'
    export QT_QPA_PLATFORM='wayland;xcb'
    export SDL_VIDEODRIVER='wayland,x11,windows'
    export CLUTTER_BACKEND='wayland'
    export ELECTRON_OZONE_PLATFORM_HINT='auto'

    export XDG_CURRENT_DESKTOP=Hyprland
    export XDG_SESSION_TYPE=wayland
    export XDG_SESSION_DESKTOP=Hyprland

    export _JAVA_AWT_WM_NONREPARENTING=1
  '';
}
