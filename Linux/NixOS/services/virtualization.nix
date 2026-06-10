{
  config,
  mysetupLib,
  pkgs,
  ...
}:

{
  config = mysetupLib.mkIfPresetOrMore "developer" config.mysetup {
    environment.systemPackages = with pkgs; [
      virt-manager
      virt-viewer
      spice-vdagent
    ];

    programs.virt-manager.enable = true;

    virtualisation = {
      libvirtd = {
        enable = true;
        qemu = {
          package = pkgs.qemu_kvm;
          runAsRoot = true;
          swtpm.enable = true;
          vhostUserPackages = [ pkgs.virtiofsd ];
        };
      };

      spiceUSBRedirection.enable = true;

      docker = {
        enable = true;
        daemon.settings = {
          inherit (mysetupLib.defaults) dns;
          log-driver = "journald";
        };
      };

      podman = {
        enable = true;
        dockerCompat = false;
        dockerSocket.enable = false;
        defaultNetwork.settings.dns_enabled = true;
      };
    };
  };
}
