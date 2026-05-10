package config

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
