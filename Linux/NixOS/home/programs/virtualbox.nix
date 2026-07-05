{
  config,
  lib,
  mysetup,
  mysetupLib,
  pkgs,
  ...
}:

let
  personal = mysetupLib.presets.personal mysetup;
  shareName = "VMShare";
  sharePath = "${config.home.homeDirectory}/${shareName}";
  ensureVirtualBoxVMShare = pkgs.writeShellApplication {
    name = "mysetup-vbox-vmshare";
    runtimeInputs = with pkgs; [
      coreutils
      virtualbox
    ];
    text = ''
      mkdir -p "${sharePath}"

      VBoxManage setextradata global GUI/ShowMiniToolBar false
      VBoxManage setextradata global GUI/Fullscreen/LegacyMode true

      VBoxManage sharedfolder remove global --name "${shareName}" >/dev/null 2>&1 || true
      VBoxManage sharedfolder add global \
        --name "${shareName}" \
        --hostpath "${sharePath}" \
        --automount
    '';
  };
in
{
  config = lib.mkIf personal {
    home.packages = [ ensureVirtualBoxVMShare ];

    systemd.user.services.virtualbox-vmshare = {
      Unit = {
        Description = "Ensure VirtualBox global VMShare shared folder";
      };

      Service = {
        Type = "oneshot";
        ExecStart = "${ensureVirtualBoxVMShare}/bin/mysetup-vbox-vmshare";
      };

      Install.WantedBy = [ "default.target" ];
    };
  };
}
