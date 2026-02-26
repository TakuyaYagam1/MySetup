{ ... }:

{
  nix.settings = {
    experimental-features = [ "nix-command" "flakes" ];
    download-buffer-size = 268435456;
  };

  nixpkgs.config = {
    allowUnfree = true;
    allowBroken = true;
    android_sdk.accept_license = true;
    permittedInsecurePackages = [
      "electron-25.9.0" 
      "olm-3.2.16"
    ];
  };

  nix.gc = {
    automatic = true;
    dates = "03:15";
    options = "-d";
  };
}
