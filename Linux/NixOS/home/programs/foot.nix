{ lib, ... }:

{
  programs.foot = {
    enable = true;
    settings = {
      main = {
        shell = "fish";
        title = "foot";
        font = lib.mkForce "JetBrainsMono Nerd Font:size=12";
        letter-spacing = 0;
        dpi-aware = "no";
        pad = "25x25";
        bold-text-in-bright = "no";
        gamma-correct-blending = "no";
      };

      scrollback = {
        lines = 10000;
      };

      cursor = {
        style = "beam";
        beam-thickness = 1.5;
      };

      "colors-dark" = {
        alpha = lib.mkForce 0.78;
      };

      key-bindings = {
        scrollback-up-page = "Page_Up";
        scrollback-down-page = "Page_Down";
        search-start = "Control+Shift+f";
      };

      search-bindings = {
        cancel = "Escape";
        find-prev = "Shift+F3";
        find-next = "F3 Control+G";
      };
    };
  };
}
