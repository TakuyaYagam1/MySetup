local v = require("variables")

hl.config({
    gestures = {
        workspace_swipe_distance = 700,
        workspace_swipe_cancel_ratio = 0.15,
        workspace_swipe_min_speed_to_force = 5,
        workspace_swipe_direction_lock = true,
        workspace_swipe_direction_lock_threshold = 10,
        workspace_swipe_create_new = true,
    },
})

hl.gesture({ fingers = v.workspaceSwipeFingers, direction = "horizontal", action = "workspace" })
hl.gesture({ fingers = v.gestureFingers, direction = "up", action = "special", workspace_name = "special" })
hl.gesture({ fingers = v.gestureFingers, direction = "down", action = "special", workspace_name = "special" })
hl.gesture({
    fingers = v.gestureFingersMore,
    direction = "down",
    action = function()
        hl.exec_cmd("systemctl suspend-then-hibernate")
    end,
})
