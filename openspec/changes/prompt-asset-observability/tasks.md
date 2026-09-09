## 1. Startup logging enrichment (`assets/assets.go`)

- [x] 1.1 Export `LoadAll() []PromptAsset` — walks every `.md` file in the embedded `prompts/` directory and the `$TR_HOME/prompts/` directory (if `$TR_HOME` set), returns one resolved `PromptAsset` per known name with `Source`, resolved path, and parsed content. Used by `tr asset list` and `tr config show`.
- [x] 1.2 Extend the override path's existing log line in `Load` to include the override's mtime (`%s`, RFC3339) and the embedded default's compile-time reference (`SourceEmbedded` build marker). Format: `[assets] prompt asset %q loaded from override at %s (mtime=%s, embedded ref=%s)`.
- [x] 1.3 Compute the delta between override's mtime and the embedded default's compile-time reference. When `delta > threshold` (threshold via new `cfg.PromptAsset.DriftWarningDays`, default 90), emit an additional line: `[assets] OVERRIDE WARNING: %s is %dd older than the embedded default — re-sync with 'tr asset sync %s'`. Threshold configurable per decision in design.md.
- [x] 1.4 Add the package-level `driftThresholdDays int` (defaulted to 90, settable via `SetDriftThreshold(days int)` so the daemon can wire the config). The package stays zero-config when the config is unset.
- [x] 1.5 Tests (`assets/assets_test.go`):
  - [x] 1.5.1 `TestLoadWarnsOnStaleOverride`: write an override with mtime 100 days ago (use `os.Chtimes`), `SetDriftThreshold(90)`, assert log line contains `OVERRIDE WARNING` and `100d older`
  - [x] 1.5.2 `TestLoadNoWarningWhenRecent`: override with mtime 10 days ago, `SetDriftThreshold(90)`, assert no `OVERRIDE WARNING` in log
  - [x] 1.5.3 `TestLoadWarnsAtConfigurableThreshold`: override 35 days old, `SetDriftThreshold(30)`, assert warning fires
  - [x] 1.5.4 `TestLoadAllEnumeratesEmbeddedAndOverrides`: with `$TR_HOME` set and one override present, `LoadAll` returns one `PromptAsset` per distinct name across both sources (override wins on name collisions)

## 2. Config wiring (`internal/config/config.go`)

- [x] 2.1 Add new struct `PromptAssetConfig { DriftWarningDays int yaml:"drift-warning-days" }` field on `UserConfig` as `PromptAsset PromptAssetConfig yaml:"prompt-asset,omitempty"`
- [x] 2.2 Default in `DefaultUserConfig`: `PromptAsset: PromptAssetConfig{DriftWarningDays: 90}`
- [x] 2.3 Config source-tracking: extend `Sources` struct with `PromptAssetDriftWarningDays string`; merge logic in `merge.go` follows the existing per-field pattern (default/repo precedence as appropriate — prompt-asset is user-level only per the two-tier config rules; `Privacy`/`AI` style discard applies if it ever appears in `.tr.yaml`)
- [x] 2.4 `internal/config/show.go` — add a "prompt assets" section to `Show()` output. After the existing config rendering, iterate the known embedded prompt names (currently just `question-generation-policy`), call `assets.Load(name)`, and print one line per asset: `  prompt: <resolved-path>  # [<source>], <human-age>` where `<human-age>` is derived from the override's mtime (e.g., `6mo old`, `2d old`) or `embedded` when no override is in effect
- [x] 2.5 Tests (`cmd/tr/config_test.go`):
  - [x] 2.5.1 `TestConfigShowListsPromptAssets`: with `$TR_HOME` set and an override present, `Show(buf)` output contains a `prompt assets:` section listing the resolved override path and `[override]` source tag
  - [x] 2.5.2 `TestConfigShowEmbeddedDefault`: with `$TR_HOME` unset, `Show(buf)` lists the asset as `embedded` with no override path
  - [x] 2.5.3 `TestDefaultDriftWarningDaysIs90`: `DefaultUserConfig().PromptAsset.DriftWarningDays == 90` (verifies the default lands)

## 3. `tr asset list` (`cmd/tr/asset.go`)

- [x] 3.1 New `assetCmd() *cobra.Command` returns a Cobra command with `Use: "asset"`, `Short: "Inspect and manage prompt-asset overrides"`. Subcommands wired via `AddCommand`.
- [x] 3.2 `listAssetCmd() *cobra.Command` with `Use: "list"`, `Short: "List resolved prompt assets"`; `RunE` calls `assets.LoadAll()`, prints one line per asset in plain text: `<name>\t<source>\t<resolved-path>\t<age>` (tab-separated to make `awk`/`grep` friendly). The `#` comment column from `config show` is omitted here; `list` is for parsing, `show` is for eyeballing.
- [x] 3.3 Register `assetCmd` in `main.go` root command tree.
- [x] 3.4 Tests (`cmd/tr/asset_test.go`):
  - [x] 3.4.1 `TestAssetListNoOverrides`: `$TR_HOME` unset, `tr asset list` output contains `question-generation-policy` with `embedded` source and the canonical `assets/prompts/...` path
  - [x] 3.4.2 `TestAssetListWithOverride`: `$TR_HOME` set with one override file, output shows `<name>\t$TR_HOME\t<override-path>\t<age>` for the overridden asset
  - [x] 3.4.3 `TestAssetListMultipleOverrides`: with two overrides, both appear; non-overridden embedded assets still appear as `embedded`

## 4. `tr asset reset [<name>]` (`cmd/tr/asset.go`)

