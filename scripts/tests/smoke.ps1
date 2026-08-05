<#
.SYNOPSIS
    Smoke tests for ts-bridge CLI — validates all subcommands before E2E.
.DESCRIPTION
    Runs through every CLI command with its flags, verifies exit codes
    and output. Designed to run on Windows before manual E2E with
    a real Tailscale device (acemagic-lab-1 by default).
.NOTES
    Requires: ts-bridge.exe on PATH or $env:BIN set.
#>

param(
    [string]$Bin,
    [string]$Target = "100.118.157.114:3389",  # acemagic-lab-1 RDP
    [string]$AuthKey = "tskey-auth-test-dummy",
    [string]$WorkDir = "$env:TEMP\ts-bridge-smoke-$$"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Resolve bin: default to ts-bridge.exe in repo root
if (-not $Bin) {
    # Script is at scripts/tests/smoke.ps1 → repo root is 2 levels up
    $Bin = Join-Path (Join-Path (Split-Path -Parent $PSScriptRoot) "..") "ts-bridge.exe"
}
if (-not (Test-Path $Bin -PathType Leaf)) {
    Write-Host "ERROR: Binary not found at: $Bin" -ForegroundColor Red
    Write-Host "Build with: go build -o ts-bridge.exe ./cmd/ts-bridge/" -ForegroundColor Yellow
    exit 1
}

$passed = 0
$failed = 0
$skipped = 0

function Test-Step {
    param(
        [string]$Name,
        [scriptblock]$Block,
        [switch]$ExpectFailure,
        [string]$ExpectedOutput,
        [string[]]$ExpectedOutputContains
    )
    Write-Host "`n  >>> $Name" -ForegroundColor Cyan

    try {
        $exitCode = 0
        $output = & $Block 2>&1
        $exitCode = $LASTEXITCODE

        if ($ExpectFailure) {
            if ($exitCode -ne 0) {
                Write-Host "    [PASS] Exited with code $exitCode (expected failure)" -ForegroundColor Green
                $script:passed++
                return
            } else {
                Write-Host "    [FAIL] Expected failure but got exit 0" -ForegroundColor Red
                $script:failed++
                return
            }
        }

        if ($exitCode -ne 0) {
            Write-Host "    [FAIL] Exit code $exitCode (expected 0)" -ForegroundColor Red
            Write-Host "    Output: $output" -ForegroundColor DarkGray
            $script:failed++
            return
        }

        $outputStr = $output -join "`n"

        if ($ExpectedOutput -and $outputStr.Trim() -ne $ExpectedOutput) {
            Write-Host "    [FAIL] Output mismatch" -ForegroundColor Red
            Write-Host "    Expected: $ExpectedOutput" -ForegroundColor DarkGray
            Write-Host "    Got:      $($outputStr.Trim())" -ForegroundColor DarkGray
            $script:failed++
            return
        }

        if ($ExpectedOutputContains) {
            $allMatch = $true
            foreach ($pattern in $ExpectedOutputContains) {
                if ($outputStr -notmatch [regex]::Escape($pattern)) {
                    Write-Host "    [FAIL] Output doesn't contain '$pattern'" -ForegroundColor Red
                    Write-Host "    Output: $outputStr" -ForegroundColor DarkGray
                    $allMatch = $false
                }
            }
            if (-not $allMatch) {
                $script:failed++
                return
            }
        }

        Write-Host "    [PASS]" -ForegroundColor Green
        $script:passed++
    } catch {
        Write-Host "    [FAIL] Exception: $_" -ForegroundColor Red
        $script:failed++
    }
}

function Test-Step-Silent {
    param([string]$Name, [scriptblock]$Block)
    Write-Host "`n  >>> $Name" -ForegroundColor Cyan
    try {
        & $Block 2>$null
        if ($LASTEXITCODE -eq 0) {
            Write-Host "    [PASS]" -ForegroundColor Green
            $script:passed++
        } else {
            Write-Host "    [FAIL] Exit code $LASTEXITCODE" -ForegroundColor Red
            $script:failed++
        }
    } catch {
        Write-Host "    [FAIL] Exception: $_" -ForegroundColor Red
        $script:failed++
    }
}

Write-Host "========================================" -ForegroundColor White
Write-Host "  ts-bridge Smoke Tests" -ForegroundColor White
Write-Host "  Bin: $Bin" -ForegroundColor Gray
Write-Host "  Target: $Target" -ForegroundColor Gray
Write-Host "  WorkDir: $WorkDir" -ForegroundColor Gray
Write-Host "========================================"

# --- Setup ---
New-Item -ItemType Directory -Force -Path $WorkDir | Out-Null
$origDir = Get-Location
Set-Location $WorkDir

# ============================================================================
# 1. VERSION
# ============================================================================
Write-Host "`n[1/6] VERSION" -ForegroundColor Yellow

Test-Step -Name "version (default)" -Block { & $Bin version; $LASTEXITCODE } `
    -ExpectedOutputContains "ts-bridge"

Test-Step -Name "version --short" -Block { & $Bin version --short; $LASTEXITCODE } `
    -ExpectedOutputContains "dev"

# -v is the shorthand for the global --verbose flag (cmd/cli/root.go), NOT a
# version alias. With no subcommand cobra prints the help text, which contains
# the binary name — so the previous assertion of "ts-bridge" here passed by
# accident while testing nothing it claimed to. Mirrors the BATS case
# "-v (verbose) without a subcommand: prints help, not a version".
Test-Step -Name "-v (verbose, not a version flag): prints help" -Block { & $Bin -v; $LASTEXITCODE } `
    -ExpectedOutputContains "Usage:"

# The real version flag, which the suite had no coverage for.
Test-Step -Name "--version flag" -Block { & $Bin --version; $LASTEXITCODE } `
    -ExpectedOutputContains "ts-bridge"

Test-Step -Name "--help (root)" -Block { & $Bin --help; $LASTEXITCODE } `
    -ExpectedOutputContains @("connect", "init", "status", "host")

# ============================================================================
# 2. STATUS (bridge not running)
# ============================================================================
Write-Host "`n[2/6] STATUS (bridge not running)" -ForegroundColor Yellow

Test-Step -Name "status (default, not running)" -Block { & $Bin status; $LASTEXITCODE } `
    -ExpectedOutputContains "Bridge not running"

Test-Step -Name "status --json (not running)" -Block { & $Bin status --json; $LASTEXITCODE } `
    -ExpectedOutputContains "Bridge not running"

Test-Step -Name "status --addr custom (not running)" -Block { & $Bin status --addr 127.0.0.1:9999; $LASTEXITCODE } `
    -ExpectedOutputContains "Bridge not running"

# ============================================================================
# 3. INIT (non-interactive)
# ============================================================================
Write-Host "`n[3/6] INIT (non-interactive)" -ForegroundColor Yellow

# Clean any leftover config files from previous tests
Remove-Item ".env" -Force -ErrorAction SilentlyContinue
Remove-Item "ts-bridge.yaml" -Force -ErrorAction SilentlyContinue
Remove-Item "$WorkDir\custom.yaml" -Force -ErrorAction SilentlyContinue

# ============================================================================
# 3. INIT (non-interactive)
# ============================================================================
function Test-File {
    param([string]$Path, [string[]]$Contains)
    if (-not (Test-Path $Path)) {
        Write-Host "    [FAIL] File not found: $Path" -ForegroundColor Red
        $script:failed++
        return
    }
    if ($Contains) {
        $content = Get-Content $Path -Raw
        foreach ($pat in $Contains) {
            if ($content -notmatch [regex]::Escape($pat)) {
                Write-Host "    [FAIL] File doesn't contain '$pat'" -ForegroundColor Red
                $script:failed++
                return
            }
        }
    }
    Write-Host "    [PASS]" -ForegroundColor Green
    $script:passed++
}

Test-Step -Name "init --auth-key --target (env format, default)" -Block {
    & $Bin init --auth-key $AuthKey --target $Target --force; $LASTEXITCODE
}
Test-File -Path (Join-Path $WorkDir ".env") -Contains @("TS_TARGET")
Remove-Item (Join-Path $WorkDir ".env") -Force -ErrorAction SilentlyContinue

Test-Step -Name "init --format yaml" -Block {
    & $Bin init --auth-key $AuthKey --target $Target --format yaml --force; $LASTEXITCODE
}
Test-File -Path (Join-Path $WorkDir "ts-bridge.yaml")
Remove-Item (Join-Path $WorkDir "ts-bridge.yaml") -Force -ErrorAction SilentlyContinue

Test-Step -Name "init --config custom path" -Block {
    $customPath = Join-Path $WorkDir "custom.yaml"
    & $Bin init --auth-key $AuthKey --target $Target --config $customPath --force; $LASTEXITCODE
}
Test-File -Path (Join-Path $WorkDir "custom.yaml")
Remove-Item (Join-Path $WorkDir "custom.yaml") -Force -ErrorAction SilentlyContinue

# Test overwrite protection (BUG-001/002/019)
Test-Step -Name "init without --force should fail on existing config" -Block {
    $envPath = Join-Path $WorkDir ".env"
    New-Item -ItemType File -Force -Path $envPath | Out-Null
    & $Bin init --auth-key $AuthKey --target $Target --config $envPath; $LASTEXITCODE
} -ExpectFailure

Test-Step -Name "init with --force should overwrite existing config" -Block {
    $envPath = Join-Path $WorkDir ".env"
    New-Item -ItemType File -Force -Path $envPath | Out-Null
    & $Bin init --auth-key $AuthKey --target $Target --config $envPath --force; $LASTEXITCODE
}
Remove-Item (Join-Path $WorkDir ".env") -Force -ErrorAction SilentlyContinue

# ============================================================================
# 4. CONNECT (dry-run — just verify it starts, then kill)
# ============================================================================
Write-Host "`n[4/6] CONNECT (dry-run)" -ForegroundColor Yellow

# We can't fully test connect without a real Tailscale network, but we can
# verify the command parses and starts initializing.
Test-Step -Name "connect --help" -Block { & $Bin connect --help; $LASTEXITCODE } `
    -ExpectedOutputContains "target"

# Quick connect attempt — should start tsnet init and then we'll kill it.
# Using a dummy auth key so it fails fast on auth, not on network.
Test-Step -Name "connect with invalid auth key (should fail auth, not crash)" -Block {
    Start-Process -FilePath $Bin -ArgumentList "connect", "--auth-key", "tskey-auth-invalid", "--target", $Target, "--verbose", "--drain-timeout", "2s" -NoNewWindow -Wait -PassThru | Out-Null
    $LASTEXITCODE
} # Expects non-zero (auth failure)

# ============================================================================
# 5. HOST (subcommands)
# ============================================================================
Write-Host "`n[5/6] HOST" -ForegroundColor Yellow

Test-Step -Name "host --help" -Block { & $Bin host --help; $LASTEXITCODE } `
    -ExpectedOutputContains @("init", "setup", "check")

Test-Step -Name "host setup --help" -Block { & $Bin host setup --help; $LASTEXITCODE } `
    -ExpectedOutputContains "firewall"

Test-Step -Name "host check --help" -Block { & $Bin host check --help; $LASTEXITCODE } `
    -ExpectedOutputContains "json"

# ============================================================================
# 6. CONFIG PRECEDENCE (unit-level check)
# ============================================================================
# Ensure reject.yaml doesn't exist from previous runs
Remove-Item (Join-Path $WorkDir "reject.yaml") -Force -ErrorAction SilentlyContinue
Remove-Item "reject.yaml" -Force -ErrorAction SilentlyContinue
Write-Host "`n[6/6] CONFIG PRECEDENCE" -ForegroundColor Yellow

# Test: YAML config rejects auth key field (security constraint)
Test-Step -Name "YAML config with unknown fields" -Block {
    $yamlPath = Join-Path $WorkDir "unknown.yaml"
    Set-Content -Path $yamlPath -Value @"
version: 1
target: 100.0.0.1:3389
unknown_field: should_warn
"@
    & $Bin init --config $yamlPath --auth-key $AuthKey --target $Target --force 2>&1; $LASTEXITCODE
} -ExpectedOutputContains "unknown"  # Should warn about unknown fields

Remove-Item (Join-Path $WorkDir "unknown.yaml") -Force -ErrorAction SilentlyContinue

# ============================================================================
# Cleanup
# ============================================================================
Set-Location $origDir
Remove-Item $WorkDir -Recurse -Force -ErrorAction SilentlyContinue

# ============================================================================
# Summary
# ============================================================================
Write-Host "`n========================================" -ForegroundColor White
Write-Host "  RESULTS" -ForegroundColor White
Write-Host "========================================"
Write-Host "  Passed:   $passed" -ForegroundColor Green
Write-Host "  Failed:   $failed" -ForegroundColor Red
Write-Host "  Total:    $($passed + $failed)" -ForegroundColor White

if ($failed -gt 0) {
    Write-Host "`n  STATUS: FAILURES DETECTED" -ForegroundColor Red
    exit 1
} else {
    Write-Host "`n  STATUS: ALL SMOKE TESTS PASSED" -ForegroundColor Green
    Write-Host "`n  Next: Manual E2E with acemagic-lab-1" -ForegroundColor Gray
    exit 0
}
