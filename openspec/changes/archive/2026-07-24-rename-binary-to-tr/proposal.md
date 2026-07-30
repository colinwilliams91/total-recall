## Why

The binary is named `total-recall` but invocations throughout the codebase use it dozens of times in user-visible strings, prompt text, and shell commands. Users will type this command dozens of times per day; `tr` is shorter, faster, and matches the existing `tr ask`, `tr init`, `tr serve`, `tr status`, `tr config` command prefixes already used in error messages and documentation. Renaming early — before later phases add more surfaces — prevents writing `total-recall` everywhere and renaming it later.

## What Changes

- Rename the source directory `cmd/total-recall/` to `cmd/tr/`. All files move via `git mv` (main.go, wire.go, ask.go, and 10 *_test.go files).
- Update `Makefile` so `make build` runs `go build -o bin/tr ./cmd/tr` and `make install` runs `go install ./cmd/tr` (the binary name is `tr` automatically because it matches the directory).
- Update `scripts/rebuild.ps1` line 8: `./cmd/total-recall` → `./cmd/tr`.
- Update `.goreleaser.yaml`: `main: ./cmd/total-recall` → `./cmd/tr`; `binary: total-recall` → `binary: tr`.
- Update `README.md`: every `total-recall init`, `total-recall serve`, etc. becomes `tr init`, `tr serve`, etc.
- Update user-visible CLI invocation strings inside hook script bodies and Go source — ("Re-run 'total-recall init' to update" → "Re-run 'tr init' to update", "Start with 'total-recall serve'" → "Start with 'tr serve'", etc.). Affected files: `internal/hooks/scripts.go` (8 string sites), `cmd/total-recall/main.go` (3 string sites), `cmd/total-recall/ask.go` (2 string sites).

**Non-changes (deliberate):**

- The Go module path `github.com/colinwilliams91/total-recall` is **unchanged**. Renaming the module path is a separate, larger effort explicitly excluded by the plan.
- All `internal/*` package import paths are unchanged (theyre keyed off the module path, not the `cmd/` subdirectory name).
- The hook sentinel `# total-recall managed` (in `internal/hooks/scripts.go:4` and `cmd/.../main.go:102`'s post-commit template) is **kept** — this is the product identity marker on disk (Total Recall is the product name; `tr` is only the typed CLI command). The installer continues to recognize existing managed hooks during re-install.
- The `[total-recall]` stderr prefix on advisory messages inside hook scripts (`[total-recall] Daemon not running...`, `[total-recall] curl not found...`) is **kept** for the same reason — it is a product label, not a typed command.
- No backwards-compat alias for `total-recall` ships — users who currently invoke `total-recall` switch to `tr`. (There are no users; this is a non-issue.)
- The post-commit hook path-baking logic (`os.Executable()` capture in `main.go:259`) is left untouched here — Y4 owns that. Any `[total-recall ...]` references inside the post-commit hook template's *content* (not the sentinel) are renamed; the path capture mechanism itself is Y4's concern.
- No `cmd/total-recall/` directory is left behind as a stub or shim. After the move, the directory does not exist.

## Capabilities

### New Capabilities

None. This is a rename, not a behavior change.

### Modified Capabilities

- `post-commit-hook`: the generated post-commit hook SCript's CLI invocation changes from `total-recall ask` to `tr ask`; the spec's existing `exec "$(which total-recall)" ask` requirement is amended to `exec "$(which tr)" ask`. (This aligns with Y4s decision to rely on PATH rather than baking the absolute binary path; the spec's `$(which ...)` form already matches the post-Y4 strategy.)

## Impact

- **Code moved**: every Go file under `cmd/total-recall/` is `git mv`'d to `cmd/tr/`. Package declarations (`package main`) and imports of `internal/...` packages are unchanged.
- **Code edited**: `Makefile`, `scripts/rebuild.ps1`, `.goreleaser.yaml`, `README.md`, plus string-literal edits in `internal/hooks/scripts.go`, `cmd/tr/main.go`, `cmd/tr/ask.go` (paths above reflect post-move locations).
- **Tests**: `*_test.go` files move with their source; no test logic changes (string assertions in `main_test.go` for `buildPostCommitHookScript` may change if they assert on `Re-run 'total-recall init'` text — needs verification).
- **Tooling**: `go build ./...`, `go vet ./...`, `go test ./...` continue to pass after the rename. `make build` still produces a working binary, now named `tr` (or `tr.exe` on Windows).
- **Daemon URL**: unchanged (`http://localhost:7331`). The binary rename does not affect the daemon transport.
- **Installed hooks on disk**: **automatically refresh on next `tr repo`** because the installer sentinel (`# total-recall managed`) is unchanged — existing managed hooks are detected and overwritten with the newly-`tr`-branded content. No user cleanup required.
- **Dependencies**: none added or removed; `go.mod` is unchanged.