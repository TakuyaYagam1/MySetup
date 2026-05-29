local mysetup = require("lib.mysetup")
local v = require("variables")

mysetup.bind_exec(
	"SUPER + SHIFT + W",
	mysetup.hypr .. "/scripts/shell-selector.sh toggle",
	{ description = "Toggle shell selector" }
)

mysetup.bind_exec(v.kbTerminal, "app2unit -- " .. v.terminal, { description = "Open terminal" })
mysetup.bind_exec(v.kbZen, "app2unit -- " .. v.zen, { description = "Open Zen" })
mysetup.bind_exec(v.kbCursor, "app2unit -- " .. v.cursor, { description = "Open Cursor" })
mysetup.bind_exec(v.kbVscode, "app2unit -- " .. v.vscode, { description = "Open VS Code" })
mysetup.bind_exec(v.kbDatagrip, "app2unit -- " .. v.datagrip, { description = "Open DataGrip" })
mysetup.bind_exec(v.kbAntigravity, "app2unit -- " .. v.antigravity, { description = "Open Antigravity" })
mysetup.bind_exec(v.kbAmneziaVPN, "app2unit -- " .. v.amneziaVPN, { description = "Open AmneziaVPN" })
mysetup.bind_exec(v.kbv2rayN, "app2unit -- " .. v.v2rayN, { description = "Open v2rayN" })
mysetup.bind_exec(v.kbVesktop, "app2unit -- " .. v.vesktop, { description = "Open Vesktop" })
mysetup.bind_exec(v.kbSpotify, mysetup.hypr .. "/scripts/spotify-toggle.sh", { description = "Toggle Spotify" })
mysetup.bind_exec(v.kbTelegram, "app2unit -- " .. v.telegram, { description = "Open Telegram" })
mysetup.bind_exec(v.kbBtop, "app2unit -- " .. v.btop, { description = "Open btop" })
mysetup.bind_exec(v.kbFileExplorer, "app2unit -- " .. v.fileExplorer, { description = "Open file explorer" })
mysetup.bind_exec("SUPER + ALT + E", "app2unit -- " .. v.fileExplorer, { description = "Open file explorer" })
mysetup.bind_exec("CTRL + ALT + V", "app2unit -- pavucontrol", { description = "Open pavucontrol" })

mysetup.bind_exec(
	"SUPER + S",
	mysetup.hypr .. "/scripts/screenshot.sh full",
	{ locked = true, description = "Screenshot" }
)
mysetup.bind_exec("SUPER + SHIFT + S", mysetup.hypr .. "/scripts/screenshot.sh region", { description = "Screen snip" })
mysetup.bind_exec(
	"SUPER + SHIFT + ALT + S",
	mysetup.hypr .. "/scripts/screenshot.sh edit",
	{ description = "Screen snip editor" }
)
mysetup.bind_exec("SUPER + C", "hyprpicker -a", { description = "Pick color" })

mysetup.bind_exec(v.kbRecord, mysetup.hypr .. "/scripts/record-toggle.sh", { description = "Toggle screen recording" })
