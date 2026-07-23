
### Build

Using Make:

```sh
make build
```

Produces `tr.exe` (Windows) or `bin/tr` (Linux/macOS).

Or directly with Go:

```sh
go build -o bin/tr.exe ./cmd/tr
```

Install to your $GOPATH/bin:

```sh
make install
# or
go install ./cmd/tr
```

#### ENV

- The daemon will run on `localhost:7331` by default, and the config file is located at `~/.tr/config.yaml`.
- You can override the config file location with `--config <path>`.
- The init should walk you through the config setup and create the file if it does not exist.
- BYOK for the lightweight AI inference pipeline.
**USE YOUR LOCAL MACHINE ENV VARIABLES FOR API KEYS AND EXTRA SECURITY**. Total Recall will not store your API keys in the repo config file.

---

### Run

After building:

```sh
./bin/tr --help
```

Available subcommands:

| Command   |	Description |
| --------- | ------------- |
| --help    | Show the help/man page                |
| serve     | Start the daemon on localhost:7331    |
| init      | Configure user-level settings (AI provider, API key, model) |
| repo      | Install hooks in a git repository     |
| config    | Read/write config values              |
| status    | Show daemon status and active config  |

Example:

```sh
./bin/tr serve
```

---

### Test

#### E2E

> You can run the E2E tests in `./scripts/e2e/` using PowerShell. These tests require the daemon to be running and will create a temporary Git repo for testing.
> They will output an agent-first JSON log of the test run to `./scripts/e2e/output/` -- add that directory to the `.gitignore` if it is not already there.
```sh
./scripts/e2e/run-all.ps1
```

#### Unit

```sh
make test
# or
go test ./...
```

> Note: No test files exist yet — `go test open-source.` will complete with no tests run. The internal packages under `internal` only contain `doc.go` stubs at this stage.

---

### Other Useful Commands

```sh
make tidy    # go mod tidy — sync dependencies
make lint    # run golangci-lint (must be installed separately)
make clean   # remove the bin/ directory
```
Build and use the `--help` flag to explore more commands and options.
