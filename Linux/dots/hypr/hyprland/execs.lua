local v = require("variables")

hl.on("hyprland.start", function()
    hl.exec_cmd("systemctl --user import-environment HYPRLAND_INSTANCE_SIGNATURE WAYLAND_DISPLAY XDG_CURRENT_DESKTOP XDG_DATA_DIRS PATH")
    hl.exec_cmd("dbus-update-activation-environment --systemd HYPRLAND_INSTANCE_SIGNATURE WAYLAND_DISPLAY XDG_CURRENT_DESKTOP XDG_DATA_DIRS")

    hl.exec_cmd("gnome-keyring-daemon --start --components=secrets,pkcs11,ssh")
    hl.exec_cmd("systemctl --user start polkit-gnome-authentication-agent-1")

    hl.exec_cmd("wl-paste --type text --watch cliphist store")
    hl.exec_cmd("wl-paste --type image --watch cliphist store")

    hl.exec_cmd("trash-empty 30")

    hl.exec_cmd("hyprctl setcursor " .. v.cursorTheme .. " " .. tostring(v.cursorSize))
    hl.exec_cmd("gsettings set org.gnome.desktop.interface cursor-theme '" .. v.cursorTheme .. "'")
    hl.exec_cmd("gsettings set org.gnome.desktop.interface cursor-size " .. tostring(v.cursorSize))

    hl.exec_cmd("gammastep")
    hl.exec_cmd("mpris-proxy")
end)
