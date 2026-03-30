#Requires -Version 5.1
<#
.SYNOPSIS
    Runs the Squeeze V1 grammar test suite.

.DESCRIPTION
    Executes all Go tests in the spec_test package (squeeze_v1_grammar_test.go)
    and reports results. Optionally produces verbose output or a JUnit-style
    XML report.

.PARAMETER Verbose
    Print per-test PASS/FAIL lines in addition to the summary.

.PARAMETER Coverage
    Collect code coverage and open the HTML report.

.PARAMETER Filter
    Run only the tests whose names match this substring (maps to -run flag).

.EXAMPLE
    .\run_v1_tests.ps1
    Runs all V1 tests with summary output only.

.EXAMPLE
    .\run_v1_tests.ps1 -Verbose
    Runs all V1 tests with verbose per-test output.

.EXAMPLE
    .\run_v1_tests.ps1 -Filter "TestRange"
    Runs only tests matching "TestRange".

.EXAMPLE
    .\run_v1_tests.ps1 -Coverage
    Runs all tests and opens an HTML coverage report.
#>

[CmdletBinding()]
param(
    [switch]$VerboseOutput,
    [switch]$Coverage,
    [string]$Filter = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------------------
# Locate repo root (parent of spec_test/)
# ---------------------------------------------------------------------------
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$repoRoot  = Split-Path -Parent $scriptDir

Push-Location $repoRoot

try {
    # ---------------------------------------------------------------------------
    # Verify Go toolchain is available
    # ---------------------------------------------------------------------------
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go toolchain not found. Please install Go and ensure it is on your PATH."
        exit 1
    }

    $goVersion = (go version) 2>&1
    Write-Host "Using $goVersion" -ForegroundColor Cyan

    # ---------------------------------------------------------------------------
    # Build argument list
    # ---------------------------------------------------------------------------
    $testArgs = @("test", "./spec_test/...")

    if ($VerboseOutput) {
        $testArgs += "-v"
    }

    if ($Filter -ne "") {
        $testArgs += "-run"
        $testArgs += $Filter
    }

    $coverFile = $null
    if ($Coverage) {
        $coverFile = Join-Path $repoRoot "spec_test\coverage.out"
        $testArgs += "-coverprofile=$coverFile"
        $testArgs += "-covermode=atomic"
    }

    # ---------------------------------------------------------------------------
    # Run tests
    # ---------------------------------------------------------------------------
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host " Squeeze V1 Grammar Tests  $timestamp" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Package : github.com/cfjello/squeeze-ai/spec_test"
    Write-Host "Command : go $($testArgs -join ' ')"
    Write-Host ""

    & go @testArgs
    $exitCode = $LASTEXITCODE

    Write-Host ""
    if ($exitCode -eq 0) {
        Write-Host "Result: ALL TESTS PASSED" -ForegroundColor Green
    } else {
        Write-Host "Result: TESTS FAILED (exit code $exitCode)" -ForegroundColor Red
    }

    # ---------------------------------------------------------------------------
    # Coverage report (optional)
    # ---------------------------------------------------------------------------
    if ($Coverage -and $coverFile -and (Test-Path $coverFile)) {
        $htmlReport = Join-Path $repoRoot "spec_test\coverage.html"
        Write-Host ""
        Write-Host "Generating HTML coverage report: $htmlReport" -ForegroundColor Cyan
        & go tool cover "-html=$coverFile" "-o=$htmlReport"
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Opening coverage report in browser..." -ForegroundColor Cyan
            Start-Process $htmlReport
        }
    }

    exit $exitCode

} finally {
    Pop-Location
}
