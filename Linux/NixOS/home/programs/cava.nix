_:

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
      color = {
        background = "'#24273a'";
        gradient = 1;
        gradient_color_1 = "'#8bd5ca'";
        gradient_color_2 = "'#91d7e3'";
        gradient_color_3 = "'#7dc4e4'";
        gradient_color_4 = "'#8aadf4'";
        gradient_color_5 = "'#c6a0f6'";
        gradient_color_6 = "'#f5bde6'";
        gradient_color_7 = "'#ee99a0'";
        gradient_color_8 = "'#ed8796'";
      };
    };
  };
}
