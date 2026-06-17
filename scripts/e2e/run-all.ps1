param(
    [string[]]$Phases,
    [string]$BinaryPath,
    [string]$ScratchDir,
    [string]$Provider,
    [string]$ApiKey,
    [string]$Model,
    [string]$BaseUrl,
    [switch]$ContinueOnError
)

$runStartTime = Get-Date
$allPhases = @("00", "01", "02", "03", "04a")
$outputDir = Join-Path $PSScriptRoot "output"

if (-not $Phases -or $Phases.Count -eq 0) {
    $Phases = $allPhases
}

if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
}

Write-Host ""
Write-Host "  Total Recall -- E2E Test Runner" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host "  Phases: $($Phases -join ', ')" -ForegroundColor DarkGray
Write-Host "  Halt on first failure: $(if ($ContinueOnError) { 'No' } else { 'Yes' })" -ForegroundColor DarkGray
Write-Host ""

$completedPhases = @()
$failedPhases = @()

foreach ($phase in $Phases) {
    $phaseLower = $phase.ToLower()
    $scriptName = "phase-$phaseLower.ps1"
    $scriptPath = Join-Path $PSScriptRoot $scriptName

    if (-not (Test-Path $scriptPath)) {
        Write-Host "  [ERROR] Phase script not found: $scriptPath" -ForegroundColor Red
        $failedPhases += $phase
        if (-not $ContinueOnError) {
            break
        }
        continue
    }

    Write-Host ""
    Write-Host "  ===========================================" -ForegroundColor Cyan
    Write-Host "  |  Running Phase $phase" -ForegroundColor Cyan
    Write-Host "  ===========================================" -ForegroundColor Cyan
    Write-Host ""

    $phaseArgs = @()
    if ($BinaryPath) { $phaseArgs += "-BinaryPath"; $phaseArgs += $BinaryPath }
    if ($ScratchDir) { $phaseArgs += "-ScratchDir"; $phaseArgs += $ScratchDir }

    if ($phaseLower -eq "03" -or $phaseLower -eq "04a") {
        if ($Provider) { $phaseArgs += "-Provider"; $phaseArgs += $Provider }
        if ($ApiKey) { $phaseArgs += "-ApiKey"; $phaseArgs += $ApiKey }
    }
    if ($phaseLower -eq "03") {
        if ($Model) { $phaseArgs += "-Model"; $phaseArgs += $Model }
        if ($BaseUrl) { $phaseArgs += "-BaseUrl"; $phaseArgs += $BaseUrl }
    }

    & $scriptPath @phaseArgs
    $phaseExit = $LASTEXITCODE

    if ($phaseExit -eq 0) {
        $completedPhases += $phase
        Write-Host "  [OK] Phase $phase completed" -ForegroundColor Green
    } else {
        $failedPhases += $phase
        Write-Host "  [FAIL] Phase $phase failed" -ForegroundColor Red

        if (-not $ContinueOnError) {
            Write-Host ""
            Write-Host "  Halting due to phase failure. Use -ContinueOnError to run all phases." -ForegroundColor Yellow
            break
        }
    }
}

$runEndTime = Get-Date
$runDurationSec = [math]::Round(($runEndTime - $runStartTime).TotalSeconds, 1)

Write-Host ""
Write-Host "  ===========================================" -ForegroundColor White
Write-Host "  |  E2E Test Run Summary" -ForegroundColor White
Write-Host "  ===========================================" -ForegroundColor White
Write-Host ""
Write-Host "  Completed phases: $($completedPhases -join ', ')" -ForegroundColor Green

if ($failedPhases.Count -gt 0) {
    Write-Host "  Failed phases:      $($failedPhases -join ', ')" -ForegroundColor Red
}

Write-Host ""

$runAllPhases = @()
$totalPassed = 0
$totalFailed = 0
$totalSkipped = 0
$totalManual = 0

$allRunPhases = $completedPhases + $failedPhases
foreach ($p in $allRunPhases) {
    $phaseJsonPath = Join-Path $outputDir "phase-$($p.ToLower()).json"
    if (Test-Path $phaseJsonPath) {
        $phaseJson = Get-Content $phaseJsonPath -Raw | ConvertFrom-Json
        $runAllPhases += $phaseJson
        $totalPassed  += $phaseJson.summary.passed
        $totalFailed  += $phaseJson.summary.failed
        $totalSkipped += $phaseJson.summary.skipped
        $totalManual  += $phaseJson.summary.manual
    }
}

$runReport = [ordered]@{
    run_type     = "all"
    phases_run   = $Phases
    started_at   = $runStartTime.ToUniversalTime().ToString("o")
    ended_at     = $runEndTime.ToUniversalTime().ToString("o")
    duration_sec = $runDurationSec
    summary      = [ordered]@{
        passed  = $totalPassed
        failed  = $totalFailed
        skipped = $totalSkipped
        manual  = $totalManual
        total   = $totalPassed + $totalFailed + $totalSkipped + $totalManual
    }
    completed_phases = $completedPhases
    failed_phases    = $failedPhases
    exit_code        = if ($failedPhases.Count -gt 0) { 1 } else { 0 }
    phases           = $runAllPhases
}

$runJson = $runReport | ConvertTo-Json -Depth 10
$runJsonPath = Join-Path $outputDir "run-all.json"
$runJson | Set-Content -Path $runJsonPath -Encoding UTF8 -Force

Write-Host "  [report] $runJsonPath" -ForegroundColor DarkGray
Write-Host ""

if ($failedPhases.Count -gt 0) {
    Write-Host "  E2E run FAILED" -ForegroundColor Red
    exit 1
} else {
    Write-Host "  E2E run PASSED" -ForegroundColor Green
    exit 0
}
