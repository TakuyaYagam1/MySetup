{ pkgs-stable, ... }:

# Cloud security: AWS/Azure pentesting, container/SBOM scanning, k8s runtime.

{
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
}
