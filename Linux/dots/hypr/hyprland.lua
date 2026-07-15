local mysetup = require("lib.mysetup")

require("hyprland.env")
mysetup.optional_require("mysetup.env")
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
require("vm-keybinds")

-- Loaded last, after every bind above is already registered, so overrides in
-- these files can hl.unbind() a default before re-binding it. Each is
-- optional and independently guarded - create only the ones you need.
mysetup.optional_require("mysetup.execs")
mysetup.optional_require("mysetup.general")
mysetup.optional_require("mysetup.rules")
mysetup.optional_require("mysetup.keybinds")
