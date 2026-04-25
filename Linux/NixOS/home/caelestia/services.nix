{ var, ... }:

{
  caelestiaShellSettings.services = {
    audioIncrement = 0.1;
    brightnessIncrement = 0.1;
    maxVolume = 1.0;
    defaultPlayer = "Spotify";
    gpuType = "";
    playerAliases = [
      { from = "com.github.th_ch.youtube_music"; to = "YT Music"; }
    ];
    weatherLocation = var.weatherLocation;
    useFahrenheit = false;
    useFahrenheitPerformance = false;
    useTwelveHourClock = false;
    smartScheme = true;
    visualiserBars = 45;
  };

  caelestiaShellSettings.paths = {
    mediaGif = "root:/assets/bongocat.gif";
    sessionGif = "root:/assets/kurukuru.gif";
    noNotifsPic = "root:/assets/dino.png";
    lockNoNotifsPic = "root:/assets/dino.png";
    wallpaperDir = "~/Pictures/Wallpapers";
    lyricsDir = "~/Music/lyrics";
  };
}
