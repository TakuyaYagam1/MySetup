package apply

import "fmt"

type Layout string

const (
	LayoutThin Layout = "thin"
	LayoutFull Layout = "full"
)

func normalizeLayout(layout Layout) (Layout, error) {
	switch layout {
	case "", LayoutThin:
		return LayoutThin, nil
	case LayoutFull:
		return LayoutFull, nil
	default:
		return "", fmt.Errorf("unknown install layout %q; expected %q or %q", layout, LayoutThin, LayoutFull)
	}
}

func hostVarsRel(layout Layout) string {
	if layout == LayoutFull {
		return "hosts/NixOS/host-vars.nix"
	}
	return "host-vars.nix"
}

func hardwareRel(layout Layout) string {
	if layout == LayoutFull {
		return "hosts/NixOS/hardware-configuration.nix"
	}
	return "hardware-configuration.nix"
}

func hashedPasswordRel(layout Layout) string {
	if layout == LayoutFull {
		return "hosts/NixOS/hashed-password.nix"
	}
	return "hashed-password.nix"
}
