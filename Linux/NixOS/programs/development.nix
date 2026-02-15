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
      libx11
      libxext
      libxrender
      libxrandr
      libxcursor
      xcbutil
      libxfixes
      libxcomposite
      pulseaudio
      qt6.qtbase
      qt6.qtwayland
      qt6.qtmultimedia
      qt6.qttools
      qt6.qtdeclarative
    ];
  };
}
