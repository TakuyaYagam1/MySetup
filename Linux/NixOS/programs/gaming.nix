{
  config,
  lib,
  mysetupLib,
  ...
}:

let
  desktopEnabled = mysetupLib.presets.desktopOrMore config.mysetup;
  gamingEnabled = mysetupLib.presets.personal config.mysetup;
in
{
  config = lib.mkMerge [
    {
      programs.gpu-screen-recorder.enable = desktopEnabled;
    }
    (lib.mkIf gamingEnabled {
      programs.steam = {
        enable = true;
        remotePlay.openFirewall = true;
        dedicatedServer.openFirewall = true;
        localNetworkGameTransfers.openFirewall = true;
        gamescopeSession.enable = true;
      };

      programs.gamemode.enable = true;
    })
  ];
}
