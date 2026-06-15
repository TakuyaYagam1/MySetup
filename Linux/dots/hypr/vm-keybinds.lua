local vmSubmap = "virtual-machine"
local toggleKeys = "SUPER + F1"

local function notify(title, body)
	hl.dispatch(hl.dsp.exec_cmd("notify-send " .. string.format("%q %q", title, body) .. " -a Hyprland"))
end

hl.define_submap(vmSubmap, function()
	hl.bind(toggleKeys, function()
		if hl.get_current_submap() == vmSubmap then
			notify("Exited Virtual Machine mode", "Hyprland keybinds re-enabled")
			hl.dispatch(hl.dsp.submap("reset"))
		else
			notify("Entered Virtual Machine mode", "Press Super+F1 to re-enable Hyprland keybinds")
			hl.dispatch(hl.dsp.submap(vmSubmap))
		end
	end, { submap_universal = true, description = "Toggle VM keyboard passthrough mode" })
end)
