param(
    [string]$BinaryPath,
    [string]$ScratchDir
)

$script:passed  = 0
$script:failed  = 0
$script:skipped = 0
$script:manual  = 0
$script:results = @()

$script:TrBin       = $null
$script:ScratchRepo = $null
$script:DaemonProc  = $null
$script:DaemonOutput = @()
$script:startTime   = $null
$script:phaseId     = "unknown"
$script:outputDir   = Join-Path $PSScriptRoot "output"
$script:CleanAfter  = $false

function Initialize-E2E {
    param(
        [string]$BinaryPath,
        [string]$ScratchDir,
        [switch]$Clean
    )

    $script:startTime = Get-Date
    $script:CleanAfter = $Clean.IsPresent

    if ($BinaryPath -and (Test-Path $BinaryPath)) {
        $script:TrBin = (Resolve-Path $BinaryPath).Path
    } elseif (Test-Path "./tr.exe") {
        $script:TrBin = (Resolve-Path "./tr.exe").Path
    } elseif (Test-Path "./total-recall.exe") {
        $script:TrBin = (Resolve-Path "./total-recall.exe").Path
    } elseif (Test-Path "./bin/total-recall.exe") {
        $script:TrBin = (Resolve-Path "./bin/total-recall.exe").Path
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
    Write-Host "  Output:  $script:outputDir" -ForegroundColor DarkGray
    if ($script:CleanAfter) {
        Write-Host "  Clean:   yes (scratch repo will be removed after tests)" -ForegroundColor DarkGray
    }
    Write-Host ""
}

function Set-PhaseId {
    param([string]$Id)
    $script:phaseId = $Id
}

function Write-Pass {
    param([string]$Id, [string]$Description)
    $script:passed++
    $script:results += @{ Id = $Id; Status = "PASS"; Description = $Description; Timestamp = (Get-Date).ToUniversalTime().ToString("o") }
    Write-Host "  [PASS] $Id — $Description" -ForegroundColor Green
}

function Write-Fail {
    param([string]$Id, [string]$Description, [string]$Detail = "")
    $script:failed++
    $script:results += @{ Id = $Id; Status = "FAIL"; Description = $Description; Detail = $Detail; Timestamp = (Get-Date).ToUniversalTime().ToString("o") }
    Write-Host "  [FAIL] $Id — $Description" -ForegroundColor Red
    if ($Detail) {
        Write-Host "         $Detail" -ForegroundColor DarkRed
    }
}

function Write-Skip {
    param([string]$Id, [string]$Reason)
    $script:skipped++
    $script:results += @{ Id = $Id; Status = "SKIP"; Description = $Reason; Timestamp = (Get-Date).ToUniversalTime().ToString("o") }
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
    if ($Verify) {
        Write-Host ""
        Write-Host "  Auto-verification will check config files and hook installation" -ForegroundColor DarkCyan
    }
    Write-Host ""
    $response = Read-Host "  Press Enter when done (or 's' to skip)"
    if ($response -eq 's' -or $response -eq 'S') {
        Write-Skip $Id "Skipped by user"
        return
    }
    if ($Verify) {
        Write-Host ""
        Write-Host "  [VERIFY] Running auto-verification..." -ForegroundColor Yellow
        $verifyResult = & $Verify
        if ($verifyResult.Passed) {
            $script:manual++
            $script:results += @{ Id = $Id; Status = "MANUAL"; Description = $Description; Timestamp = (Get-Date).ToUniversalTime().ToString("o"); Verified = $true }
            Write-Host "  [PASS] $Id — Manually verified + auto-verified" -ForegroundColor Green
        } else {
            Write-Fail $Id "Auto-verification failed" $verifyResult.Detail
        }
    } else {
        $script:manual++
        $script:results += @{ Id = $Id; Status = "MANUAL"; Description = $Description; Timestamp = (Get-Date).ToUniversalTime().ToString("o") }
        Write-Host "  [PASS] $Id — Manually verified" -ForegroundColor Green
    }
}

function Assert-ExitCode {
    param([string]$Id, [string]$Description, [int]$Expected, [int]$Actual)
    if ($Actual -eq $Expected) {
        Write-Pass $Id $Description
    } else {
        Write-Fail $Id $Description "Expected exit code $Expected, got $Actual"
    }
}

function Assert-Contains {
    param([string]$Id, [string]$Description, [string]$Text, [string]$Substring)
    if ($Text -and $Text.Contains($Substring)) {
        Write-Pass $Id $Description
    } else {
        Write-Fail $Id $Description "Expected output to contain '$Substring'"
    }
}

function Assert-NotContains {
    param([string]$Id, [string]$Description, [string]$Text, [string]$Substring)
    if (-not $Text -or -not $Text.Contains($Substring)) {
        Write-Pass $Id $Description
    } else {
        Write-Fail $Id $Description "Expected output to NOT contain '$Substring'"
    }
}

function Assert-Match {
    param([string]$Id, [string]$Description, [string]$Text, [string]$Pattern)
    if ($Text -match $Pattern) {
        Write-Pass $Id $Description
    } else {
        Write-Fail $Id $Description "Expected output to match pattern '$Pattern'"
    }
}

function Assert-StatusCode {
    param([string]$Id, [string]$Description, $Expected, $Actual)
    if ($Actual -eq $Expected) {
        Write-Pass $Id $Description
    } else {
        Write-Fail $Id $Description "Expected HTTP $Expected, got HTTP $Actual"
    }
}

function Start-TrDaemon {
    param([int]$TimeoutSec = 30)

    if ($script:DaemonProc) {
        Stop-TrDaemon
    }

    $script:DaemonProc = Start-Process -FilePath $script:TrBin `
        -ArgumentList "serve" `
        -WorkingDirectory (Split-Path $script:TrBin -Parent) `
        -PassThru `
        -WindowStyle Hidden

    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500

        if ($script:DaemonProc.HasExited) {
            Write-Fail "DAEMON" "Daemon process exited prematurely" "Exit code: $($script:DaemonProc.ExitCode)"
            $script:DaemonProc = $null
            return
        }

        try {
            $r = Invoke-WebRequest -Uri "http://localhost:7331/health" -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
            if ($r.StatusCode -eq 200) {
                Write-Host "  [daemon] Started (PID: $($script:DaemonProc.Id))" -ForegroundColor DarkGray
                return
            }
        } catch {
        }
    }

    Write-Fail "DAEMON" "Daemon failed to start within ${TimeoutSec}s" "Check that port 7331 is not in use"
    Stop-TrDaemon
}

