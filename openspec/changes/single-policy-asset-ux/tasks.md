## 1. No-arg CLI semantics (`cmd/tr/asset.go`)

- [x] 1.1 Add `soleAssetFrom(names []string) (string, error)` — one name → it; zero → `no shipped prompt assets — nothing to sync to`; several → `multiple shipped prompt assets — specify one of: <n1, n2, ...>`; wrapper `soleShippedAsset()` feeds `assets.EmbeddedNames()`
- [x] 1.2 `sync`: no args → resolve the sole shipped asset and proceed (MkdirAll, `--force` gate, teaching error, advisory unchanged); explicit-name branch unchanged
- [x] 1.3 `reset`: no-arg → resolve sole asset and `removeOverride` it (existing single-file semantics: missing → "no override for <name> — nothing to reset", exit 0); delete the batch path, `--all` and `--force` flags, `resetAllOverrides`, the TTY `confirm` helper, and the now-unused `term` import; named path unchanged (permissive stray cleanup)
- [x] 1.4 Rewrite `assetCmd` `Long` help: no-arg primary forms (`sync` / `reset`), the "one policy, one file, sync names it" rule, the restart caveat; keep the `inactive`/restart machinery line short

## 2. Tests (`cmd/tr/asset_test.go`, model isolation)

- [x] 2.1 `TestSoleAssetFrom`: table (zero → error; one → it; several → error listing names)
- [x] 2.2 `TestAssetSyncNoArgTargetsSoleAsset`: fresh slot → no-arg sync creates `<data-dir>/prompts/question-generation-policy.md` with embedded bytes + advisory
- [x] 2.3 `TestAssetResetNoArgTargetsSoleAsset`: override present → removed + advisory; absent → "nothing to reset" exit 0
- [x] 2.4 Retire `TestAssetResetMultipleRequiresAllFlag` and the no-arg refusal tests (superseded contracts); keep named forms, unresolvable-dir, stray-cleanup (`TestAssetResetUnmanagedFileStillWorks`), validation tests, `inactive` tests
- [x] 2.5 Long-help test updated for the new wording (no-arg forms, no `--all` reference)

## 3. Documentation — the singular truth

- [x] 3.1 `FEATURES.md` customization section → four-step walkthrough (`tr asset sync` → edit the file at the printed path → restart `tr serve` → `tr asset show` confirms); two-sentence naming rule ("One policy. One file. `sync` names it — never type an asset name. Anything else in the slot is listed `inactive` — present, ignored, cleanable with `tr asset reset <name>`."); keep the `/policies/` deployment-target blurb and the drift warning; delete the Community policy-sharing section, keeping one trust sentence ("a shared doc deploys by copying over the slot file — review what you import")
- [x] 3.2 `FEATURES.md` front-matter clarification trimmed to one clause (filename is how the asset is addressed)
- [x] 3.3 `README.md` teaser: drop the `<name>` from the sync hint ("`tr asset sync` names the file for you")
- [x] 3.4 `AGENTS.md` prompt-assets line + `assets/assets.go` package comment: singular phrasing (the shipped asset — the question-generation policy; no "future assets" promise)

## 4. Final verification

- [x] 4.1 `go build ./...`
- [x] 4.2 `go vet ./...`
- [x] 4.3 `go test ./...`
- [x] 4.4 `openspec validate single-policy-asset-ux`
- [x] 4.5 Manual e2e: fresh data dir → `tr asset sync` (no args) creates the override → edit → `tr asset show` shows `$TR_HOME` → `tr asset reset` (no args) removes it → `sync my-experiment` exits 1 with the name-validation + inventory error → `sync` again, edit, `sync --force` overwrites — every step argument-free unless a stray is being cleaned
