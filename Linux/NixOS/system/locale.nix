{ config, ... }:

{
  time.timeZone = config.mysetup.locale.timeZone;
  i18n.defaultLocale = config.mysetup.locale.defaultLocale;
  i18n.supportedLocales = [
    "${config.mysetup.locale.defaultLocale}/UTF-8"
    "${config.mysetup.locale.extraLocale}/UTF-8"
  ];
  console.keyMap = config.mysetup.locale.consoleKeyMap;
}
