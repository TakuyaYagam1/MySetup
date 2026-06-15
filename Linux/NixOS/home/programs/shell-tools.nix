{ config, lib, ... }:

let
  colors = config.lib.stylix.colors;
in
{
  programs = {
    direnv = {
      enable = true;
      enableFishIntegration = true;
      nix-direnv.enable = true;
    };

    eza = {
      enable = true;
      enableFishIntegration = true;
      git = true;
      icons = "auto";
      extraOptions = [
        "--group-directories-first"
        "--no-quotes"
      ];
    };

    fzf = {
      enable = true;
      enableFishIntegration = true;
      colors = lib.mkDefault {
        "fg+" = "#${colors.base0D}";
        "bg+" = "-1";
        fg = "#${colors.base05}";
        bg = "-1";
        prompt = "#${colors.base03}";
        pointer = "#${colors.base0D}";
      };
      defaultOptions = [
        "--margin=1"
        "--layout=reverse"
        "--border=none"
        "--info=hidden"
        "--prompt=/ "
        "-i"
        "--no-bold"
      ];
    };

    zoxide = {
      enable = true;
      enableFishIntegration = true;
      options = [
        "--cmd"
        "cd"
      ];
    };
  };
}
