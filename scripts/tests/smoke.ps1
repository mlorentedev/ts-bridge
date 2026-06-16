#Requires -RunAsAdministrator
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
    [string]$Bin = ".\ts-bridge.exe",
    [string]$Target = "100.73.154.225:3389",  # acemagic-lab-1 RDP
    [string]$AuthKey = "tskey-auth-test-dummy",
    [string]$WorkDir = "$env:TEMP\ts-bridge-smoke-$$"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$passed = 0
$failed = 0
$skipped = 0

function Test-Step {
    param(
        [string]$Name,
        [scriptblock]$Block,
        [switch]$ExpectFailure,
        [string]$ExpectedOutput,
        [string]$ExpectedOutputContains
    )
    Write-Host "`n  >>> $Name" -ForegroundColor Cyan

    try {
        $output = & $Block 2>&1 | Out-String
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

        if ($ExpectedOutput -and $output.Trim() -ne $ExpectedOutput) {
            Write-Host "    [FAIL] Output mismatch" -ForegroundColor Red
            Write-Host "    Expected: $ExpectedOutput" -ForegroundColor DarkGray
            Write-Host "    Got:      $($output.Trim())" -ForegroundColor DarkGray
            $script:failed++
            return
        }

        if ($ExpectedOutputContains -and $output -notmatch [regex]::Escape($ExpectedOutputContains)) {
            Write-Host "    [FAIL] Output doesn't contain '$ExpectedOutputContains'" -ForegroundColor Red
            Write-Host "    Output: $output" -ForegroundColor DarkGray
            $script:failed++
            return
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
    -ExpectedOutputContains "ts-bridge"

Test-Step -Name "version -v (deprecated flag)" -Block { & $Bin -v; $LASTEXITCODE } `
    -ExpectedOutputContains "ts-bridge"

Test-Step -Name "--help (root)" -Block { & $Bin --help; $LASTEXITCODE } `
    -ExpectedOutputContains "connect" `
    -ExpectedOutputContains "init" `
    -ExpectedOutputContains "status" `
    -ExpectedOutputContains "host"

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

Test-Step -Name "init --help" -Block { & $Bin init --help; $LASTEXITCODE } `
    -ExpectedOutputContains "auth-key"

Test-Step -Name "init --auth-key --target (env format, default)" -Block {
    & $Bin init --auth-key $AuthKey --target $Target; $LASTEXITCODE
}
Test-Step -Name "verify .env file created" -Block {
    if (Test-Path ".env") { $content = Get-Content ".env" -Raw; if ($content -match "TS_TARGET") { exit 0 } else { exit 1 } } else { exit 1 }
}

# Cleanup for next test
Remove-Item ".env" -Force 2>$null

Test-Step -Name "init --format yaml" -Block {
    & $Bin init --auth-key $AuthKey --target $Target --format yaml; $LASTEXITCODE
}
Test-Step -Name "verify YAML file created" -Block {
    if (Test-Path "ts-bridge.yaml") { exit 0 } else { exit 1 }
}

# Cleanup
Remove-Item "ts-bridge.yaml" -Force 2>$null

Test-Step -Name "init --config custom path" -Block {
    & $Bin init --auth-key $AuthKey --target $Target --config "$WorkDir\custom.yaml"; $LASTEXITCODE
}
Test-Step -Name "verify custom path created" -Block {
    if (Test-Path "$WorkDir\custom.yaml") { exit 0 } else { exit 1 }
}
Remove-Item "$WorkDir\custom.yaml" -Force 2>$null

# Test overwrite protection (BUG-001/002/019)
Test-Step -Name "init without --force should fail on existing config" -Block {
    # Create a dummy config first
    New-Item -ItemType File -Force -Path ".env" | Out-Null
    & $Bin init --auth-key $AuthKey --target $Target; $LASTEXITCODE
} -ExpectFailure

Test-Step -Name "init with --force should overwrite existing config" -Block {
    New-Item -ItemType File -Force -Path ".env" | Out-Null
    & $Bin init --auth-key $AuthKey --target $Target --force; $LASTEXITCODE
}
Remove-Item ".env" -Force 2>$null

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
    -ExpectedOutputContains "setup" `
    -ExpectedOutputContains "check"

Test-Step -Name "host setup --help" -Block { & $Bin host setup --help; $LASTEXITCODE } `
    -ExpectedOutputContains "firewall"

Test-Step -Name "host check --help" -Block { & $Bin host check --help; $LASTEXITCODE } `
    -ExpectedOutputContains "json"

# ============================================================================
# 6. CONFIG PRECEDENCE (unit-level check)
# ============================================================================
Write-Host "`n[6/6] CONFIG PRECEDENCE" -ForegroundColor Yellow

# Test: YAML config rejects auth key field (security constraint)
Test-Step -Name "YAML config rejects auth key field" -Block {
    Set-Content -Path "reject.yaml" -Value @"
version: 1
target: 100.0.0.1:3389
auth_key: should-be-rejected
"@
    & $Bin init --config "reject.yaml" --auth-key $AuthKey --target $Target 2>&1; $LASTEXITCODE
} -ExpectedOutputContains "auth"  # Should warn about auth key in YAML

Remove-Item "reject.yaml" -Force 2>$null

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
