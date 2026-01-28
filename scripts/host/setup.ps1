<#
.SYNOPSIS
    RDP Host Configuration
.DESCRIPTION
    Configures Windows host for RDP access over Tailscale:
    - Enables Tailscale unattended mode
    - Ensures Tailscale service is running
    - Enables UPnP for NAT traversal
    - Sets network profile to Private
    - Enables RDP and detects port
    - Creates firewall rule
    - Disables sleep mode
.PARAMETER SkipSleep
    Skip disabling sleep mode.
.PARAMETER FirewallRuleName
    Name for the firewall rule. Default: Tailscale-RDP-Ingress
.EXAMPLE
    .\setup.ps1
    .\setup.ps1 -SkipSleep
.NOTES
    Requires Administrator privileges.
    Run: PowerShell -ExecutionPolicy Bypass -File .\setup.ps1
#>

#Requires -RunAsAdministrator

param(
    [switch]$SkipSleep,
    [string]$FirewallRuleName = "Tailscale-RDP-Ingress"
)

$ErrorActionPreference = "Continue"

function Write-Step {
    param([int]$Num, [int]$Total, [string]$Message)
    Write-Host "  [$Num/$Total] $Message"
}

function Write-Ok {
    param([string]$Message)
    Write-Host "       $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "       $Message" -ForegroundColor Yellow
}

function Write-Err {
    param([string]$Message)
    Write-Host "       $Message" -ForegroundColor Red
}

# --- START ---
Write-Host ""
Write-Host "  HOST SETUP" -ForegroundColor Cyan
Write-Host "  ─────────────────────────────────────"

$totalSteps = if ($SkipSleep) { 6 } else { 7 }
$step = 0

# 1. TAILSCALE UNATTENDED MODE
$step++
Write-Step $step $totalSteps "Tailscale unattended mode"
try {
    $result = tailscale up --unattended 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Ok "Unattended mode enabled"
    } else {
        Write-Warn "Could not enable via CLI - check Tailscale GUI"
    }
} catch {
    Write-Warn "Tailscale CLI not available"
}

# 2. TAILSCALE SERVICE
$step++
Write-Step $step $totalSteps "Tailscale service"
$tsService = Get-Service "Tailscale" -ErrorAction SilentlyContinue
if ($tsService) {
    if ($tsService.Status -ne "Running") {
        Start-Service "Tailscale" -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
        Write-Ok "Service started"
    } else {
        Write-Ok "Service running"
    }
} else {
    Write-Err "Tailscale service not found - is Tailscale installed?"
}

# 3. UPnP SERVICES
$step++
Write-Step $step $totalSteps "UPnP services"
$upnpServices = @(
    @{Name="SSDPSRV"; Desc="SSDP Discovery"},
    @{Name="upnphost"; Desc="UPnP Device Host"}
)
$upnpOk = $true
foreach ($svc in $upnpServices) {
    try {
        Set-Service -Name $svc.Name -StartupType Automatic -ErrorAction Stop
        $status = Get-Service -Name $svc.Name
        if ($status.Status -ne "Running") {
            Start-Service -Name $svc.Name -ErrorAction Stop
        }
    } catch {
        $upnpOk = $false
    }
}
if ($upnpOk) {
    Write-Ok "UPnP services active"
} else {
    Write-Warn "Some UPnP services failed to start"
}

# 4. NETWORK PROFILE
$step++
Write-Step $step $totalSteps "Network profile"
$tsNic = Get-NetConnectionProfile | Where-Object {
    $_.InterfaceAlias -match "Tailscale" -or $_.Name -match "Tailscale"
}
if ($tsNic) {
    if ($tsNic.NetworkCategory -ne "Private") {
        $tsNic | Set-NetConnectionProfile -NetworkCategory Private
        Write-Ok "Set to Private"
    } else {
        Write-Ok "Already Private"
    }
} else {
    Write-Warn "Tailscale adapter not found - is Tailscale connected?"
}

# 5. RDP CONFIGURATION
$step++
Write-Step $step $totalSteps "RDP configuration"
try {
    Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' -Name "fDenyTSConnections" -Value 0
    $RdpPort = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp' -ErrorAction SilentlyContinue).PortNumber
    if (-not $RdpPort) { $RdpPort = 3389 }
    Write-Ok "RDP enabled on port $RdpPort"
} catch {
    $RdpPort = 3389
    Write-Warn "Could not verify RDP settings, assuming port $RdpPort"
}

# 6. FIREWALL RULE
$step++
Write-Step $step $totalSteps "Firewall rule"
try {
    # Enable built-in Remote Desktop rules
    Enable-NetFirewallRule -DisplayGroup "Remote Desktop" -ErrorAction SilentlyContinue

    # Create/update custom rule for Tailscale
    Remove-NetFirewallRule -DisplayName $FirewallRuleName -ErrorAction SilentlyContinue
    New-NetFirewallRule `
        -DisplayName $FirewallRuleName `
        -Direction Inbound `
        -Action Allow `
        -Protocol TCP `
        -LocalPort $RdpPort `
        -Profile Any `
        -Enabled True | Out-Null
    Write-Ok "Rule '$FirewallRuleName' active (port $RdpPort)"
} catch {
    Write-Err "Failed to configure firewall: $_"
}

# 7. SLEEP SETTINGS
if (-not $SkipSleep) {
    $step++
    Write-Step $step $totalSteps "Power settings"
    try {
        powercfg /change standby-timeout-ac 0 2>$null
        powercfg /change hibernate-timeout-ac 0 2>$null
        powercfg /change monitor-timeout-ac 0 2>$null
        Write-Ok "Sleep disabled (AC power)"
    } catch {
        Write-Warn "Could not modify power settings"
    }
}

# --- SUMMARY ---
Write-Host ""
Write-Host "  ─────────────────────────────────────"
Write-Host "  HOST READY" -ForegroundColor Green
Write-Host "  ─────────────────────────────────────"

try {
    $tsIp = (tailscale ip -4 2>$null)
    if ($tsIp) {
        Write-Host "  Tailscale IP: " -NoNewline
        Write-Host "$tsIp" -ForegroundColor Magenta
    }
} catch {}

Write-Host "  RDP Port:     " -NoNewline
Write-Host "$RdpPort" -ForegroundColor Magenta

Write-Host ""
Write-Host "  Client .env config:" -ForegroundColor Gray
Write-Host "  TS_TARGET=$tsIp`:$RdpPort" -ForegroundColor Yellow
Write-Host ""
