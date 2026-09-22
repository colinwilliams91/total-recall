#!/usr/bin/env bash
# adaptive-ab.sh — manual A/B spike for the adaptive difficulty resolver.
#
# Not a CI test. Sets up a scratch git repo with synthetic commits bracketing
# each adaptive signal, runs the daemon and `torec ask` after each, and logs
# the resolver's per-call difficulty selection for human side-by-side
# comparison of question quality across signal arms.
#
# Usage: scripts/e2e/adaptive-ab.sh [path-to-torec-binary]
# Requires an AI provider configured in the daemon's config (TR_HOME is
# redirected to a scratch dir; copy or edit the generated config.yaml there
# if your real ~/.tr/config.yaml is not suitable).
set -euo pipefail

TOREC="${1:-$(command -v torec || echo "$(pwd)/bin/torec")}"
SCRATCH="$(mktemp -d)"
TR_HOME="$SCRATCH/.tr"
LOG="$TR_HOME/daemon.log"
ARMS_DIR="$TR_HOME/arms"
mkdir -p "$TR_HOME" "$ARMS_DIR"
trap 'kill "$DAEMON_PID" 2>/dev/null || true' EXIT

echo "scratch repo: $SCRATCH"
echo "data dir:     $TR_HOME"

# Isolate the data dir and seed a config if none exists.
export TR_HOME
if [ ! -f "$TR_HOME/config.yaml" ]; then
  echo "no config at \$TR_HOME/config.yaml — run 'torec init' or copy your ~/.tr/config.yaml there, then re-run."
  exit 1
fi

# ── scratch repo with one commit per adaptive signal arm ─────────────────────
git init -q "$SCRATCH/repo"
cd "$SCRATCH/repo"
git config user.email spike@example.com
git config user.name "Adaptive Spike"

commit_arm() {
  local name="$1" msg="$2" lines="$3"
  {
    for i in $(seq 1 "$lines"); do
      echo "func generated_$i() int { return $i } // padding line $i for the diff signal"
      echo ""
    done
  } > "code_$name.go"
  git add .
  git commit -q -m "$msg"
  echo "$name:$msg"
}

# 1. AI-delegation: short msg + large diff (msg < 50 chars, diff > 400 chars)
commit_arm "ai-delegation" "fix:" 40
# 2. High-cluster: substantive message, small focused diff (concepts will
#    cluster at high weights over repeated commits on the same concept)
commit_arm "high-cluster" "Add exponential backoff with jitter to the retry helper so concurrent workers do not stampede the API" 3
# 3. Dispersed: many unrelated one-liner concepts across files
for i in 1 2 3 4 5; do echo "const concept_$i = $i" > "topic_$i.txt"; done
git add .
git commit -q -m "Add assorted topic notes 1 through 5, each an unrelated tiny fragment"
# 4. All-code: uniform code-sourced concepts, moderate weights
commit_arm "all-code" "Refactor retry handling across five small helper modules" 5
# 5. No-signal (fallback): tiny diff, medium message
echo "// touch" >> code_all-code.go
git add .
git commit -q -m "tidy: minor comment touch-up"

# ── daemon + per-arm question capture ────────────────────────────────────────
"$TOREC" serve >>"$LOG" 2>&1 &
DAEMON_PID=$!
sleep 2

for name in ai-delegation high-cluster dispersed all-code fallback; do
  echo "=== arm: $name ==="
  for i in $(seq 1 10); do
    "$TOREC" ask --repo "$(pwd)" --branch main >>"$ARMS_DIR/$name.txt" 2>&1 || true
  done
done

kill "$DAEMON_PID"

echo ""
echo "Resolver selections (grep the daemon log):"
grep "adaptive resolver selected" "$LOG" || echo "(none — was difficulty: adaptive configured?)"
echo ""
echo "Per-arm question captures: $ARMS_DIR/<arm>.txt"
echo "Compare question quality side-by-side, then record observations in"
echo "docs/CORE/ADAPTIVE_SPIKE.md (see openspec change adaptive-difficulty, task 5.2)."
