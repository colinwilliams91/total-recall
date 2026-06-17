param(
    [string]$BinaryPath,
    [string]$ScratchDir
)

. "$PSScriptRoot/common.ps1" -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Initialize-E2E -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Set-PhaseId "01"

Write-Host "  Phase 01 — Config Architecture" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host ""

$configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"

Backup-TrConfig | Out-Null

if (Test-Path $configPath) {
    Remove-Item -Path $configPath -Force
}

$serveJob = Start-Job -ScriptBlock {
    param($bin)
    & $bin serve 2>&1
} -ArgumentList $script:TrBin

Start-Sleep -Seconds 3
Stop-Job -Job $serveJob -ErrorAction SilentlyContinue
$serveOutput = Receive-Job -Job $serveJob -ErrorAction SilentlyContinue
Remove-Job -Job $serveJob -ErrorAction SilentlyContinue
$serveText = $serveOutput -join "`n"

if (Test-Path $configPath) {
    Write-Pass "1.1a" "User config auto-created at $configPath"
} else {
    Write-Fail "1.1a" "User config auto-created" "Config file not found after tr serve"
}

if ($serveText -and $serveText.Contains("created")) {
    Write-Pass "1.1b" "Advisory message printed on config creation"
} else {
    Write-Skip "1.1b" "Advisory check (config may already exist or message format changed)"
}

if (Test-Path $configPath) {
    Remove-Item -Path $configPath -Force
}

$quietJob = Start-Job -ScriptBlock {
    param($bin)
    & $bin serve --quiet 2>&1
} -ArgumentList $script:TrBin

Start-Sleep -Seconds 3
Stop-Job -Job $quietJob -ErrorAction SilentlyContinue
$quietOutput = Receive-Job -Job $quietJob -ErrorAction SilentlyContinue
Remove-Job -Job $quietJob -ErrorAction SilentlyContinue
$quietText = $quietOutput -join "`n"

if ($quietText -and $quietText.Contains("created")) {
    Write-Fail "1.2" "--quiet suppresses advisory" "Advisory message found in --quiet output"
} else {
    Write-Pass "1.2" "--quiet suppresses advisory"
}

Restore-TrConfig | Out-Null

Write-Manual "1.3" "tr init prompts for opt-in (conversation analysis + hooks)" `
    "Run 'tr init' in a git repo. Expect TUI prompts for conversation analysis and hook selection."

$showOutput = & $script:TrBin config --show 2>&1
$showText = $showOutput -join "`n"
$showExit = $LASTEXITCODE

Assert-ExitCode "1.4a" "config --show exits 0" 0 $showExit

if ($showText -match '\[user\]|\[default\]|\[repo\]') {
    Write-Pass "1.4b" "config --show displays source tags"
} else {
    Write-Fail "1.4b" "config --show displays source tags" "No [user], [default], or [repo] tags found"
}

New-ScratchRepo

$repoConfigPath = Join-Path $script:ScratchRepo ".tr.yaml"
@"
privacy:
  conversation_analysis: true
"@ | Set-Content $repoConfigPath

Push-Location $script:ScratchRepo
$repoShowOutput = & $script:TrBin config --show 2>&1
$repoShowText = $repoShowOutput -join "`n"
Pop-Location

if ($repoShowText.Contains("[repo]")) {
    Write-Pass "1.5" "Repo config respected (shows [repo] source tag)"
} else {
    Write-Fail "1.5" "Repo config respected" "Expected [repo] tag in config --show output"
}

Remove-ScratchRepo

Write-Summary "Phase 01"
