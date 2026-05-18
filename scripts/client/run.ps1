<#
.SYNOPSIS
    Bridge Launcher (Windows Client)
.DESCRIPTION
    Launches the Go bridge to tunnel connections through Tailscale.
    Reads configuration from .env file in project root.
    Auto mode is enabled by default for minimal friction.
.PARAMETER Reset
    Wipe previous persistent state when running in manual mode.
.PARAMETER Instance
    Optional instance alias passed as TS_INSTANCE_NAME for auto-instance mode.
.NOTES
    Run: PowerShell -ExecutionPolicy Bypass -File .\run.ps1 [-Reset] [-Instance office-laptop]
#>

param(
    [switch]$Reset,
    [string]$Instance
)

$ErrorActionPreference = "Stop"

# Resolve paths
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent (Split-Path -Parent $ScriptDir)
$EnvFile = Join-Path $ProjectRoot ".env"
$MainGo = Join-Path $ProjectRoot "main.go"
$StateDir = Join-Path $ProjectRoot "ts-state"

Write-Host ""
Write-Host "  TAILSCALE BRIDGE (Client)" -ForegroundColor Cyan
Write-Host "  ---------------------------------------"

# Check .env
if (-not (Test-Path $EnvFile)) {
    Write-Host "  ERROR: .env not found" -ForegroundColor Red
    Write-Host "  Run: Copy-Item .env.example .env" -ForegroundColor Gray
    Write-Host "  Then edit .env to set TS_AUTHKEY and TS_TARGET." -ForegroundColor Gray
    exit 1
}

# Parse .env
Get-Content $EnvFile | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
        $key = $matches[1].Trim()
        $value = $matches[2].Trim()
        [Environment]::SetEnvironmentVariable($key, $value, "Process")
    }
}

# Validate required vars
if (-not $env:TS_AUTHKEY) {
    Write-Host "  ERROR: TS_AUTHKEY not set in .env" -ForegroundColor Red
    exit 1
}
if (-not $env:TS_TARGET) {
    Write-Host "  ERROR: TS_TARGET not set in .env" -ForegroundColor Red
    exit 1
}

# Optional instance alias override
if ($Instance) {
    [Environment]::SetEnvironmentVariable("TS_INSTANCE_NAME", $Instance, "Process")
}

# Auto mode default: enabled unless explicitly disabled.
$AutoInstance = $true
if ($env:TS_AUTO_INSTANCE) {
    $AutoInstance = @("true", "1", "yes", "on") -contains $env:TS_AUTO_INSTANCE.ToLowerInvariant()
}
if ($env:TS_MANUAL_MODE) {
    if ((@("true", "1", "yes", "on") -contains $env:TS_MANUAL_MODE.ToLowerInvariant())) {
        $AutoInstance = $false
    }
}

if ($AutoInstance) {
    if ($Reset) {
        Write-Host "  Reset ignored in auto mode (state is ephemeral)" -ForegroundColor Yellow
    }
    Write-Host "  Auto instance mode enabled" -ForegroundColor Green
    if ($env:TS_INSTANCE_NAME) {
        Write-Host "  Instance: $env:TS_INSTANCE_NAME"
    }
} else {
    # Handle persistent state directory
    if ($Reset -and (Test-Path $StateDir)) {
        Remove-Item -Path $StateDir -Recurse -Force
        Write-Host "  State reset (new IP will be allocated)" -ForegroundColor Yellow
    }

    if (Test-Path $StateDir) {
        Write-Host "  Reusing existing state" -ForegroundColor Green
    } else {
        New-Item -ItemType Directory -Path $StateDir -Force | Out-Null
        Write-Host "  Created new state directory" -ForegroundColor Green
    }
}

Write-Host "  Target: $env:TS_TARGET"
Write-Host "  ---------------------------------------"
Write-Host ""

# Launch bridge
Set-Location $ProjectRoot

$BinaryPath = Join-Path $ProjectRoot "ts-bridge.exe"
if (-not (Test-Path $BinaryPath)) {
    $BinaryPath = Join-Path $ProjectRoot "ts-bridge"
}

if (Test-Path $BinaryPath) {
    & $BinaryPath
} elseif ((Get-Command "go" -ErrorAction SilentlyContinue) -and (Test-Path "main.go")) {
    Write-Host "  Binary not found, falling back to 'go run'..." -ForegroundColor Yellow
    go run main.go
} else {
    Write-Host "  ERROR: 'ts-bridge' binary not found and 'go' is not available/main.go missing." -ForegroundColor Red
    exit 1
}
