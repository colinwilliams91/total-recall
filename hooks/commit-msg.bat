@echo off
:: total-recall managed
:: Managed by Total Recall — do not edit this block manually.
:: Re-run 'tr init' to update.

set "DAEMON_URL=http://localhost:7331"
set "COMMIT_MSG_FILE=%1"

for /f "delims=" %%i in ('git rev-parse --show-toplevel 2^>nul') do set "REPO=%%i"
for /f "delims=" %%i in ('git rev-parse --abbrev-ref HEAD 2^>nul') do set "BRANCH=%%i"

powershell -NoProfile -Command ^
    "$msg = (Get-Content -Raw '%COMMIT_MSG_FILE%' 2>$null) -replace '[^\x20-\x7E]',''; " +
    "$diff = (git diff --cached 2>$null | Select-Object -First 500 | Out-String); " +
    "$body = @{ hook='commit-msg'; repo='%REPO%'; branch='%BRANCH%'; timestamp=(Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ'); payload=@{ message=$msg; diff=$diff } } | ConvertTo-Json -Compress -Depth 3; " +
    "try { Invoke-WebRequest -Uri '%DAEMON_URL%/hooks/commit-msg' -Method POST -ContentType 'application/json' -Body $body -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop | Out-Null } catch { Write-Host '[total-recall] Daemon not running. Start with tr serve.' -ForegroundColor Yellow }"

exit /b 0
