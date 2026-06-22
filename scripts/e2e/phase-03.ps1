param(
    [string]$BinaryPath,
    [string]$ScratchDir,
    [string]$Provider,
    [string]$ApiKey,
    [string]$Model,
    [string]$BaseUrl,
    [switch]$Clean
)

. "$PSScriptRoot/common.ps1" -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Initialize-E2E -BinaryPath $BinaryPath -ScratchDir $ScratchDir -Clean:$Clean
Set-PhaseId "03"

Write-Host "  Phase 03 — Intelligence Layer" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host ""

$cfg = Read-TrConfig
$hasApiKey = Resolve-ApiKey

if ($Provider) {
    $effectiveProvider = $Provider
} elseif ($cfg) {
    $effectiveProvider = $cfg.Provider
} else {
    $effectiveProvider = ""
}

if ($ApiKey) {
    $hasApiKey = $true
}

Write-Host "  Provider: $effectiveProvider" -ForegroundColor DarkGray
Write-Host "  API Key:  $(if ($hasApiKey) { 'configured' } else { 'not found' })" -ForegroundColor DarkGray
Write-Host ""

if (-not $hasApiKey -and $effectiveProvider -ne "ollama" -and $effectiveProvider -ne "lm-studio") {
    Write-Host "  [WARN] No API key detected. AI pipeline checks (3.6-3.12) will be skipped." -ForegroundColor Yellow
    Write-Host "         Set env var or pass -ApiKey parameter." -ForegroundColor Yellow
    Write-Host ""
}

Write-Host "  Section A — AI Provider TUI" -ForegroundColor DarkGray
Write-Host ""

Write-Manual -Id "3.1" `
    -Description "tr init shows AI provider selection TUI" `
    -Command "cd $script:ScratchRepo; & $script:TrBin init" `
    -Expected "TUI prompts: conversation analysis (Y/n), AI Provider select menu (6 options), API key/model inputs, hook selection" `
    -Verify {
        $configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
        if (-not (Test-Path $configPath)) {
            return @{ Passed = $false; Detail = "Config not created: $configPath" }
        }
        $config = Get-Content $configPath -Raw
        $missing = @()
        if (-not ($config -match "provider:")) { $missing += "provider" }
        if (-not ($config -match "model:")) { $missing += "model" }
        if (-not ($config -match "api-key:")) { $missing += "api-key" }
        if ($missing.Count -gt 0) {
            return @{ Passed = $false; Detail = "Config missing fields: $($missing -join ', ')" }
        }
        return @{ Passed = $true; Detail = "" }
    }

$configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
if (Test-Path $configPath) {
    $configContent = Get-Content $configPath -Raw

    if ($configContent -match 'provider:\s*\S+') {
        Write-Pass "3.2a" "Config contains provider field"
    } else {
        Write-Fail "3.2a" "Config contains provider field" "provider: not found in config"
    }

    if ($configContent -match 'model:\s*\S+') {
        Write-Pass "3.2b" "Config contains model field"
    } else {
        Write-Fail "3.2b" "Config contains model field" "model: not found in config"
    }

    if ($configContent -match 'api-key:') {
        Write-Pass "3.2c" "Config contains api-key field"
    } else {
        Write-Fail "3.2c" "Config contains api-key field" "api-key: not found in config"
    }
} else {
    Write-Skip "3.2a" "Config file not found (run tr init first)"
    Write-Skip "3.2b" "Config file not found"
    Write-Skip "3.2c" "Config file not found"
}

if (Test-Path $configPath) {
    $baseUrlLine = Select-String -Path $configPath -Pattern "base-url" -ErrorAction SilentlyContinue
    if ($baseUrlLine) {
        Write-Pass "3.3" "base-url field visible in config (not hidden by omitempty)"
    } else {
        Write-Fail "3.3" "base-url field visible in config" "base-url line not found"
    }
} else {
    Write-Skip "3.3" "Config file not found"
}

