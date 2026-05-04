{ config, lib, ... }:

let
  preset = config.var.packagePreset or "personal";
  enabled = preset == "personal";
in
{
  config = lib.mkMerge [
    {
      programs.gpu-screen-recorder.enable = true;
    }
    (lib.mkIf enabled {
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
