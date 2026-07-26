local mysetup = require("lib.mysetup")
local v = require("variables")

hl.unbind("CTRL + SUPER + ALT + Slash")
mysetup.bind_exec(
	"CTRL + SUPER + ALT + Slash",
	"xdg-open " .. mysetup.hypr .. "/end4/mysetup/keybinds.lua",
	{ description = "Wahrwelt: Edit end4 keybinds" }
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

mysetup.bind_exec(v.kbNextWs, mysetup.hypr .. "/scripts/wsaction.fish workspace +1")
mysetup.bind_exec(v.kbPrevWs, mysetup.hypr .. "/scripts/wsaction.fish workspace -1")

for _, keys in ipairs({
	"SUPER + F",
	"SUPER + X",
	"SUPER + CTRL + Space",
}) do
	hl.unbind(keys)
end

hl.bind(v.kbMoveWindow, hl.dsp.window.drag(), { mouse = true })
hl.bind(v.kbResizeWindow, hl.dsp.window.resize(), { mouse = true })
mysetup.bind_dispatch(v.kbCloseWindow, "killactive")
mysetup.bind_dispatch(v.kbToggleWindowFloating, "togglefloating")
mysetup.bind_dispatch(v.kbPinWindow, "pin")
mysetup.bind_dispatch(v.kbWindowFullscreen, "fullscreen 0")
mysetup.bind_dispatch("SUPER + CTRL + Return", "fullscreen 0")
mysetup.bind_dispatch(v.kbWindowBorderedFullscreen, "fullscreen 1")
