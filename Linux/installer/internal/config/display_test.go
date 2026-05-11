package config

import (
	"reflect"
	"testing"
)

func TestMonitorLinesPrimaryOnly(t *testing.T) {
	state := Default()
	got := state.Display.MonitorLines()
	want := []string{"monitor = ,preferred,auto,1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected single preferred line, got %#v", got)
	}
}

func TestMonitorLinesWithExtras(t *testing.T) {
	state := Default()
	state.Display.MonitorName = "eDP-1"
	state.Display.MonitorMode = "2560x1600@120"
	state.Display.MonitorPosition = "0x0"
	state.Display.MonitorScale = "1"
	state.Display.ExtraMonitors = []Monitor{
		{Name: "HDMI-A-1", Mode: "preferred", Position: "2560x0", Scale: "1"},
		{Name: "DP-2", Mode: "1920x1080@144", Position: "4480x0", Scale: "1.25"},
	}
	got := state.Display.MonitorLines()
	want := []string{
		"monitor = eDP-1, 2560x1600@120, 0x0, 1",
		"monitor = ,preferred,auto,1",
		"monitor = DP-2, 1920x1080@144, 4480x0, 1.25",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected primary + 2 extras, got %#v", got)
	}
}

func TestMonitorLineFromMonitorStruct(t *testing.T) {
	cases := []struct {
		name    string
		monitor Monitor
		want    string
	}{
		{
			name:    "preferred sentinel collapses",
			monitor: Monitor{Name: "DP-1", Mode: "preferred", Position: "0x0", Scale: "1"},
			want:    ",preferred,auto,1",
		},
		{
			name:    "auto sentinel collapses",
			monitor: Monitor{Name: "DP-1", Mode: "auto", Position: "0x0", Scale: "1.5"},
			want:    ",auto,auto,1.5",
		},
		{
			name:    "explicit mode preserves all four fields",
			monitor: Monitor{Name: "eDP-1", Mode: "1920x1080@144", Position: "0x0", Scale: "1"},
			want:    "eDP-1, 1920x1080@144, 0x0, 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.monitor.MonitorLine(); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestDisplayMonitorLineMatchesPrimaryMonitor(t *testing.T) {
	display := Display{
		MonitorName:     "eDP-1",
		MonitorMode:     "preferred",
		MonitorPosition: "0x0",
		MonitorScale:    "1",
	}
	want := Monitor{
		Name:     display.MonitorName,
		Mode:     display.MonitorMode,
		Position: display.MonitorPosition,
		Scale:    display.MonitorScale,
	}.MonitorLine()
	if got := display.MonitorLine(); got != want {
		t.Fatalf("Display.MonitorLine() must mirror primary Monitor.MonitorLine(): got %q want %q", got, want)
	}
}
