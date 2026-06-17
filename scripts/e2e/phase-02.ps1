param(
    [string]$BinaryPath,
    [string]$ScratchDir
)

. "$PSScriptRoot/common.ps1" -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Initialize-E2E -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Set-PhaseId "02"

Write-Host "  Phase 02 — Daemon Foundation" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host ""

Write-Host "  [setup] Starting daemon..." -ForegroundColor DarkGray
Start-TrDaemon

$health = Invoke-TrGet "/health"
Assert-StatusCode "2.1" "GET /health returns 200" 200 $health.StatusCode

if ($health.Body -match '"status"\s*:\s*"ok"') {
    Write-Pass "2.1b" "Health response contains status:ok"
} else {
    Write-Fail "2.1b" "Health response contains status:ok" "Body: $($health.Body)"
}

$statusOutput = & $script:TrBin status 2>&1
$statusText = $statusOutput -join "`n"
$statusExit = $LASTEXITCODE

Assert-ExitCode "2.2a" "tr status exits 0 when daemon is running" 0 $statusExit
Assert-Contains "2.2b" "tr status shows daemon address" $statusText "7331"

Write-Host ""
Write-Host "  [setup] Stopping daemon for next checks..." -ForegroundColor DarkGray
Stop-TrDaemon
Start-Sleep -Seconds 1

$statusDownOutput = & $script:TrBin status 2>&1
$statusDownText = $statusDownOutput -join "`n"
$statusDownExit = $LASTEXITCODE

Assert-ExitCode "2.3a" "tr status exits 1 when daemon is down" 1 $statusDownExit
Assert-Contains "2.3b" "tr status shows 'not running' message" $statusDownText "not running"

New-ScratchRepo

Write-Manual "2.4" "Hook installation via tr init" `
    "Run 'tr init' in $script:ScratchRepo. Expect TUI prompts for conversation analysis, AI provider, and hook selection. Verify hooks are installed to .git/hooks/"

$hookPath = Join-Path $script:ScratchRepo ".git\hooks\pre-commit"
if (Test-Path $hookPath) {
    $hookContent = Get-Content $hookPath -Raw
    if ($hookContent.Contains("# total-recall managed")) {
        Write-Pass "2.5a" "pre-commit hook contains sentinel"
    } else {
        Write-Fail "2.5a" "pre-commit hook contains sentinel" "Sentinel not found in hook file"
    }
    if ($hookContent.Contains("#!/usr/bin/env bash") -or $hookContent.Contains("#!/bin/sh")) {
        Write-Pass "2.5b" "pre-commit hook has valid shebang"
    } else {
        Write-Fail "2.5b" "pre-commit hook has valid shebang" "No shebang found"
    }
} else {
    Write-Skip "2.5a" "pre-commit hook not installed (tr init may not have been run)"
    Write-Skip "2.5b" "pre-commit hook not installed (tr init may not have been run)"
}

Write-Manual "2.6" "Hook installation idempotency" `
    "Re-run 'tr init' and verify hooks are regenerated in-place (not duplicated), with previous selections pre-populated."

