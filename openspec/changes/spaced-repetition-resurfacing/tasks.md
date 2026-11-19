# Tasks: Spaced Repetition Resurfacing

## 1. Store foundation (scheduler state)

- [x] 1.1 Add the `resurfacing` table to `internal/cache/store.go` via `CREATE TABLE IF NOT EXISTS` (additive — no `ALTER`, no `memory.db` reset) with an index on `next_eligible_after` for the due lookup, and verify an existing `memory.db` opens and migrates without deletion by running the cache tests against a pre-seeded temp DB
- [x] 1.2 Implement answer-time seeding: on a correct answer (same transaction as the answer write), upsert one row per concept slug — `next_eligible_after = now + ladder(level)`, `interval_days` recorded, decline count cleared on renewal, terminal rows never resurrected; verify with `cache_test.go` cases for seed / renew-clears-declines / exited-stays-exited, reading the answered question's difficulty from its queued `question_events` payload
- [x] 1.3 Implement the ladder and state transitions: easy=3 / intermediate=7 / hard=14 (unknown → intermediate), serve-time defer (+1 day, applied on offer), decline count with cap 3 → `declined-out`, answered-bonus → `answered-out`; verify with table-driven `cache_test.go` cases per the spec's ladder and decline scenarios

## 2. Daemon bonus contract

- [x] 2.1 Implement `GET /recall/bonus`: enabled + due concept → serve the most-overdue (largest overage, oldest-seeded tiebreak), apply the serve-time defer, return offer payload (concept slug + prose); none due or `recall.resurfacing: false` → 204; verify with headless integration tests in `integration_test.go` (startTestDaemon + seed via the store) covering due / none-due / disabled / second-request-already-deferred
- [x] 2.2 Implement decline recording and the accept flow on the same route family: decline increments the count (third → `declined-out`); accept synthesizes and stores the question (`claimed_by: shell`, queued event payload `origin: resurfaced`) and returns it in the response — no second dequeue; verify with integration tests for decline-1/decline-3 and accept-return
- [x] 2.3 Wire the resurfaced-concept synthesis path in `internal/recall`: no diff snippet, no commit context, concept passed the way `FocusConcepts` is, difficulty = the session's current adaptive resolution; synthesis failure returns a no-question outcome (serve-time defer stands, no decline); verify with engine-level tests asserting the composed synthesis request and the failure path
- [x] 2.4 Confirm the MCP surface is untouched: `recall_next` / `recall_answer` / resources produce identical contracts with resurfacing enabled and a concept due; verify with an integration test asserting unchanged responses

## 3. Config and policy

- [x] 3.1 Add `recall.resurfacing` (default `true`, user-level, deep-merged) and thread it to the engine; verify in `config_test.go` (default, merge from user/repo tiers, `torec config --show` rendering)
- [x] 3.2 Add the "resurfaced concept" framing section to `assets/prompts/question-generation-policy.md` (durable understanding, no commit context, never reference a commit) and verify `torec asset show` loads the shipped asset and the synthesis request carries the updated body

## 4. Ask session (TUI)

- [x] 4.1 Extend the ask state machine: after feedback, if an offer was served, present the bonus offer (`y` accept / `n` decline / `q`-`Esc`-kill silent defer) using the agreed copy ("Would you like a bonus question to boost your knowledge retention? y/n"); `n` posts the decline; verify with model-isolation tests in `ask_test.go` per state transition
- [x] 4.2 Render and resolve the bonus question through the existing question interaction (choices, Enter skip, q/Esc), then end the session; verify with model-isolation tests plus a golden file for the offer view (`UPDATE_GOLDEN=1` workflow per AGENTS.md)
- [x] 4.3 Guarantee the no-offer path is byte-identical to today's behavior (feedback → done, no extra HTTP call beyond the offer probe); verify with a golden run of the caught-up and question views

## 5. End-to-end validation

- [ ] 5.1 Headless e2e: seed a concept, answer it correctly, manipulate the store clock to make it due, drive the daemon hook + ask HTTP contract end to end (correct answer → offer served → accept → fresh question → answer → `answered-out`), and separately the decline-to-`declined-out` path; verify with an integration test asserting store state at each step
- [ ] 5.2 Manual e2e: drive the flow against a scratch data dir (`TR_HOME`) with a real provider per `scripts/e2e/README.md` — commit → question → bonus offer → accept → retention question → stats-visible provenance; record the walkthrough output in the change notes
- [ ] 5.3 Update `GLOSSARY.md` if new domain terms entered the ubiquitous language (e.g. "resurfacing", "bonus offer", "decline-out"), per its vocabulary-only style; verify the domain-modeling audit passes locally
- [ ] 5.4 Full gate: `go build ./... && go vet ./... && go test ./...` green; changelog entry drafted under Unreleased
