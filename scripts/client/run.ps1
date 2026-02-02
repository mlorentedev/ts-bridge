<#
.SYNOPSIS
    Bridge Launcher (Windows Client)
.DESCRIPTION
    Launches the Go bridge to tunnel connections through Tailscale.
    Reads configuration from .env file in project root.
    State is preserved by default to maintain Tailscale IP allocation.
.PARAMETER Reset
    Wipe previous session state to force new IP allocation.
.NOTES
    Run: PowerShell -ExecutionPolicy Bypass -File .\run.ps1
#>

param(
    [switch]$Reset
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
    Write-Host "  Run: cp .env.example .env" -ForegroundColor Gray
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

# Handle state directory
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
