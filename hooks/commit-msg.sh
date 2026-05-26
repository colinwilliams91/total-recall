#!/usr/bin/env bash
# total-recall managed
# Managed by Total Recall — do not edit this block manually.
# Re-run 'total-recall init' to update.

DAEMON_URL="http://localhost:7331"
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
