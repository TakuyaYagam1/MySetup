local scheme = require("scheme.default")
local v = require("variables")

hl.config({
	group = {
		col = {
			border_active = v.activeWindowBorderColour,
			border_inactive = v.inactiveWindowBorderColour,
			border_locked_active = v.activeWindowBorderColour,
			border_locked_inactive = v.inactiveWindowBorderColour,
		},
		groupbar = {
			font_family = "JetBrainsMono Nerd Font",
			font_size = 15,
			gradients = true,
			gradient_round_only_edges = false,
			gradient_rounding = 5,
			height = 25,
			indicator_height = 0,
			gaps_in = 3,
			gaps_out = 3,
			text_color = "rgb(" .. scheme.onPrimary .. ")",
			col = {
				active = "rgba(" .. scheme.primary .. "d4)",
				inactive = "rgba(" .. scheme.outline .. "d4)",
				locked_active = "rgba(" .. scheme.primary .. "d4)",
				locked_inactive = "rgba(" .. scheme.secondary .. "d4)",
			},
		},
	},
})
