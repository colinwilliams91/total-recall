# adaptive-ab.ps1 — manual A/B spike for the adaptive difficulty resolver.
#
# Windows counterpart of adaptive-ab.sh (per the hooks/ .sh + .bat pairing
# convention). Not a CI test. Sets up a scratch git repo with synthetic
# commits bracketing each adaptive signal, runs the daemon and `torec ask`
# after each, and logs the resolver's per-call difficulty selection for human
# side-by-side comparison of question quality across signal arms.
#
# Usage: .\scripts\e2e\adaptive-ab.ps1 [-TorecPath <path-to-torec.exe>]
param(
    [string]$TorecPath = (Join-Path $PSScriptRoot "..\..\bin\torec.exe")
)

$ErrorActionPreference = "Stop"
$Scratch = Join-Path ([System.IO.Path]::GetTempPath()) ("torec-ab-" + [guid]::NewGuid().ToString("N").Substring(0, 8))
$TrHome = Join-Path $Scratch ".tr"
$Log = Join-Path $TrHome "daemon.log"
$ArmsDir = Join-Path $TrHome "arms"
New-Item -ItemType Directory -Force -Path $TrHome, $ArmsDir | Out-Null
$env:TR_HOME = $TrHome

if (-not (Test-Path (Join-Path $TrHome "config.yaml"))) {
    Write-Host "no config at $TrHome\config.yaml — run 'torec init' or copy your ~/.tr/config.yaml there, then re-run."
    exit 1
}

Write-Host "scratch repo: $Scratch"
Write-Host "data dir:     $TrHome"

# Scratch repo with one commit per adaptive signal arm.
$Repo = Join-Path $Scratch "repo"
git init -q $Repo
Push-Location $Repo
try {
    git config user.email "spike@example.com"
    git config user.name "Adaptive Spike"

    function Commit-Arm([string]$Name, [string]$Msg, [int]$Lines) {
        $file = "code_$Name.go"
        for ($i = 1; $i -le $Lines; $i++) {
            Add-Content $file "func generated_$i() int { return $i } // padding line $i for the diff signal"
            Add-Content $file ""
        }
        git add .
        git commit -q -m $Msg
    }

    # 1. AI-delegation: short msg + large diff (msg < 50 chars, diff > 400 chars)
    Commit-Arm "ai-delegation" "fix:" 40
    # 2. High-cluster: substantive message, small focused diff
    Commit-Arm "high-cluster" "Add exponential backoff with jitter to the retry helper so concurrent workers do not stampede the API" 3
    # 3. Dispersed: many unrelated one-liner concepts
    for ($i = 1; $i -le 5; $i++) { Set-Content "topic_$i.txt" "const concept_$i = $i" }
    git add .
    git commit -q -m "Add assorted topic notes 1 through 5, each an unrelated tiny fragment"
    # 4. All-code: uniform code-sourced concepts, moderate weights
    Commit-Arm "all-code" "Refactor retry handling across five small helper modules" 5
    # 5. No-signal (fallback): tiny diff, medium message
    Add-Content "code_all-code.go" "// touch"
    git add .
    git commit -q -m "tidy: minor comment touch-up"

    # Daemon + per-arm question capture.
    $Daemon = Start-Process -FilePath $TorecPath -ArgumentList "serve" `
        -RedirectStandardOutput $Log -RedirectStandardError $Log -PassThru -NoNewWindow
    Start-Sleep -Seconds 2

    foreach ($name in @("ai-delegation", "high-cluster", "dispersed", "all-code", "fallback")) {
        Write-Host "=== arm: $name ==="
        for ($i = 1; $i -le 10; $i++) {
            & $TorecPath ask --repo $Repo --branch main (Add-Content (Join-Path $ArmsDir "$name.txt") "") 2>&1 |
                Add-Content (Join-Path $ArmsDir "$name.txt")
        }
    }

    Stop-Process -Id $Daemon.Id -Force -ErrorAction SilentlyContinue
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "Resolver selections (grep the daemon log):"
Select-String -Path $Log -Pattern "adaptive resolver selected" | ForEach-Object { $_.Line }
Write-Host ""
Write-Host "Per-arm question captures: $ArmsDir\<arm>.txt"
Write-Host "Compare question quality side-by-side, then record observations in"
Write-Host "docs/CORE/ADAPTIVE_SPIKE.md (see openspec change adaptive-difficulty, task 5.2)."
