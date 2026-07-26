[CmdletBinding()]
param([switch]$DryRun)

$ErrorActionPreference = 'Stop'
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

# pinned Sine artifacts (keep in sync with zip.go)
$Sine = @(
    @{ Name = 'profile'; Url = 'https://github.com/sineorg/bootloader/releases/download/v0.1.4/profile.zip'; Sha = '285b3d589cc979f11f01c9c77410b717694ccc4f32cc1cb08bd6d8909fb98e00'; Required = $true }
    @{ Name = 'engine'; Url = 'https://github.com/CosmoCreeper/Sine/releases/download/v2.3/engine.zip'; Sha = '5892add04ab4cf808018d8982495d53029de0b5cd62d80ea6905a741cf897bfd'; Required = $true }
    @{ Name = 'locales'; Url = 'https://github.com/CosmoCreeper/Sine/releases/download/v2.3/locales.zip'; Sha = 'f7d269d86738cef4635d61e035795655326aabc6cacc71e97f8acae17e7cc57f'; Required = $false }
)
$SineSentinels = @(
    'JS\sine.sys.mjs', 'JS\engine.json', 'utils\chrome.manifest',
    'sine-mods\mods.json', 'locales\en-US\sine-preferences.ftl'
)

function Find-ZenAppDir {
    foreach ($lnkRoot in @("$env:APPDATA\Microsoft\Windows\Start Menu", "$env:ProgramData\Microsoft\Windows\Start Menu")) {
        $lnk = Get-ChildItem $lnkRoot -Recurse -Filter 'Zen*.lnk' -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($lnk) {
            $t = (New-Object -ComObject WScript.Shell).CreateShortcut($lnk.FullName).TargetPath
            if ($t -and (Test-Path $t)) {
                $d = Split-Path $t
                if (Test-Path (Join-Path $d 'defaults\pref')) { return $d }
            }
        }
    }
    foreach ($c in @("$env:ProgramFiles\Zen Browser", "${env:ProgramFiles(x86)}\Zen Browser", "$env:LOCALAPPDATA\Zen Browser")) {
        if (Test-Path (Join-Path $c 'defaults\pref')) { return $c }
    }
    return $null
}

function Find-ZenProfile {
    $root = "$env:APPDATA\zen"
    $ini = Join-Path $root 'profiles.ini'
    if (Test-Path $ini) {
        $lines = Get-Content $ini
        $inInstall = $false
        foreach ($ln in $lines) {
            if ($ln -match '^\[Install') { $inInstall = $true; continue }
            if ($ln -match '^\[') { $inInstall = $false }
            if ($inInstall -and $ln -match '^\s*Default\s*=\s*(.+?)\s*$') {
                $rel = $Matches[1] -replace '/', '\'
                $p = Join-Path $root $rel
                if (Test-Path $p) { return $p }
            }
        }
    }
    $profilesDir = Join-Path $root 'Profiles'
    if (Test-Path $profilesDir) {
        $dirs = Get-ChildItem $profilesDir -Directory
        $def = $dirs | Where-Object { $_.Name -match '(?i)default' } | Select-Object -First 1
        if ($def) { return $def.FullName }
        if ($dirs) { return ($dirs | Select-Object -First 1).FullName }
    }
    return $null
}

function Test-SineInstalled([string]$chrome) {
    foreach ($s in $SineSentinels) {
        if (-not (Test-Path (Join-Path $chrome $s))) { return $false }
    }
    return $true
}

# locate Zen
$appDir = Find-ZenAppDir
$profile = Find-ZenProfile
if (-not $appDir) { Write-Warning 'Zen install dir not found; install Zen first.'; return }
if (-not $profile) { Write-Warning 'Zen profile not found; launch Zen once, then rerun this script.'; return }
$chrome = Join-Path $profile 'chrome'

Write-Host "Zen app dir : $appDir"
Write-Host "Zen profile : $profile"

