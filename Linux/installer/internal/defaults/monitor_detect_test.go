package defaults

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

type fakeRunner struct {
	output string
	err    error
	calls  [][]string
}

func (f *fakeRunner) Command(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.err
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.output, f.err
}

func (f *fakeRunner) IsDryRun() bool { return false }

func TestDetectMonitorsParsesFocusedFirst(t *testing.T) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		t.Skip("hyprctl unavailable, skipping detection test")
	}
	runner := &fakeRunner{output: `[
		{"name":"HDMI-A-1","width":1920,"height":1080,"refreshRate":60.0,"x":2560,"y":0,"scale":1.0,"focused":false,"disabled":false},
		{"name":"eDP-1","width":2560,"height":1600,"refreshRate":120.0,"x":0,"y":0,"scale":1.0,"focused":true,"disabled":false},
		{"name":"DP-2","width":1920,"height":1080,"refreshRate":144.0,"x":4480,"y":0,"scale":1.25,"focused":false,"disabled":true}
	]`}
	got, err := DetectMonitors(context.Background(), runner)
	if err != nil {
		t.Fatalf("DetectMonitors: %v", err)
	}
	want := []config.Monitor{
		{Name: "eDP-1", Mode: "preferred", Position: "0x0", Scale: "1"},
		{Name: "HDMI-A-1", Mode: "preferred", Position: "2560x0", Scale: "1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected focused-first ordering and disabled to be skipped, got %#v", got)
	}
}

func TestDetectMonitorsHandlesUnreachableSession(t *testing.T) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		t.Skip("hyprctl unavailable, skipping detection test")
	}
	runner := &fakeRunner{err: errors.New("hyprctl: not running")}
	got, err := DetectMonitors(context.Background(), runner)
	if err != nil {
		t.Fatalf("unreachable session must not surface as error, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil monitors when hyprctl errors, got %#v", got)
	}
}

func TestDetectMonitorsHandlesEmptyOutput(t *testing.T) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		t.Skip("hyprctl unavailable, skipping detection test")
	}
	runner := &fakeRunner{output: "[]"}
	got, err := DetectMonitors(context.Background(), runner)
	if err != nil {
		t.Fatalf("empty list must not error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil monitors for empty list, got %#v", got)
	}
}

func TestDetectMonitorsRejectsMalformedJSON(t *testing.T) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		t.Skip("hyprctl unavailable, skipping detection test")
	}
	runner := &fakeRunner{output: "not json"}
	_, err := DetectMonitors(context.Background(), runner)
	if err == nil {
		t.Fatal("expected parse error for malformed hyprctl output")
	}
}

func TestFormatScalePreservesPrecision(t *testing.T) {
	cases := map[float64]string{
		1.0:  "1",
		1.25: "1.25",
		1.5:  "1.5",
		0:    "1",
	}
	for in, want := range cases {
		if got := formatScale(in); got != want {
			t.Fatalf("formatScale(%v) = %q, want %q", in, got, want)
		}
	}
}
