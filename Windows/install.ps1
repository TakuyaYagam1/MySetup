# ==========================================
# Komorebi + whkd + YASB Installation Script
# Run as Administrator
# ==========================================

if (-NOT ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")) {
    Write-Warning "Run PowerShell as Administrator!"
    Exit 1
}

$ErrorActionPreference = "Stop"
$repoWindows = $PSScriptRoot
$userConfig = "$env:USERPROFILE\.config"

Write-Host "Installing Komorebi + whkd + YASB..." -ForegroundColor Cyan

# [1/8] Long paths
Write-Host "[1/8] Enabling long paths..." -ForegroundColor Yellow
Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem' -Name 'LongPathsEnabled' -Value 1

# [2/8] Binaries via winget
Write-Host "[2/8] Installing binaries via winget..." -ForegroundColor Yellow
winget install --id LGUG2Z.komorebi --accept-package-agreements --accept-source-agreements --silent
winget install --id LGUG2Z.whkd --accept-package-agreements --accept-source-agreements --silent
winget install --id AmN.yasb --accept-package-agreements --accept-source-agreements --silent

# [3/8] Refresh PATH so komorebic/yasbc are visible in this session
Write-Host "[3/8] Refreshing PATH..." -ForegroundColor Yellow
$env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")

# [4/8] Generate komorebi default tree, then overwrite with our configs
Write-Host "[4/8] Running komorebic quickstart..." -ForegroundColor Yellow
try { komorebic quickstart } catch { Write-Warning "quickstart returned non-zero, continuing: $_" }

# [5/8] Copy configs (with `username_here` -> $env:USERNAME substitution)
Write-Host "[5/8] Copying configs from repo and substituting username..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force $userConfig | Out-Null
New-Item -ItemType Directory -Force "$userConfig\yasb" | Out-Null

$script:Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
function Copy-WithUserSubst {
    param([string]$Src, [string]$Dst)
    $text = (Get-Content -Raw -Path $Src) -replace 'username_here', $env:USERNAME
    [System.IO.File]::WriteAllText($Dst, $text, $script:Utf8NoBom)
}

# komorebi.json / komorebi.bar.json / applications.json -> %USERPROFILE%
Get-ChildItem "$repoWindows\komorebi\*" -File | ForEach-Object {
    Copy-WithUserSubst $_.FullName "$env:USERPROFILE\$($_.Name)"
}

# yasb (config.yaml + styles.css) -> %USERPROFILE%\.config\yasb
Get-ChildItem "$repoWindows\yasb\*" -File | ForEach-Object {
    Copy-WithUserSubst $_.FullName "$userConfig\yasb\$($_.Name)"
}

# whkdrc -> %USERPROFILE%\.config\whkdrc
Copy-WithUserSubst "$repoWindows\whkd\whkdrc" "$userConfig\whkdrc"
Copy-WithUserSubst "$repoWindows\whkd\launch.ps1" "$userConfig\launch.ps1"

# [6/8] Autostart + (re)start komorebi with whkd
Write-Host "[6/8] Enabling autostart and starting komorebi + whkd..." -ForegroundColor Yellow
try { komorebic enable-autostart --whkd } catch { Write-Warning "enable-autostart failed: $_" }
try { komorebic stop --whkd } catch { } # ignore if not running
komorebic start --whkd

# [7/8] Extra apps (winget + Microsoft Store)
Write-Host "[7/8] Installing extra apps..." -ForegroundColor Yellow
# JetBrainsMono Nerd Font: required for YASB / komorebi-bar glyphs.
winget install --id DEVCOM.JetBrainsMonoNerdFont --accept-package-agreements --accept-source-agreements --silent
winget install --id Zen-Team.Zen-Browser --accept-package-agreements --accept-source-agreements --silent
winget install --id Discord.Discord --accept-package-agreements --accept-source-agreements --silent

try {
    Get-Process -Name Discord -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    winget install --id Vendicated.Vencord --accept-package-agreements --accept-source-agreements --override "-install -branch stable"
} catch {
    Write-Warning "Vencord patch failed (run VencordInstallerCli manually): $_"
}

winget install --id 9P8LTPGCBZXD --accept-package-agreements --accept-source-agreements --silent
winget install --id 9PF4KZ2VN4W9 --accept-package-agreements --accept-source-agreements --silent

# [8/8] TranslucentTB autostart via Startup-folder shortcut (works for MSIX/Store apps)
Write-Host "[8/8] Configuring TranslucentTB autostart..." -ForegroundColor Yellow
try {
    $ttb = Get-StartApps | Where-Object { $_.Name -like "*TranslucentTB*" } | Select-Object -First 1
    if ($ttb) {
        $startupDir = [Environment]::GetFolderPath('Startup')
        $lnkPath = Join-Path $startupDir 'TranslucentTB.lnk'
        $wsShell = New-Object -ComObject WScript.Shell
        $shortcut = $wsShell.CreateShortcut($lnkPath)
        $shortcut.TargetPath = "$env:WINDIR\explorer.exe"
        $shortcut.Arguments = "shell:AppsFolder\$($ttb.AppID)"
        $shortcut.Save()
        Write-Host "Startup shortcut created: $lnkPath"
    } else {
        Write-Warning "TranslucentTB not found in Start apps. Enable 'Open at boot' from its tray menu manually."
    }
} catch {
    Write-Warning "Failed to set TranslucentTB autostart: $_"
}

# Zen Browser: Sine mod loader + Catppuccin theme
Write-Host "[*] Zen: Sine + Catppuccin..." -ForegroundColor Yellow
try {
    & "$repoWindows\zen\setup-zen.ps1"
} catch {
    Write-Warning "Zen Sine/Catppuccin setup failed (run Windows\zen\setup-zen.ps1 manually): $_"
}

Write-Host "`nDone." -ForegroundColor Green
Write-Host @"

Manual steps left:
  1. Start YASB (if not autostarted):
     yasbc start
  2. Verify komorebi is running:
     komorebic state
  3. Vencord: launch Discord once. If mods do not appear, rerun the patch:
     winget install --id Vendicated.Vencord --override "-install -branch stable"
  4. Zen: open about:support -> "Clear startup cache", then restart Zen
     (re-run Windows\zen\setup-zen.ps1 after Zen browser updates).
"@ -ForegroundColor Yellow

Read-Host "Press Enter to exit"
