{
  lib,
  mysetup,
  mysetupLib,
  pkgs,
  ...
}:

let
  desktopOrMore = mysetupLib.presets.desktopOrMore mysetup;
  developerOrMore = mysetupLib.presets.developerOrMore mysetup;
  inherit (mysetup.features) ctfTools;
in
{
  xdg.dataFile = lib.optionalAttrs desktopOrMore {
    "applications/anytype.desktop" = {
      force = true;
      source = "${pkgs.anytype}/share/applications/anytype.desktop";
    };
  };

  xdg.desktopEntries =
    lib.optionalAttrs desktopOrMore {
      thunar = {
        name = "Thunar";
        exec = "env GDK_BACKEND=x11 thunar %F";
        icon = "Thunar";
        terminal = false;
        categories = [
          "System"
          "FileManager"
        ];
        mimeType = [ "inode/directory" ];
      };
    }
    // lib.optionalAttrs developerOrMore {
      virt-manager = {
        name = "Virtual Machine Manager";
        exec = "env GDK_BACKEND=x11 virt-manager";
        icon = "virt-manager";
        terminal = false;
        categories = [ "System" ];
      };
      "termius-app" = {
        name = "Termius";
        comment = "The SSH client that works on Desktop and Mobile";
        exec = "termius-app";
        icon = "${pkgs.termius}/share/icons/termius-app.png";
        terminal = false;
        categories = [ "Network" ];
      };
    }
    // lib.optionalAttrs ctfTools {
      burpsuitepro = {
        name = "BurpSuite Professional";
        comment = "An integrated platform for performing security testing of web applications";
        exec = "/run/current-system/sw/bin/burpsuitepro";
        icon = "burpsuitepro";
        terminal = false;
        categories = [
          "Development"
          "Security"
          "System"
        ];
      };
      "org.zim_wiki.Zim" = {
        name = "Zim Desktop Wiki";
        comment = "Edit text files \"wiki style\"";
        exec = "zim %f";
        icon = "zim";
        terminal = false;
        categories = [
          "Utility"
          "TextEditor"
        ];
        mimeType = [
          "application/x-zim-notebook"
          "text/x-zim-wiki"
        ];
      };
    };
}
