package tui

import (
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func TestServiceFieldsRendersOmniRouterPortainerAndObservability(t *testing.T) {
	s := &session{state: config.Default()}
	if got := len(serviceFields(s)); got != 3 {
		t.Fatalf("expected OmniRouter, Portainer and Observability toggles, got %d fields", got)
	}
}
