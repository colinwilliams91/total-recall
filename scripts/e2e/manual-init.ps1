param(
    [string]$BinaryPath,
    [string]$ScratchDir
)

# manual-init.ps1 — Manual e2e test for the `tr init` TUI flow.
#
# This is the ONLY remaining manual e2e test. All other e2e coverage
# (daemon, hooks, recall, MCP, config, cache, provider routing, ask
# state machine, golden views) is automated in cmd/total-recall/*_test.go.
#
# `tr init` uses huh.NewForm() which requires a real TTY. The huh
# accessible mode (TERM=dumb) exists but has a bufio.Scanner-per-field
# buffering bug that loses piped input, making automation unreliable.
#
# Usage:
#   .\scripts\e2e\manual-init.ps1
#   .\scripts\e2e\manual-init.ps1 -BinaryPath .\tr.exe

. "$PSScriptRoot/common.ps1" -BinaryPath $BinaryPath -ScratchDir $ScratchDir
Initialize-E2E -BinaryPath $BinaryPath -ScratchDir $ScratchDir

Write-Host "  Manual E2E — tr init TUI" -ForegroundColor White
Write-Host "  -------------------------------------------" -ForegroundColor DarkGray
Write-Host "  This test guides you through the tr init TUI" -ForegroundColor DarkGray
Write-Host "  and auto-verifies the results after each step." -ForegroundColor DarkGray
Write-Host ""

# ─── Setup ───────────────────────────────────────────────────────────────────

Backup-TrConfig | Out-Null
$configPath = Join-Path $env:USERPROFILE ".tr\config.yaml"
if (Test-Path $configPath) {
    Remove-Item -Path $configPath -Force
}

New-ScratchRepo

# ─── Step 1: First-run init (full TUI flow) ──────────────────────────────────

Write-Manual -Id "1.1" `
    -Description "tr init — first run (conversation analysis + AI provider + hooks)" `
    -Command "cd $script:ScratchRepo`n& $script:TrBin init" `
    -Expected @"
TUI prompts in order:
  1. Enable conversation analysis? (Y/n)
  2. AI Provider select (10 options, Anthropic is default)
  3. API Key input (suggests env:ANTHROPIC_API_KEY)
  4. Model input (suggests claude-sonnet-4-5)
  5. Enable pre-commit hook? (Y/n)
  6. Enable commit-msg hook? (Y/n)
  7. Enable pre-push hook? (Y/n)
Then: User config saved, repo config saved, hooks installed, post-commit installed.
"@ `
    -Verify {
        $issues = @()
        # Check user config
        if (-not (Test-Path $configPath)) {
            $issues += "User config not created: $configPath"
        } else {
            $config = Get-Content $configPath -Raw
            if (-not ($config -match "provider:")) { $issues += "Config missing provider" }
            if (-not ($config -match "model:")) { $issues += "Config missing model" }
            if (-not ($config -match "api-key:")) { $issues += "Config missing api-key" }
        }
        # Check repo config
        $trYaml = Join-Path $script:ScratchRepo ".tr.yaml"
        if (-not (Test-Path $trYaml)) {
            $issues += ".tr.yaml not created"
        }
        # Check pre-commit hook
        $hookPath = Join-Path $script:ScratchRepo ".git\hooks\pre-commit"
        if (-not (Test-Path $hookPath)) {
            $issues += "pre-commit hook not installed"
        } else {
            $hookContent = Get-Content $hookPath -Raw
            if (-not ($hookContent -match "# total-recall managed")) { $issues += "pre-commit missing sentinel" }
            if (-not ($hookContent -match "^#!")) { $issues += "pre-commit missing shebang" }
        }
        # Check post-commit hook
        $postCommitPath = Join-Path $script:ScratchRepo ".git\hooks\post-commit"
        if (-not (Test-Path $postCommitPath)) {
            $issues += "post-commit hook not installed"
        } else {
            $postContent = Get-Content $postCommitPath -Raw
            if (-not ($postContent -match "total-recall ask|tr\.exe ask|tr ask")) { $issues += "post-commit missing 'tr ask'" }
        }
        if ($issues.Count -gt 0) {
            return @{ Passed = $false; Detail = ($issues -join "; ") }
        }
        return @{ Passed = $true; Detail = "" }
    }

# ─── Step 2: Idempotent re-run ───────────────────────────────────────────────

$configHashBefore = $null
$hookHashBefore = $null
if (Test-Path $configPath) {
    $configHashBefore = (Get-FileHash $configPath -Algorithm SHA256).Hash
}
$hookPath = Join-Path $script:ScratchRepo ".git\hooks\pre-commit"
if (Test-Path $hookPath) {
    $hookHashBefore = (Get-FileHash $hookPath -Algorithm SHA256).Hash
}

if ($configHashBefore -and $hookHashBefore) {
    Write-Manual -Id "1.2" `
        -Description "tr init — idempotent re-run (values pre-populated, files unchanged)" `
        -Command "cd $script:ScratchRepo`n& $script:TrBin init" `
        -Expected "All prompts pre-filled with previous selections. Accept all defaults. Config and hook files should be unchanged." `
        -Verify {
            $configHashAfter = (Get-FileHash $configPath -Algorithm SHA256).Hash
            $hookHashAfter = (Get-FileHash $hookPath -Algorithm SHA256).Hash
            if ($configHashBefore -ne $configHashAfter) {
                return @{ Passed = $false; Detail = "Config file changed after re-init" }
            }
            if ($hookHashBefore -ne $hookHashAfter) {
                return @{ Passed = $false; Detail = "Hook file changed after re-init" }
            }
            return @{ Passed = $true; Detail = "" }
        }
} else {
    Write-Skip "1.2" "Cannot test idempotency (config or hooks not installed from step 1.1)"
}

# ─── Step 3: Hook chaining with existing unmanaged hook ──────────────────────

Write-Manual -Id "1.3" `
    -Description "tr init — chains with existing unmanaged hook" `
    -Command @"
cd $script:ScratchRepo
# Create a pre-existing unmanaged hook
echo '#!/usr/bin/env bash' > .git/hooks/pre-commit
echo 'echo "existing hook ran"' >> .git/hooks/pre-commit
# Now re-run init
& $script:TrBin init
"@ `
    -Expected "After init, .git/hooks/pre-commit should contain BOTH the original 'echo existing hook ran' AND the '# total-recall managed' sentinel." `
    -Verify {
        $hookContent = Get-Content $hookPath -Raw
        $hasOriginal = $hookContent -match "existing hook ran"
        $hasSentinel = $hookContent -match "# total-recall managed"
        if (-not $hasOriginal) {
            return @{ Passed = $false; Detail = "Original hook content was not preserved" }
        }
        if (-not $hasSentinel) {
            return @{ Passed = $false; Detail = "TR sentinel was not added" }
        }
        return @{ Passed = $true; Detail = "" }
    }

# ─── Cleanup ─────────────────────────────────────────────────────────────────

Restore-TrConfig | Out-Null
Write-Summary "Manual E2E — tr init"
