local mysetup = require("lib.mysetup")
local v = require("variables")

local wsaction = mysetup.hypr .. "/scripts/wsaction.fish"

for i = 1, 10 do
    local key = tostring(i % 10)
    mysetup.bind_exec(v.kbGoToWsGroup .. " + " .. key, wsaction .. " -g workspace " .. tostring(i))
end

for i = 1, 10 do
    local key = tostring(i % 10)
    mysetup.bind_exec(v.kbMoveWinToWs .. " + " .. key, wsaction .. " movetoworkspace " .. tostring(i))
end

for i = 1, 10 do
    local key = tostring(i % 10)
    mysetup.bind_exec(v.kbMoveWinToWsGroup .. " + " .. key, wsaction .. " -g movetoworkspace " .. tostring(i))
end
