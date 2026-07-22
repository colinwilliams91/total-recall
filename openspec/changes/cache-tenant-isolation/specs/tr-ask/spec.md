## MODIFIED Requirements

### Requirement: tr ask resolves repo root and branch before sending any request to the daemon
Before polling, `tr ask` SHALL resolve both the current repository root via `hooks.FindRepoRoot()` (which runs `git rev-parse --show-toplevel`) AND the current branch via a new `resolveAskBranch()` helper (which runs `git rev-parse --abbrev-ref HEAD`). The resolved `repo` and `branch` SHALL be sent as query parameters on `GET /recall/next?repo=<...>&branch=<...>` requests. When `FindRepoRoot()` fails (user is not inside a git repo) OR `resolveAskBranch()` returns `""` (detached HEAD), `tr ask` SHALL print an advisory to stderr and exit 0 without sending any HTTP request to the daemon — no "global pool" fallback exists.

#### Scenario: tr ask inside a git repo on a named branch
- **WHEN** `tr ask` is invoked inside `/home/user/myproject` (a git repo) on branch `feature-X`
- **THEN** it sends `GET /recall/next?repo=/home/user/myproject&branch=feature-X`; only questions tagged with that repo AND branch can be dequeued

#### Scenario: tr ask outside a git repo
- **WHEN** `tr ask` is invoked outside any git repository
- **THEN** it prints `[ask] not inside a git repo — skipping this recall window` to stderr and exits 0 without sending any HTTP request (no "global pool" dequeue, no polling)

#### Scenario: tr ask in detached HEAD
- **WHEN** `tr ask` is invoked inside a git repo but `git rev-parse --abbrev-ref HEAD` returns `""` (detached HEAD during rebase, submodule, or CI clone)
- **THEN** it prints `[ask] detached HEAD — no branch context; skipping this recall window` to stderr and exits 0 without sending any HTTP request

---

### Requirement: tr ask answer and skip POSTs include the resolved repo (branch accepted for symmetry, ID-keyed)
The `POST /recall/answer` endpoint is ID-keyed — the question ID is globally unique across repos and branches, so the `repo` and `branch` query parameters are accepted but do not affect the operation. `tr ask` SHALL include the resolved `repo` on `postAnswer` and `postSkip` POSTs for consistency with the `GET /recall/next` request (the daemon accepts the param symmetrically). The `branch` query parameter MAY be included for symmetry but is not required by the answer endpoint.

#### Scenario: User answers a question
- **WHEN** the user selects a choice on a question that was dequeued for `repo=/path/X` AND `branch=feature-X`
- **THEN** `tr ask` POSTs `{"id":N,"answer_index":K}` to `/recall/answer?repo=/path/X&branch=feature-X&feedback=true` (`branch` included for symmetry only)

---

### Requirement: tr ask displays a "Thinking." animation while polling
In an interactive TTY, `tr ask` SHALL display a cycling animation: `Thinking.` → `Thinking..` → `Thinking...` → `Thinking.` (reset), advancing one frame every 400ms. The animation SHALL render on a single line.

If no question has arrived and 4 seconds or less remain in the timeout window, `tr ask` SHALL replace the cycling animation with `You're all caught up on your recall questions. Great job 🤖🤖` while continuing to poll.

---

### Requirement: tr ask renders the question and captures a keypress
When `GET /recall/next` returns a question, `tr ask` SHALL clear the animation line and render the question with numbered choices. It SHALL wait for a keypress: any displayed numeric choice selects that answer, `Enter` skips, `q`/`Esc` exits silently. The answer POST SHALL include the `repo` query parameter matching the one used for dequeue (`branch` is included for symmetry, but the answer endpoint is ID-keyed).

#### Scenario: User selects a choice
- **WHEN** the user presses `'2'` with a question displayed
- **THEN** `tr ask` fires `postAnswer(id, answerIndex=1)` with URL `POST /recall/answer?repo=<...>&branch=<...>&feedback=true` and body `{"id":N,"answer_index":1}`

---

### Requirement: stateFeedback blocks on evaluation and renders "Evaluating..."
After a choice keypress, `tr ask` SHALL transition to `stateFeedback` and render `"Evaluating..."` as a static string. It SHALL block until a `feedbackMsg` is received, then transition to `stateDone` and call `tea.Quit`.

#### Scenario: Ctrl+C in stateFeedback - clean exit
- **WHEN** the developer presses Ctrl+C while in `stateFeedback`
- **THEN** `tr ask` transitions to `stateDone` and exits; the in-flight HTTP response is discarded

---

### Requirement: Post-alt-screen rendering shows verdict and feedback
After the alt-screen closes, `askCmd.RunE` SHALL inspect `am.skipped`, `am.feedbackResult`, and `am.advisory` and print the verdict/feedback/advisory as before. The repo-and-branch-scoping change does not alter this output.

---

### Requirement: Skip sends postSkip and renders gentle acknowledgement
When the user presses `Enter`, `tr ask` SHALL fire `postSkip(id)` (POST body `{"id":N,"skip":true}` to `/recall/answer?repo=<...>`), transition to `stateDone`, and after the alt-screen closes print `→ Question saved for later.`

---

### Requirement: q / Esc - silent exit, no POST
When the user presses `q` or `Esc`, `tr ask` SHALL transition to `stateDone` immediately, make no `POST /recall/answer`, and print nothing. The question remains unclaimed in the queue for its repo and branch.

---

### Requirement: tr ask shows terminal feedback on timeout and daemon unreachable
If `--timeout N` seconds elapse without a question received, `tr ask` SHALL display the caught-up message during the final 4 seconds, then exit 0 with the message stored in `am.advisory`. If the daemon is not reachable, `tr ask` SHALL store the daemon-unreachable message in `am.advisory` and exit 0.

#### Scenario: Timeout elapsed (no question for the repo and branch)
- **WHEN** 15 seconds pass with an empty queue for the resolved `repo` AND `branch`
- **THEN** `tr ask` shows the caught-up message for the final 4 seconds, exits 0, and prints it via `am.advisory`

#### Scenario: Daemon not running
- **WHEN** `GET /recall/next` returns a connection error
- **THEN** `tr ask` sets `am.advisory` to the daemon-unreachable message, exits 0, and prints it after alt-screen closes