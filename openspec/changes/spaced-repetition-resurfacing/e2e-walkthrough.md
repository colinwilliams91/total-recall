# Manual e2e walkthrough: spaced-repetition resurfacing (task 5.2)

Date: 2026-11-20 · Branch: `10-opsx-d` · Provider: OpenRouter (`openai/gpt-4o-mini`,
`env:OPENROUTER_API_KEY`) · Scratch data dir: `/tmp/opencode/tr-resurfacing-e2e`
(`TR_HOME`) · Scratch repo: `/tmp/opencode/tr-scratch-repo` (own git repo, hooks
installed via `torec repo` under a PTY).

## How it was driven

A real git commit fired the installed `pre-commit` hook against the running
daemon, so concept extraction and commit-question synthesis were real AI calls.
The post-commit `torec ask` TUI exits silently without a TTY in this
environment, so the ask-session side of the contract was driven with the exact
HTTP calls the TUI session makes (the same approach as `scripts/e2e/adaptive-ab.sh`).
The interactive y/n keystroke behavior itself is covered by the model-isolation
tests and the offer-view golden file in `cmd/torec/ask_test.go` / `golden_test.go`.

## Walkthrough (real provider, real commits)

1. **Commit → question.** Two commits; the second ("add retry with exponential
   backoff and jitter") extracted concepts and queued question id=2,
   "What invariant does code commenting preserve in a collaborative
   development environment?" (`recall.difficulty: adaptive` → resolved
   `intermediate`, signals `fallback, cold-start`).
2. **Correct answer → seeding.** `POST /recall/select` with the correct choice
   (index 2 — the engine shuffles choice order) returned `correct: true`, and
   the answer transaction seeded two scheduler rows (the question's provenance
   concepts), both at the intermediate ladder:

   ```
   slug                    seeded_at            next_eligible        interval declined resurfaced exit
   code_commenting         2026-11-19 22:24:13  2026-11-26 22:24:13  7        0        0          NULL
   version_control_basics  2026-11-19 22:24:13  2026-11-26 22:24:13  7        0        0          NULL
   ```

3. **Probe before decay → no offer.** `GET /recall/bonus` → `204` (nothing due).
4. **Clock fast-forward → offer + serve-time defer.** After moving
   `next_eligible_after` two days into the past, `GET /recall/bonus` → `200`
   `{"slug":"code_commenting","prose":"code_commenting"}`; the serve pushed the
   row one day out and `resurfaced_count` → 1. A second probe → `204` (already
   deferred — the silent-defer window is observable).
5. **Accept → fresh, marked question.** `POST /recall/bonus/accept`
   `{"slug":"code_commenting","repo":…,"branch":"master"}` → `200` with a newly
   synthesized question, "What problem does code commenting solve in the
   context of maintaining a codebase?" — concept-level framing, no commit
   referenced. Stored state: `status=delivered` with `actor=shell` (no second
   dequeue), queued payload `{"origin":"resurfaced","resolved_difficulty":
   "intermediate","signals":[]}` (stats-visible provenance), and
   `question_concepts` = the due concept only.
6. **Bonus answered → answered-out.** The bonus answer recorded normally
   (`correct: true`); the scheduler row exited: `declined_count=0,
   resurfaced_count=1, exit_reason='answered-out'`. Further probes → `204`.
7. **Decline → declined-out.** For `version_control_basics`, three rounds of
   (clock fast-forward → `200` offer → decline) returned
   `{"ok":true,"exited":false}` twice and `{"ok":true,"exited":true}` on the
   third, exiting the row `declined_count=3, exit_reason='declined-out'`; the
   concept is never offered again (`204` afterwards).

## Observations

- Commit latency unchanged: the hook returned before the async pipeline even
  started synthesis; the bonus offer only costs one extra local HTTP call per
  ask session when nothing is due.
- The offer prose mirrors the registry prose; with this run's extractor the
  prose equals the slug (`code_commenting`). TUI framing still labels it as a
  retention bonus, and richer extractor prose flows through unchanged.
- Cross-repo eligibility is by construction (user-scoped scheduler rows); this
  walkthrough used one repo, and the integration tests cover the cross-surface
  selection.
