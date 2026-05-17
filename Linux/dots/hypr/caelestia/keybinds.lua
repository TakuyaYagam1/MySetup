local mysetup = require("lib.mysetup")
local v = require("variables")

mysetup.bind_dispatch(v.kbSession, "global caelestia:session")
mysetup.bind_dispatch(v.kbShowSidebar, "global caelestia:sidebar")
mysetup.bind_dispatch(v.kbClearNotifs, "global caelestia:clearNotifs", { locked = true })
mysetup.bind_dispatch(v.kbShowPanels, "global caelestia:showall")
mysetup.bind_dispatch(v.kbLock, "global caelestia:lock")

mysetup.bind_exec(v.kbRestoreLock, mysetup.hypr .. "/scripts/restore-lock.sh caelestia", { locked = true })

mysetup.bind_dispatch("XF86MonBrightnessUp", "global caelestia:brightnessUp", { locked = true })
mysetup.bind_dispatch("XF86MonBrightnessDown", "global caelestia:brightnessDown", { locked = true })

mysetup.bind_dispatch("CTRL + SUPER + Space", "global caelestia:mediaToggle", { locked = true })
mysetup.bind_dispatch("XF86AudioPlay", "global caelestia:mediaToggle", { locked = true })
mysetup.bind_dispatch("XF86AudioPause", "global caelestia:mediaToggle", { locked = true })
mysetup.bind_dispatch("CTRL + SUPER + Equal", "global caelestia:mediaNext", { locked = true })
mysetup.bind_dispatch("XF86AudioNext", "global caelestia:mediaNext", { locked = true })
mysetup.bind_dispatch("CTRL + SUPER + Minus", "global caelestia:mediaPrev", { locked = true })
mysetup.bind_dispatch("XF86AudioPrev", "global caelestia:mediaPrev", { locked = true })
mysetup.bind_dispatch("XF86AudioStop", "global caelestia:mediaStop", { locked = true })

mysetup.bind_exec("CTRL + SUPER + SHIFT + R", "qs -c caelestia kill", { release = true })
mysetup.bind_exec("CTRL + SUPER + ALT + R", "qs -c caelestia kill; sleep .1; " .. mysetup.hypr .. "/scripts/start-shell.sh caelestia", { release = true })

mysetup.bind_exec(v.kbWindowPip, "caelestia resizer pip")

require("shell-common-keybinds")

mysetup.bind_exec("SUPER + ALT + R", "caelestia record -s")
mysetup.bind_exec("CTRL + ALT + R", "caelestia record")
mysetup.bind_exec("SUPER + SHIFT + ALT + R", "caelestia record -r")

mysetup.bind_exec("SUPER + V", "pkill fuzzel || caelestia clipboard")
mysetup.bind_exec("SUPER + ALT + V", "pkill fuzzel || caelestia clipboard -d")
mysetup.bind_exec("SUPER + Period", "pkill fuzzel || caelestia emoji -p")
