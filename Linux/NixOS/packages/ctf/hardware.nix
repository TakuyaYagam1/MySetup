{ pkgs-stable, ... }:

# Hardware/RF/wireless: 802.11, Bluetooth, SDR, RFID, embedded/JTAG, serial.

{
  environment.systemPackages = with pkgs-stable; [
    aircrack-ng
    airgeddon
    asleap
    bluesnarfer
    bluez
    bpf-linker
    braa
    bully
    chirp
    cowpatty
    crackle
    cutecom
    flashrom
    gnuradio
    hackrf
    hcxdumptool
    hcxtools
    horst
    hostapd-mana
    i2c-tools
    inspectrum
    iw
    kalibrate-rtl
    kismet
    mdk4
    minicom
    multimon-ng
    openocd
    owl
    pixiewps
    proxmark3
    redfang
    rfdump
    spooftooph
    termineter
    ubertooth
    uhd
  ];
}
