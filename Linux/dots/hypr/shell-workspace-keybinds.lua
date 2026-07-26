local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

local wsaction = wahrwelt.hypr .. "/scripts/wsaction.fish"

for i = 1, 10 do
	local key = tostring(i % 10)
	wahrwelt.bind_exec(v.kbGoToWsGroup .. " + " .. key, wsaction .. " -g workspace " .. tostring(i))
end

for i = 1, 10 do
	local key = tostring(i % 10)
	wahrwelt.bind_exec(v.kbMoveWinToWs .. " + " .. key, wsaction .. " movetoworkspace " .. tostring(i))
end

for i = 1, 10 do
	local key = tostring(i % 10)
	wahrwelt.bind_exec(v.kbMoveWinToWsGroup .. " + " .. key, wsaction .. " -g movetoworkspace " .. tostring(i))
end