# application-dir autoconfig (verbatim from zen-browser.nix)
$autoconfigJs = @'
pref("general.config.filename", "sine-config.js");
pref("general.config.obscure_value", 0);
pref("general.config.sandbox_enabled", false);
'@
$sineConfigJs = @'
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
'@
$autoconfigPath = Join-Path $appDir 'defaults\pref\autoconfig.js'
$sineConfigPath = Join-Path $appDir 'sine-config.js'

if ($DryRun) {
    Write-Host "[dry-run] write $autoconfigPath"
    Write-Host "[dry-run] write $sineConfigPath"
} else {
    [System.IO.File]::WriteAllText($autoconfigPath, $autoconfigJs.Replace("`r`n","`n"), $Utf8NoBom)
    [System.IO.File]::WriteAllText($sineConfigPath, $sineConfigJs.Replace("`r`n","`n"), $Utf8NoBom)
    Write-Host "Autoconfig written (re-run after Zen updates; updates wipe the install dir)."
}

# Firefox/Zen only loads userChrome.css / userContent.css when this pref is on.
$userJs = Join-Path $profile 'user.js'
$legacyPref = 'user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", true);'
if ($DryRun) {
    Write-Host "[dry-run] ensure '$legacyPref' in $userJs"
} else {
    $existing = if (Test-Path $userJs) { Get-Content $userJs -Raw } else { '' }
    if ($existing -notmatch 'legacyUserProfileCustomizations\.stylesheets') {
        $sep = if ($existing -and -not $existing.EndsWith("`n")) { "`n" } else { '' }
        [System.IO.File]::WriteAllText($userJs, $existing + $sep + $legacyPref + "`n", $Utf8NoBom)
        Write-Host "Enabled legacy userChrome stylesheets in user.js."
    } else {
        Write-Host "Legacy userChrome stylesheets already enabled."
    }
}

# Catppuccin chrome theme
$themeSrc = Join-Path (Resolve-Path "$PSScriptRoot\..\..") 'Linux\dots\zen\chrome'
if (-not (Test-Path $themeSrc)) {
    Write-Warning "Catppuccin theme source missing: $themeSrc"
} else {
    if ($DryRun) {
        Write-Host "[dry-run] copy $themeSrc\* -> $chrome"
    } else {
        New-Item -ItemType Directory -Force $chrome | Out-Null
        Copy-Item (Join-Path $themeSrc '*') $chrome -Recurse -Force
        Write-Host "Catppuccin chrome CSS copied to profile."
    }
}

# Sine archives (pinned + SHA256-verified)
if (Test-SineInstalled $chrome) {
    Write-Host 'Sine already present in profile; skipping download.'
} else {
    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("wahrwelt-sine-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force $tmp | Out-Null
    try {
        $fetched = @()
        foreach ($a in $Sine) {
            $dst = Join-Path $tmp "$($a.Name).zip"
            try {
                if ($DryRun) {
                    Write-Host "[dry-run] download $($a.Url)"
                    Write-Host "[dry-run] verify sha256 == $($a.Sha)"
                } else {
                    Invoke-WebRequest -Uri $a.Url -OutFile $dst -UseBasicParsing
                    $got = (Get-FileHash $dst -Algorithm SHA256).Hash.ToLower()
                    if ($got -ne $a.Sha) { throw "SHA256 mismatch for $($a.Name): got $got expected $($a.Sha)" }
                    $fetched += @{ Name = $a.Name; Path = $dst }
                }
            } catch {
                if ($a.Required) { throw "Sine $($a.Name) failed: $_" }
                Write-Warning "Sine $($a.Name) skipped (optional): $_"
            }
        }
        if (-not $DryRun) {
            New-Item -ItemType Directory -Force $chrome | Out-Null
            foreach ($f in $fetched) {
                Expand-Archive -LiteralPath $f.Path -DestinationPath $chrome -Force
            }
            Write-Host 'Sine archives verified and extracted into profile.'
        }
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Write-Host ''
Write-Host 'Zen Sine + Catppuccin done. Final step (manual):' -ForegroundColor Green
Write-Host 'about:support -> "Clear startup cache", then fully restart Zen Browser.'
