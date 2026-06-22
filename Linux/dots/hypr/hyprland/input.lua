local v = require("variables")

hl.config({
	input = {
		kb_layout = "us,ru",
		kb_options = "grp:alt_shift_toggle",
		numlock_by_default = false,
		repeat_delay = 250,
		repeat_rate = 35,
		focus_on_close = 1,
		touchpad = {
			natural_scroll = true,
			disable_while_typing = v.touchpadDisableTyping,
			scroll_factor = v.touchpadScrollFactor,
		},
	},
	binds = {
		scroll_event_delay = 0,
	},
	cursor = {
		hotspot_padding = 1,
	},
})