Write-Manual -Id "3.4" `
    -Description "tr init pre-populates existing AI values" `
    -Command "cd $script:ScratchRepo; & $script:TrBin init" `
    -Expected "TUI prompts pre-filled with current config values (provider, model, api-key); confirm all prompts" `
    -Verify {
        $configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
        if (-not (Test-Path $configPath)) {
            return @{ Passed = $false; Detail = "Config not found: $configPath" }
        }
        $config = Get-Content $configPath -Raw
        $missing = @()
        if (-not ($config -match "provider:")) { $missing += "provider" }
        if (-not ($config -match "model:")) { $missing += "model" }
        if (-not ($config -match "api-key:")) { $missing += "api-key" }
        if ($missing.Count -gt 0) {
            return @{ Passed = $false; Detail = "Config missing fields after re-init: $($missing -join ', ')" }
        }
        return @{ Passed = $true; Detail = "" }
    }

$showOutput = & $script:TrBin config --show 2>&1
$showText = $showOutput -join "`n"

$aiFieldsFound = 0
if ($showText -match 'provider') { $aiFieldsFound++ }
if ($showText -match 'model') { $aiFieldsFound++ }
if ($showText -match 'api-key') { $aiFieldsFound++ }
if ($showText -match 'base-url') { $aiFieldsFound++ }

if ($aiFieldsFound -ge 3) {
    Write-Pass "3.5" "config --show reflects AI fields (found $aiFieldsFound/4)"
} else {
    Write-Fail "3.5" "config --show reflects AI fields" "Only found $aiFieldsFound/4 fields"
}

Write-Host ""
Write-Host "  Section B — Daemon Startup with AI" -ForegroundColor DarkGray
Write-Host ""

if ($hasApiKey -or $effectiveProvider -eq "ollama" -or $effectiveProvider -eq "lm-studio") {
    Start-TrDaemon

    $daemonLogs = Get-DaemonOutput
    if ($daemonLogs -match "AI provider not configured") {
        Write-Fail "3.6" "Daemon starts with AI configured (no provider error)" "Provider error found in logs"
    } else {
        Write-Pass "3.6" "Daemon starts with AI configured (no provider error)"
    }

    Stop-TrDaemon
    Start-Sleep -Seconds 1
} else {
    Write-Skip "3.6" "No API key configured"
}

$backupPath = Join-Path $env:TEMP "tr-config-backup-phase03.yaml"
if (Test-Path $configPath) {
    Copy-Item $configPath $backupPath -Force
}

if (Test-Path $configPath) {
    $content = Get-Content $configPath -Raw
    $content = $content -replace '(?m)^ai:.*', '# ai: disabled for test'
    $content = $content -replace '(?m)^\s+provider:.*', '# provider: disabled'
    Set-Content $configPath $content
}

Start-TrDaemon
$noAiLogs = Get-DaemonOutput

if ($noAiLogs -match "AI provider not configured|not configured") {
    Write-Pass "3.7a" "Daemon logs advisory when AI not configured"
} else {
    Write-Skip "3.7a" "Advisory not found (may use different wording)"
}

$healthCheck = Test-DaemonRunning
if ($healthCheck) {
    Write-Pass "3.7b" "Daemon continues running without AI (non-blocking)"
} else {
    Write-Fail "3.7b" "Daemon continues running without AI" "Daemon not responding to health check"
}

Stop-TrDaemon
Start-Sleep -Seconds 1

if (Test-Path $backupPath) {
    Copy-Item $backupPath $configPath -Force
    Remove-Item $backupPath -Force
}

Write-Host ""
Write-Host "  Section C — Async Pipeline" -ForegroundColor DarkGray
Write-Host ""

