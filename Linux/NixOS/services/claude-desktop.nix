{
  config,
  lib,
  pkgs,
  wahrweltLib,
  ...
}:

let
  coworkEnabled = config.wahrwelt.features.claudeDesktopCowork;
  developerOrMore = wahrweltLib.presets.developerOrMore config.wahrwelt;
in
{
  config = lib.mkMerge [
    {
      assertions = lib.optional coworkEnabled {
        assertion = developerOrMore;
        message = "wahrwelt.features.claudeDesktopCowork requires the developer or personal preset";
      };
    }

    (lib.mkIf (coworkEnabled && developerOrMore) {
      environment.systemPackages = [
        pkgs.qemu_kvm
        pkgs.OVMF.fd
        pkgs.virtiofsd
      ];

      systemd.tmpfiles.rules = [
        "L+ /usr/share/OVMF/OVMF_CODE.fd - - - - ${pkgs.OVMF.fd}/FV/OVMF_CODE.fd"
        "L+ /usr/share/OVMF/OVMF_CODE_4M.fd - - - - ${pkgs.OVMF.fd}/FV/OVMF_CODE.fd"
        "L+ /usr/share/OVMF/OVMF_VARS.fd - - - - ${pkgs.OVMF.fd}/FV/OVMF_VARS.fd"
        "L+ /usr/share/OVMF/OVMF_VARS_4M.fd - - - - ${pkgs.OVMF.fd}/FV/OVMF_VARS.fd"
        "L+ /usr/libexec/virtiofsd - - - - ${pkgs.virtiofsd}/bin/virtiofsd"
      ];
    })
  ];
}
