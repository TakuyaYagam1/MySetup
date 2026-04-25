{ ... }:

{
  caelestiaShellSettings.general = {
    logo = "caelestia";
    apps = {
      terminal = [ "foot" ];
      audio = [ "pavucontrol" ];
      playback = [ "mpv" ];
      explorer = [ "thunar" ];
    };
    battery = {
      warnLevels = [
        {
          level = 20;
          title = "Low battery";
          message = "You might want to plug in a charger";
          icon = "battery_android_frame_2";
        }
        {
          level = 10;
          title = "Did you see the previous message?";
          message = "You should probably plug in a charger <b>now</b>";
          icon = "battery_android_frame_1";
        }
        {
          level = 5;
          title = "Critical battery level";
          message = "PLUG THE CHARGER RIGHT NOW!!";
          icon = "battery_android_alert";
          critical = true;
        }
      ];
      criticalLevel = 3;
    };
    idle = {
      lockBeforeSleep = true;
      inhibitWhenAudio = true;
      timeouts = [
        { timeout = 600;  idleAction = "lock"; }
        { timeout = 1800; idleAction = "dpms off"; returnAction = "dpms on"; }
        { timeout = 3600; idleAction = [ "systemctl" "suspend-then-hibernate" ]; }
      ];
    };
  };
}
