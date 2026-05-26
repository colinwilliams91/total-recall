@echo off
:: total-recall managed
:: Managed by Total Recall — do not edit this block manually.
:: Re-run 'total-recall init' to update.

set "DAEMON_URL=http://localhost:7331"
set "COMMIT_MSG_FILE=%1"

for /f "delims=" %%i in ('git rev-parse --show-toplevel 2^>nul') do set "REPO=%%i"
for /f "delims=" %%i in ('git rev-parse --abbrev-ref HEAD 2^>nul') do set "BRANCH=%%i"

powershell -NoProfile -Command ^
    "try { Invoke-WebRequest -Uri '%DAEMON_URL%/hooks/commit-msg' -Method POST -ContentType 'application/json' -Body ('{\"hook\":\"commit-msg\",\"repo\":\"%REPO%\",\"branch\":\"%BRANCH%\",\"timestamp\":\"' + (Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ') + '\",\"payload\":{\"message\":\"(see daemon log)\"}}') -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop | Out-Null } catch { Write-Host '[total-recall] Daemon not running. Start with total-recall serve.' -ForegroundColor Yellow }"

exit /b 0
