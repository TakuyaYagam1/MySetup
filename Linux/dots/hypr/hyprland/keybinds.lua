local mysetup = require("lib.mysetup")
local v = require("variables")

mysetup.load_runtime("shell-launcher.lua")

local wsaction = mysetup.hypr .. "/scripts/wsaction.fish"

for i = 1, 10 do
	local key = tostring(i % 10)
	mysetup.bind_exec(v.kbGoToWs .. " + " .. key, wsaction .. " workspace " .. tostring(i))
end

mysetup.bind_dispatch("SUPER + mouse_down", "workspace -1")
mysetup.bind_dispatch("SUPER + mouse_up", "workspace +1")
mysetup.bind_dispatch(v.kbPrevWs, "workspace -1", { repeating = true })
mysetup.bind_dispatch(v.kbNextWs, "workspace +1", { repeating = true })
mysetup.bind_dispatch("SUPER + Page_Up", "workspace -1", { repeating = true })
mysetup.bind_dispatch("SUPER + Page_Down", "workspace +1", { repeating = true })
mysetup.bind_dispatch("CTRL + SUPER + mouse_down", "workspace -10")
mysetup.bind_dispatch("CTRL + SUPER + mouse_up", "workspace +10")
mysetup.bind_dispatch(v.kbToggleSpecialWs, "togglespecialworkspace special")
require("shell-workspace-keybinds")

mysetup.bind_dispatch("SUPER + ALT + Page_Up", "movetoworkspace -1", { repeating = true })
mysetup.bind_dispatch("SUPER + ALT + Page_Down", "movetoworkspace +1", { repeating = true })
mysetup.bind_dispatch("SUPER + ALT + mouse_down", "movetoworkspace -1")
mysetup.bind_dispatch("SUPER + ALT + mouse_up", "movetoworkspace +1")
mysetup.bind_dispatch("CTRL + SUPER + SHIFT + right", "movetoworkspace +1", { repeating = true })
mysetup.bind_dispatch("CTRL + SUPER + SHIFT + left", "movetoworkspace -1", { repeating = true })
mysetup.bind_dispatch("CTRL + SUPER + SHIFT + up", "movetoworkspace special:special")
mysetup.bind_dispatch("CTRL + SUPER + SHIFT + down", "movetoworkspace e+0")
mysetup.bind_dispatch("SUPER + ALT + S", "movetoworkspace special:special")

mysetup.bind_dispatch(v.kbWindowGroupCycleNext, "cyclenext", { repeating = true })
mysetup.bind_dispatch(v.kbWindowGroupCyclePrev, "cyclenext prev", { repeating = true })
mysetup.bind_dispatch("CTRL + ALT + Tab", "changegroupactive f", { repeating = true })
mysetup.bind_dispatch("CTRL + SHIFT + ALT + Tab", "changegroupactive b", { repeating = true })
mysetup.bind_dispatch(v.kbToggleGroup, "togglegroup")
mysetup.bind_dispatch(v.kbUngroup, "moveoutofgroup")
mysetup.bind_dispatch("SUPER + SHIFT + Comma", "lockactivegroup toggle")

mysetup.bind_dispatch("SUPER + left", "movefocus l")
mysetup.bind_dispatch("SUPER + right", "movefocus r")
mysetup.bind_dispatch("SUPER + up", "movefocus u")
mysetup.bind_dispatch("SUPER + down", "movefocus d")
mysetup.bind_dispatch("SUPER + SHIFT + left", "movewindow l")
mysetup.bind_dispatch("SUPER + SHIFT + right", "movewindow r")
mysetup.bind_dispatch("SUPER + SHIFT + up", "movewindow u")
mysetup.bind_dispatch("SUPER + SHIFT + down", "movewindow d")
mysetup.bind_dispatch("SUPER + Minus", "resizeactive -10% 0", { repeating = true })
mysetup.bind_dispatch("SUPER + Equal", "resizeactive 10% 0", { repeating = true })
mysetup.bind_dispatch("SUPER + SHIFT + Minus", "resizeactive 0 -10%", { repeating = true })
mysetup.bind_dispatch("SUPER + SHIFT + Equal", "resizeactive 0 10%", { repeating = true })
mysetup.bind_dispatch("SUPER + ALT + left", "resizeactive -10% 0", { repeating = true })
mysetup.bind_dispatch("SUPER + ALT + right", "resizeactive 10% 0", { repeating = true })
mysetup.bind_dispatch("SUPER + ALT + up", "resizeactive 0 -10%", { repeating = true })
mysetup.bind_dispatch("SUPER + ALT + down", "resizeactive 0 10%", { repeating = true })
hl.bind("SUPER + mouse:272", hl.dsp.window.drag(), { mouse = true })
hl.bind(v.kbMoveWindow, hl.dsp.window.drag(), { mouse = true })
hl.bind("SUPER + mouse:273", hl.dsp.window.resize(), { mouse = true })
hl.bind(v.kbResizeWindow, hl.dsp.window.resize(), { mouse = true })
mysetup.bind_dispatch("CTRL + SUPER + Backslash", "centerwindow 1")
mysetup.bind_dispatch("CTRL + SUPER + ALT + Backslash", "resizeactive exact 55% 70%")
mysetup.bind_dispatch("CTRL + SUPER + ALT + Backslash", "centerwindow 1")
mysetup.bind_dispatch(v.kbPinWindow, "pin")
mysetup.bind_dispatch(v.kbWindowFullscreen, "fullscreen 0")
mysetup.bind_dispatch(v.kbWindowBorderedFullscreen, "fullscreen 1")
mysetup.bind_dispatch(v.kbToggleWindowFloating, "togglefloating")
mysetup.bind_exec(v.kbCloseWindow, mysetup.hypr .. "/scripts/close-active.sh")

mysetup.bind_dispatch(v.kbSystemMonitor, "togglespecialworkspace sysmon")
mysetup.bind_dispatch(v.kbMusic, "togglespecialworkspace music")
mysetup.bind_dispatch(v.kbCommunication, "togglespecialworkspace communication")
mysetup.bind_dispatch(v.kbTodo, "togglespecialworkspace todo")

mysetup.bind_exec("XF86AudioMute", "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle", { locked = true })
mysetup.bind_exec("XF86AudioMicMute", "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle", { locked = true })
mysetup.bind_exec("SUPER + SHIFT + M", "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle", { locked = true })
mysetup.bind_exec(
	"XF86AudioRaiseVolume",
	"wpctl set-mute @DEFAULT_AUDIO_SINK@ 0; wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ "
		.. tostring(v.volumeStep)
		.. "%+",
	{ locked = true, repeating = true }
)
mysetup.bind_exec(
	"XF86AudioLowerVolume",
	"wpctl set-mute @DEFAULT_AUDIO_SINK@ 0; wpctl set-volume @DEFAULT_AUDIO_SINK@ " .. tostring(v.volumeStep) .. "%-",
	{ locked = true, repeating = true }
)

mysetup.bind_exec("SUPER + SHIFT + L", "systemctl suspend-then-hibernate", { locked = true })
mysetup.bind_exec(
	"CTRL + SHIFT + ALT + V",
	[[sleep 0.5s && ydotool type -d 1 "$(cliphist list | head -1 | cliphist decode)"]],
	{ locked = true }
)

mysetup.load_runtime("shell-keybinds.lua")
