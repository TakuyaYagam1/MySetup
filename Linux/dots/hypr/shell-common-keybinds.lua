local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

wahrwelt.bind_exec(
	"SUPER + SHIFT + W",
	wahrwelt.hypr .. "/scripts/shell-selector.sh toggle",
	{ description = "Toggle shell selector" }
)

wahrwelt.bind_exec(v.kbTerminal, "app2unit -- " .. v.terminal, { description = "Open terminal" })
wahrwelt.bind_exec(v.kbZen, "app2unit -- " .. v.zen, { description = "Open Zen" })
wahrwelt.bind_exec(v.kbCursor, "app2unit -- " .. v.cursor, { description = "Open Cursor" })
wahrwelt.bind_exec(v.kbVscode, "app2unit -- " .. v.vscode, { description = "Open VS Code" })
wahrwelt.bind_exec(v.kbZed, "app2unit -- " .. v.zed, { description = "Open Zed" })
wahrwelt.bind_exec(v.kbDatagrip, "app2unit -- " .. v.datagrip, { description = "Open DataGrip" })
wahrwelt.bind_exec(v.kbAntigravity, "app2unit -- " .. v.antigravity, { description = "Open Antigravity" })
wahrwelt.bind_exec(v.kbAmneziaVPN, "app2unit -- " .. v.amneziaVPN, { description = "Open AmneziaVPN" })
wahrwelt.bind_exec(v.kbv2rayN, "app2unit -- " .. v.v2rayN, { description = "Open v2rayN" })
wahrwelt.bind_exec(v.kbVesktop, "app2unit -- " .. v.vesktop, { description = "Open Vesktop" })
wahrwelt.bind_exec(v.kbSpotify, wahrwelt.hypr .. "/scripts/spotify-toggle.sh", { description = "Toggle Spotify" })
wahrwelt.bind_exec(v.kbTelegram, "app2unit -- " .. v.telegram, { description = "Open Telegram" })
wahrwelt.bind_exec(v.kbBtop, "app2unit -- " .. v.btop, { description = "Open btop" })
wahrwelt.bind_exec(v.kbFileExplorer, "app2unit -- " .. v.fileExplorer, { description = "Open file explorer" })
wahrwelt.bind_exec("SUPER + ALT + E", "app2unit -- " .. v.fileExplorer, { description = "Open file explorer" })
wahrwelt.bind_exec("CTRL + ALT + V", "app2unit -- pavucontrol", { description = "Open pavucontrol" })

wahrwelt.bind_exec(
	"SUPER + S",
	wahrwelt.hypr .. "/scripts/screenshot.sh full",
	{ locked = true, description = "Screenshot" }
)
wahrwelt.bind_exec(
	"SUPER + SHIFT + S",
	wahrwelt.hypr .. "/scripts/screenshot.sh region",
	{ description = "Screen snip" }
)
wahrwelt.bind_exec(
	"SUPER + SHIFT + ALT + S",
	wahrwelt.hypr .. "/scripts/screenshot.sh edit",
	{ description = "Screen snip editor" }
)
wahrwelt.bind_exec("SUPER + C", "hyprpicker -a", { description = "Pick color" })

wahrwelt.bind_exec(
	v.kbRecord,
	wahrwelt.hypr .. "/scripts/record-toggle.sh",
	{ description = "Toggle screen recording" }
)
