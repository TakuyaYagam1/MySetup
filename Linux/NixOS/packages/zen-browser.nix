{ config, lib, pkgs, ... }:

let
  preset = config.var.packagePreset or "personal";
  enabled = lib.elem preset [ "desktop" "developer" "personal" ];

  zen-pkg = pkgs.zen-browser;

  sineCfgJs = pkgs.writeText "sine-config.js" ''
    unlockPref("xpinstall.signatures.required");
    lockPref("xpinstall.signatures.required", false);

    if (!Services.appinfo.inSafeMode) {
      try {
        const cmanifest = Services.dirsvc.get("UChrm", Ci.nsIFile);
        cmanifest.append("utils");
        cmanifest.append("chrome.manifest");

        if (cmanifest.exists()) {
          Components.manager.QueryInterface(Ci.nsIComponentRegistrar).autoRegister(cmanifest);
          ChromeUtils.importESModule("chrome://userscripts/content/sine.sys.mjs");
        }
      } catch (err) {}
    }
  '';

  autoconfigPrefsJs = pkgs.writeText "autoconfig.js" ''
    pref("general.config.filename", "sine-config.js");
    pref("general.config.obscure_value", 0);
    pref("general.config.sandbox_enabled", false);
  '';

  zen-with-sine = pkgs.runCommand "zen-browser-with-sine"
    { meta = zen-pkg.meta; }
    ''
      cp -rL ${zen-pkg} $out
      chmod -R u+w $out

      zenLib=$(echo $out/lib/zen-*)
      [ -d "$zenLib" ] || { echo "zen lib dir not found: $out/lib/zen-*"; exit 1; }

      install -Dm644 ${sineCfgJs}         "$zenLib/sine-config.js"
      install -Dm644 ${autoconfigPrefsJs} "$zenLib/defaults/pref/autoconfig.js"

      ln -sf "../lib/$(basename $zenLib)/zen" "$out/bin/.zen-wrapped"
      substituteInPlace "$out/bin/zen" \
        --replace-fail "${zen-pkg}/bin/.zen-wrapped" "$out/bin/.zen-wrapped"
    '';
in
{
  config = lib.mkIf enabled {
    environment.systemPackages = [ zen-with-sine ];
  };
}
