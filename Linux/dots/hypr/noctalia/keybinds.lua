local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

local noctalia = wahrwelt.hypr .. "/scripts/noctalia-msg.sh"

wahrwelt.bind_exec(v.kbSession, noctalia .. " session-menu-toggle")
wahrwelt.bind_exec(v.kbShowSidebar, noctalia .. " control-center-toggle")
wahrwelt.bind_exec(v.kbClearNotifs, noctalia .. " notifications-clear", { locked = true })
wahrwelt.bind_exec(v.kbShowPanels, noctalia .. " settings-toggle")
wahrwelt.bind_exec(v.kbLock, noctalia .. " lock")

wahrwelt.bind_exec(v.kbRestoreLock, wahrwelt.hypr .. "/scripts/restore-lock.sh noctalia", { locked = true })

wahrwelt.bind_exec("XF86MonBrightnessUp", noctalia .. " brightness-up", { locked = true })
wahrwelt.bind_exec("XF86MonBrightnessDown", noctalia .. " brightness-down", { locked = true })

wahrwelt.bind_exec("CTRL + SUPER + Space", noctalia .. " media toggle", { locked = true })
wahrwelt.bind_exec("XF86AudioPlay", noctalia .. " media toggle", { locked = true })
wahrwelt.bind_exec("XF86AudioPause", noctalia .. " media toggle", { locked = true })
wahrwelt.bind_exec("CTRL + SUPER + Equal", noctalia .. " media next", { locked = true })
wahrwelt.bind_exec("XF86AudioNext", noctalia .. " media next", { locked = true })
wahrwelt.bind_exec("CTRL + SUPER + Minus", noctalia .. " media previous", { locked = true })
wahrwelt.bind_exec("XF86AudioPrev", noctalia .. " media previous", { locked = true })
wahrwelt.bind_exec("XF86AudioStop", noctalia .. " media stop", { locked = true })

require("shell-common-keybinds")

wahrwelt.bind_exec(v.kbRecord, wahrwelt.hypr .. "/scripts/record-toggle.sh")

wahrwelt.bind_exec("SUPER + V", noctalia .. " clipboard-toggle")
wahrwelt.bind_exec("SUPER + ALT + V", noctalia .. " clipboard-toggle")
wahrwelt.bind_exec("SUPER + Period", noctalia .. " emoji-toggle")