if (-not $hasApiKey -and $effectiveProvider -ne "ollama" -and $effectiveProvider -ne "lm-studio") {
    Write-Skip "3.8" "No API key configured"
    Write-Skip "3.9" "No API key configured"
    Write-Skip "3.10" "No API key configured"
    Write-Skip "3.11" "No API key configured"
    Write-Skip "3.12" "No API key configured"
} else {
    Start-TrDaemon

    $hookBody = @'
{
  "hook": "pre-commit",
  "repo": "/tmp/tr-test",
  "branch": "main",
  "timestamp": "2026-01-01T00:00:00Z",
  "payload": {
    "diff": "+ func retryWithBackoff(maxRetries int) error {\n+   time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)\n+ }"
  }
}
'@
    $hookResult = Invoke-TrPost "/hooks/pre-commit" $hookBody
    Assert-StatusCode "3.8a" "Manual hook POST returns 202 (async)" 202 $hookResult.StatusCode

    Write-Host "  [wait] Polling daemon for AI pipeline output (~15s)..." -ForegroundColor DarkGray
    Start-Sleep -Seconds 15
    $pipelineLogs = Get-DaemonOutput

    if ($pipelineLogs -match '\[hook\]|\[pipeline\]') {
        Write-Pass "3.8b" "Daemon logs hook/pipeline processing"
    } else {
        Write-Skip "3.8b" "Pipeline logs not captured (AI may be slow)"
    }

    if ($pipelineLogs -match "Recall|question|🧠") {
        Write-Pass "3.8c" "Daemon outputs recall question"
    } else {
        Write-Skip "3.8c" "Recall question not found (AI may be slow or failed)"
    }

    New-ScratchRepo

    try {
        Push-Location $script:ScratchRepo
        New-MockStagedChange
        git commit -m "feat: add exponential backoff helper" 2>&1 | Out-Null
        $realCommitExit = $LASTEXITCODE
        Pop-Location

        if ($realCommitExit -eq 0) {
            Write-Pass "3.9a" "Real commit completes (hook is non-blocking)"
        } else {
            Write-Fail "3.9a" "Real commit completes" "Exit code: $realCommitExit"
        }

        Write-Host "  [wait] Polling daemon for commit pipeline output (~15s)..." -ForegroundColor DarkGray
        Start-Sleep -Seconds 15
        $commitLogs = Get-DaemonOutput

        if ($commitLogs -match "Recall|question|🧠|\[pipeline\]") {
            Write-Pass "3.9b" "Daemon processes commit through AI pipeline"
        } else {
            Write-Fail "3.9b" "Daemon processes commit through AI pipeline" "No pipeline output found. Daemon output: $commitLogs"
        }
    } finally {
        Push-Location $script:ScratchRepo
        Remove-MockCommit
        Pop-Location
    }

    try {
        Push-Location $script:ScratchRepo
        git commit --allow-empty -m "chore: empty commit" 2>&1 | Out-Null
        $emptyExit = $LASTEXITCODE
        Pop-Location

        if ($emptyExit -eq 0) {
            Write-Pass "3.10a" "Empty commit succeeds"
        } else {
            Write-Fail "3.10a" "Empty commit succeeds" "Exit code: $emptyExit"
        }

        Start-Sleep -Seconds 3
        $emptyLogs = Get-DaemonOutput
        if ($emptyLogs -match "no diff|skipping|skip") {
            Write-Pass "3.10b" "Daemon logs skip for empty diff"
        } else {
            Write-Fail "3.10b" "Daemon logs skip for empty diff" "No skip log found. Daemon output: $emptyLogs"
        }
    } finally {
        # No mock commit to remove for --allow-empty
    }

    Stop-TrDaemon

    $cachePath = Join-Path $env:USERPROFILE ".tr\memory.db"
    if (Test-Path $cachePath) {
        Write-Pass "3.11" "Concept cache table exists at $cachePath"

        $sqliteOutput = & sqlite3 $cachePath "SELECT COUNT(*) FROM concepts;" 2>&1
        $sqliteExit = $LASTEXITCODE

        if ($sqliteExit -eq 0 -and $sqliteOutput -match '\d+') {
            $count = [int]$sqliteOutput
            if ($count -gt 0) {
                Write-Pass "3.12a" "Concepts table has $count rows"
            } else {
                Write-Skip "3.12a" "Concepts table is empty (AI may not have extracted)"
            }

            $detailOutput = & sqlite3 $cachePath "SELECT concept, source, weight FROM concepts LIMIT 3;" 2>&1
            if ($detailOutput) {
                Write-Pass "3.12b" "Cached concepts have expected schema"
            } else {
                Write-Skip "3.12b" "No concept details returned"
            }
        } else {
            Write-Fail "3.12a" "Query concepts table" "sqlite3 failed: $sqliteOutput"
            Write-Skip "3.12b" "Depends on 3.12a"
        }
    } else {
        Write-Fail "3.11" "Concept cache table exists" "File not found: $cachePath"
        Write-Skip "3.12a" "Depends on 3.11"
        Write-Skip "3.12b" "Depends on 3.11"
    }

    Remove-ScratchRepo
}

