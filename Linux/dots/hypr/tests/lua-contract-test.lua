local script_path = debug.getinfo(1, "S").source:sub(2)
local source_root = script_path:match("^(.*)/tests/[^/]+$")

local function fail(message)
	error(message, 2)
end

local function assert_equal(actual, expected, message)
	if actual ~= expected then
		fail(string.format("%s: expected %q, got %q", message, tostring(expected), tostring(actual)))
	end
end

local function assert_same(actual, expected, message)
	if not rawequal(actual, expected) then
		fail(message)
	end
end

local function assert_contains(value, expected, message)
	if not tostring(value):find(expected, 1, true) then
		fail(message .. ": " .. tostring(value))
	end
end

local function assert_sequence(actual, expected, message)
	assert_equal(#actual, #expected, message .. " length")
	for index, value in ipairs(expected) do
		assert_equal(actual[index], value, message .. " item " .. index)
	end
end

local function load_module(path, environment)
	setmetatable(environment, { __index = _G })
	local chunk, load_error = loadfile(path, "t", environment)
	if chunk == nil then
		fail(load_error)
	end
	return chunk()
end

local function run_canonical(default_present, failing_module)
	local events = {}
	local wahrwelt = {}

	function wahrwelt.load_runtime(name)
		table.insert(events, "runtime:" .. name)
	end

	function wahrwelt.optional_require(name)
		table.insert(events, "optional:" .. name)
		if name == failing_module then
			error("module failure: " .. name)
		end
		return default_present and name == "wahrwelt.default"
	end

	local environment = {
		package = { searchers = {} },
		require = function(name)
			table.insert(events, "require:" .. name)
			if name == "lib.wahrwelt" then
				return wahrwelt
			end
			return {}
		end,
	}
	local ok, result = pcall(function()
		load_module(source_root .. "/hyprland.lua", environment)
	end)
	return ok, result, events
end

local canonical_modules = {
	"require:lib.wahrwelt",
	"require:hyprland.env",
	"require:hyprland.general",
	"require:hyprland.input",
	"require:hyprland.misc",
	"require:hyprland.animations",
	"require:hyprland.decoration",
	"require:hyprland.group",
	"require:hyprland.execs",
	"require:hyprland.rules",
	"require:hyprland.gestures",
	"require:hyprland.scrolling",
	"require:hyprland.keybinds",
	"runtime:shell-profile.lua",
	"runtime:shell-launcher.lua",
	"runtime:shell-keybinds.lua",
	"require:vm-keybinds",
}

local ok, _, default_events = run_canonical(true)
assert_equal(ok, true, "canonical default load")
local expected_default = {}
for _, event in ipairs(canonical_modules) do
	table.insert(expected_default, event)
end
table.insert(expected_default, "optional:wahrwelt.default")
assert_sequence(default_events, expected_default, "canonical default order")

local fallback_ok, _, fallback_events = run_canonical(false)
assert_equal(fallback_ok, true, "canonical fallback load")
local expected_fallback = {}
for _, event in ipairs(canonical_modules) do
	table.insert(expected_fallback, event)
end
for _, name in ipairs({
	"wahrwelt.default",
	"wahrwelt.execs",
	"wahrwelt.general",
	"wahrwelt.rules",
	"wahrwelt.keybinds",
}) do
	table.insert(expected_fallback, "optional:" .. name)
end
assert_sequence(fallback_events, expected_fallback, "canonical fallback order")

local error_ok, error_result = run_canonical(false, "wahrwelt.rules")
assert_equal(error_ok, false, "canonical optional module errors propagate")
assert_contains(error_result, "module failure: wahrwelt.rules", "canonical module error")

local function test_wahrwelt_user_searcher()
	local events = {}
	local wahrwelt = {
		config_file = function(name)
			return "/home/test/.config/hypr/" .. name
		end,
		load_runtime = function() end,
		optional_require = function()
			return true
		end,
	}
	local package_table = { searchers = {} }
	local environment = {
		package = package_table,
		table = table,
		require = function(name)
			if name == "lib.wahrwelt" then
				return wahrwelt
			end
			if name == "user.execs" then
				table.insert(events, name)
				return "migrated"
			end
			return {}
		end,
	}
	load_module(source_root .. "/hyprland.lua", environment)
	load_module(source_root .. "/hyprland.lua", environment)
	local searcher_count = 0
	for _, searcher in ipairs(package_table.searchers) do
		if searcher == package_table._wahrwelt_user_searcher then
			searcher_count = searcher_count + 1
		end
	end
	assert_equal(searcher_count, 1, "Wahrwelt user searcher is installed once across reloads")
	local loader, path = package_table.searchers[1]("wahrwelt.execs")
	assert_equal(type(loader), "function", "Wahrwelt module searcher loader: " .. tostring(loader))
	assert_equal(path, "/home/test/.config/hypr/user/execs.lua", "Wahrwelt module searcher path")
	assert_equal(loader(), "migrated", "Wahrwelt module searcher result")
	assert_equal(package_table.searchers[1]("lib.wahrwelt"), nil, "Wahrwelt module searcher scope")
	assert_contains(
		package_table.searchers[1]("wahrwelt..bad"),
		"invalid Wahrwelt user module",
		"Wahrwelt module searcher rejects invalid names"
	)
	assert_sequence(events, { "user.execs" }, "Wahrwelt module searcher delegates to the physical user path")
end

test_wahrwelt_user_searcher()

local function test_optional_require()
	local existing = {}
	local open_failures = {}
	local required = {}
	local fake_package = { path = "original-package-path", loaded = {} }
	local environment = {
		package = fake_package,
		os = {
			getenv = function(name)
				if name == "HOME" then
					return "/home/test"
				end
				return nil
			end,
		},
		io = {
			open = function(path)
				local failure = open_failures[path]
				if failure ~= nil then
					return nil, failure.message, failure.code
				end
				if not existing[path] then
					return nil, path .. ": No such file or directory", 2
				end
				return { close = function() end }
			end,
		},
		require = function(name)
			table.insert(required, name)
			if name == "wahrwelt.broken" then
				error("broken optional module")
			end
			return {}
		end,
	}
	local wahrwelt = load_module(source_root .. "/lib/wahrwelt.lua", environment)
	assert_equal(fake_package.path, "original-package-path", "lib import must preserve package.path")
	assert_equal(wahrwelt.optional_require("wahrwelt.absent"), false, "absent optional module")
	assert_equal(#required, 0, "absent optional module must not call require")

	for _, failure in ipairs({
		{ module = "permission-denied", message = "Permission denied", code = 13 },
		{ module = "io-error", message = "Input/output error", code = 5 },
		{ module = "mock-error", message = "mock open failure" },
	}) do
		local path = "/home/test/.config/hypr/user/" .. failure.module .. ".lua"
		open_failures[path] = failure
		local open_ok, open_error = pcall(wahrwelt.optional_require, "wahrwelt." .. failure.module)
		assert_equal(open_ok, false, failure.module .. " optional module open error propagation")
		assert_contains(open_error, failure.message, failure.module .. " optional module open error")
		assert_equal(#required, 0, failure.module .. " optional module must not call require")
	end

	existing["/home/test/.config/hypr/user/present.lua"] = true
	assert_equal(wahrwelt.optional_require("wahrwelt.present"), true, "present optional module")
	assert_sequence(required, { "wahrwelt.present" }, "present optional module require")

	existing["/home/test/.config/hypr/user/legacy.lua"] = true
	assert_equal(wahrwelt.optional_require("wahrwelt.legacy"), true, "legacy optional module")
	assert_sequence(required, { "wahrwelt.present", "wahrwelt.legacy" }, "Wahrwelt optional module require")

	existing["/home/test/.config/hypr/user/broken.lua"] = true
	local require_ok, require_error = pcall(wahrwelt.optional_require, "wahrwelt.broken")
	assert_equal(require_ok, false, "optional require error propagation")
	assert_contains(require_error, "broken optional module", "optional require error")
end

test_optional_require()

local function test_user_namespace_transition()
	local transition_path = source_root
		.. "/../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-namespace-transition.lua"
	local hypr_root = "/config/test/hypr"

	local function run_transition(readable_namespaces, adapter_error)
		local loaded = {}
		local readable = {}
		for _, namespace in ipairs(readable_namespaces) do
			readable[hypr_root .. "/" .. namespace .. "/hyprland.lua"] = true
		end
		local package_table = { path = "canonical-package-path" }
		local environment = {
			package = package_table,
			os = {
				getenv = function(name)
					if name == "HOME" then
						return "/home/test"
					elseif name == "XDG_CONFIG_HOME" then
						return "/config/test"
					end
					return nil
				end,
			},
			io = {
				open = function(path)
					if not readable[path] then
						return nil
					end
					return { close = function() end }
				end,
			},
			dofile = function(path)
				table.insert(loaded, path)
				if adapter_error ~= nil then
					error(adapter_error)
				end
			end,
		}
		local ok, result = pcall(load_module, transition_path, environment)
		return ok, result, loaded, package_table.path
	end

	for _, namespace in ipairs({ "user", "wahrwelt" }) do
		local ok, _, loaded, package_path = run_transition({ namespace })
		assert_equal(ok, true, "namespace transition loads " .. namespace)
		assert_sequence(loaded, { hypr_root .. "/" .. namespace .. "/hyprland.lua" }, "namespace transition path")
		assert_equal(
			package_path,
			hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;canonical-package-path",
			"namespace transition package.path"
		)
	end

	for _, scenario in ipairs({
		{ namespaces = {}, count = 0 },
		{ namespaces = { "user", "wahrwelt" }, count = 2 },
	}) do
		local ok, result, loaded = run_transition(scenario.namespaces)
		assert_equal(ok, false, "namespace transition rejects adapter count " .. scenario.count)
		assert_contains(
			result,
			"expected exactly one readable Hypr user adapter, found " .. scenario.count,
			"namespace transition count"
		)
		assert_equal(#loaded, 0, "invalid namespace transition must not load an adapter")
	end

	local error_ok, error_result, error_loaded = run_transition({ "user" }, "chosen adapter failed")
	assert_equal(error_ok, false, "namespace transition propagates adapter errors")
	assert_contains(error_result, "chosen adapter failed", "namespace transition adapter error")
	assert_sequence(error_loaded, { hypr_root .. "/user/hyprland.lua" }, "namespace transition failed adapter path")
end

test_user_namespace_transition()

local function make_adapter(upstream_error)
	local bind_calls = {}
	local config_calls = {}
	local define_submap_calls = {}
	local gesture_calls = {}
	local env_calls = {}
	local unbind_calls = {}
	local active_scope = "global"
	local original_bind = function(keys, dispatcher, options)
		table.insert(bind_calls, { scope = active_scope, keys = keys, dispatcher = dispatcher, options = options })
	end
	local original_config = function(config)
		table.insert(config_calls, config)
		return "config-result"
	end
	local original_gesture = function(gesture)
		table.insert(gesture_calls, gesture)
	end
	local original_env = function(name, value)
		table.insert(env_calls, name .. "=" .. tostring(value))
		return "env-result"
	end
	local original_exec_cmd = function(command)
		return { kind = "exec", command = command }
	end
	local original_unbind = function(keys)
		table.insert(unbind_calls, { scope = active_scope, keys = keys })
	end
	local original_define_submap = function(name, callback)
		table.insert(define_submap_calls, name)
		local previous_scope = active_scope
		active_scope = "submap:" .. tostring(name)
		local ok, result = pcall(callback)
		active_scope = previous_scope
		if not ok then
			error(result, 0)
		end
		return "submap-result"
	end

	local fake_hl = {
		bind = original_bind,
		config = original_config,
		define_submap = original_define_submap,
		dsp = { exec_cmd = original_exec_cmd },
		gesture = original_gesture,
		env = original_env,
		unbind = original_unbind,
	}
	local saved = {
		hyprland = {},
		["hyprland.env"] = {},
		["hyprland/cache"] = {},
		custom = {},
		["custom.rules"] = {},
		["custom/cache"] = {},
		workspaces = {},
		monitors = {},
	}
	local unrelated = {
		[17] = {},
		["lib.wahrwelt"] = {},
		["shell-common-rules"] = {},
	}
	local loaded = {}
	for name, value in pairs(saved) do
		loaded[name] = value
	end
	for name, value in pairs(unrelated) do
		loaded[name] = value
	end

	local fake_package = {
		path = "canonical-package-path",
		loaded = loaded,
	}
	local upstream_config = {
		input = { kb_layout = "us" },
		gestures = { workspace_swipe_distance = 700 },
		general = { gaps_in = 4 },
		misc = { vrr = 2 },
	}
	local non_string_keys = { kind = "opaque-keys" }
	local dofile_calls = {}
	local environment = {
		package = fake_package,
		hl = fake_hl,
		os = {
			getenv = function(name)
				if name == "HOME" then
					return "/home/test"
				elseif name == "XDG_CONFIG_HOME" then
					return "/config/test"
				end
				return nil
			end,
		},
		dofile = function(path)
			table.insert(dofile_calls, path)
			assert_equal(
				fake_package.path,
				"/config/test/hypr/end4/?.lua;/config/test/hypr/end4/?/init.lua;canonical-package-path",
				"End4 package.path"
			)
			for name in pairs(saved) do
				assert_equal(loaded[name], nil, "canonical cache isolated: " .. name)
			end
			loaded["hyprland.env"] = "upstream env"
			loaded["hyprland.new"] = "upstream module"
			loaded["hyprland/new"] = "upstream slash module"
			loaded["custom.new"] = "upstream custom module"
			loaded["custom/new"] = "upstream custom slash module"
			loaded.workspaces = "upstream workspaces"
			loaded.monitors = "upstream monitors"
			loaded["upstream.unrelated"] = "keep"
			fake_hl.config(upstream_config)
			fake_hl.gesture({ fingers = 4 })
			fake_hl.env("qsConfig", "wrong")
			fake_hl.env("XDG_DATA_DIRS", "/upstream/share")
			fake_hl.bind("SUPER + Q", { kind = "upstream-close-primary" })
			fake_hl.bind("SUPER + Q", { kind = "upstream-close-fallback" })
			fake_hl.define_submap("test-submap", function()
				fake_hl.bind("SUPER + Q", { kind = "upstream-submap-primary" })
				fake_hl.bind("SUPER + Q", { kind = "upstream-submap-secondary" })
			end)
			fake_hl.bind(non_string_keys, { kind = "opaque-dispatcher" })
			fake_hl.bind = function() end
			fake_hl.define_submap = function() end
			fake_hl.unbind = function() end
			fake_hl.dsp.exec_cmd = function() end
			if upstream_error then
				error("upstream load failed")
			end
		end,
	}
	local adapter = load_module(source_root .. "/end4-adapter.lua", environment)
	return {
		adapter = adapter,
		bind_calls = bind_calls,
		config_calls = config_calls,
		define_submap_calls = define_submap_calls,
		dofile_calls = dofile_calls,
		env_calls = env_calls,
		gesture_calls = gesture_calls,
		hl = fake_hl,
		loaded = loaded,
		original_bind = original_bind,
		original_config = original_config,
		original_define_submap = original_define_submap,
		original_env = original_env,
		original_exec_cmd = original_exec_cmd,
		original_gesture = original_gesture,
		original_unbind = original_unbind,
		package = fake_package,
		non_string_keys = non_string_keys,
		saved = saved,
		unrelated = unrelated,
		unbind_calls = unbind_calls,
		upstream_config = upstream_config,
	}
end

local function assert_adapter_cleanup(state)
	assert_equal(state.package.path, "canonical-package-path", "adapter restores package.path")
	assert_same(state.hl.bind, state.original_bind, "adapter preserves hl.bind")
	assert_same(state.hl.config, state.original_config, "adapter restores hl.config")
	assert_same(state.hl.define_submap, state.original_define_submap, "adapter restores hl.define_submap")
	assert_same(state.hl.dsp.exec_cmd, state.original_exec_cmd, "adapter preserves hl.dsp.exec_cmd")
	assert_same(state.hl.gesture, state.original_gesture, "adapter restores hl.gesture")
	assert_same(state.hl.env, state.original_env, "adapter restores hl.env")
	assert_same(state.hl.unbind, state.original_unbind, "adapter preserves hl.unbind")
	for name, value in pairs(state.saved) do
		assert_same(state.loaded[name], value, "adapter restores cached module " .. name)
	end
	for name, value in pairs(state.unrelated) do
		assert_same(state.loaded[name], value, "adapter preserves unrelated module " .. name)
	end
	for _, name in ipairs({
		"hyprland.new",
		"hyprland/new",
		"custom.new",
		"custom/new",
	}) do
		assert_equal(state.loaded[name], nil, "adapter removes upstream collision " .. name)
	end
	if #state.dofile_calls > 0 then
		assert_equal(state.loaded["upstream.unrelated"], "keep", "adapter preserves newly loaded unrelated module")
	else
		assert_equal(state.loaded["upstream.unrelated"], nil, "validation must not load unrelated modules")
	end
end

local function assert_upstream_binding_replacement(state, managed_restart)
	local expected_unbind_count = managed_restart and 3 or 2
	assert_equal(#state.unbind_calls, expected_unbind_count, "End4 replacement unbind count")
	assert_equal(state.unbind_calls[1].scope, "global", "global replacement scope")
	assert_equal(state.unbind_calls[1].keys, "SUPER + Q", "global replacement keys")
	assert_equal(state.unbind_calls[2].scope, "submap:test-submap", "submap replacement scope")
	assert_equal(state.unbind_calls[2].keys, "SUPER + Q", "submap replacement keys")

	local expected_bind_count = managed_restart and 6 or 5
	assert_equal(#state.bind_calls, expected_bind_count, "End4 forwarded bind count")
	for index = 1, 2 do
		assert_equal(state.bind_calls[index].scope, "global", "global upstream bind scope")
		assert_equal(state.bind_calls[index].keys, "SUPER + Q", "global upstream bind keys")
	end
	for index = 3, 4 do
		assert_equal(state.bind_calls[index].scope, "submap:test-submap", "submap upstream bind scope")
		assert_equal(state.bind_calls[index].keys, "SUPER + Q", "submap upstream bind keys")
	end
	assert_same(state.bind_calls[5].keys, state.non_string_keys, "non-string bind keys pass through unchanged")
	assert_equal(#state.define_submap_calls, 1, "End4 define_submap call count")
	assert_equal(state.define_submap_calls[1], "test-submap", "End4 define_submap name")
end

local function assert_managed_restart(state, profile)
	assert_upstream_binding_replacement(state, true)
	local unbind = state.unbind_calls[3]
	assert_equal(unbind.scope, "global", profile .. " restart unbind scope")
	assert_equal(unbind.keys, "CTRL + SUPER + R", profile .. " restart unbind")
	local binding = state.bind_calls[6]
	assert_equal(binding.keys, "CTRL + SUPER + R", profile .. " restart keys")
	assert_equal(binding.dispatcher.kind, "exec", profile .. " restart dispatcher")
	assert_equal(
		binding.dispatcher.command,
		"'/config/test/hypr/scripts/start-shell.sh' " .. profile,
		profile .. " managed restart command"
	)
	assert_equal(binding.options.description, "Shell: Restart widgets", profile .. " restart description")
end

local success = make_adapter(false)
success.adapter.load({ profile = "end4-pc", quickshell_config = "/config/test/quickshell/end4-pC" })
assert_sequence(success.dofile_calls, { "/config/test/hypr/end4/hyprland.lua" }, "End4 entrypoint")
assert_adapter_cleanup(success)
assert_equal(#success.config_calls, 1, "filtered End4 config call count")
local filtered = success.config_calls[1]
assert_equal(filtered.input, nil, "End4 input must be filtered")
assert_equal(filtered.gestures, nil, "End4 gestures config must be filtered")
assert_same(filtered.general, success.upstream_config.general, "End4 general config must be forwarded")
assert_same(filtered.misc, success.upstream_config.misc, "End4 misc config must be forwarded")
assert_same(success.upstream_config.input.kb_layout, "us", "adapter must not mutate upstream config")
assert_equal(#success.gesture_calls, 0, "End4 gesture calls must be ignored")
assert_sequence(success.env_calls, {
	"qsConfig=/config/test/quickshell/end4-pC",
	"XDG_DATA_DIRS=/upstream/share",
	"qsConfig=/config/test/quickshell/end4-pC",
}, "End4 environment forwarding")
assert_managed_restart(success, "end4-pc")

local official = make_adapter(false)
official.adapter.load({ profile = "end4", quickshell_config = "/config/test/quickshell/ii" })
assert_adapter_cleanup(official)
assert_managed_restart(official, "end4")

local failed = make_adapter(true)
local load_ok, load_error = pcall(failed.adapter.load, {
	profile = "end4",
	quickshell_config = "/config/test/quickshell/ii",
})
assert_equal(load_ok, false, "End4 upstream error")
assert_contains(load_error, "upstream load failed", "End4 upstream traceback")
assert_adapter_cleanup(failed)
assert_sequence(failed.env_calls, {
	"qsConfig=/config/test/quickshell/ii",
	"XDG_DATA_DIRS=/upstream/share",
}, "End4 error environment forwarding")
assert_upstream_binding_replacement(failed, false)
assert_equal(#failed.unbind_calls, 2, "failed End4 load must not replace restart binding")
assert_equal(#failed.bind_calls, 5, "failed End4 load must not install restart binding")

local invalid = make_adapter(false)
local invalid_ok, invalid_error = pcall(invalid.adapter.load, {
	profile = "other",
	quickshell_config = "relative/path",
})
assert_equal(invalid_ok, false, "End4 invalid args")
assert_contains(invalid_error, "profile must be end4 or end4-pc", "End4 invalid profile")
assert_equal(#invalid.dofile_calls, 0, "invalid args must not load End4")
assert_equal(#invalid.env_calls, 0, "invalid args must not mutate environment")
assert_adapter_cleanup(invalid)

print("lua contract tests: ok")
