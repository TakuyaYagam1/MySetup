hl.window_rule({ match = { class = "btop" }, workspace = "special:sysmon" })

hl.window_rule({
	match = {
		class = "(?i)(feishin|spotify|supersonic|cider|com\\.github\\.th_ch\\.youtube_music|plexamp|com-maxrave-simpmusic-mainkt)",
	},
	workspace = "special:music",
})
hl.window_rule({ match = { initial_title = "Spotify( Free)?" }, workspace = "special:music" })

hl.window_rule({
	match = { class = "(?i)(discord|equibop|vesktop|whatsapp)" },
	workspace = "special:communication",
})
hl.window_rule({ match = { class = "(?i)todoist" }, workspace = "special:todo" })
