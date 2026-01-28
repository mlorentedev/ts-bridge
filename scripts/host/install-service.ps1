<#
.SYNOPSIS
    Scheduled Task Manager for Host Setup
.DESCRIPTION
    Registers setup.ps1 to run automatically at Windows startup with SYSTEM privileges.
    The script runs hidden, 30 seconds after boot, ensuring network services are ready.
.PARAMETER TargetScript
    PowerShell script to register. Default: setup.ps1
.PARAMETER TaskName
    Name for the scheduled task. Default: Tailscale-RDP-Host
.PARAMETER Uninstall
    Remove the scheduled task instead of installing.
.EXAMPLE
    .\install-service.ps1
    .\install-service.ps1 -Uninstall
    .\install-service.ps1 -TargetScript "custom.ps1" -TaskName "MyTask"
.NOTES
    Requires Administrator privileges.
    Run: PowerShell -ExecutionPolicy Bypass -File .\install-service.ps1
#>

#Requires -RunAsAdministrator

param(
    [string]$TargetScript = "setup.ps1",
    [string]$TaskName = "Tailscale-RDP-Host",
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"

Write-Host ""

# --- UNINSTALL MODE ---
if ($Uninstall) {
    Write-Host "  UNINSTALL SERVICE" -ForegroundColor Yellow
    Write-Host "  ---------------------------------------"

    $existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existingTask) {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Host "  Task '$TaskName' removed" -ForegroundColor Green
    } else {
        Write-Host "  Task '$TaskName' not found" -ForegroundColor Gray
    }

    Write-Host "  ---------------------------------------"
    Write-Host ""
    exit 0
}

# --- INSTALL MODE ---
Write-Host "  INSTALL SERVICE" -ForegroundColor Cyan
Write-Host "  ---------------------------------------"

# Resolve script path (relative to this script's directory)
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$TargetPath = Join-Path $ScriptDir $TargetScript

if (-not (Test-Path $TargetPath)) {
    Write-Host "  ERROR: '$TargetScript' not found" -ForegroundColor Red
    Write-Host "  Expected: $TargetPath" -ForegroundColor Gray
    exit 1
}

# Get absolute path
$FullPath = (Resolve-Path $TargetPath).Path

Write-Host "  Target: $FullPath" -ForegroundColor Gray
Write-Host "  Task:   $TaskName" -ForegroundColor Gray

# Define scheduled task components
$Action = New-ScheduledTaskAction `
    -Execute "PowerShell.exe" `
    -Argument "-ExecutionPolicy Bypass -WindowStyle Hidden -File `"$FullPath`""

# 30s delay ensures network/Tailscale services are ready
$Trigger = New-ScheduledTaskTrigger -AtStartup
$Trigger.Delay = "PT30S"

# SYSTEM account runs without user login
$Principal = New-ScheduledTaskPrincipal `
    -UserId "SYSTEM" `
    -LogonType ServiceAccount `
    -RunLevel Highest

# Settings for reliability
$Settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 3 `
    -RestartInterval (New-TimeSpan -Minutes 1)

# Register task
try {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $Action `
        -Trigger $Trigger `
        -Principal $Principal `
        -Settings $Settings `
        -Force | Out-Null

    Write-Host ""
    Write-Host "  Task '$TaskName' installed" -ForegroundColor Green
    Write-Host "  Runs 30s after boot (as SYSTEM)" -ForegroundColor Gray
    Write-Host "  ---------------------------------------"
    Write-Host ""
} catch {
    Write-Host "  ERROR: Failed to register task" -ForegroundColor Red
    Write-Host "  $_" -ForegroundColor Red
    exit 1
}
