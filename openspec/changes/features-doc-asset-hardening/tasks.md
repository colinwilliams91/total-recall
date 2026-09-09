## 1. `FEATURES.md` (new, repo root)

- [ ] 1.1 Write `FEATURES.md` per the design IA: "What you get today" (commit-time quizzes; delivery surfaces terminal TUI + MCP; drop-in prompt customization with the override loop, `tr asset list|sync|reset`, drift warning, restart caveat; `tr config --show` observability), "Coming soon" (adaptive difficulty — explicitly marked in development), "Community" (policy-sharing concept + the trust note: policy files are prompts, review what you import). Target: readable in two minutes, no prerequisite framing
- [ ] 1.2 Every command shown in FEATURES.md must be real and current (`tr --help` cross-check); no flags that don't exist

## 2. `README.md` slim-down + teaser

- [ ] 2.1 Replace the "Managing prompt-asset overrides" section body with the concept hook (quizzes are shaped by a markdown policy doc shipped inside the binary; drop in your own — no recompile; `tr asset list|sync|reset` manage it) + teaser sentence linking `FEATURES.md` ("the full tour: what Total Recall does, how to bend it to your domain, and what's coming")
- [ ] 2.2 Remove the drift-warning/config-show detail from the README (now owned by FEATURES.md); keep restart caveat to one clause
- [ ] 2.3 Verify no FEATURES.md content is duplicated verbatim in the README and both docs cross-link (README → FEATURES.md; FEATURES.md → README setup)

## 3. `tr asset` long help (`cmd/tr/asset.go`)

- [ ] 3.1 Add `Long` to `assetCmd()` covering: what an override is (markdown file in the data dir's `prompts/` replacing the shipped default), list vs reset vs sync intent, restart caveat. Per design: loop, not marketing
- [ ] 3.2 Test: assert `tr asset --help` / `tr help asset` output contains the override-loop description and the restart caveat (model-isolation test in `cmd/tr/asset_test.go`)

## 4. Asset-name validation (`cmd/tr/asset.go`)

- [ ] 4.1 Add `validateAssetName(name string) error` — accepts `^[a-z0-9-]+$`; error message names the offending argument and the expected form (`invalid asset name '<arg>' — expected a single lowercase-hyphenated name, e.g. 'question-generation-policy'`)
- [ ] 4.2 Wire into `reset` (explicit-name path) and `sync` before any path construction; batch reset path untouched; refusal exits 1 via RunE error per existing pattern
- [ ] 4.3 Tests (`cmd/tr/asset_test.go`): table tests — canonical name accepted; `../../sensitive` rejected; `../prompts` rejected; `"policy.md"` (user-supplied `.md` suffix) rejected; `"Question Policy!"` rejected; valid `reset`/`sync` behavior unchanged for accepted names
- [ ] 4.4 Update `openspec/changes/prompt-asset-observability` cross-reference? No — its specs describe reset/sync accepting `<name>`; the validation requirement composes additively. No edits needed there (deliberate check)

## 5. Documentation sync

- [ ] 5.1 `AGENTS.md` — no change (architecture doc; FEATURES.md is user-facing). Deliberate check, no edit
- [ ] 5.2 `DOCS/CONTRIBUTING.md` — add one line to the docs inventory if one exists; otherwise no change. Verify and decide

## 6. Final verification

- [ ] 6.1 `go build ./...`
- [ ] 6.2 `go vet ./...`
- [ ] 6.3 `go test ./...`
- [ ] 6.4 `openspec validate features-doc-asset-hardening`
- [ ] 6.5 Manual: `tr asset --help` shows the loop; `tr asset sync ../prompts` and `tr asset reset ../../x` exit 1 with the validation message; README teaser link resolves to FEATURES.md on disk
