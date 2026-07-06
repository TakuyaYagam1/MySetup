package config

const (
	SourceChannelStable      = "stable"
	SourceChannelDevelopment = "development"

	NoctaliaVersionV4 = "v4"
	NoctaliaVersionV5 = "v5"

	legacyNoctaliaV4MySetupFlakeURL = "github:TakuyaYagam1/MySetup/noctalia-v4?dir=Linux/NixOS"
	noctaliaFlakeURLV5              = "github:noctalia-dev/noctalia/main"
	noctaliaShellFlakeURLV4         = "github:noctalia-dev/noctalia-shell/v4.7.7"
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

var ZapretConfigs = []string{
	"general",
	"general (FAKE_TLS_AUTO)",
	"general (FAKE_TLS_AUTO_ALT)",
	"general (FAKE_TLS_AUTO_ALT2)",
	"general (FAKE_TLS_AUTO_ALT3)",
	"general (SIMPLE FAKE)",
	"general (SIMPLE FAKE ALT)",
	"general (SIMPLE_FAKE_ALT2)",
	"general(ALT)",
	"general(ALT2)",
	"general(ALT3)",
	"general(ALT4)",
	"general(ALT5)",
	"general(ALT6)",
	"general(ALT7)",
	"general(ALT8)",
	"general(ALT9)",
	"general(ALT10)",
	"general(ALT11)",
}

func IsSourceChannel(value string) bool {
	return oneOf(value, SourceChannels...)
}

func IsNoctaliaVersion(value string) bool {
	return oneOf(value, NoctaliaVersions...)
}

func MySetupFlakeURL(channel string) string {
	switch channel {
	case SourceChannelDevelopment:
		return "github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS"
	default:
		return "github:TakuyaYagam1/MySetup/main?dir=Linux/NixOS"
	}
}

func KnownMySetupFlakeURLs() []string {
	return []string{
		MySetupFlakeURL(SourceChannelStable),
		"github:TakuyaYagam1/MySetup?dir=Linux/NixOS",
		MySetupFlakeURL(SourceChannelDevelopment),
		legacyNoctaliaV4MySetupFlakeURL,
	}
}

func NoctaliaV5FlakeURL() string {
	return noctaliaFlakeURLV5
}

func NoctaliaV4FlakeURL() string {
	return noctaliaShellFlakeURLV4
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

func IsZapretConfig(value string) bool {
	return oneOf(value, ZapretConfigs...)
}
