-- Wahrwelt Hypr user namespace transition entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path

local readable_adapters = {}
for _, namespace in ipairs({ "user", "wahrwelt" }) do
    local path = hypr_root .. "/" .. namespace .. "/hyprland.lua"
    local file = io.open(path, "r")
    if file ~= nil then
        file:close()
        table.insert(readable_adapters, path)
    end
end

if #readable_adapters ~= 1 then
    error(
        "Wahrwelt user namespace transition: expected exactly one readable Hypr user adapter, found "
            .. #readable_adapters
    )
end

dofile(readable_adapters[1])
