{ pkgs-stable, ... }:

{
  fonts = {
    enableDefaultPackages = true;

    packages = with pkgs-stable; [
      jetbrains-mono
      nerd-fonts.jetbrains-mono
      nerd-fonts.caskaydia-cove
      nerd-fonts.fira-code
      liberation_ttf
      corefonts
      noto-fonts
      noto-fonts-cjk-sans
      noto-fonts-cjk-serif
      noto-fonts-emoji-blob-bin
      noto-fonts-color-emoji
      google-fonts
      cascadia-code
      material-symbols
      nerd-fonts.symbols-only
      vista-fonts
      vista-fonts-chs
      vista-fonts-cht
      wqy_zenhei
      wqy_microhei
      symbola
    ];

    fontconfig = {
      defaultFonts = {
        serif = [ "Noto Serif" "Liberation Serif" "Times New Roman" ];
        sansSerif = [ "Noto Sans" "Liberation Sans" ];
        monospace = [
          "JetBrains Mono"
          "JetBrainsMono Nerd Font"
          "Cascadia Code"
          "Liberation Mono"
          "Courier New"
        ];
      };
    };
  };
}
