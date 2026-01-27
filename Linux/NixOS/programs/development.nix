{ pkgs, ... }:

{
  programs.nix-ld = {
    enable = true;
    libraries = with pkgs; [
      stdenv.cc.cc.lib
      glibc
      zlib
      libpng
      nss
      nspr
      openssl
      libxml2
      dbus
      expat
      libdrm
      libxi
      libxkbfile
      libbsd
      libGL
      libGLU
      libxkbcommon
      xcbutilxrm
      libxcb-keysyms
      libxcb-wm
      libxcb-render-util
      libxcb-image
      libxcb-cursor
      pcre
      libepoxy
      xorg.libX11
      xorg.libXext
      xorg.libXrender
      xorg.libXrandr
      xorg.libXcursor
      xorg.libxcb
      xorg.libXfixes
      xorg.libXcomposite
      pulseaudio
      qt6.qtbase
      qt6.qtwayland
      qt6.qtmultimedia
      qt6.qttools
      qt6.qtdeclarative
    ];
  };
}
