_:

{
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
