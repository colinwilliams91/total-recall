## ADDED Requirements

### Requirement: Daemon hook response uses a typed HookResponse struct
The daemon's hook handlers SHALL return a typed `HookResponse` JSON body. In Phase 2 the `recall` field is always omitted (nil). In Phase 3 it will be populated with the synthesized question. This forward-compatible shape ensures hook scripts and future clients can check `status` without a breaking wire-format change.

Phase 2 response shape:
```json
{ "status": "received" }
```

Phase 3 response shape (struct defined now, populated later):
```json
{ "status": "recall_ready", "recall": { "question": "...", "choices": ["..."] } }
```

#### Scenario: Hook response in Phase 2
- **WHEN** the daemon receives a hook POST in this phase
- **THEN** the response body is `{"status":"received"}` with the `recall` field absent (omitempty)

#### Scenario: Hook script handles recall_ready status (forward-compatible)
- **WHEN** a hook script receives a response with `status: "recall_ready"` (Phase 3 behavior)
- **THEN** the script is able to parse and act on the `recall` field without modification

---

### Requirement: pre-commit hook captures staged diff and POSTs to daemon
The `pre-commit` hook script SHALL capture `git diff --cached` output and relevant metadata (staged file list, LOC delta, branch name, repo path), then POST a JSON payload to `localhost:7331/hooks/pre-commit` within a 2-second hard timeout. The commit operation SHALL NOT be blocked by this step regardless of the daemon's response.

#### Scenario: Successful dispatch on pre-commit
- **WHEN** the user runs `git commit` with staged changes and the daemon is running
- **THEN** the hook POSTs the staged diff payload to `/hooks/pre-commit`, receives 202 Accepted, and exits 0

#### Scenario: Daemon unreachable on pre-commit
- **WHEN** the user runs `git commit` and the daemon is not running at :7331
- **THEN** the hook prints `[total-recall] Daemon not running at :7331. Start with 'total-recall serve'. Skipping recall check.` to stderr and exits 0 without blocking the commit

#### Scenario: Dispatch timeout
- **WHEN** the daemon is running but does not respond within 2 seconds
- **THEN** the hook aborts the HTTP call and exits 0 without blocking the commit

---

### Requirement: commit-msg hook reads commit message and POSTs to daemon
The `commit-msg` hook script SHALL read the commit message from the file path provided as `$1`, then POST a JSON payload to `localhost:7331/hooks/commit-msg` within a 2-second hard timeout. The commit operation SHALL NOT be blocked by this step.

#### Scenario: Successful dispatch on commit-msg
- **WHEN** the user finalizes a commit message and the daemon is running
- **THEN** the hook POSTs the commit message payload to `/hooks/commit-msg` and exits 0

#### Scenario: Daemon unreachable on commit-msg
- **WHEN** the daemon is not running during a `commit-msg` hook invocation
- **THEN** the hook prints the standard advisory and exits 0

---

### Requirement: pre-push hook reads ref list and POSTs to daemon
The `pre-push` hook script SHALL read the ref list from stdin (as provided by Git), then POST a JSON payload to `localhost:7331/hooks/pre-push` within a 2-second hard timeout. The push operation SHALL NOT be blocked by this step.

#### Scenario: Successful dispatch on pre-push
- **WHEN** the user runs `git push` and the daemon is running
- **THEN** the hook POSTs the ref list payload to `/hooks/pre-push` and exits 0

#### Scenario: Daemon unreachable on pre-push
- **WHEN** the daemon is not running during a `pre-push` hook invocation
- **THEN** the hook prints the standard advisory and exits 0

---

### Requirement: Hook POST payload conforms to the shared envelope schema
All hook POST requests SHALL use `Content-Type: application/json` and a shared envelope structure:

```json
{
  "hook": "<hook-type>",
  "repo": "<absolute repo path>",
  "branch": "<current branch name>",
  "timestamp": "<RFC3339 UTC>",
  "payload": { }
}
```

Hook-specific payload fields:
- `pre-commit`: `diff` (string), `staged_files` (string array), `loc_delta` (integer)
- `commit-msg`: `message` (string — full commit message content)
- `pre-push`: `refs` (array of objects with `local_ref`, `local_sha`, `remote_ref`, `remote_sha`)

#### Scenario: Valid JSON envelope on pre-commit POST
- **WHEN** the pre-commit hook fires and dispatches to the daemon
- **THEN** the request body is valid JSON matching the envelope schema with `"hook": "pre-commit"` and a populated `payload.diff` field

---

### Requirement: pre-commit hook scans .tr.yaml for raw API credentials (P0 security)
Before capturing the diff, the `pre-commit` hook SHALL scan `.tr.yaml` (if it exists in the repo root) for any `api-key:` value that is not in `env:<VARNAME>` format. If a raw key is detected, the hook SHALL emit a security alert and exit 1, blocking the commit.

#### Scenario: Raw api-key detected in .tr.yaml
- **WHEN** `.tr.yaml` contains `api-key: sk-abc123` (a literal value, not `env:VAR`)
- **THEN** the hook prints a 🚨 security alert advising the user to rotate the key, use `env:` format, and purge Git history, then exits 1 blocking the commit

#### Scenario: env-ref api-key in .tr.yaml is safe
- **WHEN** `.tr.yaml` contains `api-key: env:ANTHROPIC_API_KEY`
- **THEN** the hook considers the credential safe and proceeds with normal dispatch

#### Scenario: No .tr.yaml in repo root
- **WHEN** `.tr.yaml` does not exist in the repository root
- **THEN** the credential scan is skipped and the hook proceeds normally

---

### Requirement: tr status reports daemon health
`total-recall status` SHALL attempt `GET localhost:7331/health` and report whether the daemon is alive or dead, along with the active config summary when alive.

#### Scenario: Daemon is running
- **WHEN** the user runs `total-recall status` and the daemon is running
- **THEN** the command prints `✓ Daemon running on :7331` and a summary of the active config

#### Scenario: Daemon is not running
- **WHEN** the user runs `total-recall status` and no daemon is bound to :7331
- **THEN** the command prints `✗ Daemon not running — start with 'total-recall serve'` and exits with code 1
