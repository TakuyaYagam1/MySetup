[CmdletBinding()]
param(
    [Parameter(Mandatory, Position = 0)]
    [string]$Name,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

function Resolve-App {
    param([string]$Query)

    $apps = @(Get-StartApps | Where-Object { $_.Name -and $_.AppID })

    $hit = $apps | Where-Object { $_.Name -ieq $Query } | Select-Object -First 1
    if ($hit) { return $hit }

    $apps |
        Where-Object {
            $_.Name -like "*$Query*" -and
            $_.Name -notmatch '(?i)uninstall'
        } |
        Sort-Object { $_.Name.Length } |
        Select-Object -First 1
}

$app = Resolve-App -Query $Name
if (-not $app) {
    Write-Error "launch: no Start menu app matches '$Name'"
    exit 1
}

if ($DryRun) {
    '{0} -> {1}' -f $app.Name, $app.AppID
    return
}

if ($app.AppID -match '^[A-Za-z]:\\' -and (Test-Path -LiteralPath $app.AppID)) {
    Start-Process -FilePath $app.AppID
} else {
    Start-Process -FilePath explorer.exe -ArgumentList "shell:AppsFolder\$($app.AppID)"
}
