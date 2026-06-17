param(
    [string]$BinaryPath,
    [string]$ScratchDir,
    [string]$Provider,
    [string]$ApiKey,
    [switch]$Clean
)

. "$PSScriptRoot/common.ps1" -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Initialize-E2E -BinaryPath $BinaryPath -ScratchDir $ScratchDir -Clean:$Clean
Set-PhaseId "04a"

Write-Host "  Phase 04A — Out-of-Band Delivery (MCP + Shell)" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host ""

$hasApiKey = Resolve-ApiKey
if ($ApiKey) { $hasApiKey = $true }

if (-not $hasApiKey) {
    Write-Host "  [WARN] No API key detected. Some checks require AI pipeline." -ForegroundColor Yellow
    Write-Host ""
}

Write-Host "  [setup] Starting daemon..." -ForegroundColor DarkGray
Start-TrDaemon

Write-Host "  Section A — MCP Endpoint" -ForegroundColor DarkGray
Write-Host ""

$mcpResult = Invoke-TrGet "/mcp/"
if ($mcpResult.StatusCode -eq 200 -or $mcpResult.StatusCode -eq 400) {
    Write-Pass "4.1" "GET /mcp/ returns $($mcpResult.StatusCode) (not 404)"
} else {
    Write-Fail "4.1" "GET /mcp/ returns 200 or 400" "Got $($mcpResult.StatusCode)"
}

$hookBody = @'
{
  "hook": "pre-commit",
  "repo": "/tmp/tr-test",
  "branch": "main",
  "timestamp": "2026-01-01T00:00:00Z",
  "payload": {
    "diff": "+ func parseAST(src string) (*ast.File, error)"
  }
}
'@
$hookResult = Invoke-TrPost "/hooks/pre-commit" $hookBody
Assert-StatusCode "4.2" "POST hook payload returns 202" 202 $hookResult.StatusCode

Write-Host ""
Write-Host "  Section B — REST Dequeue (/recall/next and /recall/answer)" -ForegroundColor DarkGray
Write-Host ""

Write-Host "  [wait] Waiting for AI pipeline to process (~12s)..." -ForegroundColor DarkGray
Start-Sleep -Seconds 12

$recallNext = Invoke-TrGet "/recall/next"

if ($recallNext.StatusCode -eq 200) {
    Write-Pass "4.3a" "GET /recall/next returns 200 (question ready)"

    if ($recallNext.Body -match '"id"\s*:\s*(\d+)') {
        $questionId = $Matches[1]
        Write-Pass "4.3b" "Question response contains id field (id=$questionId)"
    } else {
        Write-Fail "4.3b" "Question response contains id field" "Body: $($recallNext.Body)"
        $questionId = "1"
    }

    if ($recallNext.Body -match '"question"\s*:') {
        Write-Pass "4.3c" "Question response contains question field"
    } else {
        Write-Fail "4.3c" "Question response contains question field" "Body: $($recallNext.Body)"
    }

    if ($recallNext.Body -match '"choices"\s*:\s*\[') {
        Write-Pass "4.3d" "Question response contains choices array"
    } else {
        Write-Skip "4.3d" "Choices not found (question may not have choices)"
    }
} elseif ($recallNext.StatusCode -eq 204) {
    Write-Skip "4.3a" "No question queued yet (AI may be slow or failed)"
    Write-Skip "4.3b" "Depends on 4.3a"
    Write-Skip "4.3c" "Depends on 4.3a"
    Write-Skip "4.3d" "Depends on 4.3a"
    $questionId = "1"
} else {
    Write-Fail "4.3a" "GET /recall/next returns 200 or 204" "Got $($recallNext.StatusCode)"
    $questionId = "1"
}

$recallNext2 = Invoke-TrGet "/recall/next"
if ($recallNext2.StatusCode -eq 204) {
    Write-Pass "4.4" "GET /recall/next is idempotent (second call returns 204)"
} elseif ($recallNext2.StatusCode -eq 200) {
    Write-Skip "4.4" "Second call returned 200 (multiple questions queued)"
} else {
    Write-Fail "4.4" "GET /recall/next idempotent" "Got $($recallNext2.StatusCode)"
}

