local v = require("variables")

hl.monitor({
    output = "",
    mode = "preferred",
    position = "auto",
    scale = 1,
})

hl.config({
    general = {
        layout = "dwindle",
        allow_tearing = true,
        gaps_workspaces = v.workspaceGaps,
        gaps_in = v.windowGapsIn,
        gaps_out = v.windowGapsOut,
        border_size = v.windowBorderSize,
        col = {
            active_border = v.activeWindowBorderColour,
            inactive_border = v.inactiveWindowBorderColour,
        },
    },
    dwindle = {
        preserve_split = true,
        smart_split = false,
        smart_resizing = true,
    },
})
