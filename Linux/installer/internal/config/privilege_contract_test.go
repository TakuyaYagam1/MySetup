package config

import (
	"os"
	"strings"
	"testing"
)

func readPrivilegeContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestPrimaryUserDoesNotReceiveSilentRootCapabilities(t *testing.T) {
	settings := readPrivilegeContractFile(t, "../../../NixOS/system/settings.nix")
	for _, forbidden := range []string{
		`"@wheel"`,
		"config.wahrwelt.user.username",
	} {
		if strings.Contains(settings, forbidden) {
			t.Fatalf("Nix trusted-users retains %q\n%s", forbidden, settings)
		}
	}

	security := readPrivilegeContractFile(t, "../../../NixOS/system/security.nix")
	if strings.Contains(security, "NOPASSWD") || strings.Contains(security, "extraRules") {
		t.Fatalf("sudo policy retains an unrestricted passwordless root path\n%s", security)
	}

	user := readPrivilegeContractFile(t, "../../../NixOS/users/user.nix")
	if strings.Contains(user, `"docker"`) {
		t.Fatalf("primary user remains root-equivalent through the Docker group\n%s", user)
	}
}

func TestDeveloperContainersUseRootlessPodmanCompatibility(t *testing.T) {
	virtualization := readPrivilegeContractFile(t, "../../../NixOS/services/virtualization.nix")
	for _, required := range []string{
		"enable = lib.mkDefault false;",
		"dockerCompat = !config.services.portainer.enable;",
		"dockerSocket.enable = false;",
		`"unix://$XDG_RUNTIME_DIR/podman/podman.sock"`,
	} {
		if !strings.Contains(virtualization, required) {
			t.Fatalf("rootless container contract is missing %q\n%s", required, virtualization)
		}
	}
}
