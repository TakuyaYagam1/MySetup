local wahrwelt = require("lib.wahrwelt")
local v = require("variables")

local wsaction = wahrwelt.hypr .. "/scripts/wsaction.fish"

local function replace_exec(keys, command)
	hl.unbind(keys)
	wahrwelt.bind_exec(keys, command)
end

for i = 1, 10 do
	local key = tostring(i % 10)
	replace_exec(v.kbGoToWsGroup .. " + " .. key, wsaction .. " -g workspace " .. tostring(i))
end

for i = 1, 10 do
	local key = tostring(i % 10)
	replace_exec(v.kbMoveWinToWs .. " + " .. key, wsaction .. " movetoworkspace " .. tostring(i))
end

for i = 1, 10 do
	local key = tostring(i % 10)
	replace_exec(v.kbMoveWinToWsGroup .. " + " .. key, wsaction .. " -g movetoworkspace " .. tostring(i))
end
