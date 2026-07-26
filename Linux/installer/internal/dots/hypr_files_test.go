package dots

import (
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

func TestRenderHyprLocalLuaWritesMachineOverrides(t *testing.T) {
	state := config.Default()
	state.Display.MonitorName = "eDP-1"
	state.Display.MonitorMode = "2560x1600@120"
	state.Display.MonitorPosition = "0x0"
	state.Display.MonitorScale = "1"
	state.Display.ExtraMonitors = []config.Monitor{{
		Name:     "HDMI-A-1",
		Mode:     "preferred",
		Position: "2560x0",
		Scale:    "1.25",
	}}
	state.Locale.KeyboardLayouts = "us,ru"
	state.Locale.KeyboardToggle = "grp:alt_shift_toggle"

	got := renderHyprLocalLua(state)
	for _, want := range []string{
		`hl.monitor({ output = "eDP-1", mode = "2560x1600@120", position = "0x0", scale = "1" })`,
		`hl.monitor({ output = "", mode = "preferred", position = "auto", scale = "1.25" })`,
		`kb_layout = "us,ru"`,
		`kb_options = "grp:alt_shift_toggle"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated local Lua missing %q\n%s", want, got)
		}
	}
}

func TestRenderLuaMonitorDefaultsAutoModes(t *testing.T) {
	got := renderLuaMonitor(config.Monitor{Scale: "1"})
	want := `hl.monitor({ output = "", mode = "preferred", position = "auto", scale = "1" })`
	if !strings.Contains(got, want) {
		t.Fatalf("expected auto monitor form %q, got:\n%s", want, got)
	}
}
