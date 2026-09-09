# Contributing

## *Quickstart 30 seconds*
==--
```sh
go install ./cmd/tr
tr init          # setup anywhere
tr repo          # inside a git repo
tr serve         # persist in 2nd terminal
```
==--
## Build

The Makefile wraps everything:

```sh
make build       # → bin/tr.exe (Windows) or bin/tr (Linux/macOS)
make install     # → $GOPATH/bin
make tidy        # go mod tidy — sync dependencies
make lint        # golangci-lint (must be installed separately)
make clean       # remove the bin/ directory
```

Or directly with Go:

```sh
go build -o bin/tr.exe ./cmd/tr
go install ./cmd/tr
```

## Run

```sh
./bin/tr --help
./bin/tr serve   # start the daemon on localhost:7331
```

Available subcommands:

```sh
--help           # Show the help/man page
init             # Set Provider, API key, LLM -- User scope
config           # Read/write config values
repo             # Install git hooks -- Repo scope
serve            # Start the daemon on localhost:7331
status           # Show daemon status + active config
asset            # Inspect/manage prompt-asset overrides: list | reset | sync
```

**Environment**: the daemon binds `localhost:7331` and user config lives at `~/.tr/config.yaml`, deep-merged with the repo-level `.tr.yaml`. `tr init` walks you through setup and creates the config file if missing. BYOK — you supply your own API keys via local environment variables; Total Recall never stores them in repo config.

## Test

### Automated (Go-native)

No external runners. Tests live in `cmd/tr/*_test.go` and use three strategies: model isolation (pure `Update(msg)`/`View()` calls), headless integration (in-process daemon via `startTestDaemon`), and golden-file snapshots of TUI views.

```sh
go test ./...            # entire repo
go test ./cmd/tr/...     # all tr CLI tests (the bulk of the suite)
```

Verify the full build pipeline before pushing:

```sh
go build ./... && go vet ./... && go test ./...
```

Golden files live in `cmd/tr/testdata/*.golden`. After changing a TUI view, regenerate them:

```sh
$env:UPDATE_GOLDEN=1; go test -run TestGolden ./cmd/tr/...
go test -run TestGolden ./cmd/tr/...   # re-run without the flag to verify
```

### Manual E2E

One flow is still manual: `tr init`, because its `huh` TUI requires a real TTY. Run it with:

```powershell
.\scripts\e2e\manual-init.ps1
```

See `scripts/e2e/README.md` for what it covers and why it can't be automated yet.
