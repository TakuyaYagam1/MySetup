local v = require("variables")

hl.config({
	decoration = {
		rounding = v.windowRounding,
		blur = {
			enabled = v.blurEnabled,
			xray = v.blurXray,
			special = v.blurSpecialWs,
			ignore_opacity = true,
			new_optimizations = true,
			popups = v.blurPopups,
			input_methods = v.blurInputMethods,
			size = v.blurSize,
			passes = v.blurPasses,
		},
		shadow = {
			enabled = v.shadowEnabled,
			range = v.shadowRange,
			render_power = v.shadowRenderPower,
			color = v.shadowColour,
		},
	},
})
