## Requirements

### Requirement: tr ask exits silently in non-interactive contexts
On startup, `tr ask` SHALL check `term.IsTerminal(int(os.Stdout.Fd()))`. If false (CI/CD, piped output, non-TTY), it SHALL exit 0 immediately with no output.

#### Scenario: Non-TTY context (CI)
- **WHEN** `tr ask` is invoked from a CI pipeline without a TTY
- **THEN** the command exits 0 with no output, no network call, no error

---

### Requirement: tr ask displays a "Thinking." animation while polling
In an interactive TTY, `tr ask` SHALL display a cycling animation: `Thinking.` → `Thinking..` → `Thinking...` → `Thinking.` (reset), advancing one frame every 400ms. The animation SHALL render on a single line (overwrite previous frame).

#### Scenario: Animation cycling
- **WHEN** `tr ask` is running and no question has been received
- **THEN** the terminal shows the cycling animation, one frame per 400ms

---

### Requirement: tr ask renders the question and captures a keypress
When `GET /recall/next` returns a question, `tr ask` SHALL clear the animation line and render the question with numbered choices. It SHALL wait for a keypress: `'1'`-`'3'` selects a choice, `Enter` skips.

#### Scenario: User selects a choice
- **WHEN** the user presses `'2'` with a question displayed
- **THEN** `tr ask` posts `{"id":N,"answer":<choices[1]>}` to `/recall/answer`, prints `"✓ recorded"`, and exits 0

#### Scenario: User presses Enter to skip
- **WHEN** the user presses Enter
- **THEN** `tr ask` posts `{"id":N,"answer":"skip"}` to `/recall/answer`, prints `"→ skipped"`, and exits 0

---

### Requirement: tr ask exits silently on timeout or daemon unreachable
If `--timeout N` seconds elapse without a question received, `tr ask` SHALL exit 0 silently. If the daemon is not reachable (connection refused), `tr ask` SHALL exit 0 silently with no output.

#### Scenario: Timeout elapsed (no question)
- **WHEN** 15 seconds pass with an empty queue
- **THEN** `tr ask` clears the animation and exits 0 with no output

#### Scenario: Daemon not running
- **WHEN** `GET /recall/next` returns a connection error
- **THEN** `tr ask` exits 0 immediately with no output
