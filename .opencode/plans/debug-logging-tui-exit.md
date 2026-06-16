# Debug Logging for TUI Exit Flow

## Problem

After the user selects a quiz answer in the `tr ask` TUI, the answer is recorded in SQLite but the program appears to hang — the user must press an additional key before the process exits and returns them to their terminal. There is no "press any key to continue..." prompt, so the user doesn't know they need to press a key.

## Goal

Add timestamped stderr logging at 7 critical points in the exit flow to identify exactly where execution stalls. This is a diagnostic step — once we see the log output from a reproduction, we'll know whether the hang is inside the Go code (Bubble Tea, HTTP, process exit) or outside it (PowerShell, ConHost).

## Execution Flow & Log Points

All changes are in `cmd/total-recall/ask.go`.

### Helper function (new)

Add a `trDebug` helper that writes timestamped messages to stderr (so they don't interfere with Bubble Tea's stdout rendering):

```go
func trDebug(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "[TR-DEBUG %s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
```

### Log Point 1 — Keypress received (line ~189)

In `updateQuestion()`, right after confirming the message is a `tea.KeyMsg`:

```go
trDebug("keypress received key=%s", k.String())
```

### Log Point 2 — About to post answer (line ~218)

In `updateQuestion()`, just before calling `postAnswer`:

```go
trDebug("posting answer id=%d answer=%q", m.question.id, answer)
```

### Log Point 3 — postAnswer returned (line ~219)

Immediately after `postAnswer` returns:

```go
trDebug("postAnswer returned")
```

### Log Point 4 — Returning tea.Quit (line ~224)

Just before returning from `updateQuestion` with `tea.Quit`:

```go
trDebug("returning tea.Quit state=%d feedback=%q", m.state, m.feedback)
```

### Log Point 5 — p.Run() returned (line ~44)

In `askCmd()`, immediately after `p.Run()` returns:

```go
trDebug("p.Run() returned err=%v", err)
```

### Log Point 6 — About to print feedback (line ~49)

Just before `fmt.Println(am.feedback)`:

```go
trDebug("about to print feedback=%q", am.feedback)
```

### Log Point 7 — Returning nil (line ~52)

Just before `return nil` at the end of `RunE`:

```go
trDebug("returning nil — process should exit")
```

## What the Logs Will Tell Us

| Scenario | Expected log pattern |
|----------|---------------------|
| `postAnswer` blocks (HTTP timeout) | Gap of up to 3s between L2 and L3 |
| Bubble Tea exit hangs | Gap between L4 and L5 |
| Process exit blocked (PowerShell) | All 7 logs fire quickly, but process doesn't return to shell |
| Terminal state issue | All logs fire, feedback printed, but terminal doesn't update until keypress |

## Files Changed

- `cmd/total-recall/ask.go` — add `trDebug` helper + 7 log calls

## Verification

1. `go build ./...` — must compile
2. `go vet ./...` — must pass
3. `go test ./...` — must pass
4. Reproduce the bug: run `git commit` in a repo with the hook installed, answer the quiz question, and observe stderr output
