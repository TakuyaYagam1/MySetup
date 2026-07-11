package tui

import (
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func TestServiceFieldsRendersOmniRouterAndObservability(t *testing.T) {
	s := &session{state: config.Default()}
	if got := len(serviceFields(s)); got != 2 {
		t.Fatalf("expected OmniRouter and Observability toggles, got %d fields", got)
	}
}
