// Package v1_to_v2 contains read-only recognizers for the one-way v1 to v2
// installation upgrade. It must not own filesystem mutation or fresh runtime
// behavior.
package v1_to_v2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	LegacyWahrweltStatePath = "/etc/nixos/wahrwelt/state.json"
	LegacyMySetupStatePath  = "/etc/nixos/mysetup/state.json"

	LegacyWahrweltNamespace = "wahrwelt"
	LegacyMySetupNamespace  = "mysetup"
	CanonicalUserNamespace  = "user"
)

type EntrypointKind uint8

type PathMove struct {
	Source string
	Target string
}

const (
	EntrypointUnknown EntrypointKind = iota
	EntrypointLegacyUser
	EntrypointHomeManagerSeededUser
	EntrypointNamespaceTransition
	EntrypointDirectEnd4
	EntrypointDirectEnd4PC
)

var historicalManagedHyprEntrypointDigests = map[[sha256.Size]byte]struct{}{
	mustDigest("cecf44b96c7afd4886d498abe0de382b2574c66281a5cf78bbac06586c1b071c"): {},
	mustDigest("e28d16bde1d68fa2fa43c755630284f00b3c6a14f75656e89cfb5514f8633263"): {},
	mustDigest("18c3eb7f48101e0bd0b57918a683778784c74c833a215af7f7b0f1d416a0a5df"): {},
	mustDigest("24229642cd871aa3eb3d27c44b0d72357395951aec076a09d173b45ca17231a0"): {},
	mustDigest("1d8e001faf0c6078a7d9a34e4c592fcb523afd817d2ff56099c7b2fe16407506"): {},
	mustDigest("a547d710e9fd13ca8829e17caa378a14ee9d6a0d114426731e0ab363e9328118"): {},
	mustDigest("3666c398dbba460e9b3dac54f396a7f53ad2093f49967c05e4588e66c41f08eb"): {},
}

var historicalDirectEnd4AssetDigests = map[string]map[[sha256.Size]byte]struct{}{
	"hyprland.lua": {
		mustDigest("24229642cd871aa3eb3d27c44b0d72357395951aec076a09d173b45ca17231a0"): {},
	},
	"lib/wahrwelt.lua": {
		mustDigest("5e9d935004de1ca3cff466fad857d8c4576377a14b76f42219b88afd84c933bb"): {},
	},
	"hyprland/keybinds.lua": {
		mustDigest("d2fddcdfb7a6bfa0ee78ad001671498bb70ccae0498125e0979d2017aaffceab"): {},
	},
	"hyprland/rules.lua": {
		mustDigest("555ca50800228be02331e08a6a0a59f79bb5cf410ffa7860e13970f89306b4d2"): {},
	},
	"scripts/start-shell.sh": {
		mustDigest("0111a0fa50477b20452ecb3d11dcf749afb5c91bf65b562cd398fb6c034c1c56"): {},
	},
	"scripts/shell-runtime.sh": {
		mustDigest("3ce850afb7e88ff9916fa506ce8315cc3b8d2a6ecd2d5197f2c0cee6443c5ee1"): {},
	},
	"scripts/shell-runtime-env.sh": {
		mustDigest("59f97d24ddc727b4bbee44229570aa75406d7dca7234a825ec296d6d400e1501"): {},
	},
	"scripts/shell-profile-sync.sh": {
		mustDigest("a0623406522ad1fa29a6178e8fab829a764f0052e5b353960ca0b1d6748cf780"): {},
	},
	"scripts/shell-process.sh": {
		mustDigest("cf6eb94d4e8cb3db695e6d7854a69b085ba956debd66ac535041ff9fa4ca761f"): {},
	},
	"scripts/restore-lock.sh": {
		mustDigest("352fcf918656c47996d5279a1a714bf59aaaeb989dfae21b24896bf869c88681"): {},
	},
	"scripts/shell-selector.sh": {
		mustDigest("10f22897ef5d5b470196c00be7bbbf02f5a236ac65d1f44edfe4d288f1da01b4"): {},
	},
	"scripts/record-toggle.sh": {
		mustDigest("7d48d039280736b4ad482857e6aeba643ad28de68eb21cb489b548843298d1d3"): {},
	},
	"scripts/noctalia-launcher.sh": {
		mustDigest("fc6575dc117a30d755e64459bca14f3f5885d78fd38451bfe7c9815c464e64dc"): {},
	},
	"scripts/close-active.sh": {
		mustDigest("2f1667514bb97840ebeab4310d26d1c6af5ac1e9d51ca27b31d9eb4522f004b5"): {},
	},
}

func mustDigest(value string) [sha256.Size]byte {
	var digest [sha256.Size]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		panic("invalid v1_to_v2 digest: " + value)
	}
	copy(digest[:], decoded)
	return digest
}

func LegacyDraftPath(stateHome string) string {
	return filepath.Join(stateHome, LegacyMySetupNamespace, "draft.json")
}

func LegacyActiveShellStatePath(stateHome string) string {
	return filepath.Join(stateHome, LegacyMySetupNamespace, "active-shell")
}

func LegacyHyprUserDirectories(hyprDir string) []string {
	return []string{
		filepath.Join(hyprDir, LegacyWahrweltNamespace),
		filepath.Join(hyprDir, LegacyMySetupNamespace),
	}
}

