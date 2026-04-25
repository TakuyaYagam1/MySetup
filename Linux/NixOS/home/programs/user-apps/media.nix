{ pkgs, ... }:

{
  home.packages = with pkgs; [
    # Audio visualisers / recording
    cava
    libcava
    aubio
    gpu-screen-recorder

    # Audio control
    pwvucontrol
    pavucontrol
    pamixer
    playerctl
  ];
}
