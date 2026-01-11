{ pkgs, ... }:

let
  pkgs312 = pkgs.extend (final: prev: {
    python3 = prev.python312;
    python3Packages = prev.python312Packages;
  });

  py = pkgs312.python3Packages;
in
{
  environment.systemPackages = with pkgs312; [
    # === RECONNAISSANCE & OSINT ===
    amass                   # Subdomain enumeration
    subfinder               # Fast subdomain discovery
    assetfinder             # Find domains and subdomains
    theharvester            # OSINT gathering
    sherlock                # Username search across platforms
    maigret                 # Username OSINT
    socialscan              # Social media username checker
    dnsenum                 # DNS enumeration
    dnsrecon                # DNS reconnaissance
    dnstwist                # Domain permutation engine
    dnsx                    # DNS toolkit
    fierce                  # DNS scanner
    findomain               # Subdomain finder
    xsubfind3r              # Subdomain finder
    subjs                   # Subdomain scraper
    subprober               # Subdomain prober
    h8mail                  # Email OSINT
    ghunt                   # Google account OSINT
    holehe                  # Email to account finder
    checkip                 # IP info checker
    whatweb                 # Web fingerprinting
    webanalyze              # Web technology detection
    wtfis                   # Domain/IP lookup
    
    # === NETWORK SCANNING ===
    nmap                    # Network mapper
    masscan                 # Fast port scanner
    rustscan                # Modern port scanner
    naabu                   # Port scanning tool
    sx-go                   # Fast network scanner
    netdiscover             # ARP scanner
    arp-scan                # ARP scanner
    arp-scan-rs             # Rust ARP scanner
    nbtscan                 # NetBIOS scanner
    nbtscanner              # NetBIOS name scanner
    zmap                    # Internet scanner
    
    # === VULNERABILITY SCANNING ===
    nuclei                  # Fast vulnerability scanner
    nuclei-templates        # Templates for nuclei
    nikto                   # Web server scanner
    wpscan                  # WordPress scanner
    wprecon                 # WordPress recon
    trivy                   # Container vulnerability scanner
    grype                   # Vulnerability scanner
    lynis                   # Security auditing
    vulnix                  # NixOS vulnerability scanner
    vuls                    # Vulnerability scanner
    osv-scanner             # OSV vulnerability scanner
    osv-detector            # Detect vulnerable dependencies
    safety-cli              # Python dependency checker
    pip-audit               # Python package auditor
    cargo-audit             # Rust crate auditor
    terrascan               # IaC security scanner
    tfsec                   # Terraform security scanner
    checkov                 # IaC scanner
    kubeaudit               # Kubernetes auditor
    kube-hunter             # Kubernetes penetration tool
    kube-score              # Kubernetes security checker
    kubescape               # Kubernetes security platform
    popeye                  # Kubernetes cluster sanitizer
    
    # === WEB EXPLOITATION ===
    burpsuite               # Web application testing
    sqlmap                  # SQL injection tool
    commix                  # Command injection
    dalfox                  # XSS scanner
    crackql                 # GraphQL password brute-forcer
    graphqlmap              # GraphQL exploitation
    graphw00f               # GraphQL fingerprinting
    ghauri                  # SQL injection tool
    nosqli                  # NoSQL injection
    wfuzz                   # Web fuzzer
    ffuf                    # Fast web fuzzer
    feroxbuster             # Content discovery
    gobuster                # Directory/DNS brute-forcer
    dirbuster               # Directory brute-force
    wad                     # Web application detector
    wafw00f                 # WAF detector
    waf-tester              # WAF tester
    nomore403               # Bypass 403
    dontgo403               # 403 bypass tool
    arjun                   # HTTP parameter discovery
    httpx                   # HTTP toolkit
    httpie                  # HTTP client
    curl                    # Transfer tool
    wget                    # Network downloader
    katana                  # Web crawler
    gospider                # Web spider
    hakrawler               # Web crawler
    photon                  # Fast crawler
    gau                     # Get all URLs
    waybackurls             # Wayback machine URLs
    urlfinder               # URL finder
    unfurl                  # URL parser
    qsreplace               # Query string replacer
    
    # === REVERSE ENGINEERING ===
    ghidra-bin              # Reverse engineering suite
    radare2                 # Reverse engineering framework
    rizin                   # Fork of radare2
    cutter                  # GUI for rizin/radare2
    iaito                   # GUI for radare2
    binaryninja-free        # Binary analysis
    hopper                  # Disassembler
    binwalk                 # Firmware analysis
    foremost                # File carving
    bulk_extractor          # Digital forensics
    scalpel                 # File carving
    ltrace                  # Library call tracer
    strace                  # System call tracer
    checksec                # Binary security checker
    patchelf                # ELF patcher
    one_gadget              # One gadget finder
    ropgadget               # ROP exploitation
    py.angr                 # Binary analysis (Python 3.12)
    py.angrop               # ROP gadgets (Python 3.12)
    qemu                    # Emulator
    unicorn                 # CPU emulator
    py.unicorn              # Python unicorn (Python 3.12)
    capstone                # Disassembly framework
    keystone                # Assembler framework
    
    # === BINARY EXPLOITATION ===
    pwntools                # CTF framework
    gef                     # GDB Enhanced Features
    gdb                     # GNU debugger
    lldb                    # LLVM debugger
    metasploit              # Exploitation framework
    exploitdb               # Exploit database
    msfpc                   # MSF payload creator
    pwncat                  # Netcat for pwn
    netcat-gnu              # Network tool
    socat                   # Socket relay
    
    # === CRYPTOGRAPHY ===
    hashcat                 # Password recovery
    hashcat-utils           # Hashcat utilities
    john                    # John the Ripper
    hashid                  # Hash identifier
    hash-identifier         # Identify hash types
    xortool                 # XOR analysis
    xorex                   # XOR extractor
    cyberchef               # Data manipulation
    cryptsetup              # LUKS encryption
    bruteforce-luks         # LUKS brute-force
    truecrack               # TrueCrypt cracker
    
    # === PASSWORD CRACKING ===
    hashcat                 # GPU cracker
    john                    # CPU cracker
    hydra                   # Network login cracker
    thc-hydra               # THC Hydra
    medusa                  # Parallel login bracker
    ncrack                  # Network cracker
    crowbar                 # Brute forcing tool
    crunch                  # Wordlist generator
    cewl                    # Custom wordlist generator
    
    # === WIRELESS ===
    aircrack-ng             # WiFi security
    wifite2                 # Automated WiFi attacks
    reaverwps-t6x           # WPS attacks
    reaverwps               # WPS brute-force
    bully                   # WPS brute-force
    pixiewps                # WPS attack tool
    cowpatty                # WPA-PSK attacks
    hcxtools                # WiFi analysis
    hcxdumptool             # WiFi dumping
    kismet                  # Wireless detector
    horst                   # Wireless monitor
    wavemon                 # Wireless monitor
    
    # === FORENSICS ===
    volatility3             # Memory forensics
    autopsy                 # Digital forensics
    sleuthkit               # Forensics toolkit
    foremost                # File recovery
    testdisk                # Data recovery
    extundelete             # ext3/4 recovery
    ext4magic               # ext4 recovery
    ddrescue                # Data recovery
    dcfldd                  # Enhanced dd
    afflib                  # Advanced forensics format
    exiftool                # Metadata tool
    exiv2                   # Image metadata
    hachoir                 # Binary parser
    hexedit                 # Hex editor
    ghex                    # GTK hex editor
    imhex                   # Hex editor
    xxd                     # Hex dump
    hexyl                   # Hex viewer
    
    # === STEGANOGRAPHY ===
    steghide                # Steganography tool
    stegseek                # Steghide cracker
    zsteg                   # PNG/BMP stego
    stegsolve               # Image analysis
    
    # === NETWORK TOOLS ===
    wireshark               # Network analyzer
    wireshark-cli           # CLI wireshark
    tshark                  # Terminal wireshark
    tcpdump                 # Packet analyzer
    tcpflow                 # TCP flow recorder
    tcpreplay               # Packet replay
    ngrep                   # Network grep
    netsniff-ng             # Network sniffer
    sniffglue               # Packet sniffer
    dsniff                  # Network sniffing
    ettercap                # MITM framework
    bettercap               # MITM framework
    mitmproxy               # MITM proxy
    mitmproxy2swagger       # Convert to swagger
    mitm6                   # IPv6 MITM
    responder               # LLMNR/NBT-NS poisoner
    yersinia                # Layer 2 attacks
    lldpd                   # LLDP daemon
    
    # === TUNNELING & PIVOTING ===
    chisel                  # Fast TCP/UDP tunnel
    ligolo-ng               # Tunneling tool
    sshuttle                # VPN over SSH
    proxychains             # Proxy chains
    proxychains-ng          # Modern proxychains
    iodine                  # DNS tunnel
    udptunnel               # UDP tunnel
    httptunnel              # HTTP tunnel
    wstunnel                # WebSocket tunnel
    tunnelgraf              # Tunnel manager
    
    # === CLOUD SECURITY ===
    prowler                 # AWS security
    pacu                    # AWS exploitation
    cloudfox                # Cloud enumeration
    cloudlist               # Cloud asset discovery
    cloudbrute              # Cloud infrastructure scanner
    cloud-nuke              # Cloud resource cleanup
    azurehound              # Azure AD recon
    py.roadtools            # Azure AD tools (Python 3.12)
    gcp-scanner             # GCP scanner
    gato                    # GitHub attack toolkit
    trufflehog              # Secret scanner
    gitleaks                # Git secret scanner
    detect-secrets          # Secret detection
    secretscanner           # Secret scanner
    deepsecrets             # Deep secret scan
    gitxray                 # GitHub recon
    gitjacker               # GitHub tool
    gitls                   # Git enumeration
    
    # === ACTIVE DIRECTORY ===
    bloodhound              # AD relationships
    bloodhound-py           # Python bloodhound
    rusthound-ce            # Rust bloodhound
    netexec                 # Network execution
    evil-winrm              # WinRM shell
    evil-winrm-py           # Python evil-winrm
    py.impacket             # Python network protocols (Python 3.12)
    kerbrute                # Kerberos attacks
    coercer                 # Coerce authentication
    certipy                 # AD certificate abuse
    certi                   # Certificate tool
    certsync                # Certificate sync
    ldapdomaindump          # LDAP info dumper
    ldeep                   # LDAP enumeration
    ldapnomnom              # LDAP parser
    msldapdump              # LDAP dumper
    py.lsassy               # Credential dumper (Python 3.12)
    py.pypykatz             # Mimikatz in Python (Python 3.12)
    donpapi                 # Secrets dumper
    hekatomb                # Credential dumper
    go365                   # Office365 tool
    py.roadrecon            # Azure AD recon (Python 3.12)
    adidnsdump              # AD DNS dumper
    adreaper                # AD enumeration
    adenum                  # AD enumeration
    autobloody              # AD exploitation
    enum4linux              # SMB enumeration
    enum4linux-ng           # Modern enum4linux
    smbmap                  # SMB share enum
    smbclient-ng            # SMB client
    cifs-utils              # CIFS utilities
    erosmb                  # SMB enumeration
    pysqlrecon              # SQL reconnaissance
    powerview               # PowerShell AD tool
    pywhisker               # AD manipulation
    
    # === WEB APIS ===
    kiterunner              # API path discovery
    swagger-codegen         # Swagger generator
    go-swagger              # Swagger toolkit
    postman                 # API testing
    yaak                    # API client
    insomnia                # REST client
    grpcurl                 # gRPC client
    
    # === CONTAINER SECURITY ===
    trivy                   # Container scanner
    grype                   # Vulnerability scanner
    clair                   # Container scanner
    dockle                  # Container linter
    dive                    # Docker image explorer
    lazydocker              # Docker TUI
    docker-compose          # Container orchestration
    podman-compose          # Podman compose
    cdk-go                  # Container penetration
    
    # === KUBERNETES ===
    kubectl                 # K8s CLI
    k9s                     # K8s TUI
    kubectx                 # Context switcher
    helm                    # K8s package manager
    kustomize               # K8s customization
    kubeaudit               # K8s auditor
    kube-hunter             # K8s pentester
    kube-score              # K8s checker
    kubescape               # K8s security
    popeye                  # K8s sanitizer
    kubestroyer             # K8s tool
    kdigger                 # K8s discovery
    
    # === MOBILE ===
    apktool                 # Android APK tool
    apkeep                  # APK downloader
    jadx                    # Android decompiler
    dex2jar                 # DEX to JAR
    android-tools           # ADB and fastboot
    scrcpy                  # Android screen mirror
    genymotion              # Android emulator
    
    # === HARDWARE ===
    esptool                 # ESP32/8266 flasher
    flashrom                # Firmware flasher
    openocd                 # Debugger
    avrdude                 # AVR programmer
    teensy-loader-cli       # Teensy loader
    tytools                 # Teensy manager
    py.pyi2cflash           # I2C flash (Python 3.12)
    py.pyspiflash           # SPI flash (Python 3.12)
    py.cantools             # CAN bus (Python 3.12)
    cantoolz                # CAN toolkit
    py.emv                  # EMV protocol (Python 3.12)
    libnfc                  # NFC library
    libfreefare             # RFID library
    mfcuk                   # Mifare cracker
    mfoc                    # Mifare classic
    ubertooth               # Bluetooth tool
    killerbee               # Zigbee tools
    py.bleak                # Bluetooth Low Energy (Python 3.12)
    bluez                   # Bluetooth stack
    bluewalker              # Bluetooth tool
    
    # === FUZZING ===
    aflplusplus             # American Fuzzy Lop
    honggfuzz               # Fuzzer
    radamsa                 # Fuzzer
    boofuzz                 # Network fuzzer
    
    # === REPORTING ===
    eyewitness              # Screenshot tool
    gowitness               # Screenshot tool
    
    # === UTILITIES ===
    cyberchef               # Data manipulation
    jq                      # JSON processor
    yq-go                   # YAML processor
    bat                     # Cat alternative
    ripgrep                 # Fast grep
    fd                      # Fast find
    fzf                     # Fuzzy finder
    eza                     # Modern ls
    tmux                    # Terminal multiplexer
    zellij                  # Modern tmux
    
    # === MISC SECURITY ===
    lynis                   # Security auditing
    aide                    # File integrity
    py.aiosseclient         # HIDS (Python 3.12)
    fail2ban                # Intrusion prevention
    snort                   # IDS/IPS
    suricata                # IDS/IPS
    zeek                    # Network monitor
    
    # === SECRET SCANNING ===
    tell-me-your-secrets    # Secret finder
    whispers                # Secret scanner
    
    # === SUPPLY CHAIN ===
    syft                    # SBOM generator
    packj                   # Package analyzer
    
    # === TRAFFIC ANALYSIS ===
    termshark               # Terminal wireshark
    sngrep                  # SIP analyzer
    
    # === DNS TOOLS ===
    dig                     # DNS lookup
    bind                    # DNS utilities
    dnsutils                # DNS tools
    massdns                 # Mass DNS resolver
    
    # === ENCODING/DECODING ===
    basez                   # Multiple encodings
    
    # === HTTP UTILITIES ===
    aria2                   # Downloader
    
    # === WORDLISTS ===
    wordlists               # Common wordlists
    seclists                # Security wordlists
    
    # === CODE ANALYSIS ===
    semgrep                 # Static analysis
    codeql                  # Code analysis
    snyk                    # Dependency scanner
    bandit                  # Python security
    brakeman                # Rails security
    gosec                   # Go security
    
    # === VPN & PROXIES ===
    openvpn                 # VPN client
    wireguard-tools         # WireGuard VPN
    tor                     # Anonymous network
    
    # === STRESS TESTING ===
    vegeta                  # HTTP load tester
    hey                     # HTTP benchmarker
    ddosify                 # DDoS simulator
    slowlorust              # Slow loris
    
    # === THREAT INTELLIGENCE ===
    threatest               # Threat detection
    yara                    # Pattern matching
    yara-x                  # Next-gen YARA
    
    # === REVERSE SHELL ===
    rustcat                 # Modern netcat
    
    # === LOG ANALYSIS ===
    goaccess                # Log analyzer
    lnav                    # Log viewer
    
    # === DOCUMENTATION ===
    tldr                    # Simplified man pages
    cheat                   # Cheatsheets
  ];
}
