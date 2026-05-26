#!/usr/bin/env bash
# total-recall managed
# Managed by Total Recall — do not edit this block manually.
# Re-run 'total-recall init' to update.

DAEMON_URL="http://localhost:7331"
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
