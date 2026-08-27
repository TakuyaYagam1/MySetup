{
  config,
  lib,
  wahrweltLib,
  pkgs,
  ...
}:

let
  cfg = config.wahrwelt;
  developerOrMore = wahrweltLib.presets.developerOrMore cfg;
  personal = wahrweltLib.presets.personal cfg;
  virtioWinIso = pkgs.runCommand "virtio-win.iso" { nativeBuildInputs = [ pkgs.xorriso ]; } ''
    xorriso -as mkisofs -iso-level 3 -J -R -V virtio-win -o "$out" ${pkgs.virtio-win}
  '';
in
{
  config = lib.mkMerge [
    (lib.mkIf developerOrMore {
      virtualisation = {
        docker = {
          # Portainer may explicitly enable the rootful daemon. Developer
          # presets otherwise use the rootless Podman socket below.
          enable = lib.mkDefault false;
          daemon.settings = {
            inherit (wahrweltLib.defaults) dns;
            log-driver = "journald";
          };
        };

        podman = {
          enable = true;
          dockerCompat = !config.services.portainer.enable;
          # The system-wide compatibility socket is root-equivalent. Podman
          # already enables the per-user socket at %t/podman/podman.sock.
          dockerSocket.enable = false;
          defaultNetwork.settings.dns_enabled = true;
        };
      };

      home-manager.users.${cfg.user.username}.home.sessionVariables.DOCKER_HOST = lib.mkIf (
        !config.services.portainer.enable
      ) "unix://$XDG_RUNTIME_DIR/podman/podman.sock";
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
