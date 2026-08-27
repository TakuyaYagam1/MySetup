local wahrwelt = require("lib.wahrwelt")

require("hyprland.env")
wahrwelt.optional_require("wahrwelt.env")
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
wahrwelt.optional_require("wahrwelt.execs")
wahrwelt.optional_require("wahrwelt.general")
wahrwelt.optional_require("wahrwelt.rules")
wahrwelt.optional_require("wahrwelt.keybinds")
