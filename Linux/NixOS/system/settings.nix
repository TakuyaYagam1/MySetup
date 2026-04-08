{ ... }:

{
  nix = {
    settings = {
      experimental-features = [ "nix-command" "flakes" ];
      trusted-users = [ "root" "@wheel" "takuya" ];

      # GitHub token to avoid API rate limits when updating flake inputs.
      # Create /etc/nix/netrc manually (outside git), then rebuild:
      #
      #   sudo bash -c 'cat > /etc/nix/netrc << EOF
      #   machine api.github.com
      #     login token
      #     password YOUR_GITHUB_TOKEN
      #
      #   machine github.com
      #     login token
      #     password YOUR_GITHUB_TOKEN
      #   EOF
      #   chmod 600 /etc/nix/netrc'
      netrc-file = "/etc/nix/netrc";
      download-buffer-size = 268435456;

      substituters = [
        "https://cache.nixos.org"
        "https://nix-community.cachix.org"
        "https://hyprland.cachix.org"
        "https://quickshell.cachix.org"
      ];
      trusted-substituters = [
        "https://nix-community.cachix.org"
        "https://hyprland.cachix.org"
        "https://quickshell.cachix.org"
      ];
      trusted-public-keys = [
        "cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY="
        "nix-community.cachix.org-1:mB9FSh9qf2dCimDSUo8Zy7bkq5CX+/rkCWyvRCYg3Fs="
        "hyprland.cachix.org-1:a7pgxzMz7+chwVL3/pzj6jIBMioiJM7ypFP8PwtkuGc="
        "quickshell.cachix.org-1:tjWMR3PQd01gN6YtjSRUdHHHUgrSLFIgwqrCQjFXVOU="
      ];
    };

    daemonCPUSchedPolicy = "idle";
    daemonIOSchedClass = "idle";

    optimise = {
      automatic = true;
      dates = [ "weekly" ];
    };

    # GC is handled by programs.nh.clean in programs/system-tools.nix
  };

  nixpkgs.config = {
    allowUnfree = true;
    android_sdk.accept_license = true;
    permittedInsecurePackages = [
      "electron-25.9.0"
      "olm-3.2.16"
      "python3.12-pypdf2-3.0.1"
    ];
  };

}
