#!/usr/bin/env bash
# adaptive-ab.sh — manual A/B spike for the adaptive difficulty resolver.
#
# Not a CI test. Seeds a scratch Total Recall data dir (copying your
# ~/.tr/config.yaml), starts a scratch daemon, and drives the daemon's hook
# HTTP contract with payloads bracketing each adaptive signal arm. After each
# arm it polls GET /recall/next to capture questions for human side-by-side
# comparison, and greps the daemon log for the resolver's per-call selection
# lines.
#
# Why HTTP instead of `git commit` + `torec ask`: the ask TUI requires a real
# TTY and derives repo/branch from the current directory — it cannot be driven
# headless. POSTing hook envelopes exercises the same pipeline the hooks do.
#
# Usage: scripts/e2e/adaptive-ab.sh [path-to-torec-binary]
set -euo pipefail

TOREC="${1:-$(pwd)/bin/torec}"
SCRATCH="$(mktemp -d)"
TR_HOME="$SCRATCH/.tr"
LOG="$TR_HOME/daemon.log"
ARMS_DIR="$TR_HOME/arms"
BASE="http://localhost:7331"
REPO="spike://adaptive-ab"
mkdir -p "$TR_HOME" "$ARMS_DIR"

DAEMON_PID=""
cleanup() {
  if [ -n "${DAEMON_PID:-}" ]; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

if [ ! -x "$TOREC" ]; then
  echo "torec binary not found at '$TOREC' — pass the path: adaptive-ab.sh ./bin/torec"
  exit 1
fi

echo "scratch dir: $SCRATCH"
echo "data dir:    $TR_HOME"

export TR_HOME

# A second daemon cannot share localhost:7331 — refusing avoids this script's
# questions colliding with a live daemon (or its config).
if curl -fsS --max-time 2 "$BASE/health" >/dev/null 2>&1; then
  echo "a daemon is already running on $BASE — stop it first with 'torec stop', then re-run."
  exit 1
fi

# Seed the scratch config from your real one so the provider under test is
# what gets exercised. API keys should be env: references (never raw values).
if [ -f "$HOME/.tr/config.yaml" ]; then
  cp "$HOME/.tr/config.yaml" "$TR_HOME/config.yaml"
  echo "copied $HOME/.tr/config.yaml into the scratch data dir"
else
  echo "no config at $HOME/.tr/config.yaml — run 'torec init' first, then re-run."
  exit 1
fi

# ── scratch daemon ───────────────────────────────────────────────────────────
"$TOREC" serve >>"$LOG" 2>&1 &
DAEMON_PID=$!

healthy=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 2 "$BASE/health" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  sleep 0.5
done
if [ "$healthy" -ne 1 ]; then
  echo "scratch daemon did not become healthy within 15s — daemon.log tail:"
  tail -20 "$LOG"
  exit 1
fi
echo "scratch daemon running (pid $DAEMON_PID)"

# ── helpers ──────────────────────────────────────────────────────────────────
# json_escape makes a string safe for embedding in a JSON double-quoted value:
# backslash, double-quote, newline, tab.
json_escape() {
  printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | sed -z 's/\n/\\n/g'
}

# diff_of renders a synthetic Go-ish diff body: $2 focused concept lines,
# blank-line separated, sized so the AI-delegation arm clears the 400-char
# snippet threshold and the others do not.
diff_of() {
  local name="$1" lines="$2" i
  for i in $(seq 1 "$lines"); do
    echo "func ${name}_helper_${i}() int { return $i } // ${name} padding line $i for the diff signal"
    echo ""
  done
}

# run_arm POSTs the same hook payload 10 times (fresh concept extraction +
# synthesis each round), polling GET /recall/next until each question lands.
# Results accumulate in $ARMS_DIR/<arm>.txt, one JSON object per line.
run_arm() {
  local name="$1" msg="$2" diff_lines="$3"
  local branch="arm-$name"
  local diff_body escaped_diff escaped_msg captured=0
  diff_body="$(diff_of "$name" "$diff_lines")"
  escaped_diff="$(json_escape "$diff_body")"
  escaped_msg="$(json_escape "$msg")"

  echo "=== arm: $name ==="
  for _ in $(seq 1 10); do
    curl -fsS --max-time 10 -X POST "$BASE/hooks/commit-msg" \
      -H 'Content-Type: application/json' \
      -d "{\"hook\":\"commit-msg\",\"repo\":\"$REPO\",\"branch\":\"$branch\",\"timestamp\":\"2026-01-01T00:00:00Z\",\"payload\":{\"diff\":\"$escaped_diff\",\"message\":\"$escaped_msg\"}}" \
      >/dev/null || { echo "  hook POST failed (arm $name)"; continue; }

    # The pipeline is async (two AI calls): poll the queue up to 30s.
    local q="" waited=0 code
    while [ "$waited" -lt 60 ]; do
      code=$(curl -sS -o "$ARMS_DIR/$name.txt.tmp" -w '%{http_code}' \
        --get "$BASE/recall/next" \
        --data-urlencode "repo=$REPO" \
        --data-urlencode "branch=$branch" 2>/dev/null || echo 000)
      if [ "$code" = "200" ]; then
        q=$(cat "$ARMS_DIR/$name.txt.tmp")
        break
      fi
      sleep 0.5
      waited=$((waited + 1))
    done
    rm -f "$ARMS_DIR/$name.txt.tmp"
    if [ -n "$q" ]; then
      captured=$((captured + 1))
      printf '%s\n' "$q" >>"$ARMS_DIR/$name.txt"
    else
      echo "  (no question within 30s)"
    fi
  done
  echo "captured $captured/10 questions -> $ARMS_DIR/$name.txt"
}

# ── arms: one payload set per adaptive signal ────────────────────────────────
# 1. AI-delegation: short msg + large diff (msg < 50 chars, diff > 400 chars)
run_arm "ai-delegation" "fix:" 40
# 2. High-cluster: substantive message (no delegation signal), focused diff
run_arm "high-cluster" "Add exponential backoff with jitter to the retry helper so concurrent workers do not stampede the shared API client and thundering-herd issues are avoided across the request path" 3
# 3. Dispersed: many unrelated one-liner concepts across files
run_arm "dispersed" "Add assorted topic notes one through five, each an unrelated tiny fragment with no shared theme" 5
# 4. All-code: uniform code-sourced concepts, moderate weights
run_arm "all-code" "Refactor retry handling across several small helper modules in this package" 8
# 5. No-signal (fallback): tiny diff, medium message
run_arm "fallback" "tidy: minor comment touch-up" 1

echo ""
echo "Resolver selections (daemon log):"
grep "adaptive resolver selected" "$LOG" | head -60 || echo "(none — was difficulty: adaptive configured?)"
echo ""
echo "Per-arm question captures: $ARMS_DIR/<arm>.txt"
echo "Compare question quality side-by-side, then record observations in"
echo "docs/CORE/ADAPTIVE_SPIKE.md (see openspec change adaptive-difficulty, task 5.2)."
echo "Scratch dir left for inspection: $SCRATCH"
