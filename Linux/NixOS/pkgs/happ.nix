{
  happ,
  makeWrapper,
  symlinkJoin,
}:

symlinkJoin {
  name = "${happ.name}-wahrwelt";
  paths = [ happ ];
  nativeBuildInputs = [ makeWrapper ];

  postBuild = ''
    rm -f "$out/bin/happ"
    makeWrapper ${happ}/bin/happ "$out/bin/happ" \
      --unset QT_PLUGIN_PATH \
      --unset QT_QPA_PLATFORMTHEME \
      --set QT_STYLE_OVERRIDE Fusion

    desktopFile="$out/share/applications/Happ.desktop"
    if [ -e "$desktopFile" ]; then
      target="$(readlink -f "$desktopFile")"
      rm -f "$desktopFile"
      substitute "$target" "$desktopFile" \
        --replace-fail "Exec=${happ}/bin/happ" "Exec=$out/bin/happ"
    fi
  '';

  meta = happ.meta or { };
}
