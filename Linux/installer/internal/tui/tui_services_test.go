package tui

import (
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func TestServicesPreviewHidesZapretConfigUntilEnabled(t *testing.T) {
	state := config.Default()
	state.Zapret.Enable = false
	model := sectionModel{cursor: sectionIndex(t, "Services"), state: state}
	preview := strings.Join(sectionPreview(model), "\n")
	if strings.Contains(preview, "## Zapret config") {
		t.Fatalf("disabled zapret must hide config from services preview, got:\n%s", preview)
	}

	state.Zapret.Enable = true
	model.state = state
	preview = strings.Join(sectionPreview(model), "\n")
	if !strings.Contains(preview, "## Zapret config") {
		t.Fatalf("enabled zapret must show config in services preview, got:\n%s", preview)
	}
}

func TestServiceFieldsHideZapretConfigUntilEnabled(t *testing.T) {
	disabled := &session{state: config.Default()}
	disabled.state.Zapret.Enable = false
	if got := len(serviceFields(disabled)); got != 3 {
		t.Fatalf("disabled zapret should render OmniRouter, Observability and Zapret toggles, got %d fields", got)
	}

	enabled := &session{state: config.Default()}
	enabled.state.Zapret.Enable = true
	if got := len(serviceFields(enabled)); got != 4 {
		t.Fatalf("enabled zapret should render OmniRouter, Observability, Zapret and config fields, got %d fields", got)
	}
}

func TestZapretConfigOptionsExposeCurrentTopLevelUpstreamChoices(t *testing.T) {
	names := zapretConfigNames()
	want := []string{
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
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected zapret config names\ngot:  %v\nwant: %v", names, want)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "old configs/") {
			t.Fatalf("legacy zapret configs should not be shown in the default selector: %v", names)
		}
	}
}

func TestZapretConfigOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := zapretConfigOptions("custom")
	if len(options) == 0 || options[0].Value != "custom" {
		t.Fatalf("expected custom current zapret config to be first option, got %#v", options)
	}
}