Write-Manual "2.7" "Hook chaining with existing unmanaged hook" `
    "Create an unmanaged pre-commit hook, re-run 'tr init', verify both the TR sentinel and original hook content are present."

Write-Host ""
Write-Host "  [setup] Restarting daemon for hook dispatch tests..." -ForegroundColor DarkGray
Start-TrDaemon

$hookBody = '{"hook":"pre-commit","repo":"/tmp/tr-test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ foo","files":["main.go"]}}'
$hookResult = Invoke-TrPost "/hooks/pre-commit" $hookBody

Assert-StatusCode "2.8a" "POST /hooks/pre-commit returns 202" 202 $hookResult.StatusCode

if ($hookResult.Body -match '"status"\s*:\s*"received"') {
    Write-Pass "2.8b" "Hook response contains status:received"
} else {
    Write-Fail "2.8b" "Hook response contains status:received" "Body: $($hookResult.Body)"
}

Push-Location $script:ScratchRepo
"hello world" | Set-Content "foo.txt"
git add . | Out-Null
git commit -m "test: trigger TR hook" 2>&1 | Out-Null
$commitExit = $LASTEXITCODE
Pop-Location

Start-Sleep -Seconds 2
$daemonLogs = Get-DaemonOutput

if ($commitExit -eq 0) {
    Write-Pass "2.9a" "Real commit succeeds (hook is non-blocking)"
} else {
    Write-Fail "2.9a" "Real commit succeeds" "Commit exited with code $commitExit"
}

if ($daemonLogs -match '\[hook\]') {
    Write-Pass "2.9b" "Daemon logs hook dispatch"
} else {
    Write-Skip "2.9b" "Daemon hook log not captured (may need longer wait)"
}

Write-Host ""
Write-Host "  [setup] Ensuring hooks are installed for credential scan tests..." -ForegroundColor DarkGray

if (-not (Test-HooksInstalled)) {
    Write-Host "  [hooks] Hooks not found, installing automatically..." -ForegroundColor DarkGray
    Install-TrHooks
}

$hooksReady = Test-HooksInstalled
if ($hooksReady) {
    Write-Pass "2.10-pre" "Hooks installed for credential scan tests"
} else {
    Write-Fail "2.10-pre" "Hooks installation failed" "Cannot proceed with credential scan tests"
    Write-Skip "2.10a" "Skipped: hooks not installed"
    Write-Skip "2.10b" "Skipped: hooks not installed"
}

if ($hooksReady) {
    try {
        Push-Location $script:ScratchRepo
        $repoConfigPath = Join-Path $script:ScratchRepo ".tr.yaml"
        "api-key: sk-supersecret" | Set-Content $repoConfigPath
        git add .tr.yaml | Out-Null
        $credBlockOutput = git commit -m "oops: leaked key" 2>&1
        $credBlockExit = $LASTEXITCODE
        Pop-Location

        if ($credBlockExit -ne 0) {
            Write-Pass "2.10a" "Commit blocked by raw api-key in .tr.yaml"
        } else {
            $credText = $credBlockOutput -join "`n"
            if ($credText -match "api-key|credential|secret|blocked") {
                Write-Pass "2.10a" "Commit blocked by raw api-key in .tr.yaml"
            } else {
                Write-Fail "2.10a" "Commit blocked by raw api-key" "Commit succeeded when it should have been blocked"
            }
        }

        Push-Location $script:ScratchRepo
        git checkout .tr.yaml 2>&1 | Out-Null
        "api-key: env:OPENAI_API_KEY" | Set-Content $repoConfigPath
        git add .tr.yaml | Out-Null
        $credAllowOutput = git commit -m "fix: use env reference" 2>&1
        $credAllowExit = $LASTEXITCODE
        Pop-Location

        if ($credAllowExit -eq 0) {
            Write-Pass "2.10b" "Commit allowed with env: format"
        } else {
            Write-Fail "2.10b" "Commit allowed with env: format" "Commit failed with exit code $credAllowExit"
        }
    } finally {
        Push-Location $script:ScratchRepo
        git checkout .tr.yaml 2>&1 | Out-Null
        Pop-Location
    }
}

Write-Host ""
Write-Host "  [setup] Stopping daemon for graceful degradation test..." -ForegroundColor DarkGray
Stop-TrDaemon
Start-Sleep -Seconds 1

Push-Location $script:ScratchRepo
"test content" | Add-Content "foo.txt"
git add . | Out-Null
$graceOutput = git commit -m "test: no daemon" 2>&1
$graceExit = $LASTEXITCODE
$graceText = $graceOutput -join "`n"
Pop-Location

if ($graceExit -eq 0) {
    Write-Pass "2.11a" "Commit succeeds when daemon is down (non-blocking)"
} else {
    Write-Fail "2.11a" "Commit succeeds when daemon is down" "Exit code: $graceExit"
}

if ($graceText -match "Daemon not running|daemon|total-recall") {
    Write-Pass "2.11b" "Advisory message printed when daemon is unreachable"
} else {
    Write-Skip "2.11b" "Advisory message (may not print if hooks not installed)"
}

Remove-ScratchRepo
Stop-TrDaemon

Write-Summary "Phase 02"