function Stop-TrDaemon {
    if ($script:DaemonProc -and -not $script:DaemonProc.HasExited) {
        $script:DaemonProc.Kill()
        $script:DaemonProc.WaitForExit(5000)
    }
    $script:DaemonProc = $null

    $procs = Get-Process -Name "tr" -ErrorAction SilentlyContinue | Where-Object {
        $_.Path -eq $script:TrBin
    }
    if ($procs) {
        $procs | Stop-Process -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 500
    }
}

function Get-DaemonOutput {
    return ($script:DaemonOutput -join "`n")
}

function Test-DaemonRunning {
    try {
        $r = Invoke-WebRequest -Uri "http://localhost:7331/health" -TimeoutSec 5 -UseBasicParsing -ErrorAction Stop
        return ($r.StatusCode -eq 200)
    } catch {
        return $false
    }
}

function Invoke-TrGet {
    param([string]$Path)
    try {
        $r = Invoke-WebRequest -Uri "http://localhost:7331$Path" -UseBasicParsing -SkipHttpErrorCheck -ErrorAction Stop
        return @{
            StatusCode = $r.StatusCode
            Body       = $r.Content
        }
    } catch {
        return @{
            StatusCode = 0
            Body       = $_.Exception.Message
        }
    }
}

function Invoke-TrPost {
    param([string]$Path, [string]$Body)
    try {
        $r = Invoke-WebRequest -Uri "http://localhost:7331$Path" `
            -Method POST `
            -ContentType "application/json" `
            -Body $Body `
            -UseBasicParsing `
            -SkipHttpErrorCheck `
            -ErrorAction Stop
        return @{
            StatusCode = $r.StatusCode
            Body       = $r.Content
        }
    } catch {
        return @{
            StatusCode = 0
            Body       = $_.Exception.Message
        }
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

function Cleanup-ScratchRepo {
    if ($script:CleanAfter) {
        Remove-ScratchRepo
        Write-Host "  [clean] Removed scratch repo" -ForegroundColor DarkGray
    }
}

function Install-TrHooks {
    param([string]$RepoPath = $script:ScratchRepo)

    $hookDir = Join-Path $RepoPath ".git\hooks"
    if (-not (Test-Path $hookDir)) {
        New-Item -ItemType Directory -Path $hookDir -Force | Out-Null
    }

    $hooksDir = Join-Path (Split-Path $script:TrBin -Parent) "hooks"
    $hookNames = @("pre-commit", "commit-msg", "pre-push")

    foreach ($hookName in $hookNames) {
        $sourceSh = Join-Path $hooksDir "$hookName.sh"
        $sourceBat = Join-Path $hooksDir "$hookName.bat"
        $destSh = Join-Path $hookDir $hookName
        $destBat = Join-Path $hookDir "$hookName.bat"

        if (Test-Path $sourceSh) {
            Copy-Item -Path $sourceSh -Destination $destSh -Force
            if ($IsWindows -or $env:OS -eq "Windows_NT") {
                icacls $destSh /inheritance:r /grant:r "${env:USERNAME}:(RX)" 2>$null | Out-Null
            }
        }

        if (Test-Path $sourceBat) {
            Copy-Item -Path $sourceBat -Destination $destBat -Force
        }
    }

    Write-Host "  [hooks] Installed hooks to $hookDir" -ForegroundColor DarkGray
}

function Test-HooksInstalled {
    param([string]$RepoPath = $script:ScratchRepo)

    $preCommitPath = Join-Path $RepoPath ".git\hooks\pre-commit"
    if (-not (Test-Path $preCommitPath)) {
        return $false
    }

    $content = Get-Content $preCommitPath -Raw -ErrorAction SilentlyContinue
    if (-not $content) {
        return $false
    }

    return ($content.Contains("# total-recall managed") -or $content.Contains("total-recall"))
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

function Read-TrConfig {
    $configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
    if (-not (Test-Path $configPath)) {
        return $null
    }

    $content = Get-Content $configPath -Raw
    $result = @{
        Provider = ""
        Model    = ""
        ApiKey   = ""
        BaseUrl  = ""
    }

    if ($content -match 'provider:\s*(.+)') {
        $result.Provider = $Matches[1].Trim()
    }
    if ($content -match 'model:\s*(.+)') {
        $result.Model = $Matches[1].Trim()
    }
    if ($content -match 'api-key:\s*(.+)') {
        $result.ApiKey = $Matches[1].Trim()
    }
    if ($content -match 'base-url:\s*(.*)') {
        $result.BaseUrl = $Matches[1].Trim()
    }

    return $result
}

function Resolve-ApiKey {
    $cfg = Read-TrConfig
    if (-not $cfg) {
        return $false
    }

    $keyRef = $cfg.ApiKey
    if ($keyRef -match '^env:(.+)$') {
        $envVar = $Matches[1]
        $value = [System.Environment]::GetEnvironmentVariable($envVar)
        return (-not [string]::IsNullOrWhiteSpace($value))
    }

    return (-not [string]::IsNullOrWhiteSpace($keyRef))
}

function Write-JsonReport {
    param([string]$PhaseName)

    $endTime = Get-Date
    $durationSec = 0
    if ($script:startTime) {
        $durationSec = [math]::Round(($endTime - $script:startTime).TotalSeconds, 1)
    }

    $daemonLog = ""
    if ($script:DaemonOutput.Count -gt 0) {
        $daemonLog = $script:DaemonOutput -join "`n"
    }

    $report = [ordered]@{
        phase      = $script:phaseId
        phase_name = $PhaseName
        binary     = $script:TrBin
        scratch    = $script:ScratchRepo
        started_at = if ($script:startTime) { $script:startTime.ToUniversalTime().ToString("o") } else { "" }
        ended_at   = $endTime.ToUniversalTime().ToString("o")
        duration_sec = $durationSec
        summary    = [ordered]@{
            passed  = $script:passed
            failed  = $script:failed
            skipped = $script:skipped
            manual  = $script:manual
            total   = $script:passed + $script:failed + $script:skipped + $script:manual
        }
        exit_code  = if ($script:failed -gt 0) { 1 } else { 0 }
        results    = @()
        daemon_log = $daemonLog
    }

    foreach ($r in $script:results) {
        $entry = [ordered]@{
            id          = $r.Id
            status      = $r.Status
            description = $r.Description
            timestamp   = $r.Timestamp
        }
        if ($r.Detail) {
            $entry.detail = $r.Detail
        }
        $report.results += $entry
    }

    $json = $report | ConvertTo-Json -Depth 5

    $fileName = "phase-$($script:phaseId).json"
    $filePath = Join-Path $script:outputDir $fileName
    $json | Set-Content -Path $filePath -Encoding UTF8 -Force

    Write-Host "  [report] $filePath" -ForegroundColor DarkGray
}

function Write-Summary {
    param([string]$PhaseName = "E2E")

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

    Write-JsonReport $PhaseName
    Cleanup-ScratchRepo

    if ($script:failed -gt 0) {
        Write-Host "  FAILURES:" -ForegroundColor Red
        foreach ($r in $script:results) {
            if ($r.Status -eq "FAIL") {
                Write-Host "    $($r.Id): $($r.Description)" -ForegroundColor Red
                if ($r.Detail) {
                    Write-Host "           $($r.Detail)" -ForegroundColor DarkRed
                }
            }
        }
        Write-Host ""
        exit 1
    }

    exit 0
}
