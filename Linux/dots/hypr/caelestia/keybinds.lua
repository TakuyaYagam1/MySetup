local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

wahrwelt.bind_dispatch(v.kbSession, "global caelestia:session")
wahrwelt.bind_dispatch(v.kbShowSidebar, "global caelestia:sidebar")
wahrwelt.bind_dispatch(v.kbClearNotifs, "global caelestia:clearNotifs", { locked = true })
wahrwelt.bind_dispatch(v.kbShowPanels, "global caelestia:showall")
wahrwelt.bind_dispatch(v.kbLock, "global caelestia:lock")

wahrwelt.bind_exec(v.kbRestoreLock, wahrwelt.hypr .. "/scripts/restore-lock.sh caelestia", { locked = true })

wahrwelt.bind_dispatch("XF86MonBrightnessUp", "global caelestia:brightnessUp", { locked = true })
wahrwelt.bind_dispatch("XF86MonBrightnessDown", "global caelestia:brightnessDown", { locked = true })

wahrwelt.bind_dispatch("CTRL + SUPER + Space", "global caelestia:mediaToggle", { locked = true })
wahrwelt.bind_dispatch("XF86AudioPlay", "global caelestia:mediaToggle", { locked = true })
wahrwelt.bind_dispatch("XF86AudioPause", "global caelestia:mediaToggle", { locked = true })
wahrwelt.bind_dispatch("CTRL + SUPER + Equal", "global caelestia:mediaNext", { locked = true })
wahrwelt.bind_dispatch("XF86AudioNext", "global caelestia:mediaNext", { locked = true })
wahrwelt.bind_dispatch("CTRL + SUPER + Minus", "global caelestia:mediaPrev", { locked = true })
wahrwelt.bind_dispatch("XF86AudioPrev", "global caelestia:mediaPrev", { locked = true })
wahrwelt.bind_dispatch("XF86AudioStop", "global caelestia:mediaStop", { locked = true })

wahrwelt.bind_exec("CTRL + SUPER + SHIFT + R", "qs -c caelestia kill", { release = true })
wahrwelt.bind_exec(
	"CTRL + SUPER + ALT + R",
	"qs -c caelestia kill; sleep .1; " .. wahrwelt.hypr .. "/scripts/start-shell.sh caelestia",
	{ release = true }
)

wahrwelt.bind_exec(v.kbWindowPip, "caelestia resizer pip")

require("shell-common-keybinds")

wahrwelt.bind_exec("SUPER + ALT + R", "caelestia record -s")
wahrwelt.bind_exec("CTRL + ALT + R", "caelestia record")
wahrwelt.bind_exec("SUPER + SHIFT + ALT + R", "caelestia record -r")

wahrwelt.bind_exec("SUPER + V", "pkill fuzzel || caelestia clipboard")
wahrwelt.bind_exec("SUPER + ALT + V", "pkill fuzzel || caelestia clipboard -d")
wahrwelt.bind_exec("SUPER + Period", "pkill fuzzel || caelestia emoji -p")
