local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

hl.unbind("CTRL + SUPER + ALT + Slash")
wahrwelt.bind_exec(
	"CTRL + SUPER + ALT + Slash",
	"xdg-open " .. wahrwelt.hypr .. "/user/default.lua",
	{ description = "Wahrwelt: Edit user Hyprland config" }
)

for _, keys in ipairs({
	"SUPER + Return",
	"SUPER + E",
	"SUPER + SHIFT + C",
	"SUPER + SHIFT + Z",
	"SUPER + SHIFT + A",
	"SUPER + SHIFT + N",
	"SUPER + SHIFT + T",
	"SUPER + SHIFT + B",
	"CTRL + SHIFT + Escape",
	"SUPER + S",
	"SUPER + SHIFT + S",
	"SUPER + C",
}) do
	hl.unbind(keys)
end

require("shell-common-keybinds")
require("shell-workspace-keybinds")

hl.unbind(v.kbNextWs)
wahrwelt.bind_exec(v.kbNextWs, wahrwelt.hypr .. "/scripts/wsaction.fish workspace +1")
hl.unbind(v.kbPrevWs)
wahrwelt.bind_exec(v.kbPrevWs, wahrwelt.hypr .. "/scripts/wsaction.fish workspace -1")

for _, keys in ipairs({
	"SUPER + F",
	"SUPER + X",
	"SUPER + CTRL + Space",
}) do
	hl.unbind(keys)
end

hl.unbind(v.kbMoveWindow)
hl.bind(v.kbMoveWindow, hl.dsp.window.drag(), { mouse = true })
hl.unbind(v.kbResizeWindow)
hl.bind(v.kbResizeWindow, hl.dsp.window.resize(), { mouse = true })
hl.unbind(v.kbCloseWindow)
wahrwelt.bind_exec(v.kbCloseWindow, wahrwelt.hypr .. "/scripts/close-active.sh")
hl.unbind(v.kbToggleWindowFloating)
wahrwelt.bind_dispatch(v.kbToggleWindowFloating, "togglefloating")
hl.unbind(v.kbPinWindow)
wahrwelt.bind_dispatch(v.kbPinWindow, "pin")
hl.unbind(v.kbWindowFullscreen)
wahrwelt.bind_dispatch(v.kbWindowFullscreen, "fullscreen 0")
hl.unbind("SUPER + CTRL + Return")
wahrwelt.bind_dispatch("SUPER + CTRL + Return", "fullscreen 0")
hl.unbind(v.kbWindowBorderedFullscreen)
wahrwelt.bind_dispatch(v.kbWindowBorderedFullscreen, "fullscreen 1")
