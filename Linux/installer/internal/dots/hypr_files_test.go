package dots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceBlockIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hyprland.conf")

	initial := strings.Join([]string{
		"# preamble",
		"$hypr = ~/.config/hypr",
		"# >>> MYSETUP MONITORS",
		"monitor = ,preferred,auto,1",
		"# <<< MYSETUP MONITORS",
		"source = $hl/general.conf",
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	first := []string{
		"monitor = eDP-1, 2560x1600@120, 0x0, 1",
		"monitor = HDMI-A-1, preferred, 2560x0, 1",
	}
	if err := replaceBlock(path, monitorBlockBegin, monitorBlockEnd, first); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "monitor = eDP-1, 2560x1600@120, 0x0, 1\nmonitor = HDMI-A-1, preferred, 2560x0, 1") {
		t.Fatalf("expected first body inside markers, got:\n%s", contents)
	}
	if strings.Contains(string(contents), "monitor = ,preferred,auto,1") {
		t.Fatalf("original line should have been replaced, got:\n%s", contents)
	}
	if !strings.Contains(string(contents), "source = $hl/general.conf") {
		t.Fatalf("trailing content must be preserved:\n%s", contents)
	}

	second := []string{"monitor = ,preferred,auto,1.5"}
	if err := replaceBlock(path, monitorBlockBegin, monitorBlockEnd, second); err != nil {
		t.Fatal(err)
	}

	contents, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(contents), monitorBlockBegin) != 1 {
		t.Fatalf("expected exactly one begin marker after second write:\n%s", contents)
	}
	if strings.Count(string(contents), monitorBlockEnd) != 1 {
		t.Fatalf("expected exactly one end marker after second write:\n%s", contents)
	}
	if !strings.Contains(string(contents), "monitor = ,preferred,auto,1.5") {
		t.Fatalf("expected second body to land between markers, got:\n%s", contents)
	}
	if strings.Contains(string(contents), "monitor = HDMI-A-1") {
		t.Fatalf("first body must be evicted after second write, got:\n%s", contents)
	}
}

func TestReplaceBlockAppendsWhenMarkersMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hyprland.conf")

	if err := os.WriteFile(path, []byte("# old config\nsource = $hl/general.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	body := []string{"monitor = ,preferred,auto,1"}
	if err := replaceBlock(path, monitorBlockBegin, monitorBlockEnd, body); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(contents)
	if !strings.Contains(got, monitorBlockBegin) {
		t.Fatalf("expected begin marker to be appended:\n%s", got)
	}
	if !strings.Contains(got, monitorBlockEnd) {
		t.Fatalf("expected end marker to be appended:\n%s", got)
	}
	if !strings.Contains(got, "monitor = ,preferred,auto,1") {
		t.Fatalf("expected new body inside appended block:\n%s", got)
	}
	if !strings.Contains(got, "source = $hl/general.conf") {
		t.Fatalf("original content must be preserved:\n%s", got)
	}
}
