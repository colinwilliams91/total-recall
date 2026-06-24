## Requirements

### Requirement: tr ask exits silently in non-interactive contexts
On startup, `tr ask` SHALL check `term.IsTerminal(int(os.Stdout.Fd()))`. If false (CI/CD, piped output, non-TTY), it SHALL exit 0 immediately with no output.

#### Scenario: Non-TTY context (CI)
- **WHEN** `tr ask` is invoked from a CI pipeline without a TTY
- **THEN** the command exits 0 with no output, no network call, no error

---

### Requirement: tr ask resolves repo root and sends it to the daemon
Before polling, `tr ask` SHALL attempt to resolve the current repository root via `hooks.FindRepoRoot()` (which runs `git rev-parse --show-toplevel`). The resolved path SHALL be sent as the `repo` query parameter on `GET /recall/next` and `POST /recall/answer` requests. When `FindRepoRoot()` fails (not inside a git repo), `tr ask` SHALL fall back to `repo=""` (global dequeue) and log `[ask] not inside a git repo — falling back to global recall queue` to stderr.

#### Scenario: tr ask inside a git repo
- **WHEN** `tr ask` is invoked inside `/home/user/myproject` (a git repo)
- **THEN** it sends `GET /recall/next?repo=/home/user/myproject` and only questions tagged with that repo can be dequeued

#### Scenario: tr ask outside a git repo — global fallback
- **WHEN** `tr ask` is invoked outside any git repository
- **THEN** it sends `GET /recall/next` (no `repo` param) and logs the fallback advisory to stderr; the daemon dequeues from the global pool

---

### Requirement: tr ask displays a "Thinking." animation while polling
In an interactive TTY, `tr ask` SHALL display a cycling animation: `Thinking.` → `Thinking..` → `Thinking...` → `Thinking.` (reset), advancing one frame every 400ms. The animation SHALL render on a single line.

If no question has arrived and 4 seconds or less remain in the timeout window, `tr ask` SHALL replace the cycling animation with `You're all caught up on your recall questions. Great job 🤖💗` while continuing to poll.

---

### Requirement: tr ask renders the question and captures a keypress
When `GET /recall/next` returns a question, `tr ask` SHALL clear the animation line and render the question with numbered choices. It SHALL wait for a keypress: any displayed numeric choice selects that answer, `Enter` skips, `q`/`Esc` exits silently. The answer POST SHALL include the `repo` query parameter matching the one used for dequeue.

#### Scenario: User selects a choice
- **WHEN** the user presses `'2'` with a question displayed
- **THEN** `tr ask` fires `postAnswer(id, answerIndex=1)` with URL `POST /recall/answer?repo=<...>&feedback=true` and body `{"id":N,"answer_index":1}`

---

### Requirement: stateFeedback blocks on evaluation and renders "Evaluating..."
After a choice keypress, `tr ask` SHALL transition to `stateFeedback` and render `"Evaluating..."` as a static string. It SHALL block until a `feedbackMsg` is received, then transition to `stateDone` and call `tea.Quit`.

#### Scenario: Ctrl+C in stateFeedback — clean exit
- **WHEN** the developer presses Ctrl+C while in `stateFeedback`
- **THEN** `tr ask` transitions to `stateDone` and exits; the in-flight HTTP response is discarded

---

### Requirement: Post-alt-screen rendering shows verdict and feedback
After the alt-screen closes, `askCmd.RunE` SHALL inspect `am.skipped`, `am.feedbackResult`, and `am.advisory` and print the verdict/feedback/advisory as before. The repo-scoping change does not alter this output.

---

### Requirement: Skip sends postSkip and renders gentle acknowledgement
When the user presses `Enter`, `tr ask` SHALL fire `postSkip(id)` (POST body `{"id":N,"skip":true}` to `/recall/answer?repo=<...>`), transition to `stateDone`, and after the alt-screen closes print `→ Question saved for later.`

---

### Requirement: q / Esc — silent exit, no POST
When the user presses `q` or `Esc`, `tr ask` SHALL transition to `stateDone` immediately, make no `POST /recall/answer`, and print nothing. The question remains unclaimed in the queue for its repo.

---

### Requirement: tr ask shows terminal feedback on timeout and daemon unreachable
If `--timeout N` seconds elapse without a question received, `tr ask` SHALL display the caught-up message during the final 4 seconds, then exit 0 with the message stored in `am.advisory`. If the daemon is not reachable, `tr ask` SHALL store the daemon-unreachable message in `am.advisory` and exit 0.

#### Scenario: Timeout elapsed (no question for the repo)
- **WHEN** 15 seconds pass with an empty queue for the resolved repo
- **THEN** `tr ask` shows the caught-up message for the final 4 seconds, exits 0, and prints it via `am.advisory`

#### Scenario: Daemon not running
- **WHEN** `GET /recall/next` returns a connection error
- **THEN** `tr ask` sets `am.advisory` to the daemon-unreachable message, exits 0, and prints it after alt-screen closes
