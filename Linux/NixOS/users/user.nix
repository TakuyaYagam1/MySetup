{ ... }:

{
  users.users.takuya = {
    isNormalUser = true;
    extraGroups = [
      "networkmanager"
      "wheel"
      "video"
      "audio"
      "input"
      "docker"
      "kvm"
      "libvirtd"
      "adbusers"
      "vboxuser"
    ];
    initialPassword = "your_password_here"; # dont forget change password
  };

  system.userActivationScripts = {
    android-adb-fix = {
      text = ''
        mkdir -p ~/Android/Sdk/platform-tools
        rm -f ~/Android/Sdk/platform-tools/adb
        ln -s /run/current-system/sw/bin/adb ~/Android/Sdk/platform-tools/adb
      '';
    };
  };
}
