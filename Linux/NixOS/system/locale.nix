{ config, ... }:

{
  time.timeZone = config.var.timeZone;
  i18n.defaultLocale = config.var.defaultLocale;

  # defaultLocale already includes ".UTF-8"; result "<name>.UTF-8/UTF-8" is canonical glibc form.
  i18n.supportedLocales = [
    "${config.var.defaultLocale}/UTF-8"
    "${config.var.extraLocale}/UTF-8"
  ];
  console.keyMap = config.var.consoleKeyMap;
}