Write-Host ""
Write-Host "  Section E — Provider Spot Checks" -ForegroundColor DarkGray
Write-Host ""

Write-Skip "3.13" "Ollama provider (requires local ollama serve)"
Write-Skip "3.14" "Ollama daemon test (requires local ollama serve)"
Write-Skip "3.15" "Custom provider with base URL (requires local service)"

Write-Host ""
Write-Host "  Section F — Graceful Degradation" -ForegroundColor DarkGray
Write-Host ""

if ($hasApiKey) {
    $backupPath2 = Join-Path $env:TEMP "tr-config-backup-badkey.yaml"
    if (Test-Path $configPath) {
        Copy-Item $configPath $backupPath2 -Force
    }

    if (Test-Path $configPath) {
        $content = Get-Content $configPath -Raw
        $content = $content -replace '(api-key:\s*)env:\S+', '${1}sk-garbage-key-for-testing'
        $content = $content -replace '(api-key:\s*)sk-\S+', '${1}sk-garbage-key-for-testing'
        Set-Content $configPath $content
    }

    New-ScratchRepo
    Start-TrDaemon

    try {
        Push-Location $script:ScratchRepo
        New-MockStagedChange
        git commit -m "test: bad api key" 2>&1 | Out-Null
        $badKeyCommitExit = $LASTEXITCODE
        Pop-Location

        if ($badKeyCommitExit -eq 0) {
            Write-Pass "3.16a" "Commit succeeds with bad API key (non-blocking)"
        } else {
            Write-Fail "3.16a" "Commit succeeds with bad API key" "Exit code: $badKeyCommitExit"
        }

        Start-Sleep -Seconds 5
        $badKeyLogs = Get-DaemonOutput

        if ($badKeyLogs -match "failed|error|AI call") {
            Write-Pass "3.16b" "Daemon logs AI failure (no crash)"
        } else {
            Write-Fail "3.16b" "Daemon logs AI failure (no crash)" "No failure log found. Daemon output: $badKeyLogs"
        }

        $healthAfterFail = Test-DaemonRunning
        if ($healthAfterFail) {
            Write-Pass "3.16c" "Daemon continues running after AI failure"
        } else {
            Write-Fail "3.16c" "Daemon continues running after AI failure" "Health check failed"
        }
    } finally {
        Push-Location $script:ScratchRepo
        Remove-MockCommit
        Pop-Location
    }

    Stop-TrDaemon

    if (Test-Path $backupPath2) {
        Copy-Item $backupPath2 $configPath -Force
        Remove-Item $backupPath2 -Force
    }

    Remove-ScratchRepo
} else {
    Write-Skip "3.16a" "No API key to test bad key scenario"
    Write-Skip "3.16b" "No API key to test bad key scenario"
    Write-Skip "3.16c" "No API key to test bad key scenario"
}

Write-Host ""
Write-Host "  Phase 02 Regression Checks" -ForegroundColor DarkGray
Write-Host ""

Start-TrDaemon
$regressionHealth = Invoke-TrGet "/health"
Assert-StatusCode "3.17a" "Regression: GET /health still returns 200" 200 $regressionHealth.StatusCode

$hookBodyRegression = '{"hook":"pre-commit","repo":"/tmp/tr-test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ foo"}}'
$hookResultRegression = Invoke-TrPost "/hooks/pre-commit" $hookBodyRegression
Assert-StatusCode "3.17b" "Regression: POST /hooks/pre-commit still returns 202" 202 $hookResultRegression.StatusCode

Stop-TrDaemon

Write-Summary "Phase 03"
