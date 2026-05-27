#!/usr/bin/env bash
# total-recall managed
# Managed by Total Recall — do not edit this block manually.
# Re-run 'total-recall init' to update.

DAEMON_URL="http://localhost:7331"
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
    || printf "[total-recall] Daemon not running at %s — skipping recall check. Start with 'total-recall serve'.\n" "${DAEMON_URL}" >&2

exit 0