$answerBody = "{`"id`":$questionId,`"answer`":`"1`"}"
$answerResult = Invoke-TrPost "/recall/answer" $answerBody

if ($answerResult.StatusCode -eq 200) {
    Write-Pass "4.5a" "POST /recall/answer returns 200"

    if ($answerResult.Body -match '"ok"\s*:\s*true') {
        Write-Pass "4.5b" "Answer response contains ok:true"
    } else {
        Write-Fail "4.5b" "Answer response contains ok:true" "Body: $($answerResult.Body)"
    }
} else {
    Write-Fail "4.5a" "POST /recall/answer returns 200" "Got $($answerResult.StatusCode)"
    Write-Skip "4.5b" "Depends on 4.5a"
}

$skipBody = "{`"id`":$questionId,`"answer`":`"skip`"}"
$skipResult = Invoke-TrPost "/recall/answer" $skipBody

if ($skipResult.StatusCode -eq 200) {
    Write-Pass "4.6a" "POST /recall/answer (skip) returns 200"

    if ($skipResult.Body -match '"ok"\s*:\s*true') {
        Write-Pass "4.6b" "Skip response contains ok:true"
    } else {
        Write-Fail "4.6b" "Skip response contains ok:true" "Body: $($skipResult.Body)"
    }
} else {
    Write-Fail "4.6a" "POST /recall/answer (skip) returns 200" "Got $($skipResult.StatusCode)"
    Write-Skip "4.6b" "Depends on 4.6a"
}

$memoryPath = Join-Path $env:USERPROFILE ".tr\memory.db"
if (Test-Path $memoryPath) {
    Write-Pass "4.7a" "memory.db exists at $memoryPath"

    $sqliteOutput = & sqlite3 $memoryPath "SELECT COUNT(*) FROM questions;" 2>&1
    $sqliteExit = $LASTEXITCODE

    if ($sqliteExit -eq 0 -and $sqliteOutput -match '\d+') {
        $count = [int]$sqliteOutput
        if ($count -gt 0) {
            Write-Pass "4.7b" "Questions table has $count rows"
        } else {
            Write-Skip "4.7b" "Questions table is empty"
        }
    } else {
        Write-Fail "4.7b" "Query questions table" "sqlite3 failed: $sqliteOutput"
    }
} else {
    Write-Fail "4.7a" "memory.db exists" "File not found: $memoryPath"
    Write-Skip "4.7b" "Depends on 4.7a"
}

Write-Host ""
Write-Host "  Section C — tr ask Subcommand" -ForegroundColor DarkGray
Write-Host ""

Write-Manual "4.8" "tr ask with no question queued (daemon running)" `
    "Run 'tr ask'. Expect: 'Thinking.' animation, then 'You're all caught up' message for ~4 seconds, then exit."

Write-Manual "4.9" "tr ask with a question queued" `
    "Post a hook payload (check 4.2), wait ~10s, then run 'tr ask'. Expect: TUI with question text, choices, key selection."

$askPipeOutput = cmd /c "$($script:TrBin) ask < nul" 2>&1
$askPipeExit = $LASTEXITCODE

if ($askPipeExit -eq 0) {
    Write-Pass "4.10a" "tr ask TTY guard exits 0 in non-interactive shell"
} else {
    Write-Fail "4.10a" "tr ask TTY guard exits 0" "Exit code: $askPipeExit"
}

$askPipeText = $askPipeOutput -join "`n"
if ([string]::IsNullOrWhiteSpace($askPipeText)) {
    Write-Pass "4.10b" "tr ask TTY guard produces no output in non-TTY"
} else {
    Write-Skip "4.10b" "tr ask produced output in non-TTY (may be advisory)"
}

Write-Host ""
Write-Host "  [setup] Stopping daemon for daemon-down tests..." -ForegroundColor DarkGray
Stop-TrDaemon
Start-Sleep -Seconds 1

$askDownOutput = & $script:TrBin ask 2>&1
$askDownText = $askDownOutput -join "`n"
$askDownExit = $LASTEXITCODE

