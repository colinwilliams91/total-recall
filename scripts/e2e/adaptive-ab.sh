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

# ── learner arms: answered-history seeding ──────────────────────────────────
# The performance resolver reads windowed answered history. These companion
# arms seed it through the same HTTP contract the ask TUI drives (claim via
# GET /recall/next, answer via POST /recall/select), then fire a final
# synthesis so the log shows the level-shifted resolution.
#
# The delivery wire format deliberately withholds which choice is correct
# (anti-leak contract), so the correct position is read from the scratch
# memory.db instead. Requires python3 for that lookup.

# json_field extracts the first numeric value for a key from a JSON body.
# First-match matters: the question's own id is the first field. Absent keys
# yield an empty result without failing the caller under set -e.
json_field() {
  local v
  v="$(printf '%s' "$1" | grep -oE "\"$2\":-?[0-9]+" | head -1 | grep -oE -- "-?[0-9]+" || true)"
  printf '%s' "$v"
}

# db_field runs a scalar parameterized SQL query against the scratch
# memory.db (read-only; python3 required).
db_field() {
  python3 -c "
import sqlite3, sys
db = sqlite3.connect(sys.argv[1])
row = db.execute(sys.argv[2], sys.argv[3:]).fetchone()
print(row[0] if row is not None and row[0] is not None else '')
" "$TR_HOME/memory.db" "$@"
}

# answer_capture answers one captured /recall/next question: the correct
# choice when $2=1, an incorrect one otherwise.
answer_capture() {
  local json="$1" correct="$2" id cidx nchoices sidx
  id="$(json_field "$json" id)"
  [ -n "$id" ] || return 0
  cidx="$(db_field "SELECT position FROM choices WHERE question_id = ? AND is_correct = 1" "$id")"
  [ -n "$cidx" ] || { echo "  (no choices found for question $id — skipping)"; return 0; }
  if [ "$correct" = "1" ]; then
    sidx="$cidx"
  else
    nchoices="$(db_field "SELECT COUNT(*) FROM choices WHERE question_id = ?" "$id")"
    [ -n "$nchoices" ] && [ "$nchoices" -gt 0 ] || return 0
    sidx=$(( (cidx + 1) % nchoices ))
  fi
  curl -fsS --max-time 10 -X POST "$BASE/recall/select" \
    -H 'Content-Type: application/json' \
    -d "{\"id\":$id,\"selected_index\":$sidx}" >/dev/null \
    || echo "  (select failed for question $id)"
}

# seed_history answers the captured questions on a branch: the first $2
# correctly, the rest incorrectly.
seed_history() {
  local name="$1" correct_count="$2" answered=0 verdict
  local file="$ARMS_DIR/$name.txt"
  [ -f "$file" ] || { echo "  (no captures for $name — seeding skipped)"; return 0; }
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    if [ "$answered" -lt "$correct_count" ]; then verdict=1; else verdict=0; fi
    answer_capture "$line" "$verdict"
    answered=$((answered + 1))
  done <"$file"
  echo "  seeded $answered answered on branch arm-$name ($correct_count correct)"
}

echo ""
echo "=== learner arms (answered-history seeding via /recall/select) ==="
# Correct-heavy: two capture rounds fill the 20-question window with 19
# correct — the final synthesis should log learner-tier escalating to hard.
run_arm "learner-correct" "tidy: minor comment touch-up" 1
run_arm "learner-correct" "tidy: minor comment touch-up, second batch" 1
seed_history "learner-correct" 19
run_arm "learner-correct" "tidy: final touch-up after a strong record" 1
echo "--- correct-heavy branch resolutions (expect learner-tier, escalated to hard) ---"
grep "performance resolver selected" "$LOG" | tail -3 || true

# Miss-heavy: 7/20 correct — the final synthesis should log learner-tier
# de-escalating to easy, plus the focus concepts the misses rank.
run_arm "learner-miss" "tidy: minor comment touch-up" 1
run_arm "learner-miss" "tidy: minor comment touch-up, second batch" 1
seed_history "learner-miss" 7
run_arm "learner-miss" "tidy: final touch-up after a rough record" 1
echo "--- miss-heavy branch resolutions (expect learner-tier, de-escalated to easy, plus focus concepts) ---"
grep "performance resolver selected\|performance focus concepts" "$LOG" | tail -4 || true

echo ""
echo "Resolver selections (daemon log):"
grep -E "adaptive resolver selected|performance resolver selected|performance focus concepts" "$LOG" | tail -80 || echo "(none — was difficulty: adaptive configured?)"
echo ""
echo "Per-arm question captures: $ARMS_DIR/<arm>.txt"
echo "Compare question quality side-by-side, then record observations in"
echo "docs/CORE/ADAPTIVE_SPIKE.md (see openspec change adaptive-difficulty, task 5.2)."
echo "Scratch dir left for inspection: $SCRATCH"
