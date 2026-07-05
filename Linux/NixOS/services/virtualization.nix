{
  config,
  lib,
  mysetupLib,
  pkgs,
  ...
}:

let
  cfg = config.mysetup;
  developerOrMore = mysetupLib.presets.developerOrMore cfg;
  personal = mysetupLib.presets.personal cfg;
  virtioWinIso = pkgs.runCommand "virtio-win.iso" { nativeBuildInputs = [ pkgs.xorriso ]; } ''
    xorriso -as mkisofs -iso-level 3 -J -R -V virtio-win -o "$out" ${pkgs.virtio-win}
  '';
in
{
  config = lib.mkMerge [
    (lib.mkIf developerOrMore {
      virtualisation = {
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
    })

    (lib.mkIf personal {
      environment = {
        systemPackages = with pkgs; [
          virt-manager
          virt-viewer
          virtio-win
          spice-vdagent
        ];
        etc."vm/virtio-win".source = pkgs.virtio-win;
        etc."vm/virtio-win.iso".source = virtioWinIso;
      };

      programs.virt-manager.enable = true;

      systemd.tmpfiles.rules = [
        "d ${cfg.user.homeDirectory}/VMShare 0750 ${cfg.user.username} users - -"
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

        virtualbox.host = {
          enable = true;
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
          virsh_qemu() {
            virsh -c qemu:///system "$@"
          }

          network_active() {
            virsh_qemu net-info default | grep -Eq '^Active:[[:space:]]+yes$'
          }

          if ! virsh_qemu net-info default >/dev/null 2>&1; then
            virsh_qemu net-define /var/lib/libvirt/qemu/networks/default.xml
          fi

          virsh_qemu net-autostart default

          if ! network_active; then
            virsh_qemu net-start default || true
          fi

          network_active
        '';
      };
    })
  ];
}
