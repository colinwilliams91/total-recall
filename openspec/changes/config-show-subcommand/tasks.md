## 1. `tr config show` subcommand + deprecated alias (`cmd/tr/main.go`)

- [ ] 1.1 Add `configShowCmd()` — `Use: "show"`, `Short: "Print the fully resolved config with source annotations"`, `Args: cobra.NoArgs`, `RunE` → existing `runConfigShow()`; register as subcommand of `configCmd()` (which keeps its bare-help behavior)
- [ ] 1.2 Keep the `--show` flag on the parent `config` command but mark it hidden (`cmd.Flags().MarkHidden("show")`); when set, print the deprecation notice `note: 'tr config --show' is deprecated — use 'tr config show'` to stderr before `runConfigShow()`, suppressed when `quiet` is set
- [ ] 1.3 Tests (`cmd/tr/config_test.go` or colocated in the config test file):
  - [ ] 1.3.1 `TestConfigShowSubcommand`: `configCmd()` with args `["show"]` → stdout contains `[user]` annotations and the `prompt assets:` section
  - [ ] 1.3.2 `TestConfigShowFlagDeprecatedNotice`: args `["--show"]` → stdout shows the config AND stderr contains the deprecation notice (use `captureStdout`/`captureStderr` helpers)
  - [ ] 1.3.3 `TestConfigShowFlagQuietHidesNotice`: args `["--show", "--quiet"]` → notice absent, config still renders
  - [ ] 1.3.4 `TestConfigHelpListsShowNotShowFlag`: `configCmd()` with `["--help"]` → help lists the `show` subcommand and does NOT mention `--show`
  - [ ] 1.3.5 `TestConfigShowRejectsArgs`: args `["show", "extra"]` → error (NoArgs)

## 2. Documentation sweep (`tr config --show` → `tr config show`)

- [ ] 2.1 `README.md` Configuration section
- [ ] 2.2 `FEATURES.md` observability section
- [ ] 2.3 `AGENTS.md` config-show mention (Config line / anywhere `tr config --show` appears)
- [ ] 2.4 Repo-wide sweep: `grep -rn "config --show"` outside openspec/ archives must return zero doc hits (openspec archived artifacts and canonical specs are historical truth — the canonical `config-override-chain` scenario stays `--show` because the flag form remains functional; leave it)

## 3. Final verification

- [ ] 3.1 `go build ./...`
- [ ] 3.2 `go vet ./...`
- [ ] 3.3 `go test ./...`
- [ ] 3.4 `openspec validate config-show-subcommand`
- [ ] 3.5 Manual: `tr config show` renders fully incl. `prompt assets:`; `tr config --help` lists `show`, hides `--show`; `tr config --show` still renders + notice; `tr config --show --quiet` renders silently; `tr config` (bare) unchanged help
