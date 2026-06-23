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
When `GET /recall/next` returns a question, `tr ask` SHALL clear the animation line and render the question with numbered choices. It SHALL wait for a keypress: any displayed numeric choice selects that answer (0-based index computed from the key), `Enter` skips, `q`/`Esc` exits silently.

#### Scenario: User selects a choice
- **WHEN** the user presses `'2'` with a question displayed
- **THEN** `tr ask` transitions to `stateFeedback`, fires `postAnswer(id, answerIndex=1)` as a `tea.Cmd`, and the HTTP body is `{"id":N,"answer_index":1}` with URL `POST /recall/answer?feedback=true` (answer text is NOT sent — the server looks it up from DB)

#### Scenario: Question contains four choices
- **WHEN** `GET /recall/next` returns four choices
- **THEN** `tr ask` renders all four choices
- **AND** pressing `'4'` fires `postAnswer(id, answerIndex=3)`

---

### Requirement: stateFeedback blocks on evaluation and renders "Evaluating..."
After a choice keypress, `tr ask` SHALL transition to `stateFeedback` and render `"Evaluating..."` as a static string (no animation frame). It SHALL block until a `feedbackMsg` is received from the `postAnswer` HTTP response, then transition to `stateDone` and call `tea.Quit`.

#### Scenario: stateFeedback static render
- **WHEN** `tr ask` is in `stateFeedback`
- **THEN** `View()` returns `"Evaluating..."` and does NOT advance an animation frame

#### Scenario: Ctrl+C in stateFeedback — clean exit
- **WHEN** the developer presses Ctrl+C while in `stateFeedback`
- **THEN** `tr ask` transitions to `stateDone` and exits; the in-flight HTTP response is discarded

---

### Requirement: Post-alt-screen rendering shows verdict and feedback
After the alt-screen closes, `askCmd.RunE` SHALL inspect `am.skipped`, `am.feedbackResult`, and `am.advisory` (mutually exclusive — exactly one is set per run) and print:
- Skip (`am.skipped`): `→ Question saved for later.`
- Correct (`am.feedbackResult.correct`): `✓ Correct.` followed by an indented feedback paragraph (omitted if feedback is empty)
- Incorrect (`am.feedbackResult` populated, `correct` false): `✗ The answer was: <correct_text>` followed by an indented feedback paragraph (omitted if feedback is empty)
- Advisory (`am.advisory != ""`): the advisory message verbatim (caught-up / daemon-unreachable)
- q/Esc exit: nothing

#### Scenario: Correct answer — verdict + feedback
- **WHEN** `feedbackMsg{correct: true, correctText: "...", feedback: "..."}` is received
- **THEN** after alt-screen closes, prints `✓ Correct.` and the indented feedback paragraph

#### Scenario: Incorrect answer — correct answer named
- **WHEN** `feedbackMsg{correct: false, correctText: "Prevent storms...", feedback: "Jitter randomizes..."}` is received
- **THEN** after alt-screen closes, prints `✗ The answer was: Prevent storms...` and the indented feedback paragraph

#### Scenario: Feedback AI failure — verdict only
- **WHEN** `postAnswer` receives a response with `feedback: null`
- **THEN** `feedbackMsg.feedback` is empty and only the verdict line is printed (no feedback paragraph)

#### Scenario: Daemon unreachable during postAnswer — silent exit
- **WHEN** `postAnswer` receives a connection error
- **THEN** `feedbackMsg` is the zero value (`correctText` is empty), `tr ask` prints nothing, and exits 0

---

### Requirement: Skip sends postSkip and renders gentle acknowledgement
When the user presses `Enter`, `tr ask` SHALL fire `postSkip(id)` as a `tea.Cmd` (POST body `{"id":N,"skip":true}` to `/recall/answer`), transition directly to `stateDone` (no `stateFeedback` — skip never blocks), and after the alt-screen closes print `→ Question saved for later.`

#### Scenario: Skip submitted
- **WHEN** the user presses Enter
- **THEN** `postSkip` is called, `tr ask` transitions to `stateDone`, and after alt-screen closes prints `→ Question saved for later.`

---

### Requirement: q / Esc — silent exit, no POST
When the user presses `q` or `Esc`, `tr ask` SHALL transition to `stateDone` immediately, make no `POST /recall/answer`, and print nothing after the alt-screen closes. The question remains unclaimed in the queue.

#### Scenario: q/Esc exit
- **WHEN** the user presses `q` or `Esc`
- **THEN** `tr ask` exits silently, no POST is made, and the question is available for future `tr ask` or MCP poll

---

### Requirement: tr ask shows terminal feedback on timeout and daemon unreachable
If `--timeout N` seconds elapse without a question received, `tr ask` SHALL display `You're all caught up on your recall questions. Great job 🤖💗` during the final 4 seconds of the wait, then exit 0 with the message stored in `am.advisory` (printed after alt-screen closes). If the daemon is not reachable (connection refused), `tr ask` SHALL store `[total-recall] Daemon not running. Start with total-recall serve.` in `am.advisory` and exit 0.

#### Scenario: Timeout elapsed (no question)
- **WHEN** 15 seconds pass with an empty queue
- **THEN** `tr ask` shows the caught-up message for the final 4 seconds, exits 0, and prints the caught-up message via `am.advisory` after alt-screen closes

#### Scenario: Daemon not running
- **WHEN** `GET /recall/next` returns a connection error
- **THEN** `tr ask` sets `am.advisory` to the daemon-unreachable message, exits 0, and prints it after alt-screen closes
