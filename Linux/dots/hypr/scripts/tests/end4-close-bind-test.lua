local hypr_root = arg[1]
local keybinds_path = arg[2]

if hypr_root == nil or keybinds_path == nil then
	error("usage: end4-close-bind-test.lua HYPR_ROOT END4_KEYBINDS")
end

package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path

local bindings = {}

local function bind(keys, dispatcher, options)
	bindings[keys] = bindings[keys] or {}
	table.insert(bindings[keys], { dispatcher = dispatcher, options = options })
end

hl = {
	bind = bind,
	unbind = function(keys)
		bindings[keys] = {}
	end,
	dsp = {
		exec_cmd = function(command)
			return { kind = "exec", command = command }
		end,
		focus = function(options)
			return { kind = "focus", options = options }
		end,
		window = {
			drag = function()
				return { kind = "drag" }
			end,
			resize = function(options)
				return { kind = "resize", options = options }
			end,
			float = function(options)
				return { kind = "float", options = options }
			end,
			pin = function()
				return { kind = "pin" }
			end,
			fullscreen = function(options)
				return { kind = "fullscreen", options = options }
			end,
		},
	},
}

local variables = require("variables")
local managed_keys = {
	{ label = "next workspace", keys = variables.kbNextWs },
	{ label = "previous workspace", keys = variables.kbPrevWs },
	{ label = "move window", keys = variables.kbMoveWindow },
	{ label = "resize window", keys = variables.kbResizeWindow },
	{ label = "close window", keys = variables.kbCloseWindow },
	{ label = "toggle floating", keys = variables.kbToggleWindowFloating },
	{ label = "pin window", keys = variables.kbPinWindow },
	{ label = "fullscreen", keys = variables.kbWindowFullscreen },
	{ label = "bordered fullscreen", keys = variables.kbWindowBorderedFullscreen },
	{ label = "alternate fullscreen", keys = "SUPER + CTRL + Return" },
}

local replacement_keys = {}
local replacement_labels = {}

local function add_replacement(keys, label)
	if replacement_labels[keys] == nil then
		table.insert(replacement_keys, keys)
		replacement_labels[keys] = label
	end
end

for _, binding in ipairs(managed_keys) do
	add_replacement(binding.keys, binding.label)
end

for _, binding in ipairs({
	{ "SUPER + SHIFT + W", "shell selector" },
	{ variables.kbTerminal, "terminal" },
	{ variables.kbZen, "Zen" },
	{ variables.kbCursor, "Cursor" },
	{ variables.kbVscode, "VS Code" },
	{ variables.kbZed, "Zed" },
	{ variables.kbDatagrip, "DataGrip" },
	{ variables.kbAntigravity, "Antigravity" },
	{ variables.kbAmneziaVPN, "AmneziaVPN" },
	{ variables.kbv2rayN, "v2rayN" },
	{ variables.kbVesktop, "Vesktop" },
	{ variables.kbSpotify, "Spotify" },
	{ variables.kbTelegram, "Telegram" },
	{ variables.kbBtop, "btop" },
	{ variables.kbFileExplorer, "file explorer" },
	{ "SUPER + ALT + E", "alternate file explorer" },
	{ "CTRL + ALT + V", "pavucontrol" },
	{ "SUPER + S", "screenshot" },
	{ "SUPER + SHIFT + S", "screen snip" },
	{ "SUPER + SHIFT + ALT + S", "screen snip editor" },
	{ "SUPER + C", "color picker" },
	{ variables.kbRecord, "recording" },
}) do
	add_replacement(binding[1], binding[2])
end

for index = 1, 10 do
	local key = tostring(index % 10)
	add_replacement(variables.kbGoToWsGroup .. " + " .. key, "group workspace " .. index)
	add_replacement(variables.kbMoveWinToWs .. " + " .. key, "move workspace " .. index)
	add_replacement(variables.kbMoveWinToWsGroup .. " + " .. key, "group move workspace " .. index)
end

for _, keys in ipairs(replacement_keys) do
	local label = replacement_labels[keys]
	bind(keys, { kind = "canonical-" .. label })
	bind(keys, { kind = "upstream-" .. label })
end

dofile(keybinds_path)

for _, keys in ipairs(replacement_keys) do
	local effective = bindings[keys] or {}
	if #effective ~= 1 then
		error(
			string.format(
				"expected one effective End4 %s bind for %s, got %d",
				replacement_labels[keys],
				keys,
				#effective
			)
		)
	end
end

local close_keys = variables.kbCloseWindow
local close_bindings = bindings[close_keys]
local dispatcher = close_bindings[1].dispatcher
if dispatcher.kind ~= "exec" or not dispatcher.command:match("/scripts/close%-active%.sh$") then
	error("effective End4 close bind does not use close-active.sh")
end

print("OK End4 has one effective bind for every managed replacement")
