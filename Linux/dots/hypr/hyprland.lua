local wahrwelt = require("lib.wahrwelt")

local wahrwelt_user_searcher = package._wahrwelt_user_searcher
if type(wahrwelt_user_searcher) ~= "function" then
	wahrwelt_user_searcher = function(module)
		local prefix = "wahrwelt."
		if module:sub(1, #prefix) ~= prefix then
			return nil
		end
		local suffix = module:sub(#prefix + 1)
		if
			suffix == ""
			or suffix:find("[^A-Za-z0-9_.-]")
			or suffix:match("^%.")
			or suffix:match("%.$")
			or suffix:find("%.%.")
		then
			return "\n\tinvalid Wahrwelt user module " .. module
		end
		for part in suffix:gmatch("[^.]+") do
			if not part:match("^[A-Za-z_][A-Za-z0-9_%-]*$") then
				return "\n\tinvalid Wahrwelt user module " .. module
			end
		end
		local mapped = "user." .. suffix
		return function()
			return require(mapped)
		end,
			wahrwelt.config_file("user/" .. suffix:gsub("%.", "/") .. ".lua")
	end
	package._wahrwelt_user_searcher = wahrwelt_user_searcher
end

for index = #package.searchers, 1, -1 do
	if package.searchers[index] == wahrwelt_user_searcher then
		table.remove(package.searchers, index)
	end
end
table.insert(package.searchers, 1, wahrwelt_user_searcher)

require("hyprland.env")
require("hyprland.general")
require("hyprland.input")
require("hyprland.misc")
require("hyprland.animations")
require("hyprland.decoration")
require("hyprland.group")
require("hyprland.execs")
require("hyprland.rules")
require("hyprland.gestures")
require("hyprland.scrolling")
require("hyprland.keybinds")

wahrwelt.load_runtime("shell-profile.lua")
wahrwelt.load_runtime("shell-launcher.lua")
wahrwelt.load_runtime("shell-keybinds.lua")
require("vm-keybinds")

if not wahrwelt.optional_require("wahrwelt.default") then
	wahrwelt.optional_require("wahrwelt.execs")
	wahrwelt.optional_require("wahrwelt.general")
	wahrwelt.optional_require("wahrwelt.rules")
	wahrwelt.optional_require("wahrwelt.keybinds")
end
