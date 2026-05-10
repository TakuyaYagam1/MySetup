{
  config,
  lib,
  mysetupLib,
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
    "libvirtd"
    "adbusers"
  ];
in
{
  users.users.${config.mysetup.user.username} = {
    isNormalUser = true;
    description = config.mysetup.user.fullName;
    extraGroups =
      baseGroups ++ lib.optionals (mysetupLib.presets.developerOrMore config.mysetup) developerGroups;
  };
}
