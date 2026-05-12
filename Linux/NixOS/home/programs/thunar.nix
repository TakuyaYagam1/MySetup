{ lib, pkgs, ... }:

{
  xfconf.settings.thunar = {
    "misc-thumbnail-mode" = "THUNAR_THUMBNAIL_MODE_ALWAYS";
    "misc-thumbnail-max-file-size" = 1073741824;
    "misc-thumbnail-draw-frames" = true;
  };

  home.activation.restartThumbnailDaemons = lib.hm.dag.entryAfter [ "writeBoundary" ] ''
    for daemon in gvfsd gvfsd-fuse Thunar thunar; do
      $DRY_RUN_CMD ${pkgs.procps}/bin/pkill -u "$USER" -x "$daemon" 2>/dev/null || true
    done
    $DRY_RUN_CMD ${pkgs.procps}/bin/pkill -u "$USER" -f 'gvfs-udisks2-volume-monitor' 2>/dev/null || true
    $DRY_RUN_CMD ${pkgs.procps}/bin/pkill -u "$USER" -f 'tumbler-1/tumblerd' 2>/dev/null || true
  '';

  xdg.configFile."Thunar/thunar-volman.xml".text = ''
    <?xml version="1.0" encoding="UTF-8"?>

    <channel name="thunar-volman" version="1.0">
      <property name="automount-drives" type="empty">
        <property name="enabled" type="bool" value="false"/>
      </property>
      <property name="automount-media" type="empty">
        <property name="enabled" type="bool" value="false"/>
      </property>
    </channel>
  '';

  xdg.configFile."Thunar/uca.xml".text = ''
    <?xml version="1.0" encoding="UTF-8"?>
    <actions>
    <action>
    	<icon>utilities-terminal</icon>
    	<name>Open Terminal Here</name>
    	<submenu></submenu>
    	<unique-id>1710575157271461-1</unique-id>
    	<command>foot -D %f</command>
    	<description>Open the current directory in foot</description>
    	<range></range>
    	<patterns>*</patterns>
    	<startup-notify/>
    	<directories/>
    </action>
    </actions>
  '';
}
