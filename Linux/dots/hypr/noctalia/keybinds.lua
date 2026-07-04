local mysetup = require("lib.mysetup")
local v = require("variables")

local noctalia = "noctalia msg"

mysetup.bind_exec(v.kbSession, noctalia .. " panel-toggle session")
mysetup.bind_exec(v.kbShowSidebar, noctalia .. " panel-toggle control-center")
mysetup.bind_exec(v.kbClearNotifs, noctalia .. " notification-clear-active", { locked = true })
mysetup.bind_exec(v.kbShowPanels, noctalia .. " settings-toggle")
mysetup.bind_exec(v.kbLock, noctalia .. " session lock")

mysetup.bind_exec(v.kbRestoreLock, mysetup.hypr .. "/scripts/restore-lock.sh noctalia", { locked = true })

mysetup.bind_exec("XF86MonBrightnessUp", noctalia .. " brightness-up", { locked = true })
mysetup.bind_exec("XF86MonBrightnessDown", noctalia .. " brightness-down", { locked = true })

mysetup.bind_exec("CTRL + SUPER + Space", noctalia .. " media toggle", { locked = true })
mysetup.bind_exec("XF86AudioPlay", noctalia .. " media toggle", { locked = true })
mysetup.bind_exec("XF86AudioPause", noctalia .. " media toggle", { locked = true })
mysetup.bind_exec("CTRL + SUPER + Equal", noctalia .. " media next", { locked = true })
mysetup.bind_exec("XF86AudioNext", noctalia .. " media next", { locked = true })
mysetup.bind_exec("CTRL + SUPER + Minus", noctalia .. " media previous", { locked = true })
mysetup.bind_exec("XF86AudioPrev", noctalia .. " media previous", { locked = true })
mysetup.bind_exec("XF86AudioStop", noctalia .. " media stop", { locked = true })

require("shell-common-keybinds")

mysetup.bind_exec(v.kbRecord, mysetup.hypr .. "/scripts/record-toggle.sh")

mysetup.bind_exec("SUPER + V", noctalia .. " panel-toggle clipboard")
mysetup.bind_exec("SUPER + ALT + V", noctalia .. " panel-toggle clipboard")
mysetup.bind_exec("SUPER + Period", noctalia .. " panel-toggle launcher /emo")