- [x] 4.1 `resetAssetCmd() *cobra.Command` with `Use: "reset [<name>]"`, `Short: "Remove an override so the embedded default takes effect on next daemon restart"`. Accepts optional `<name>` positional arg.
- [x] 4.2 Behavior — `<name>` given: `os.Remove($TR_HOME/prompts/<name>.md)`; if the file doesn't exist, exit 0 with a short "no override for <name> — nothing to reset" message (no error). If `$TR_HOME` is unset, exit 1 with "TR_HOME is not set; nothing to reset" (no override could exist).
- [x] 4.3 Behavior — no `<name>`: enumerate every `.md` under `$TR_HOME/prompts/` (if it exists). If zero files, exit 0 with "no overrides to reset". If exactly one, prompt for confirmation (in interactive TTY only via `term.IsTerminal`); in non-TTY, require an explicit `--all` flag to proceed. If more than one, require `--all` in both TTY and non-TTY; confirm interactively before removing when TTY.
- [x] 4.4 Add `--all` flag and `--force` flag; `--all` enables the no-arg multi-remove path; `--force` skips the TTY confirmation prompt.
- [x] 4.5 On success, log `[assets] removed override at <path>; restart 'tr serve' to pick up the change`. Exit 0.
- [x] 4.6 Tests:
  - [x] 4.6.1 `TestAssetResetSingleOverride`: create one override, run `reset <name>`, assert file gone, stdout contains the restart advisory
  - [x] 4.6.2 `TestAssetResetNoOverrideIsNoOp`: no override present, `reset <name>` exits 0 with the "nothing to reset" message
  - [x] 4.6.3 `TestAssetResetMultipleRequiresAllFlag`: two overrides, `reset` (no args) without `--all` refuses and exits non-zero; with `--all --force` both files are gone
  - [x] 4.6.4 `TestAssetResetNoTrHomeExits1`: `$TR_HOME` unset, exits 1 with the "TR_HOME not set" message

## 5. `tr asset sync [<name>]` (`cmd/tr/asset.go`)

- [x] 5.1 `syncAssetCmd() *cobra.Command` with `Use: "sync [<name>]"`, `Short: "Refresh an override with the canonical embedded default content"`. Accepts optional `<name>` positional arg.
- [x] 5.2 Behavior — `<name>` given: read embedded bytes for `<name>.md` from `assets.embedFS`, write them to `$TR_HOME/prompts/<name>.md`. If the target file exists and is non-empty, require `--force` (default refuses with `error: <path> exists and is non-empty — pass --force to overwrite, or 'tr asset reset <name>' to start from defaults`). If `--force` is passed, overwrite.
- [x] 5.3 Behavior — no `<name>` argument: refuses with `error: sync requires an explicit asset name` (unlike `reset`, sync-all is too easy to mistake for "reset everything to defaults" — a destructive operation users might misread as restoring defaults when it actually *creates* overrides). The single-name form forces intent.
- [x] 5.4 If `$TR_HOME` is unset, exit 1 with "TR_HOME is not set; nothing to sync to" (no override location exists).
- [x] 5.5 If the embedded bytes for `<name>` are unavailable (corrupt build), exit 1 with `error: embedded asset <name> not found`.
- [x] 5.6 On success, log `[assets] synced <name> to <path>; restart 'tr serve' to pick up the change`. Exit 0.
- [x] 5.7 Tests:
  - [x] 5.7.1 `TestAssetSyncCreatesNewOverride`: no override present, `sync <name>` writes the embedded bytes to `$TR_HOME/prompts/<name>.md`; `assets.Load` now returns `Source==$TR_HOME` after restart equivalent (assert via re-reading the file)
  - [x] 5.7.2 `TestAssetSyncOverwritesWithForce`: existing non-empty override, `sync <name>` without `--force` refuses; with `--force`, content matches embedded bytes
  - [x] 5.7.3 `TestAssetSyncEmptyNoNameRefuses`: no args, exits non-zero with the explicit-name error
  - [x] 5.7.4 `TestAssetSyncNoTrHomeExits1`: `$TR_HOME` unset, exits 1

## 6. Documentation sync

- [x] 6.1 `AGENTS.md` — append to the existing Prompt assets line a brief mention of `tr asset list|reset|sync` and the override observability surface
- [x] 6.2 `DOCS/CORE/DATA_ANALYSIS.md` — no change; the existing "Status: wired" note from `synthesize-from-context` already covers the override mechanism; observability is an implementation detail
- [x] 6.3 Update `synthesize-from-context` design.md "Risk" section to cross-reference this change ("Override drift / silent rot mitigated by `prompt-asset-observability`").
- [x] 6.4 `README.md` — add one line in the Setup block mentioning `tr --help` (or `tr help`) lists all commands/flags. Help is pre-existing Cobra behavior (`tr --help`, `tr -h`, and `tr help` all work today); this is a docs-only line, no code change.
- [x] 6.5 `README.md` — add a short "Managing prompt-asset overrides" note covering `tr asset list|reset|sync`: what each does (`list` inspects resolved sources/ages, `reset` discards an override, `sync` re-baselines from the embedded default) and the reset/sync restart-advisory caveat (daemon restart required to pick up file mutations).
- [x] 6.6 `DOCS/CONTRIBUTING.md` — add an `asset` row to the "Available subcommands" list in the Run section so the per-command inventory stays current.

## 7. Final verification

- [x] 7.1 `go build ./...`
- [x] 7.2 `go vet ./...`
- [x] 7.3 `go test ./...`
- [x] 7.4 `openspec validate prompt-asset-observability`
- [x] 7.5 Manual: `tr asset sync question-generation-policy` from a fresh `$TR_HOME`, edit the synced override, `tr asset list` shows `[override]` with correct age, `tr config show` shows the override in its new section, `tr asset reset` removes it, daemon restart picks up the change each time
