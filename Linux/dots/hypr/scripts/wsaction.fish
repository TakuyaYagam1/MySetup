#!/usr/bin/env fish

set -l group_mode 0
if test (count $argv) -gt 0; and test "$argv[1]" = '-g'
    set group_mode 1
    set -e argv[1]
end

if test (count $argv) -ne 2
    echo 'Wrong number of arguments. Usage: ./wsaction.fish [-g] <dispatcher> <workspace>'
    exit 1
end

set -l active_ws (hyprctl activeworkspace -j | jq -r '.id')

function dispatch_workspace --argument-names action target
    switch "$action"
        case workspace
            hyprctl dispatch "hl.dsp.focus({ workspace = \"$target\" })"
        case movetoworkspace
            hyprctl dispatch "hl.dsp.window.move({ workspace = \"$target\" })"
        case '*'
            echo "unsupported wsaction dispatcher: $action" >&2
            return 2
    end
end

if test "$group_mode" -eq 1
    # Move to group
    set -l active_slot (math "(($active_ws - 1) % 10) + 1")
    dispatch_workspace $argv[1] (math "($argv[2] - 1) * 10 + $active_slot")
else
    # Move to ws in group
    dispatch_workspace $argv[1] (math "floor(($active_ws - 1) / 10) * 10 + $argv[2]")
end
