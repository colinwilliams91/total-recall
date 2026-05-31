## Context

The binary, config system, and folder stubs exist (Phase 00 + config-architecture). `total-recall serve` prints "not implemented". All hook scripts in `hooks/` exit 0 immediately. Nothing actually runs at runtime. This design connects the pieces: a real HTTP daemon that hook scripts POST to, with `tr init` responsible for getting those scripts into `.git/hooks/`.

The architecture decision from prior phases: hooks are HTTP clients, the daemon is the server. Transient mode is deferred indefinitely. Hooks that cannot reach the daemon MUST exit 0 — they never block a Git operation.

## Goals / Non-Goals

**Goals:**
- Real HTTP server bound to `localhost:7331` with logical route groups (`/hooks/*`, `/mcp/*`)
- `tr init` installs managed hook scripts into `.git/hooks/` with safe chaining
- Hook scripts capture relevant context per hook type and POST to daemon within a 2-second hard ceiling
- `tr status` pings daemon and reports health + active config summary
- P0 security scan in pre-commit blocks commits that contain a raw `api-key:` in `.tr.yaml`
- Graceful degradation: daemon down → hook advisory + exit 0

**Non-Goals:**
- AI provider calls (Phase 3)
- SQLite concept cache (Phase 3)
- Concept extraction or question synthesis (Phase 3)
- MCP protocol implementation (future phase)
- Daemon autostart / system service registration (deferred, noted in ROADMAP)
- Filesystem watcher / Git index watcher (future background runtime)

## Decisions

### 1. Daemon: `net/http` stdlib, no framework

**Decision**: Use `net/http` + `encoding/json` from the standard library. No external HTTP framework.

**Rationale**: The daemon serves a small, fixed set of routes to localhost clients only. Framework overhead (Gin, Echo, Chi) adds transitive dependencies for no meaningful gain. `net/http` is sufficient and keeps `go.mod` lean. If route complexity grows significantly in later phases, migrating to Chi (stdlib-compatible) is trivial.

**Alternative considered**: `github.com/go-chi/chi` — compatible with `net/http`, good middleware model, but adding it now would be premature.

---

### 2. Hook → Daemon transport: shell `curl` / PowerShell `Invoke-WebRequest`

**Decision**: Hook scripts use system-available HTTP clients (`curl` on sh, `Invoke-WebRequest` on bat/PowerShell) to POST to `:7331`. No compiled binary involved in the hook scripts themselves.

**Rationale**: Hook scripts must be self-contained and runnable in any shell environment without requiring `total-recall` binary on PATH at hook execution time. `curl` is available on all modern macOS, Linux, and Windows (Windows 10+ ships curl.exe). PowerShell is available on all supported Windows versions.

**Alternative considered**: Hook script execs `total-recall hook pre-commit` — cleaner but creates a hard binary dependency. Deferred for a future "hook agent" enhancement.

**Windows note**: Git on Windows runs hook scripts via Git Bash (sh), so `.sh` hooks work. The `.bat` versions are provided for environments running hooks outside of Git Bash (e.g., direct PowerShell invocation). `tr init` installs the `.sh` variant as the primary on all platforms.

---

### 3. Hook chaining: sentinel-guard + append

**Decision**: When `tr init` detects an existing `.git/hooks/<hook>` that is NOT a Total Recall managed script, it wraps both into a new script:

```sh
#!/usr/bin/env bash
# --- BEGIN total-recall managed ---
# <existing hook content>
# --- END existing hook ---

# total-recall hook dispatch
<total-recall hook content>
exit 0
```

The existing hook runs first; total-recall hook runs after. If the existing hook exits non-zero, the chain exits non-zero before total-recall runs (intentional — existing hook failures still block the commit).

**Managed hook detection**: Managed hooks are identified by the sentinel `# total-recall managed` in the first 5 lines of the file. `tr init` re-run on a managed hook regenerates it without re-wrapping.

**Alternative considered**: Ask user to confirm and replace entirely — poor UX for teams with existing hook tooling (husky, lefthook, etc.).

---

### 4. Daemon-unreachable = exit 0, advisory to stderr

**Decision**: If the hook's HTTP POST times out or receives a connection-refused error, the hook MUST print an advisory to stderr and exit 0. The Git operation proceeds.