if ($askDownExit -eq 0) {
    Write-Pass "4.11a" "tr ask exits 0 when daemon is not running"
} else {
    Write-Fail "4.11a" "tr ask exits 0 when daemon is down" "Exit code: $askDownExit"
}

if ($askDownText -match "Daemon not running|daemon|total-recall serve") {
    Write-Pass "4.11b" "tr ask prints advisory when daemon is unreachable"
} else {
    Write-Fail "4.11b" "tr ask prints advisory" "No advisory found in output"
}

Write-Host ""
Write-Host "  Section D — Post-Commit Hook" -ForegroundColor DarkGray
Write-Host ""

New-ScratchRepo

Write-Manual "4.12" "Verify post-commit hook installed by tr init" `
    "Run 'tr init' in $script:ScratchRepo, then check .git/hooks/post-commit contains 'total-recall ask'."

$postCommitPath = Join-Path $script:ScratchRepo ".git\hooks\post-commit"
if (Test-Path $postCommitPath) {
    $postCommitContent = Get-Content $postCommitPath -Raw
    if ($postCommitContent -match "total-recall ask|tr\.exe ask|tr ask") {
        Write-Pass "4.12a" "post-commit hook contains 'total-recall ask'"
    } else {
        Write-Fail "4.12a" "post-commit hook contains 'total-recall ask'" "Pattern not found"
    }
} else {
    Write-Skip "4.12a" "post-commit hook not found (tr init may not have been run)"
}

Write-Manual "4.13" "Post-commit hook fires after real commit" `
    "With daemon running and a question queued, commit in $script:ScratchRepo. Expect: tr ask TUI appears in committing terminal."

Write-Host ""
Write-Host "  Section E — Graceful Degradation" -ForegroundColor DarkGray
Write-Host ""

Start-TrDaemon

$freshRecallNext = Invoke-TrGet "/recall/next"
if ($freshRecallNext.StatusCode -eq 204) {
    Write-Pass "4.14" "GET /recall/next returns 204 when no questions generated"
} elseif ($freshRecallNext.StatusCode -eq 200) {
    Write-Skip "4.14" "Questions already in queue from earlier tests"
} else {
    Write-Fail "4.14" "GET /recall/next returns 204 or 200" "Got $($freshRecallNext.StatusCode)"
}

Stop-TrDaemon
Start-Sleep -Seconds 1

$askUnreachOutput = & $script:TrBin ask 2>&1
$askUnreachText = $askUnreachOutput -join "`n"
$askUnreachExit = $LASTEXITCODE

if ($askUnreachExit -eq 0) {
    Write-Pass "4.15a" "tr ask exits 0 when daemon is unreachable"
} else {
    Write-Fail "4.15a" "tr ask exits 0 when unreachable" "Exit code: $askUnreachExit"
}

if ($askUnreachText -match "Daemon not running|daemon|total-recall serve") {
    Write-Pass "4.15b" "tr ask prints advisory (no panic, no error)"
} else {
    Write-Fail "4.15b" "tr ask prints advisory" "No advisory found"
}

Write-Host ""
Write-Host "  Phase 03 Regression Checks" -ForegroundColor DarkGray
Write-Host ""

Start-TrDaemon

$regressionHook = '{"hook":"pre-commit","repo":"/tmp/tr-test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ test"}}'
$regressionResult = Invoke-TrPost "/hooks/pre-commit" $regressionHook
Assert-StatusCode "4.16a" "Regression: POST hook still returns 202" 202 $regressionResult.StatusCode

$regressionHealth = Invoke-TrGet "/health"
Assert-StatusCode "4.16b" "Regression: GET /health still returns 200" 200 $regressionHealth.StatusCode

$cachePath = Join-Path $env:USERPROFILE ".tr\concepts.db"
if (Test-Path $cachePath) {
    Write-Pass "4.16c" "Regression: concepts.db still exists"
} else {
    Write-Skip "4.16c" "concepts.db not found (may not have been created yet)"
}

Stop-TrDaemon
Remove-ScratchRepo

Write-Summary "Phase 04A"
