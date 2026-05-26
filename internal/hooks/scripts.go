package hooks

// sentinel is the first-5-lines marker that identifies a Total Recall managed hook.
const sentinel = "# total-recall managed"

// hookHeader is prepended to every managed hook script.
const hookHeader = "#!/usr/bin/env bash\n" +
	sentinel + "\n" +
	"# Managed by Total Recall — do not edit this block manually.\n" +
	"# Re-run 'total-recall init' to update.\n\n"

// preCommitBody is the sh hook body for the pre-commit hook (no shebang/sentinel).
// Order: P0 credential scan → curl check → capture context → POST (non-blocking).
const preCommitBody = `DAEMON_URL="http://localhost:7331"
HOOK_TIMEOUT=2

# ─── P0: Credential Scan ─────────────────────────────────────────────────────
if [ -f ".tr.yaml" ]; then
    if grep -qE '^\s*api-key:\s+' .tr.yaml 2>/dev/null && \
       ! grep -qE '^\s*api-key:\s+env:' .tr.yaml 2>/dev/null; then
        printf "🚨 SECURITY: .tr.yaml contains a raw api-key value.\n" >&2
        printf "   Rotate the exposed key immediately.\n" >&2
        printf "   Use 'api-key: env:YOUR_VAR_NAME' instead.\n" >&2
        printf "   Purge from Git history: git filter-repo or BFG.\n" >&2
        exit 1
    fi
fi

# ─── Check curl ───────────────────────────────────────────────────────────────
if ! command -v curl >/dev/null 2>&1; then
    printf "[total-recall] curl not found — install curl to enable dispatch.\n" >&2
    exit 0
fi

# ─── Capture staged context ───────────────────────────────────────────────────
REPO="$(git rev-parse --show-toplevel 2>/dev/null || printf "")"
BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || printf "")"
TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || printf "")"
LOC_ADDED="$(git diff --cached --numstat 2>/dev/null | awk '{s+=$1}END{print s+0}')"
LOC_REMOVED="$(git diff --cached --numstat 2>/dev/null | awk '{s+=$2}END{print s+0}')"
LOC_DELTA="$((LOC_ADDED - LOC_REMOVED))"

if command -v python3 >/dev/null 2>&1; then
    DIFF_JSON="$(git diff --cached 2>/dev/null | head -c 32768 | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')"
    STAGED_JSON="$(git diff --cached --name-only 2>/dev/null | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read().splitlines()))')"
else
    DIFF_JSON='"<python3 required for diff encoding>"'
    STAGED_JSON='[]'
fi

# ─── POST to daemon (non-blocking — TR must never block git operations) ────────
curl --silent --max-time "${HOOK_TIMEOUT}" \
    --request POST \
    --header "Content-Type: application/json" \
    --data "{\"hook\":\"pre-commit\",\"repo\":\"${REPO}\",\"branch\":\"${BRANCH}\",\"timestamp\":\"${TIMESTAMP}\",\"payload\":{\"diff\":${DIFF_JSON},\"staged_files\":${STAGED_JSON},\"loc_delta\":${LOC_DELTA}}}" \
    "${DAEMON_URL}/hooks/pre-commit" >/dev/null 2>&1 \
    || printf "[total-recall] Daemon not running at %s. Start with 'total-recall serve'.\n" "${DAEMON_URL}" >&2

exit 0
`

// commitMsgBody is the sh hook body for the commit-msg hook.
const commitMsgBody = `DAEMON_URL="http://localhost:7331"
HOOK_TIMEOUT=2

if ! command -v curl >/dev/null 2>&1; then
    exit 0
fi

COMMIT_MSG_FILE="${1}"
REPO="$(git rev-parse --show-toplevel 2>/dev/null || printf "")"
BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || printf "")"
TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || printf "")"

if command -v python3 >/dev/null 2>&1; then
    MSG_JSON="$(cat "${COMMIT_MSG_FILE}" 2>/dev/null | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')"
else
    MSG_JSON='"<python3 required for message encoding>"'
fi

curl --silent --max-time "${HOOK_TIMEOUT}" \
    --request POST \
    --header "Content-Type: application/json" \
    --data "{\"hook\":\"commit-msg\",\"repo\":\"${REPO}\",\"branch\":\"${BRANCH}\",\"timestamp\":\"${TIMESTAMP}\",\"payload\":{\"message\":${MSG_JSON}}}" \
    "${DAEMON_URL}/hooks/commit-msg" >/dev/null 2>&1 \
    || printf "[total-recall] Daemon not running at %s. Start with 'total-recall serve'.\n" "${DAEMON_URL}" >&2

exit 0
`

// prePushBody is the sh hook body for the pre-push hook.
const prePushBody = `DAEMON_URL="http://localhost:7331"
HOOK_TIMEOUT=2

if ! command -v curl >/dev/null 2>&1; then
    exit 0
fi

# Read the ref list from stdin before doing anything else.
STDIN_DATA="$(cat)"

REPO="$(git rev-parse --show-toplevel 2>/dev/null || printf "")"
BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || printf "")"
TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || printf "")"

if command -v python3 >/dev/null 2>&1; then
    REFS_JSON="$(printf '%s\n' "${STDIN_DATA}" | python3 -c '
import json, sys
lines = [l for l in sys.stdin.read().splitlines() if l.strip()]
refs = []
for line in lines:
    parts = line.split()
    if len(parts) >= 4:
        refs.append({"local_ref": parts[0], "local_sha": parts[1], "remote_ref": parts[2], "remote_sha": parts[3]})
print(json.dumps(refs))
')"
else
    REFS_JSON='[]'
fi

curl --silent --max-time "${HOOK_TIMEOUT}" \
    --request POST \
    --header "Content-Type: application/json" \
    --data "{\"hook\":\"pre-push\",\"repo\":\"${REPO}\",\"branch\":\"${BRANCH}\",\"timestamp\":\"${TIMESTAMP}\",\"payload\":{\"refs\":${REFS_JSON}}}" \
    "${DAEMON_URL}/hooks/pre-push" >/dev/null 2>&1 \
    || printf "[total-recall] Daemon not running at %s. Start with 'total-recall serve'.\n" "${DAEMON_URL}" >&2

exit 0
`

// preCommitBat is the Windows batch hook for pre-commit.
// Primary on all platforms is the .sh variant (runs via Git Bash).
// This .bat is for environments invoking hooks outside of Git Bash.
const preCommitBat = `@echo off
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
`

// commitMsgBat is the Windows batch hook for commit-msg.
const commitMsgBat = `@echo off
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
`

// prePushBat is the Windows batch hook for pre-push.
const prePushBat = `@echo off
:: total-recall managed
:: Managed by Total Recall — do not edit this block manually.
:: Re-run 'total-recall init' to update.

set "DAEMON_URL=http://localhost:7331"

for /f "delims=" %%i in ('git rev-parse --show-toplevel 2^>nul') do set "REPO=%%i"
for /f "delims=" %%i in ('git rev-parse --abbrev-ref HEAD 2^>nul') do set "BRANCH=%%i"

powershell -NoProfile -Command ^
    "try { Invoke-WebRequest -Uri '%DAEMON_URL%/hooks/pre-push' -Method POST -ContentType 'application/json' -Body ('{\"hook\":\"pre-push\",\"repo\":\"%REPO%\",\"branch\":\"%BRANCH%\",\"timestamp\":\"' + (Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ') + '\",\"payload\":{\"refs\":[]}}') -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop | Out-Null } catch { Write-Host '[total-recall] Daemon not running. Start with total-recall serve.' -ForegroundColor Yellow }"

exit /b 0
`
