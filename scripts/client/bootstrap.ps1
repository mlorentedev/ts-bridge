<#
.SYNOPSIS
    Bootstrap minimal .env for ts-bridge client mode.
.DESCRIPTION
    Creates or updates .env with the minimum required settings and sane defaults
    for low-friction auto mode operation.
.PARAMETER AuthKey
    Tailscale auth key (tskey-...).
.PARAMETER Target
    Target address in HOST:PORT format.
.PARAMETER Instance
    Optional stable alias for deterministic local port selection.
.PARAMETER PortRange
    Optional port range for auto mode (default: 33389-34388).
.NOTES
    Run:
      PowerShell -ExecutionPolicy Bypass -File .\scripts\client\bootstrap.ps1 -AuthKey tskey-auth-... -Target 100.x.x.x:3389 -Instance office-laptop
#>

param(
    [Parameter(Mandatory = $true)]
    [string]$AuthKey,
    [Parameter(Mandatory = $true)]
    [string]$Target,
    [string]$Instance = "",
    [string]$PortRange = "33389-34388"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $ScriptDir)
$EnvFile = Join-Path $ProjectRoot ".env"
$EnvExample = Join-Path $ProjectRoot ".env.example"

if (-not (Test-Path $EnvFile)) {
    if (-not (Test-Path $EnvExample)) {
        Write-Host "ERROR: neither .env nor .env.example exist in project root." -ForegroundColor Red
        exit 1
    }
    Copy-Item -Path $EnvExample -Destination $EnvFile -Force
}

$lines = [System.Collections.Generic.List[string]]::new()
$lines.AddRange((Get-Content $EnvFile))

function Set-EnvVar {
    param(
        [System.Collections.Generic.List[string]]$Content,
        [string]$Key,
        [string]$Value
    )

    $pattern = "^\s*" + [Regex]::Escape($Key) + "="
    $updated = $false

    for ($i = 0; $i -lt $Content.Count; $i++) {
        if ($Content[$i] -match $pattern) {
            $Content[$i] = "$Key=$Value"
            $updated = $true
        }
    }

    if (-not $updated) {
        $Content.Add("$Key=$Value")
    }
}

Set-EnvVar -Content $lines -Key "TS_AUTHKEY" -Value $AuthKey
Set-EnvVar -Content $lines -Key "TS_TARGET" -Value $Target
Set-EnvVar -Content $lines -Key "TS_AUTO_INSTANCE" -Value "true"
Set-EnvVar -Content $lines -Key "TS_MANUAL_MODE" -Value "false"
Set-EnvVar -Content $lines -Key "TS_PORT_RANGE" -Value $PortRange

if ($Instance) {
    Set-EnvVar -Content $lines -Key "TS_INSTANCE_NAME" -Value $Instance
}

$utf8NoBom = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllLines($EnvFile, $lines, $utf8NoBom)

Write-Host ""
Write-Host "ts-bridge .env configured successfully." -ForegroundColor Green
Write-Host "  File: $EnvFile"
Write-Host "  Target: $Target"
if ($Instance) {
    Write-Host "  Instance: $Instance"
}
Write-Host ""
Write-Host "Next step:"
if ($Instance) {
    Write-Host "  PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1 -Instance $Instance"
} else {
    Write-Host "  PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1"
}
