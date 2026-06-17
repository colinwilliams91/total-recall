param(
    [string]$BinaryPath,
    [string]$ScratchDir
)

. "$PSScriptRoot/common.ps1" -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Initialize-E2E -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Set-PhaseId "00"

Write-Host "  Phase 00 — Foundation" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host ""

$helpOutput = & $script:TrBin --help 2>&1
$helpText = $helpOutput -join "`n"
$helpExit = $LASTEXITCODE

Assert-ExitCode "0.1a" "--help exits 0" 0 $helpExit

$expectedCommands = @("serve", "init", "config", "status")
$allFound = $true
foreach ($cmd in $expectedCommands) {
    if (-not $helpText.Contains($cmd)) {
        $allFound = $false
        break
    }
}

if ($allFound) {
    Write-Pass "0.1b" "--help lists serve, init, config, status"
} else {
    Write-Fail "0.1b" "--help lists serve, init, config, status" "Missing one or more commands in output"
}

$versionOutput = & $script:TrBin --version 2>&1
$versionText = $versionOutput -join "`n"
$versionExit = $LASTEXITCODE

Assert-ExitCode "0.2a" "--version exits 0" 0 $versionExit
Assert-Match "0.2b" "--version outputs version string" $versionText "total-recall version"

Write-Summary "Phase 00"
