param(
    [string]$BinaryPath,
    [string]$ScratchDir
)

# common.ps1 — shared helpers for the manual tr init e2e test.
# All daemon/HTTP/assertion helpers were removed when the automated
# e2e phases migrated to Go tests in cmd/tr/*_test.go.

$script:passed  = 0
$script:failed  = 0
$script:skipped = 0
$script:manual  = 0
$script:results = @()

$script:TrBin       = $null
$script:ScratchRepo = $null
$script:startTime   = $null
$script:outputDir   = Join-Path $PSScriptRoot "output"

function Initialize-E2E {
    param([string]$BinaryPath, [string]$ScratchDir)

    $script:startTime = Get-Date

    if ($BinaryPath -and (Test-Path $BinaryPath)) {
        $script:TrBin = (Resolve-Path $BinaryPath).Path
    } elseif (Test-Path "./tr.exe") {
        $script:TrBin = (Resolve-Path "./tr.exe").Path
    } elseif (Test-Path "./bin/tr.exe") {
        $script:TrBin = (Resolve-Path "./bin/tr.exe").Path
    } else {
        Write-Host "[ERROR] Cannot find tr.exe. Build first with: .\scripts\rebuild.ps1" -ForegroundColor Red
        exit 1
    }

    if ($ScratchDir) {
        $script:ScratchRepo = $ScratchDir
    } else {
        $script:ScratchRepo = Join-Path $PSScriptRoot "scratch"
    }

    if (-not (Test-Path $script:outputDir)) {
        New-Item -ItemType Directory -Path $script:outputDir -Force | Out-Null
    }

    Write-Host ""
    Write-Host "  Binary:  $script:TrBin" -ForegroundColor DarkGray
    Write-Host "  Scratch: $script:ScratchRepo" -ForegroundColor DarkGray
    Write-Host ""
}

function Write-Pass {
    param([string]$Id, [string]$Description)
    $script:passed++
    $script:results += @{ Id = $Id; Status = "PASS"; Description = $Description }
    Write-Host "  [PASS] $Id — $Description" -ForegroundColor Green
}

function Write-Fail {
    param([string]$Id, [string]$Description, [string]$Detail = "")
    $script:failed++
    $script:results += @{ Id = $Id; Status = "FAIL"; Description = $Description; Detail = $Detail }
    Write-Host "  [FAIL] $Id — $Description" -ForegroundColor Red
    if ($Detail) { Write-Host "         $Detail" -ForegroundColor DarkRed }
}

function Write-Skip {
    param([string]$Id, [string]$Reason)
    $script:skipped++
    $script:results += @{ Id = $Id; Status = "SKIP"; Description = $Reason }
    Write-Host "  [SKIP] $Id — $Reason" -ForegroundColor Yellow
}

function Write-Manual {
    param(
        [string]$Id,
        [string]$Description,
        [string]$Command,
        [string]$Expected,
        [scriptblock]$Verify
    )
    Write-Host ""
    Write-Host "  [MANUAL] $Id — $Description" -ForegroundColor Cyan
    if ($Command) {
        Write-Host ""
        Write-Host "  Run this command:" -ForegroundColor DarkCyan
        foreach ($line in $Command -split "`n") {
            Write-Host "    $line" -ForegroundColor White
        }
    }
    if ($Expected) {
        Write-Host ""
        Write-Host "  Expected: $Expected" -ForegroundColor DarkCyan
    }
    Write-Host ""
    $response = Read-Host "  Press Enter when done (or 's' to skip)"
    if ($response -eq 's' -or $response -eq 'S') {
        Write-Skip $Id "Skipped by user"
        return
    }
    if ($Verify) {
        Write-Host "  [VERIFY] Running auto-verification..." -ForegroundColor Yellow
        $verifyResult = & $Verify
        if ($verifyResult.Passed) {
            $script:manual++
            $script:results += @{ Id = $Id; Status = "MANUAL"; Description = $Description; Verified = $true }
            Write-Host "  [PASS] $Id — Manually verified + auto-verified" -ForegroundColor Green
        } else {
            Write-Fail $Id "Auto-verification failed" $verifyResult.Detail
        }
    } else {
        $script:manual++
        $script:results += @{ Id = $Id; Status = "MANUAL"; Description = $Description }
        Write-Host "  [PASS] $Id — Manually verified" -ForegroundColor Green
    }
}

function New-ScratchRepo {
    if (Test-Path $script:ScratchRepo) {
        Remove-Item -Path $script:ScratchRepo -Recurse -Force
    }
    New-Item -ItemType Directory -Path $script:ScratchRepo -Force | Out-Null
    Push-Location $script:ScratchRepo
    git init | Out-Null
    git config user.name "E2E Test"
    git config user.email "e2e@test.local"
    Pop-Location
    Write-Host "  [scratch] Created $script:ScratchRepo" -ForegroundColor DarkGray
}

function Remove-ScratchRepo {
    if (Test-Path $script:ScratchRepo) {
        Get-ChildItem -Path $script:ScratchRepo -Recurse -Force | ForEach-Object {
            $_.Attributes = 'Normal'
        }
        Remove-Item -Path $script:ScratchRepo -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Backup-TrConfig {
    $configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
    $backupPath = Join-Path $env:TEMP "tr-config-backup.yaml"
    if (Test-Path $configPath) {
        Copy-Item -Path $configPath -Destination $backupPath -Force
        Write-Host "  [config] Backed up $configPath" -ForegroundColor DarkGray
        return $true
    }
    return $false
}

function Restore-TrConfig {
    $configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
    $backupPath = Join-Path $env:TEMP "tr-config-backup.yaml"
    if (Test-Path $backupPath) {
        Copy-Item -Path $backupPath -Destination $configPath -Force
        Remove-Item -Path $backupPath -Force
        Write-Host "  [config] Restored $configPath" -ForegroundColor DarkGray
        return $true
    }
    return $false
}

function Write-Summary {
    param([string]$PhaseName = "Manual E2E")

    Write-Host ""
    Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
    Write-Host "  $PhaseName Summary" -ForegroundColor White
    Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
    Write-Host "  Passed:  $script:passed" -ForegroundColor Green
    Write-Host "  Failed:  $script:failed" -ForegroundColor $(if ($script:failed -gt 0) { "Red" } else { "Green" })
    Write-Host "  Skipped: $script:skipped" -ForegroundColor Yellow
    Write-Host "  Manual:  $script:manual" -ForegroundColor Cyan
    Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
    Write-Host ""

    Remove-ScratchRepo

    if ($script:failed -gt 0) {
        Write-Host "  FAILURES:" -ForegroundColor Red
        foreach ($r in $script:results) {
            if ($r.Status -eq "FAIL") {
                Write-Host "    $($r.Id): $($r.Description)" -ForegroundColor Red
                if ($r.Detail) { Write-Host "           $($r.Detail)" -ForegroundColor DarkRed }
            }
        }
        Write-Host ""
        exit 1
    }
    exit 0
}
