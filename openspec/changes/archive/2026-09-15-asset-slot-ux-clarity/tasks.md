## 1. Unmanaged-slot detection (`assets` package)

- [x] 1.1 Add `UnmanagedNames() []string` — names of `.md` files in the data dir's `prompts/` directory that are not shipped (embedded) asset names; sorted; empty when the slot is empty/absent/unresolvable
- [x] 1.2 Add `WarnUnmanagedOverrides()` — logs one line per unmanaged name: `[assets] unmanaged file %q in prompts/ — no shipped asset with this name; not active (run 'tr asset list')`; silent when the enumeration is empty
- [x] 1.3 Wire in `serveCmd` (`cmd/tr/main.go`): call `assets.WarnUnmanagedOverrides()` immediately after `assets.SetDriftThreshold(...)`
- [x] 1.4 Tests (`assets/assets_test.go`): slot with one shipped-name override + one orphan → `WarnUnmanagedOverrides` logs only the orphan line; empty/absent/unresolvable slot → silent; shipped-name override only → silent

## 2. `tr asset list` inactive tag + `sync` teaching error (`cmd/tr/asset.go`)

- [x] 2.1 In the `list` renderer, classify each name using `assets.EmbeddedNames()`: names not in the shipped set print source `inactive` instead of `$TR_HOME`; path/age columns unchanged; four-column format preserved
- [x] 2.2 Extend the `sync` unknown-asset error to `embedded asset '<name>' not found — run 'tr asset list' to see the available asset names` (still exit 1, still no write)
- [x] 2.3 Tests (`cmd/tr/asset_test.go`):
  - [x] 2.3.1 `TestAssetListTagsUnmanagedAsInactive`: slot with a shipped-name override + an orphan → orphan line has source `inactive`, shipped override line keeps `$TR_HOME`
  - [x] 2.3.2 `TestAssetListNoUnmanagedTag`: embedded-only environment → no `inactive` tag anywhere
  - [x] 2.3.3 `TestAssetSyncUnknownNamePointsAtList`: error contains `not found` and `run 'tr asset list'`; exit 1; nothing written
  - [x] 2.3.4 `TestAssetResetUnmanagedFileStillWorks`: reset on an orphan name removes it with the restart advisory (cleanup path preserved)

## 3. Documentation (FEATURES.md + README)

- [x] 3.1 `FEATURES.md` — rewrite the customization section's slot semantics: (a) the naming rule box: filename is the address, must exactly match a shipped asset name, `tr asset list` shows the finite set; don't hand-name — `tr asset sync <name>` names the file correctly; (b) deployment-target paragraph: ideas/forks/versioning live in git or a policies folder, the slot activates exactly one, swap = copy over the slot, `sync`/`reset` are the operators; (c) the `inactive` tag explanation (drop-ins that don't match a shipped name are listed but not active)
- [x] 3.2 `FEATURES.md` — add the front-matter clarification: front-matter `name:`/`description:` are descriptive metadata; the filename is how the asset is addressed
- [x] 3.3 `README.md` — review the "Make it yours" teaser against the new semantics; expected to be a no-op or a one-clause adjustment; keep it conversion tight
- [x] 3.4 Cross-check: no README/FEATURES.md text claims an arbitrary slot file will be loaded; both surfaces agree on the naming rule

## 4. Final verification

- [x] 4.1 `go build ./...`
- [x] 4.2 `go vet ./...`
- [x] 4.3 `go test ./...`
- [x] 4.4 `openspec validate asset-slot-ux-clarity`
- [x] 4.5 Manual e2e: fresh data dir → `sync question-generation-policy` → drop `my-experiment.md` by hand → `tr asset list` shows `inactive` for `my-experiment` and `$TR_HOME` for the override → start daemon → startup log contains the unmanaged line; `reset my-experiment` removes it; `sync unknown-name` exits 1 with the `tr asset list` hint
