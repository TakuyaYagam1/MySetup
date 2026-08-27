{
  config,
  lib,
  wahrweltLib,
  ...
}:

let
  desktopEnabled = wahrweltLib.presets.desktopOrMore config.wahrwelt;
  gamingEnabled = wahrweltLib.presets.personal config.wahrwelt;
in
{
  config = lib.mkMerge [
    {
      programs.gpu-screen-recorder.enable = desktopEnabled;
    }
    (lib.mkIf gamingEnabled {
      programs.steam = {
        enable = true;
        gamescopeSession.enable = true;
      };

      programs.gamemode.enable = true;
    })
  ];
}