**Rationale**: Total Recall is an opt-in developer tool. It MUST NOT become a development workflow blocker under any failure condition. The daemon's ability to be down without impact is a stated architectural requirement.

**Advisory copy**: `[total-recall] Daemon not running at :7331. Start with 'total-recall serve'. Skipping recall check.`

---

### 5. HTTP POST payload schema

All hook POSTs share a common envelope:

```json
{
  "hook": "pre-commit",
  "repo": "/absolute/path/to/repo",
  "branch": "feature/retry-logic",
  "timestamp": "2026-05-19T18:00:00Z",
  "payload": { }
}
```

Hook-specific payload:
- `pre-commit`: `{ "diff": "<git diff --cached output>", "staged_files": ["..."], "loc_delta": 42 }`
- `commit-msg`: `{ "message": "<full commit message text>" }`
- `pre-push`: `{ "refs": [{ "local_ref": "...", "local_sha": "...", "remote_ref": "...", "remote_sha": "..." }] }`

---

### 6. P0 credential scan implementation

The pre-commit hook scans `.tr.yaml` before capturing the diff. Implementation uses grep (sh) / Select-String (PowerShell) to detect lines matching:

```
api-key:\s+[^$'"]
```

This matches a raw value (not `env:VAR` format, not a YAML reference). If matched, the hook emits a `:rotating_light: Security` message and exits 1, blocking the commit.

## Risks / Trade-offs

**[Risk] `curl` not available on some systems** → Mitigation: detect `curl` at hook execution time; fall back to advisory + exit 0 if absent. Document `curl` as a runtime dependency.

**[Risk] Hook chaining order** — existing hooks run first. If an existing hook is slow or interactive, it delays total-recall dispatch. → Mitigation: document this; hooks that take >2s are the user's existing problem, not ours.

**[Risk] Port 7331 collision** — another process may occupy the port. → Mitigation: daemon startup checks port availability and exits with a clear message if already bound. Future: make port configurable in `~/.tr/config.yaml`.

**[Risk] Windows Git Bash path handling** — absolute paths may differ between Git Bash and native Windows paths. → Mitigation: use `git rev-parse --show-toplevel` within hook scripts to get the canonical repo path in the current shell environment.

**[Risk] Long-lived daemon process management** — users may forget to start `tr serve`. → Mitigation: `tr status` is fast and informative. Daemon autostart is flagged in ROADMAP for a near-future phase.

## Decisions (Continued)

### 7. `tr init` prompts for each hook via Huh with brief workflow impact description

**Decision**: `tr init` SHALL present a Huh multi-select or three sequential confirm prompts — one per hook — each with a one-line description of its workflow impact. The selections are written into `.tr.yaml`'s `hooks:` section (creating `.tr.yaml` if it doesn't exist).

Hook descriptions shown in the prompt:
- `pre-commit` — *"Recall check at every commit. Highest signal, most frequent."*
- `commit-msg` — *"Enriches recall with your commit intent. Runs silently after pre-commit."*
- `pre-push` — *"Architecture-level recall across all commits in the push. Less frequent."*

Only hooks set to `true` in `.tr.yaml` are installed into `.git/hooks/`.

**Rationale**: Developers should make an informed choice about which hooks affect their workflow. Pre-commit fires most often and is the highest-friction point — opt-in per hook prevents surprise interruptions.

---

### 8. Hook response body is a typed struct for Phase 3 forward-compatibility

**Decision**: The daemon's hook handler response SHALL use a typed Go struct `HookResponse` (not a raw map) so Phase 3 can extend it without breaking the wire format:

```go
type HookResponse struct {
    Status  string       `json:"status"`           // "received" | "recall_ready"
    Recall  *RecallPrompt `json:"recall,omitempty"` // nil in Phase 2; populated in Phase 3
}

type RecallPrompt struct {
    Question string   `json:"question"`
    Choices  []string `json:"choices,omitempty"`
}
```

Phase 2: `HookResponse{Status: "received", Recall: nil}` → `{"status":"received"}`
Phase 3: `HookResponse{Status: "recall_ready", Recall: &RecallPrompt{...}}` → full payload

**Rationale**: Defining the struct now ensures Phase 3 extends the response without a breaking wire-format change. Hook scripts and any future clients can check `status` to decide whether to render a recall prompt.
