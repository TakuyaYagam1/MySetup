{ config, lib, pkgs-stable, ... }:

# Cloud security: AWS/Azure pentesting, container/SBOM scanning, k8s runtime.

{
  config = lib.mkIf (config.var.features.ctfTools or false) {
  environment.systemPackages = with pkgs-stable; [
    azurehound
    grype
    hubble
    osv-scanner
    pacu
    syft
    tetragon
    trivy
  ];
  };
}
