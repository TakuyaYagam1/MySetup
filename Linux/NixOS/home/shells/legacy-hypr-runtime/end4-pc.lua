-- Active Hyprland profile: end4-pc
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate end4 Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
local end4_root = hypr_root .. "/end4"
package.path = end4_root .. "/?.lua;" .. end4_root .. "/?/init.lua;" .. hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(end4_root .. "/hyprland.lua")
