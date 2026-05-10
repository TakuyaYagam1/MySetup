{
  config,
  mysetupLib,
  ...
}:

{
  config = mysetupLib.mkIfPresetOrMore "developer" config.mysetup {
    system.userActivationScripts.android-adb-fix.text = ''
      ADB_DIR="$HOME/Android/Sdk/platform-tools"
      ADB_LINK="$ADB_DIR/adb"
      ADB_TARGET="/run/current-system/sw/bin/adb"
      mkdir -p "$ADB_DIR"
      if [ ! -L "$ADB_LINK" ] || [ "$(readlink "$ADB_LINK")" != "$ADB_TARGET" ]; then
        rm -f "$ADB_LINK"
        ln -s "$ADB_TARGET" "$ADB_LINK"
      fi
    '';
  };
}
