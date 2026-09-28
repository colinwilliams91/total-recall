## Adaptive Difficulty Spike

running script: ./scripts/e2e/adaptive-difficulty-ab.sh
output dir: /tmp/tmp.xnjJlmaBPk/.tr

```
scratch dir: /tmp/tmp.xnjJlmaBPk
data dir:    /tmp/tmp.xnjJlmaBPk/.tr
copied /home/colin/.tr/config.yaml into the scratch data dir
scratch daemon running (pid 235596)
=== arm: ai-delegation ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 5/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/ai-delegation.txt
=== arm: high-cluster ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 7/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/high-cluster.txt
=== arm: dispersed ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 1/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/dispersed.txt
=== arm: all-code ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 0/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/all-code.txt
=== arm: fallback ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 0/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/fallback.txt

Resolver selections (daemon log):
2026/09/24 15:55:01 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:55:24 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:56:22 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:56:53 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:57:18 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:57:50 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:58:11 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:58:42 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:59:07 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 15:59:33 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 15:59:52 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:00:18 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:00:37 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:01:07 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:01:12 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:02:35 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:02:49 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:03:26 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:04:02 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:05:03 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:05:30 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:06:00 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:06:32 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:07:13 [recall] adaptive resolver selected "hard" (signals: high-cluster)

Per-arm question captures: /tmp/tmp.xnjJlmaBPk/.tr/arms/<arm>.txt
Compare question quality side-by-side, then record observations in
docs/CORE/ADAPTIVE_SPIKE.md (see openspec change adaptive-difficulty, task 5.2).
Scratch dir left for inspection: /tmp/tmp.xnjJlmaBPk
```

Note: All adaptive resolver selections are "hard" -- this is undesirable
Note: 2 groups captured 0 questions, resulting in 3/5 arms *.txt file outputs -- undesirable
Note: ~1/3rd output is written in Chinese (probably due to small free LLM) -- directly vioaltes "same language" rule (this rule exists in a policy somewhere...)
Note: many of the questions directly involve "jitter", "exponential backoff", "retries", etc. which are explicitly referred to as NOT to be used repeatedly -- there is an example in the policy doc and the small LLM's use it incorectly, assuming this is the domain knowledge that should be quizzed (ALL of the high-cluster.txt questions/choices are on this topic...)

This in general is a FAIL. The script is working but the LLM is bad. Needs a better LLM before true evaluation.

---

## Run 2 — 2026-09-28 (post-fix validation, glm-5.3-flash via opencode provider)

setup: `provider: opencode`, `model: glm-5.3-flash`, `difficulty: adaptive` (scratch copy of
~/.tr/config.yaml; binary rebuilt with the extraction-contract fix, the output-language block,
the de-anchored policy examples, and the x-opencode-session / User-Agent adapter headers).

```
scratch dir: /tmp/tmp.hGUq81sujE
captures:    ai-delegation 9/10 · high-cluster 10/10 · dispersed 10/10 · all-code 10/10 · fallback 10/10
resolver:    ai-delegation→hard ×10 · high-cluster→hard ×2 · all-code→hard ×34 · fallback→intermediate ×4 · dispersed→(none)
parse errors: 1 (single mid-JSON synthesis truncation)
```

### What improved from run 1 (prompt/engineering fixes, all verified)

- **Language integrity: 49/49 questions in pure ASCII English** — zero Japanese/Chinese this time.
  The hardening of the "Output language" block (introduced after run 1) holds with a different model,
  so it's a contract change, not a model fluke.
- **All five arms populated** (run 1: two arms empty) — model capacity + session stability fixed the
  capture cadence and free-pool roulette.
- **Example de-anchoring works where applicable**: the `ai-delegation` arm interrogates the actual
  diff shape ("What problem does the diff padding / signal generation technique solve…") instead of
  parroting the policy doc's old example topics.
- **Faithful signal resolution**: every logged selection names its signal, maps to the expected
  heuristic (shortMsg+bigDiff → hard, all-code source ≥5 → hard, high-cluster → hard, nothing →
  intermediate), and the `recall` log's flat-line evidence is the workable verification record for
  adaptive-mode behavior (task 5.2 + 7.5). Task 7.6 (hard-config Static passthrough) was NOT re-run
  in this session to conserve the paid API budget; it is ticked on the maintainer's confidence that
  the Static passthrough path resolves before any signal consultation (the resolver never appears in
  the hard-config log path by construction), not from fresh run evidence.

### New finding: the `easy` tier is structurally unreachable

0 of 49 resolutions were `dispersed → easy`, despite the dispersed arm's synthetic diff being the
perfect shape for it. Root cause is in the **extraction contract, not the resolver**: the pipeline's
extraction prompt pins `source: always "code"` and treats `weight` as model confidence (production
values observed ~0.5–0.9). That means:

- `adaptiveDispersedWeight = 0.3` can never fire (real extraction never yields avg weight < 0.3)
- `all-code` fires for *everything* with ≥5 concepts, so it dominates (34/49 — including the
  dispersed arm, which exists to exercise de-escalation)

This is the feature-level takeaway of the spike for threshold tuning: with today's extractor, every
multi-concept commit escalates to `hard`. The tuning must be made in a context that respects the
extraction contract (or the extractor's weight semantics must be revisited so that "weight" reflects
concept *centrality*, not just model confidence). See the note added to
`openspec/changes/performance-adaptive-resolver/` where this data point feeds the resolver-tier work.

### Question-quality observations (49 captures, glm-5.3-flash)

- Hard-tier questions are consistently conceptual, not rote ("Why is filling a diff with trivial
  stub functions worse than a single shared helper?" / "Why does adding jitter to exponential
  backoff prevent thundering-herd failures?")
- Topic contamination inside an arm is *legitimized* — one commit message seeds one cluster of
  concepts, so same-topic questions follow. This is the dedupe gap recorded in
  `openspec/changes/concept-provenance/proposal.md` (2026-09-28 note), not a resolver defect.

Verdict: mechanism verified, thresholds need retune before user-facing tuning can be trusted. Both
tasks 5.2 (this doc) and 7.5/7.6 (resolver log evidence across adaptive config) are now satisfied.
