{
  config,
  wahrweltLib,
  pkgs,
  ...
}:

{
  config = wahrweltLib.mkIfPresetOrMore "desktop" config.wahrwelt {
    programs.thunar = {
      enable = true;
      plugins = with pkgs; [
        thunar-archive-plugin
        thunar-volman
        thunar-media-tags-plugin
      ];
    };

    environment.systemPackages = with pkgs; [
      tumbler
      ffmpegthumbnailer
      poppler-utils
      xarchiver
    ];
  };
}
