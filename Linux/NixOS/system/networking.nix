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
      
      # Allow ports
      # MySQL(3316), PG(5442), Redis(6389), ClickHouse(8133,9010), 
      # Grafana(3010), Prometheus(9100), Loki(3110)
      allowedTCPPorts = [ 
        3316 5442 6389 8133 9010 3010 9100 3110
      ];
      
      allowedUDPPorts = [ ];
      
      logRefusedConnections = true;
      logRefusedPackets = false;
      
      extraCommands = ''
        iptables -P INPUT DROP
        iptables -P FORWARD DROP
        iptables -P OUTPUT ACCEPT
        iptables -A INPUT -i lo -j ACCEPT
        iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
        iptables -A INPUT -m state --state INVALID -j DROP
        iptables -A INPUT -p tcp --syn -m limit --limit 1/s --limit-burst 3 -j ACCEPT
        iptables -A INPUT -p tcp --syn -j DROP
        iptables -A INPUT -p tcp --tcp-flags ALL NONE -j DROP
        iptables -A INPUT -p tcp --tcp-flags ALL ALL -j DROP
      '';
      
      extraStopCommands = ''
        iptables -P INPUT ACCEPT
        iptables -P FORWARD ACCEPT
        iptables -P OUTPUT ACCEPT
      '';
    };
    
    firewall.checkReversePath = "strict";
  };

  boot.kernel.sysctl = {
    "net.core.rmem_max" = 7500000;
    "net.core.wmem_max" = 7500000;
  };
}
