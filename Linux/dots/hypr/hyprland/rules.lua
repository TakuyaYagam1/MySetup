local v = require("variables")

hl.window_rule({ match = { fullscreen = false }, opacity = tostring(v.windowOpacity) .. " override" })

hl.window_rule({ match = { class = "equibop|org\\.quickshell|imv|swappy" }, opaque = true })
hl.window_rule({ match = { float = true, xwayland = false }, center = true })

for _, class in ipairs({
	"guifetch",
	"yad",
	"zenity",
	"wev",
	"org\\.gnome\\.FileRoller",
	"file-roller",
	"blueman-manager",
	"com\\.github\\.GradienceTeam\\.Gradience",
	"feh",
	"imv",
	"system-config-printer",
	"org\\.quickshell",
}) do
	hl.window_rule({ match = { class = class }, float = true })
end

for _, rule in ipairs({
	{ match = { class = "foot", title = "nmtui" }, size = "60% 70%" },
	{ match = { class = "org\\.gnome\\.Settings" }, size = "70% 80%" },
	{ match = { class = "org\\.pulseaudio\\.pavucontrol|yad-icon-browser" }, size = "60% 70%" },
	{ match = { class = "nwg-look" }, size = "50% 60%" },
}) do
	hl.window_rule({ match = rule.match, float = true })
	hl.window_rule({ match = rule.match, size = rule.size })
	hl.window_rule({ match = rule.match, center = true })
end

hl.window_rule({ match = { class = "btop" }, workspace = "special:sysmon" })
hl.window_rule({
	match = {
		class = "(?i)(feishin|spotify|supersonic|cider|com\\.github\\.th_ch\\.youtube_music|plexamp|com-maxrave-simpmusic-mainkt)",
	},
	workspace = "special:music",
})
hl.window_rule({ match = { initial_title = "Spotify( Free)?" }, workspace = "special:music" })
hl.window_rule({ match = { class = "(?i)(discord|equibop|vesktop|whatsapp)" }, workspace = "special:communication" })
hl.window_rule({ match = { class = "(?i)todoist" }, workspace = "special:todo" })

for _, title in ipairs({
	"(Select|Open)( a)? (File|Folder)(s)?",
	"File (Operation|Upload)( Progress)?",
	".* Properties",
	"Export Image as PNG",
	"GIMP Crash Debug",
	"Save As",
	"Library",
}) do
	hl.window_rule({ match = { title = title }, float = true })
end

local pip_title = "Picture(-| )in(-| )[Pp]icture"
hl.window_rule({
	match = { title = pip_title },
	move = { "monitor_w-window_w-(monitor_w*0.02)", "monitor_h-window_h-(monitor_h*0.03)" },
})
hl.window_rule({ match = { title = pip_title }, keep_aspect_ratio = true })
hl.window_rule({ match = { title = pip_title }, float = true })
hl.window_rule({ match = { title = pip_title }, pin = true })

hl.window_rule({
	match = { class = "krita|gimp|inkscape|darktable|resolve|kdenlive|shotcut|blender|godot" },
	opaque = true,
})

hl.window_rule({ match = { class = "^(ueberzugpp_.*)$" }, float = true })
hl.window_rule({ match = { class = "^(ueberzugpp_.*)$" }, no_initial_focus = true })

hl.window_rule({ match = { class = "steam" }, rounding = 10 })
hl.window_rule({ match = { class = "steam", title = "Friends List" }, float = true })

local game_class = "(steam_app_(default|[0-9]+))|gamescope"
hl.window_rule({ match = { class = game_class }, opaque = true })
hl.window_rule({ match = { class = game_class }, immediate = true })
hl.window_rule({ match = { class = game_class }, idle_inhibit = "always" })

hl.window_rule({ match = { class = "com-atlauncher-App", title = "ATLauncher Console" }, float = true })
hl.window_rule({ match = { class = "PandoraLauncher", title = "Minecraft Game Output" }, float = true })

hl.window_rule({ match = { class = "fusion360\\.exe", title = "Fusion360|(Marking Menu)" }, no_blur = true })

hl.window_rule({ match = { xwayland = true, title = "win[0-9]+" }, no_dim = true })
hl.window_rule({ match = { xwayland = true, title = "win[0-9]+" }, no_shadow = true })
hl.window_rule({ match = { xwayland = true, title = "win[0-9]+" }, rounding = 10 })

hl.workspace_rule({ workspace = "w[tv1]s[false]", gaps_out = v.singleWindowGapsOut })
hl.workspace_rule({ workspace = "f[1]s[false]", gaps_out = v.singleWindowGapsOut })

hl.layer_rule({ match = { namespace = "hyprpicker" }, animation = "fade" })
hl.layer_rule({ match = { namespace = "logout_dialog" }, animation = "fade" })
hl.layer_rule({ match = { namespace = "selection" }, animation = "fade" })
hl.layer_rule({ match = { namespace = "wayfreeze" }, animation = "fade" })

hl.layer_rule({ match = { namespace = "launcher" }, animation = "popin 80%" })
hl.layer_rule({ match = { namespace = "launcher" }, blur = true })

hl.layer_rule({ match = { namespace = "caelestia-(border-exclusion|area-picker)" }, no_anim = true })
hl.layer_rule({ match = { namespace = "caelestia-(drawers|background)" }, animation = "fade" })
