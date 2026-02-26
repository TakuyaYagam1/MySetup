{ pkgs-stable, ... }:

# CTF & Security tools
# Based on https://fabaff.github.io/nix-security-box/list

{
  environment.systemPackages = with pkgs-stable; [

    # === RECONNAISSANCE ===
    nmap
    masscan
    rustscan
    amass
    subfinder
    dnsx
    dnsenum
    dnsrecon
    dnstwist
    fierce
    theharvester
    whatweb
    wafw00f
    sherlock
    maigret
    holehe

    # === WEB EXPLOITATION ===
    burpsuite
    sqlmap
    dalfox
    nikto
    wpscan
    wfuzz
    ffuf
    feroxbuster
    gobuster
    arjun
    httpx
    katana
    gospider
    hakrawler
    gau
    waybackurls
    unfurl
    nuclei

    # === REVERSE ENGINEERING ===
    ghidra-bin
    radare2
    rizin
    cutter
    binwalk
    foremost
    ltrace
    strace
    checksec
    patchelf
    ropgadget

    # === BINARY EXPLOITATION ===
    pwntools
    gdb
    metasploit
    netcat-gnu
    socat

    # === CRYPTOGRAPHY ===
    hashcat
    hashcat-utils
    john
    hashid
    xortool

    # === PASSWORD CRACKING ===
    hydra
    medusa
    ncrack
    crunch
    cewl

    # === WIRELESS ===
    aircrack-ng
    hcxtools
    hcxdumptool
    kismet

    # === FORENSICS ===
    volatility3
    sleuthkit
    testdisk
    ddrescue
    exiftool
    exiv2
    imhex
    hexyl
    bulk_extractor

    # === STEGANOGRAPHY ===
    steghide
    stegseek
    zsteg

    # === NETWORK ===
    wireshark
    tshark
    tcpdump
    tcpflow
    ngrep
    mitmproxy
    ettercap
    bettercap
    responder
    chisel
    ligolo-ng
    sshuttle
    proxychains-ng
    iodine
    wstunnel

    # === ACTIVE DIRECTORY ===
    bloodhound
    netexec
    evil-winrm
    kerbrute
    enum4linux-ng
    smbmap
    ldapdomaindump
    ldeep

    # === CLOUD & SECRETS ===
    trufflehog
    gitleaks
    semgrep
    syft
    trivy
    grype

    # === VULNERABILITY SCANNING ===
    lynis
    vulnix
    osv-scanner

    # === FUZZING ===
    aflplusplus
    honggfuzz
    radamsa

    # === MOBILE ===
    apktool
    jadx
    dex2jar

    # === MISC ===
    yara
    openvpn
    wireguard-tools
    tor
    wordlists
    seclists
    goaccess
    gowitness
    termshark
    sngrep
    aria2
  ];
}
