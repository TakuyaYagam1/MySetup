local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

wahrwelt.load_runtime("shell-launcher.lua")

local wsaction = wahrwelt.hypr .. "/scripts/wsaction.fish"

for i = 1, 10 do
	local key = tostring(i % 10)
	wahrwelt.bind_exec(v.kbGoToWs .. " + " .. key, wsaction .. " workspace " .. tostring(i))
end

wahrwelt.bind_dispatch("SUPER + mouse_down", "workspace -1")
wahrwelt.bind_dispatch("SUPER + mouse_up", "workspace +1")
wahrwelt.bind_dispatch(v.kbPrevWs, "workspace -1", { repeating = true })
wahrwelt.bind_dispatch(v.kbNextWs, "workspace +1", { repeating = true })
wahrwelt.bind_dispatch("SUPER + Page_Up", "workspace -1", { repeating = true })
wahrwelt.bind_dispatch("SUPER + Page_Down", "workspace +1", { repeating = true })
wahrwelt.bind_dispatch("CTRL + SUPER + mouse_down", "workspace -10")
wahrwelt.bind_dispatch("CTRL + SUPER + mouse_up", "workspace +10")
wahrwelt.bind_dispatch(v.kbToggleSpecialWs, "togglespecialworkspace special")
require("shell-workspace-keybinds")

wahrwelt.bind_dispatch("SUPER + ALT + Page_Up", "movetoworkspace -1", { repeating = true })
wahrwelt.bind_dispatch("SUPER + ALT + Page_Down", "movetoworkspace +1", { repeating = true })
wahrwelt.bind_dispatch("SUPER + SHIFT + mouse_down", "movetoworkspace -1")
wahrwelt.bind_dispatch("SUPER + SHIFT + mouse_up", "movetoworkspace +1")
wahrwelt.bind_dispatch("CTRL + SUPER + SHIFT + right", "movetoworkspace +1", { repeating = true })
wahrwelt.bind_dispatch("CTRL + SUPER + SHIFT + left", "movetoworkspace -1", { repeating = true })
wahrwelt.bind_dispatch("CTRL + SUPER + SHIFT + up", "movetoworkspace special:special")
wahrwelt.bind_dispatch("CTRL + SUPER + SHIFT + down", "movetoworkspace e+0")
wahrwelt.bind_dispatch("SUPER + ALT + S", "movetoworkspace special:special")

wahrwelt.bind_dispatch(v.kbWindowGroupCycleNext, "cyclenext", { repeating = true })
wahrwelt.bind_dispatch(v.kbWindowGroupCyclePrev, "cyclenext prev", { repeating = true })
wahrwelt.bind_dispatch("CTRL + ALT + Tab", "changegroupactive f", { repeating = true })
wahrwelt.bind_dispatch("CTRL + SHIFT + ALT + Tab", "changegroupactive b", { repeating = true })
wahrwelt.bind_dispatch(v.kbToggleGroup, "togglegroup")
wahrwelt.bind_dispatch(v.kbUngroup, "moveoutofgroup")
wahrwelt.bind_dispatch("SUPER + SHIFT + Comma", "lockactivegroup toggle")

wahrwelt.bind_dispatch("SUPER + left", "movefocus l")
wahrwelt.bind_dispatch("SUPER + right", "movefocus r")
wahrwelt.bind_dispatch("SUPER + up", "movefocus u")
wahrwelt.bind_dispatch("SUPER + down", "movefocus d")
wahrwelt.bind_dispatch("SUPER + SHIFT + left", "movewindow l")
wahrwelt.bind_dispatch("SUPER + SHIFT + right", "movewindow r")
wahrwelt.bind_dispatch("SUPER + SHIFT + up", "movewindow u")
wahrwelt.bind_dispatch("SUPER + SHIFT + down", "movewindow d")
wahrwelt.bind_dispatch("SUPER + Minus", "resizeactive -10% 0", { repeating = true })
wahrwelt.bind_dispatch("SUPER + Equal", "resizeactive 10% 0", { repeating = true })
wahrwelt.bind_dispatch("SUPER + SHIFT + Minus", "resizeactive 0 -10%", { repeating = true })
wahrwelt.bind_dispatch("SUPER + SHIFT + Equal", "resizeactive 0 10%", { repeating = true })
wahrwelt.bind_dispatch("SUPER + ALT + left", "resizeactive -10% 0", { repeating = true })
wahrwelt.bind_dispatch("SUPER + ALT + right", "resizeactive 10% 0", { repeating = true })
wahrwelt.bind_dispatch("SUPER + ALT + up", "resizeactive 0 -10%", { repeating = true })
wahrwelt.bind_dispatch("SUPER + ALT + down", "resizeactive 0 10%", { repeating = true })
hl.bind("SUPER + mouse:272", hl.dsp.window.drag(), { mouse = true })
hl.bind(v.kbMoveWindow, hl.dsp.window.drag(), { mouse = true })
hl.bind("SUPER + mouse:273", hl.dsp.window.resize(), { mouse = true })
hl.bind(v.kbResizeWindow, hl.dsp.window.resize(), { mouse = true })
wahrwelt.bind_dispatch("CTRL + SUPER + Backslash", "centerwindow 1")
wahrwelt.bind_dispatch("CTRL + SUPER + ALT + Backslash", "resizeactive exact 55% 70%")
wahrwelt.bind_dispatch("CTRL + SUPER + ALT + Backslash", "centerwindow 1")
wahrwelt.bind_dispatch(v.kbPinWindow, "pin")
wahrwelt.bind_dispatch(v.kbWindowFullscreen, "fullscreen 0")
wahrwelt.bind_dispatch(v.kbWindowBorderedFullscreen, "fullscreen 1")
wahrwelt.bind_dispatch(v.kbToggleWindowFloating, "togglefloating")
wahrwelt.bind_exec(v.kbCloseWindow, wahrwelt.hypr .. "/scripts/close-active.sh")

wahrwelt.bind_dispatch(v.kbSystemMonitor, "togglespecialworkspace sysmon")
wahrwelt.bind_dispatch(v.kbMusic, "togglespecialworkspace music")
wahrwelt.bind_dispatch(v.kbCommunication, "togglespecialworkspace communication")
wahrwelt.bind_dispatch(v.kbTodo, "togglespecialworkspace todo")

wahrwelt.bind_exec("XF86AudioMute", "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle", { locked = true })
wahrwelt.bind_exec("XF86AudioMicMute", "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle", { locked = true })
wahrwelt.bind_exec("SUPER + SHIFT + M", "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle", { locked = true })
wahrwelt.bind_exec(
	"XF86AudioRaiseVolume",
	"wpctl set-mute @DEFAULT_AUDIO_SINK@ 0; wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ "
		.. tostring(v.volumeStep)
		.. "%+",
	{ locked = true, repeating = true }
)
wahrwelt.bind_exec(
	"XF86AudioLowerVolume",
	"wpctl set-mute @DEFAULT_AUDIO_SINK@ 0; wpctl set-volume @DEFAULT_AUDIO_SINK@ " .. tostring(v.volumeStep) .. "%-",
	{ locked = true, repeating = true }
)

wahrwelt.bind_exec("SUPER + SHIFT + L", "systemctl suspend-then-hibernate", { locked = true })
wahrwelt.bind_exec(
	"CTRL + SHIFT + ALT + V",
	[[sleep 0.5s && ydotool type -d 1 "$(cliphist list | head -1 | cliphist decode)"]],
	{ locked = true }
)

wahrwelt.load_runtime("shell-keybinds.lua")
