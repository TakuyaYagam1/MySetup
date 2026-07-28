package config

const (
	SourceChannelStable      = "stable"
	SourceChannelDevelopment = "development"

	NoctaliaVersionV4 = "v4"
	NoctaliaVersionV5 = "v5"

	legacyNoctaliaV4WahrweltFlakeURL    = "github:TakuyaYagam1/wahrwelt/noctalia-v4?dir=Linux/NixOS"
	legacyNoctaliaV4MySetupRepoFlakeURL = "github:TakuyaYagam1/MySetup/noctalia-v4?dir=Linux/NixOS"
)

var SourceChannels = []string{
	SourceChannelStable,
	SourceChannelDevelopment,
}

var NoctaliaVersions = []string{
	NoctaliaVersionV5,
	NoctaliaVersionV4,
}

var PackagePresets = []string{
	"personal",
	"minimal",
	"desktop",
	"developer",
}

var GPUProfiles = []string{
	"amd",
	"intel",
	"nvidia",
	"other",
}

var KeyboardToggles = []string{
	"grp:alt_shift_toggle",
	"grp:win_space_toggle",
	"grp:ctrl_shift_toggle",
	"grp:caps_toggle",
}

func IsSourceChannel(value string) bool {
	return oneOf(value, SourceChannels...)
}

func IsNoctaliaVersion(value string) bool {
	return oneOf(value, NoctaliaVersions...)
}

func WahrweltFlakeURL(channel string) string {
	switch channel {
	case SourceChannelDevelopment:
		return "github:TakuyaYagam1/wahrwelt/dev?dir=Linux/NixOS"
	default:
		return "github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS"
	}
}

func KnownWahrweltFlakeURLs() []string {
	return []string{
		WahrweltFlakeURL(SourceChannelStable),
		"github:TakuyaYagam1/wahrwelt?dir=Linux/NixOS",
		WahrweltFlakeURL(SourceChannelDevelopment),
		legacyNoctaliaV4WahrweltFlakeURL,
		"github:TakuyaYagam1/MySetup/main?dir=Linux/NixOS",
		"github:TakuyaYagam1/MySetup?dir=Linux/NixOS",
		"github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS",
		legacyNoctaliaV4MySetupRepoFlakeURL,
	}
}

func IsPackagePreset(value string) bool {
	return oneOf(value, PackagePresets...)
}

func IsGPUProfile(value string) bool {
	return oneOf(value, GPUProfiles...)
}

func IsKeyboardToggle(value string) bool {
	return oneOf(value, KeyboardToggles...)
}
