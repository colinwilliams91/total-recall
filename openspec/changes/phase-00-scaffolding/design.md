# Phase 00: Design

## Decision 1 — Module Path

```
github.com/colinwilliams91/total-recall
```

**Rationale:** Matches the current GitHub remote. The README install path (`github.com/total-recall/tr`) was aspirational and is updated to reflect reality. No org migration is planned for now.

---

## Decision 2 — Binary Name: `total-recall`

The binary is named `total-recall`, installed to `$PATH` as `total-recall`.

**Rationale:** `tr` is a standard POSIX utility (translate/delete characters) present on every Unix system. Using it as a binary name creates a PATH conflict risk for any global install. `total-recall` is unambiguous, readable, and consistent with the project name.

---

## Decision 3 — All Application Code Lives in `internal/`

No exported packages. The `internal/` directory owns all subsystem logic.

**Rationale:** Total Recall is a CLI tool, not a library. Exporting packages would signal a public API contract that does not exist. `internal/` prevents accidental import by external consumers and keeps the package graph intentional.

---

## Decision 4 — CLI Framework: Cobra

```
github.com/spf13/cobra
```

**Rationale:** Standard choice for Go CLIs with multiple subcommands. Supports persistent flags (needed for `--quiet`, `--json`, etc. across subcommands), auto-generated `--help`, and shell completion out of the box. Avoids reinventing subcommand dispatch.

Initial subcommands stubbed in Phase 00:

| Subcommand | Purpose |
|---|---|
| `total-recall serve` | Start the daemon |
| `total-recall init` | Initialize project config + install hooks |
| `total-recall config` | Read/write config values |
| `total-recall status` | Show daemon status and active config |

---

## Decision 5 — HTTP Server: Standard Library

The daemon's HTTP server uses `net/http` from the standard library. No external HTTP framework.

**Rationale:** The daemon serves two logical APIs (Internal Hook API, MCP API) on `localhost:7331`. The routing needs are simple and well within what `net/http` handles cleanly. Adding a framework (gin, chi, echo) introduces weight and abstractions that aren't needed at this scale.

The two logical APIs are separated by URL prefix:

```
localhost:7331/hooks/...   ← Internal Hook API (hooks, CLI, daemon control)
localhost:7331/mcp/...     ← MCP API (MCP clients, AI IDEs)
```

---

## Decision 6 — Cache Storage: SQLite via `modernc.org/sqlite`

```
modernc.org/sqlite
```

**Rationale:** Pure Go SQLite driver — no CGO, no system dependencies. Cross-platform by construction. Concept fingerprints, confidence scores, repo/branch context, and retention metadata are structured enough to benefit from SQL queries. Inspectable with any SQLite tool. The alternative (BoltDB, BadgerDB) is faster at pure KV but harder to query and harder to inspect during development.

---

## Decision 7 — MCP Library: `github.com/mark3labs/mcp-go`

```
github.com/mark3labs/mcp-go
```

**Rationale:** Community standard for Go MCP server/client implementations. Actively maintained. Handles protocol-level MCP concerns (JSON-RPC 2.0, tool registration, session lifecycle) so `internal/mcp/` can focus on Total Recall's tool implementations.

---

## Decision 8 — Cross-Platform Strategy: No CGO, Dual Hook Templates

The project builds with `CGO_ENABLED=0` across all targets. Windows is the primary development machine; Mac/Linux are first-class targets.

Hook templates are provided in two formats:

- `hooks/*.sh` — POSIX shell (Mac, Linux, Git for Windows with bash)
- `hooks/*.bat` — Windows batch (native Windows without bash)

`total-recall init` detects the OS at install time and copies the appropriate template to `.git/hooks/`.

**Rationale:** Git hooks on Windows can run either bash scripts (if Git for Windows is in PATH) or `.bat` files. Supporting both covers all common Windows setups without requiring the user to have a bash environment. The templates themselves are thin HTTP callers to `localhost:7331` — logic lives in the daemon, not the hook scripts.

---

## Package Structure

```
total-recall/
│
├── cmd/
│   └── total-recall/
│       └── main.go              ← cobra root + subcommand stubs
│
├── internal/
│   ├── config/                  ← schema, loader, deep-merge
│   ├── engine/                  ← Core Engine orchestration + daemon lifecycle
│   ├── eventmonitor/            ← FS watcher, Git index watcher, hook events, MCP events
│   ├── pipeline/                ← Incremental Analysis Pipeline
│   ├── cache/                   ← Background Concept Cache (SQLite)
│   ├── recall/                  ← Recall Engine: question synthesis and dispatch
│   ├── ai/                      ← AI provider abstraction layer
│   │   ├── anthropic/
│   │   └── openai/
│   ├── mcp/                     ← MCP API handler (mcp-go)
│   └── presentation/            ← Presentation layer dispatch
│       ├── terminal/
│       └── mcp/
│
├── hooks/
│   ├── pre-commit.sh
│   ├── commit-msg.sh
│   ├── pre-push.sh
│   ├── pre-commit.bat
│   ├── commit-msg.bat
│   └── pre-push.bat
│
├── Makefile
├── go.mod
└── go.sum
```

---

## Open Questions

None. All Phase 00 decisions are resolved.