func LegacyNamespaceMoves(configHome, stateHome string) []PathMove {
	return []PathMove{
		{
			Source: filepath.Join(configHome, LegacyMySetupNamespace),
			Target: filepath.Join(configHome, LegacyWahrweltNamespace),
		},
		{
			Source: filepath.Join(stateHome, LegacyMySetupNamespace),
			Target: filepath.Join(stateHome, LegacyWahrweltNamespace),
		},
	}
}

func LegacyManagedLinks(configHome string) []string {
	return []string{
		filepath.Join(configHome, "hypr", "lib", "mysetup.lua"),
		filepath.Join(configHome, "quickshell", "mysetup-shell-selector"),
	}
}

func LegacyCacheMove(cacheHome string) PathMove {
	return PathMove{
		Source: filepath.Join(cacheHome, LegacyMySetupNamespace),
		Target: filepath.Join(cacheHome, LegacyWahrweltNamespace),
	}
}

func LegacyInstallPaths(nixosDest, configHome, stateHome, cacheHome string) []string {
	return []string{
		filepath.Join(nixosDest, LegacyMySetupNamespace),
		filepath.Join(nixosDest, "private"),
		filepath.Join(nixosDest, LegacyWahrweltNamespace, "state.json"),
		filepath.Join(configHome, LegacyMySetupNamespace),
		filepath.Join(configHome, "hypr", LegacyMySetupNamespace),
		filepath.Join(configHome, "hypr", LegacyWahrweltNamespace),
		filepath.Join(configHome, "hypr", "lib", "mysetup.lua"),
		filepath.Join(configHome, "quickshell", "mysetup-shell-selector"),
		filepath.Join(stateHome, LegacyMySetupNamespace),
		filepath.Join(cacheHome, LegacyMySetupNamespace),
	}
}

func RewritePrivateUserPathToken(token string) string {
	const (
		legacyPrefix    = "./private/"
		canonicalPrefix = "./user/"
	)
	switch {
	case token == "./private":
		return "./user"
	case strings.HasPrefix(token, legacyPrefix):
		return canonicalPrefix + strings.TrimPrefix(token, legacyPrefix)
	default:
		return token
	}
}

func RecognizeEntrypoint(content, defaultProfile string) EntrypointKind {
	switch {
	case content == UserNamespaceTransitionEntrypoint():
		return EntrypointNamespaceTransition
	case content == LegacyUserEntrypoint() || content == LegacyHomeManagerUserEntrypoint(defaultProfile):
		return EntrypointLegacyUser
	case content == HistoricalHomeManagerSeededUserEntrypoint(defaultProfile, LegacyWahrweltNamespace),
		content == HistoricalHomeManagerSeededUserEntrypoint(defaultProfile, CanonicalUserNamespace):
		return EntrypointHomeManagerSeededUser
	case content == DirectEnd4Entrypoint("end4"):
		return EntrypointDirectEnd4
	case content == DirectEnd4Entrypoint("end4-pc"):
		return EntrypointDirectEnd4PC
	default:
		return EntrypointUnknown
	}
}

func IsHistoricalManagedHyprEntrypointDigest(digest [sha256.Size]byte) bool {
	_, ok := historicalManagedHyprEntrypointDigests[digest]
	return ok
}

func IsHistoricalDirectEnd4Asset(sourceRel string, content []byte) bool {
	digests := historicalDirectEnd4AssetDigests[sourceRel]
	_, ok := digests[sha256.Sum256(content)]
	return ok
}

func LegacyUserEntrypoint() string {
	return `-- Wahrwelt canonical Hyprland runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/wahrwelt/hyprland.lua")
`
}

func LegacyHomeManagerUserEntrypoint(defaultProfile string) string {
	return fmt.Sprintf(`-- Active Hyprland profile: wahrwelt (%s)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/wahrwelt/hyprland.lua")
`, defaultProfile)
}

func HistoricalHomeManagerSeededUserEntrypoint(defaultProfile, namespace string) string {
	if namespace != LegacyWahrweltNamespace && namespace != CanonicalUserNamespace {
		return ""
	}
	return fmt.Sprintf(`-- Active Hyprland profile: wahrwelt (%s)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local state_home = os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")
local hypr_root = config_home .. "/hypr"
local runtime_root = state_home .. "/wahrwelt/hypr-runtime"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/%s/hyprland.lua")
dofile(runtime_root .. "/shell-profile.lua")
`, defaultProfile, namespace)
}

func UserNamespaceTransitionEntrypoint() string {
	return `-- Wahrwelt Hypr user namespace transition entrypoint
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
`
}

func DirectEnd4Entrypoint(profile string) string {
	if profile != "end4" && profile != "end4-pc" {
		return ""
	}
	return fmt.Sprintf(`-- Active Hyprland profile: %s
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate end4 Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
local end4_root = hypr_root .. "/end4"
package.path = end4_root .. "/?.lua;" .. end4_root .. "/?/init.lua;" .. hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(end4_root .. "/hyprland.lua")
`, profile)
}

func DirectEnd4LauncherPlaceholder(profile string) string {
	if profile != "end4" && profile != "end4-pc" {
		return ""
	}
	return fmt.Sprintf("-- Active shell launcher profile: %s\n-- end4 registers launcher bindings from its own Hyprland Lua modules.\n", profile)
}

func DirectEnd4KeybindsPlaceholder(profile string) string {
	if profile != "end4" && profile != "end4-pc" {
		return ""
	}
	return fmt.Sprintf("-- Active shell keybind profile: %s\n-- end4 registers keybinds from its own Hyprland Lua modules.\n", profile)
}
