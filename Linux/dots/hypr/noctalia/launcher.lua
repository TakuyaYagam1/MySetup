local wahrwelt = require("lib.wahrwelt")

wahrwelt.bind_exec("SUPER + SUPER_L", wahrwelt.hypr .. "/scripts/noctalia-launcher.sh press")
wahrwelt.bind_exec("SUPER + SUPER_L", wahrwelt.hypr .. "/scripts/noctalia-launcher.sh release", { release = true })
wahrwelt.bind_exec(
	"SUPER + mouse:272",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse:273",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse:274",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse:275",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse:276",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse:277",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse_up",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
wahrwelt.bind_exec(
	"SUPER + mouse_down",
	wahrwelt.hypr .. "/scripts/noctalia-launcher.sh interrupt",
	{ non_consuming = true }
)
