local M = {}

local function is_absolute(path)
	return path:sub(1, 1) == "/"
end

local function shell_quote(value)
	return "'" .. value:gsub("'", "'\\''") .. "'"
end

local function is_collision_module(name)
	if type(name) ~= "string" then
		return false
	end
	return name == "hyprland"
		or name:sub(1, 9) == "hyprland."
		or name:sub(1, 9) == "hyprland/"
		or name == "custom"
		or name:sub(1, 7) == "custom."
		or name:sub(1, 7) == "custom/"
		or name == "workspaces"
		or name == "monitors"
end

local function validate(args)
	if type(args) ~= "table" then
		error("end4-adapter.load expects a table", 3)
	end
	if args.profile ~= "end4" and args.profile ~= "end4-pc" then
		error("end4-adapter profile must be end4 or end4-pc", 3)
	end
	if type(args.quickshell_config) ~= "string" or not is_absolute(args.quickshell_config) then
		error("end4-adapter quickshell_config must be an absolute path", 3)
	end

	local home = os.getenv("HOME")
	if type(home) ~= "string" or home == "" or not is_absolute(home) then
		error("HOME must be an absolute path for end4-adapter", 3)
	end
	local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
	if config_home == "" or not is_absolute(config_home) then
		error("XDG_CONFIG_HOME must be an absolute path for end4-adapter", 3)
	end
	if type(hl) ~= "table" then
		error("end4-adapter requires the Hyprland Lua API", 3)
	end
	for _, name in ipairs({ "config", "gesture", "env" }) do
		if type(hl[name]) ~= "function" then
			error("end4-adapter requires hl." .. name, 3)
		end
	end
	for _, name in ipairs({ "bind", "unbind" }) do
		if type(hl[name]) ~= "function" then
			error("end4-adapter requires hl." .. name, 3)
		end
	end
	if type(hl.dsp) ~= "table" or type(hl.dsp.exec_cmd) ~= "function" then
		error("end4-adapter requires hl.dsp.exec_cmd", 3)
	end

	return config_home, config_home .. "/hypr/end4"
end

local function traceback(message)
	return debug.traceback(message, 2)
end

local unpack_values = table.unpack or unpack

local function pack_values(...)
	return { n = select("#", ...), ... }
end

local function collision_modules()
	local names = {}
	for name in pairs(package.loaded) do
		if is_collision_module(name) then
			table.insert(names, name)
		end
	end
	return names
end

function M.load(args)
	local config_home, end4_root = validate(args)
	local saved_path = package.path
	local saved_hl = {
		bind = hl.bind,
		config = hl.config,
		define_submap = hl.define_submap,
		exec_cmd = hl.dsp.exec_cmd,
		gesture = hl.gesture,
		env = hl.env,
		unbind = hl.unbind,
	}
	local saved_modules = {}
	local current_bind_scope = "global"
	local replaced_bindings = {}

	for _, name in ipairs(collision_modules()) do
		saved_modules[name] = package.loaded[name]
		package.loaded[name] = nil
	end

	package.path = end4_root .. "/?.lua;" .. end4_root .. "/?/init.lua;" .. saved_path
	hl.bind = function(keys, ...)
		if type(keys) == "string" then
			local binding = current_bind_scope .. "\0" .. keys
			if not replaced_bindings[binding] then
				replaced_bindings[binding] = true
				saved_hl.unbind(keys)
			end
		end
		return saved_hl.bind(keys, ...)
	end
	if type(saved_hl.define_submap) == "function" then
		hl.define_submap = function(...)
			local arguments = pack_values(...)
			local callback_index
			for index = arguments.n, 1, -1 do
				if type(arguments[index]) == "function" then
					callback_index = index
					break
				end
			end
			if callback_index == nil then
				return saved_hl.define_submap(unpack_values(arguments, 1, arguments.n))
			end

			local callback = arguments[callback_index]
			local scope = current_bind_scope .. "\0submap:" .. tostring(arguments[1])
			arguments[callback_index] = function(...)
				local callback_arguments = pack_values(...)
				local previous_scope = current_bind_scope
				current_bind_scope = scope
				local results = pack_values(xpcall(function()
					return callback(unpack_values(callback_arguments, 1, callback_arguments.n))
				end, traceback))
				current_bind_scope = previous_scope
				if not results[1] then
					error(results[2], 0)
				end
				return unpack_values(results, 2, results.n)
			end
			return saved_hl.define_submap(unpack_values(arguments, 1, arguments.n))
		end
	end
	hl.config = function(config, ...)
		if type(config) ~= "table" then
			return saved_hl.config(config, ...)
		end
		local filtered = {}
		for name, value in pairs(config) do
			if name ~= "input" and name ~= "gestures" then
				filtered[name] = value
			end
		end
		return saved_hl.config(filtered, ...)
	end
	hl.gesture = function() end
	hl.env = function(name, ...)
		if name == "qsConfig" then
			return
		end
		return saved_hl.env(name, ...)
	end

	local ok, result = xpcall(function()
		saved_hl.env("qsConfig", args.quickshell_config)
		dofile(end4_root .. "/hyprland.lua")
		saved_hl.env("qsConfig", args.quickshell_config)
		saved_hl.unbind("CTRL + SUPER + R")
		saved_hl.bind(
			"CTRL + SUPER + R",
			saved_hl.exec_cmd(shell_quote(config_home .. "/hypr/scripts/start-shell.sh") .. " " .. args.profile),
			{ description = "Shell: Restart widgets" }
		)
	end, traceback)

	for _, name in ipairs(collision_modules()) do
		package.loaded[name] = nil
	end
	for name, value in pairs(saved_modules) do
		package.loaded[name] = value
	end
	package.path = saved_path
	hl.bind = saved_hl.bind
	hl.config = saved_hl.config
	hl.define_submap = saved_hl.define_submap
	hl.dsp.exec_cmd = saved_hl.exec_cmd
	hl.gesture = saved_hl.gesture
	hl.env = saved_hl.env
	hl.unbind = saved_hl.unbind

	if not ok then
		error(result, 0)
	end
end

return M
