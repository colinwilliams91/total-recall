## Requirements

### Requirement: tr ask exits silently in non-interactive contexts
On startup, `tr ask` SHALL check `term.IsTerminal(int(os.Stdout.Fd()))`. If false (CI/CD, piped output, non-TTY), it SHALL exit 0 immediately with no output.

#### Scenario: Non-TTY context (CI)
- **WHEN** `tr ask` is invoked from a CI pipeline without a TTY
- **THEN** the command exits 0 with no output, no network call, no error

---

### Requirement: tr ask displays a "Thinking." animation while polling
In an interactive TTY, `tr ask` SHALL display a cycling animation: `Thinking.` → `Thinking..` → `Thinking...` → `Thinking.` (reset), advancing one frame every 400ms. The animation SHALL render on a single line (overwrite previous frame).

If no question has arrived and 4 seconds or less remain in the timeout window, `tr ask` SHALL replace the cycling animation with `You're all caught up on your recall questions. Great job 🤖💗` while continuing to poll for a late-arriving question.

#### Scenario: Animation cycling
- **WHEN** `tr ask` is running and no question has been received
- **THEN** the terminal shows the cycling animation, one frame per 400ms

---

### Requirement: tr ask renders the question and captures a keypress
When `GET /recall/next` returns a question, `tr ask` SHALL clear the animation line and render the question with numbered choices. It SHALL wait for a keypress: any displayed numeric choice selects that answer, `Enter` skips.

#### Scenario: User selects a choice
- **WHEN** the user presses `'2'` with a question displayed
- **THEN** `tr ask` posts `{"id":N,"answer":<choices[1]>}` to `/recall/answer`, prints `"✓ recorded"`, and exits 0

#### Scenario: Question contains four choices
- **WHEN** `GET /recall/next` returns four choices
- **THEN** `tr ask` renders all four choices
- **THEN** pressing `'4'` posts `{"id":N,"answer":<choices[3]>}` to `/recall/answer`

#### Scenario: User presses Enter to skip
- **WHEN** the user presses Enter
- **THEN** `tr ask` posts `{"id":N,"answer":"skip"}` to `/recall/answer`, prints `"→ skipped"`, and exits 0

---

### Requirement: tr ask shows terminal feedback on timeout and daemon unreachable
If `--timeout N` seconds elapse without a question received, `tr ask` SHALL display `You're all caught up on your recall questions. Great job 🤖💗` during the final 4 seconds of the wait, then exit 0. If the daemon is not reachable (connection refused), `tr ask` SHALL display `[total-recall] Daemon not running. Start with total-recall serve.` and exit 0.

#### Scenario: Timeout elapsed (no question)
- **WHEN** 15 seconds pass with an empty queue
- **THEN** `tr ask` shows the caught-up message for the final 4 seconds, exits 0, and leaves that message visible when returning to the main terminal screen

#### Scenario: Daemon not running
- **WHEN** `GET /recall/next` returns a connection error
- **THEN** `tr ask` exits 0 and leaves `[total-recall] Daemon not running. Start with total-recall serve.` visible when returning to the main terminal screen
