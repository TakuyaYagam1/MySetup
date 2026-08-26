local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

local function replace_exec(keys, command, options)
	hl.unbind(keys)
	wahrwelt.bind_exec(keys, command, options)
end

replace_exec(
	"SUPER + SHIFT + W",
	wahrwelt.hypr .. "/scripts/shell-selector.sh toggle",
	{ description = "Toggle shell selector" }
)

replace_exec(v.kbTerminal, "app2unit -- " .. v.terminal, { description = "Open terminal" })
replace_exec(v.kbZen, "app2unit -- " .. v.zen, { description = "Open Zen" })
replace_exec(v.kbCursor, "app2unit -- " .. v.cursor, { description = "Open Cursor" })
replace_exec(v.kbVscode, "app2unit -- " .. v.vscode, { description = "Open VS Code" })
replace_exec(v.kbZed, "app2unit -- " .. v.zed, { description = "Open Zed" })
replace_exec(v.kbDatagrip, "app2unit -- " .. v.datagrip, { description = "Open DataGrip" })
replace_exec(v.kbAntigravity, "app2unit -- " .. v.antigravity, { description = "Open Antigravity" })
replace_exec(v.kbAmneziaVPN, "app2unit -- " .. v.amneziaVPN, { description = "Open AmneziaVPN" })
replace_exec(v.kbv2rayN, "app2unit -- " .. v.v2rayN, { description = "Open v2rayN" })
replace_exec(v.kbVesktop, "app2unit -- " .. v.vesktop, { description = "Open Vesktop" })
replace_exec(v.kbSpotify, wahrwelt.hypr .. "/scripts/spotify-toggle.sh", { description = "Toggle Spotify" })
replace_exec(v.kbTelegram, "app2unit -- " .. v.telegram, { description = "Open Telegram" })
replace_exec(v.kbBtop, "app2unit -- " .. v.btop, { description = "Open btop" })
replace_exec(v.kbFileExplorer, "app2unit -- " .. v.fileExplorer, { description = "Open file explorer" })
replace_exec("SUPER + ALT + E", "app2unit -- " .. v.fileExplorer, { description = "Open file explorer" })
replace_exec("CTRL + ALT + V", "app2unit -- pavucontrol", { description = "Open pavucontrol" })

replace_exec("SUPER + S", wahrwelt.hypr .. "/scripts/screenshot.sh full", { locked = true, description = "Screenshot" })
replace_exec("SUPER + SHIFT + S", wahrwelt.hypr .. "/scripts/screenshot.sh region", { description = "Screen snip" })
replace_exec(
	"SUPER + SHIFT + ALT + S",
	wahrwelt.hypr .. "/scripts/screenshot.sh edit",
	{ description = "Screen snip editor" }
)
replace_exec("SUPER + C", "hyprpicker -a", { description = "Pick color" })

replace_exec(v.kbRecord, wahrwelt.hypr .. "/scripts/record-toggle.sh", { description = "Toggle screen recording" })
