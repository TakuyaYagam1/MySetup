{ pkgs, ... }:

{
  networking = {
    hostName = "NixOS";
    
    networkmanager = {
      enable = true;
      wifi = {
        powersave = false;
        macAddress = "preserve";
      };
      plugins = with pkgs; [
        networkmanager-openvpn
        networkmanager-openconnect
      ];
    };
    
    nameservers = [ "8.8.8.8" "1.1.1.1" "77.88.8.8" ];
    
    # Firewall configuration
    firewall = {
      enable = true;
      
      # MySQL(3316), PG(5442), Redis(6389), ClickHouse(8133,9010),
      # Grafana(3010), Prometheus(9100), Loki(3110)
      allowedTCPPorts = [
        3316 5442 6389 8133 9010 3010 9100 3110
      ];

      allowedUDPPorts = [ ];
      logRefusedConnections = true;
      logRefusedPackets = false;
      checkReversePath = "strict";
    };
  };

  boot.kernel.sysctl = {
    # Network performance
    "net.core.rmem_max" = 7500000;
    "net.core.wmem_max" = 7500000;

    # SYN flood protection
    "net.ipv4.tcp_syncookies" = 1;

    # Ignore ICMP redirects (prevents MITM via routing manipulation)
    "net.ipv4.conf.all.accept_redirects" = 0;
    "net.ipv4.conf.default.accept_redirects" = 0;
    "net.ipv6.conf.all.accept_redirects" = 0;

    # Reverse-path filtering (already set via firewall.checkReversePath, belt-and-suspenders)
    "net.ipv4.conf.all.rp_filter" = 1;
    "net.ipv4.conf.default.rp_filter" = 1;

    # Restrict dmesg to root (prevents info leakage about kernel/hardware)
    "kernel.dmesg_restrict" = 1;

    # Required for rootless podman / user namespaces
    "kernel.unprivileged_userns_clone" = 1;
  };
}
