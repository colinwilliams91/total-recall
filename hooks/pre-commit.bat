@echo off
:: total-recall managed
:: Managed by Total Recall — do not edit this block manually.
:: Re-run 'total-recall init' to update.

set "DAEMON_URL=http://localhost:7331"

:: P0: Credential scan
if exist ".tr.yaml" (
    powershell -NoProfile -Command ^
        "if((Get-Content '.tr.yaml' | Where-Object {$_ -match '^\s*api-key:\s+' -and $_ -notmatch '^\s*api-key:\s+env:'}).Count -gt 0){ exit 1 }" 2>nul
    if %errorlevel% equ 1 (
        echo [SECURITY] .tr.yaml contains a raw api-key value. >&2
        echo    Rotate the key, use env:YOUR_VAR_NAME format, then purge from Git history. >&2
        exit /b 1
    )
)

for /f "delims=" %%i in ('git rev-parse --show-toplevel 2^>nul') do set "REPO=%%i"
for /f "delims=" %%i in ('git rev-parse --abbrev-ref HEAD 2^>nul') do set "BRANCH=%%i"

powershell -NoProfile -Command ^
    "try { Invoke-WebRequest -Uri '%DAEMON_URL%/hooks/pre-commit' -Method POST -ContentType 'application/json' -Body ('{\"hook\":\"pre-commit\",\"repo\":\"%REPO%\",\"branch\":\"%BRANCH%\",\"timestamp\":\"' + (Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ') + '\",\"payload\":{}}') -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop | Out-Null } catch { Write-Host '[total-recall] Daemon not running. Start with total-recall serve.' -ForegroundColor Yellow }"

exit /b 0
