{ config, ... }:

{
  time.timeZone = config.wahrwelt.locale.timeZone;
  i18n.defaultLocale = config.wahrwelt.locale.defaultLocale;
  i18n.supportedLocales = [
    "${config.wahrwelt.locale.defaultLocale}/UTF-8"
    "${config.wahrwelt.locale.extraLocale}/UTF-8"
  ];
  console.keyMap = config.wahrwelt.locale.consoleKeyMap;
}
