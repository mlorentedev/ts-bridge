<#
.SYNOPSIS
    Bridge Launcher (Windows Client)
.DESCRIPTION
    Launches the Go bridge to tunnel connections through Tailscale.
    Reads configuration from .env file in project root.
    State is cleared by default on each run.
.PARAMETER KeepState
    Preserve previous session state instead of clearing it.
.NOTES
    Run: PowerShell -ExecutionPolicy Bypass -File .\run.ps1
#>

param(
    [switch]$KeepState
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
Write-Host "  ─────────────────────────────────────"

# Check Go
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "  ERROR: Go not found in PATH" -ForegroundColor Red
    Write-Host "  Install from: https://go.dev/dl/" -ForegroundColor Gray
    exit 1
}

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

# Clear state by default
if (-not $KeepState -and (Test-Path $StateDir)) {
    Remove-Item -Path $StateDir -Recurse -Force
    Write-Host "  State cleared" -ForegroundColor Yellow
}

Write-Host "  Target: $env:TS_TARGET"
Write-Host "  ─────────────────────────────────────"
Write-Host ""

# Launch bridge
Set-Location $ProjectRoot
go run main.go
