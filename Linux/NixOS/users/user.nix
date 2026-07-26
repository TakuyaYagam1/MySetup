{
  config,
  lib,
  wahrweltLib,
  ...
}:

let
  baseGroups = [
    "networkmanager"
    "wheel"
    "video"
    "audio"
    "input"
    "kvm"
  ];
  developerGroups = [
    "docker"
  ];
  personalGroups = [
    "libvirtd"
    "adbusers"
    "vboxusers"
  ];
  ctfGroups = [
    "wireshark"
  ];
in
{
  users.users.${config.wahrwelt.user.username} = {
    isNormalUser = true;
    description = config.wahrwelt.user.fullName;
    extraGroups =
      baseGroups
      ++ lib.optionals (wahrweltLib.presets.developerOrMore config.wahrwelt) developerGroups
      ++ lib.optionals (wahrweltLib.presets.personal config.wahrwelt) personalGroups
      ++ lib.optionals config.wahrwelt.features.ctfTools ctfGroups;
  };
}
