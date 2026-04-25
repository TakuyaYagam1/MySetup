{ ... }:

{
  programs.cava = {
    enable = true;
    settings = {
      general = {
        bars = 0;
        bar_width = 2;
        bar_spacing = 1;
        bar_height = 32;
        framerate = 60;
      };
      output = {
        method = "ncurses";
        orientation = "bottom";
        bar_delimiters = 0;
        channels = "stereo";
        mono_option = "average";
        reverse = 0;
      };
      smoothing = {
        integral = 77;
        monstercat = 1;
        waves = 0;
        gravity = 100;
        noise_reduction = 77;
      };
    };
  };
}
