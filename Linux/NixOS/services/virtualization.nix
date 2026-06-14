{
  config,
  mysetupLib,
  pkgs,
  ...
}:

let
  virtioWinIso = pkgs.runCommand "virtio-win.iso" { nativeBuildInputs = [ pkgs.xorriso ]; } ''
    xorriso -as mkisofs -iso-level 3 -J -R -V virtio-win -o "$out" ${pkgs.virtio-win}
  '';
in
{
  config = mysetupLib.mkIfPresetOrMore "developer" config.mysetup {
    environment.systemPackages = with pkgs; [
      virt-manager
      virt-viewer
      virtio-win
      spice-vdagent
    ];
    environment.etc."vm/virtio-win".source = pkgs.virtio-win;
    environment.etc."vm/virtio-win.iso".source = virtioWinIso;

    programs.virt-manager.enable = true;

    systemd.tmpfiles.rules = [
      "d ${config.mysetup.user.homeDirectory}/VMShare 0750 ${config.mysetup.user.username} users - -"
    ];

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

    systemd.services.libvirt-default-network = {
      description = "Ensure libvirt default network is active";
      wantedBy = [ "multi-user.target" ];
      requires = [ "libvirtd.service" ];
      after = [ "libvirtd.service" ];
      path = with pkgs; [
        gnugrep
        libvirt
      ];
      serviceConfig = {
        Type = "oneshot";
        RemainAfterExit = true;
      };
      script = ''
        if ! virsh -c qemu:///system net-info default >/dev/null 2>&1; then
          virsh -c qemu:///system net-define /var/lib/libvirt/qemu/networks/default.xml
        fi

        virsh -c qemu:///system net-autostart default

        if ! virsh -c qemu:///system net-list --name --state-running | grep -qx default; then
          virsh -c qemu:///system net-start default
        fi
      '';
    };
  };
}
