package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/hooks"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var daemonBaseURL = "http://localhost:7331"

// resolveAskRepo resolves the current git repository root for repo-scoped recall.
// On error (not inside a git repo, or git unavailable) it returns the error.
// Callers MUST check the error and abort — there is no "global pool" fallback.
func resolveAskRepo() (string, error) {
	return hooks.FindRepoRoot()
}

// resolveAskBranch resolves the current git branch for repo+branch-scoped
// recall. It runs `git rev-parse --abbrev-ref HEAD`; on success, returns the
// trimmed branch name. On error (detached HEAD, git unavailable, empty
// stdout) it returns the error. Detached HEAD is reported by git as either
// "HEAD" or empty output, so we treat empty trimmed output as an error.
func resolveAskBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("detached HEAD — no branch context")
	}
	return branch, nil
}

const (
	// defaultTimeout is how long torec ask polls for a question before showing
	// the "caught up" message and exiting. Must be >= the pipeline context
	// timeout in internal/engine/server.go so the poller doesn't give up
	// before the pipeline produces a question. See the timeout relationship
	// documented on ai.DefaultHTTPTimeout.
	defaultTimeout = 60 * time.Second
	animTick       = 400 * time.Millisecond
	// caughtUpWindow is how long before the poll timeout the "all caught up"
	// message replaces the thinking animation. Shorter than the old 4s: the
	// line carries no new information — it only reassures the user the hook
	// ran — so lingering four seconds made the post-commit pause feel broken
	// rather than calm. Three seconds keeps it readable without reading as a
	// hang (issue #30).
	caughtUpWindow = 3 * time.Second
)

var animFrames = []string{"Thinking.", "Thinking..", "Thinking..."}

const caughtUpMessage = "You're all caught up on your recall questions. Great job 🤖💗"

const daemonUnavailableMessage = "[total-recall] Daemon not running. Start with torec serve."

// bonusOfferPrompt is the agreed offer copy, presented once per session after
// the commit question's feedback renders.
const bonusOfferPrompt = "Would you like a bonus question to boost your knowledge retention? y/n"

// askCmd is the Cobra command for surfacing a recall question in the terminal.
func askCmd() *cobra.Command {
	var timeout int

	cmd := &cobra.Command{
		Use:   "ask",
		Short: "Surface the next recall question in your terminal",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdout.Fd())) {
				return nil
			}
			repo, repoErr := resolveAskRepo()
			if repoErr != nil {
				fmt.Fprintln(os.Stderr, "[ask] not inside a git repo — skipping this recall window")
				return nil
			}
			branch, branchErr := resolveAskBranch()
			if branchErr != nil {
				fmt.Fprintln(os.Stderr, "[ask] detached HEAD — no branch context; skipping this recall window")
				return nil
			}
			m := newAskModel(time.Duration(timeout)*time.Second, repo, branch)
			p := tea.NewProgram(m)
			finalModel, err := p.Run()
			if err != nil {
				return err
			}
			am, ok := finalModel.(askModel)
			if !ok {
				return nil
			}
			if out := renderPostAltScreen(am); out != "" {
				fmt.Print(out)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&timeout, "timeout", 60, "Seconds to wait for a question before exiting")
	return cmd
}

// ── state machine ─────────────────────────────────────────────────────────────

type askState int

const (
	stateThinking askState = iota
	stateQuestion
	stateFeedback
	stateOffer
	stateDone
)

// ── messages ──────────────────────────────────────────────────────────────────

type tickMsg struct{}
type questionMsg struct {
	id           int64
	questionType string
	question     string
	choices      []string
}
type noQuestionMsg struct{}
type daemonUnreachableMsg struct{}
type feedbackMsg struct {
	correct     bool
	correctText string
	feedback    string
}
type skipMsg struct{}

// bonusOfferMsg is a served retention-bonus offer: the due concept's slug and
// the registry prose the offer is framed with.
type bonusOfferMsg struct {
	slug  string
	prose string
}

// noOfferMsg means no bonus offer this session: nothing due, the kill-switch
// off, or the probe failed. The session ends exactly as it does without
// resurfacing.
type noOfferMsg struct{}

// acceptFailedMsg means the accepted bonus could not be fetched (synthesis or
// daemon error). No decline is recorded — the serve-time defer stands.
type acceptFailedMsg struct{}

// declinePostedMsg confirms the daemon recorded the decline.
type declinePostedMsg struct{}

// ── model ─────────────────────────────────────────────────────────────────────

type askModel struct {
	state          askState
	frame          int
	started        time.Time
	timeout        time.Duration
	polling        bool // true while an HTTP poll is in flight
	question       questionMsg
	feedbackResult feedbackMsg
	skipped        bool
	advisory       string
	httpClient     *http.Client
	feedbackClient *http.Client
	repo           string
	branch         string

	// Bonus-offer session state. offerProbed guarantees exactly one probe per
	// session (the offer is made at most once); offer is the served offer;
	// offerShown marks that the feedback rendered inline in the offer view so
	// the post-exit output does not repeat it. questionIsBonus/bonusAnswered
	// distinguish the bonus question's own resolution.
	offerProbed     bool
	offer           *bonusOfferMsg
	offerShown      bool
	awaitingBonus   bool
	questionIsBonus bool
	bonusAnswered   bool
}

func newAskModel(timeout time.Duration, repo, branch string) askModel {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return askModel{
		state:          stateThinking,
		started:        time.Now(),
		timeout:        timeout,
		httpClient:     &http.Client{Timeout: 3 * time.Second},
		feedbackClient: &http.Client{Timeout: ai.DefaultHTTPTimeout},
		repo:           repo,
		branch:         branch,
	}
}

func (m askModel) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(animTick, func(_ time.Time) tea.Msg { return tickMsg{} })
}

func (m askModel) pollCmd() tea.Cmd {
	client := m.httpClient
	repo := m.repo
	branch := m.branch
	return func() tea.Msg {
		u := daemonBaseURL + "/recall/next?repo=" + url.QueryEscape(repo) + "&branch=" + url.QueryEscape(branch)
		resp, err := client.Get(u)
		if err != nil {
			return daemonUnreachableMsg{}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			return noQuestionMsg{}
		}
		var body struct {
			ID           int64    `json:"id"`
			QuestionType string   `json:"question_type"`
			Question     string   `json:"question"`
			Choices      []string `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return noQuestionMsg{}
		}
		return questionMsg{id: body.ID, questionType: body.QuestionType, question: body.Question, choices: body.Choices}
	}
}

func (m askModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateThinking:
		return m.updateThinking(msg)
	case stateQuestion:
		return m.updateQuestion(msg)
	case stateFeedback:
		return m.updateFeedback(msg)
	case stateOffer:
		return m.updateOffer(msg)
	}
	return m, tea.Quit
}

func (m askModel) updateThinking(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.frame = (m.frame + 1) % len(animFrames)
		if time.Since(m.started) >= m.timeout {
			m.state = stateDone
			m.advisory = caughtUpMessage
			return m, tea.Quit
		}
		var cmds []tea.Cmd
		cmds = append(cmds, tick())
		if !m.polling {
			m.polling = true
			cmds = append(cmds, m.pollCmd())
		}
		return m, tea.Batch(cmds...)

	case questionMsg:
		m.polling = false
		m.state = stateQuestion
		m.question = msg
		return m, nil

	case noQuestionMsg:
		m.polling = false
		return m, nil

	case daemonUnreachableMsg:
		m.state = stateDone
		m.advisory = daemonUnavailableMessage
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.state = stateDone
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m askModel) updateQuestion(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if idx, ok := parseChoiceSelection(k.String(), len(m.question.choices)); ok {
		if idx < len(m.question.choices) {
			if m.questionIsBonus {
				m.bonusAnswered = true
			}
			m.state = stateFeedback
			return m, postAnswer(m.question.id, idx, m.repo, m.branch, m.feedbackClient)
		}
		return m, nil
	}

	switch k.String() {
	case "enter":
		m.state = stateDone
		m.skipped = true
		if m.questionIsBonus {
			m.bonusAnswered = true
		}
		return m, postSkip(m.question.id, m.repo, m.branch, m.httpClient)
	case "q", "esc":
		m.state = stateDone
		return m, tea.Quit
	case "ctrl+c":
		m.state = stateDone
		return m, tea.Quit
	}
	return m, nil
}

func (m askModel) updateFeedback(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case feedbackMsg:
		m.feedbackResult = msg
		if msg.correctText == "" {
			m.advisory = "[total-recall] Could not reach daemon — answer not recorded."
			m.state = stateDone
			return m, tea.Quit
		}
		// The offer probe fires exactly once per session, after the answer is
		// recorded (seeding committed) and the feedback is in hand. The model
		// stays in stateFeedback until the probe resolves — the "Evaluating..."
		// view covers the local round-trip.
		if !m.offerProbed {
			m.offerProbed = true
			return m, probeOfferCmd(m.httpClient)
		}
		m.state = stateDone
		return m, tea.Quit
	case bonusOfferMsg:
		offer := msg
		m.offer = &offer
		m.offerShown = true
		m.state = stateOffer
		return m, nil
	case noOfferMsg:
		m.state = stateDone
		return m, tea.Quit
	case skipMsg:
		m.state = stateDone
		m.skipped = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.state = stateDone
			return m, tea.Quit
		}
	}
	return m, nil
}

// updateOffer is the bonus-offer interaction: y accepts (synthesis runs behind
// the thinking animation), n posts the decline, and q/Esc/Ctrl+C (or any
// terminal closed) ends the session without posting anything — the daemon's
// serve-time defer already fired.
func (m askModel) updateOffer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.awaitingBonus {
			m.frame = (m.frame + 1) % len(animFrames)
			return m, tick()
		}
		return m, nil
	case questionMsg:
		m.awaitingBonus = false
		m.questionIsBonus = true
		m.question = msg
		m.state = stateQuestion
		return m, nil
	case acceptFailedMsg:
		m.state = stateDone
		return m, tea.Quit
	case declinePostedMsg:
		m.state = stateDone
		return m, tea.Quit
	case tea.KeyMsg:
		key := msg.String()
		if m.awaitingBonus {
			if key == "ctrl+c" || key == "q" || key == "esc" {
				m.state = stateDone
				return m, tea.Quit
			}
			return m, nil
		}
		switch key {
		case "y":
			m.awaitingBonus = true
			return m, tea.Batch(tick(), acceptOfferCmd(m.offer.slug, m.repo, m.branch, m.feedbackClient))
		case "n":
			return m, declineOfferCmd(m.offer.slug, m.httpClient)
		case "q", "esc", "ctrl+c":
			m.state = stateDone
			return m, tea.Quit
		}
	}
	return m, nil
}

func postAnswer(id int64, selectedIndex int, repo, branch string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{"id": id, "selected_index": selectedIndex})
		u := daemonBaseURL + "/recall/select?feedback=true&repo=" + url.QueryEscape(repo) + "&branch=" + url.QueryEscape(branch)
		resp, err := client.Post(u, "application/json", bytes.NewReader(body))
		if err != nil {
			return feedbackMsg{}
		}
		defer resp.Body.Close()
		var out struct {
			OK          bool   `json:"ok"`
			Correct     bool   `json:"correct"`
			CorrectText string `json:"correct_answer"`
			Feedback    string `json:"feedback"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return feedbackMsg{}
		}
		return feedbackMsg{
			correct:     out.Correct,
			correctText: out.CorrectText,
			feedback:    out.Feedback,
		}
	}
}

func postSkip(id int64, repo, branch string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{"id": id, "skip": true})
		u := daemonBaseURL + "/recall/select?repo=" + url.QueryEscape(repo) + "&branch=" + url.QueryEscape(branch)
		resp, err := client.Post(u, "application/json", bytes.NewReader(body))
		if err != nil {
			return skipMsg{}
		}
		defer resp.Body.Close()
		return skipMsg{}
	}
}

// probeOfferCmd asks the daemon for this session's one bonus offer, after the
// commit question's answer was recorded. Anything but a 200 offer payload is
// noOfferMsg — the session then ends exactly as it does without resurfacing.
func probeOfferCmd(client *http.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.Get(daemonBaseURL + "/recall/bonus")
		if err != nil {
			return noOfferMsg{}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return noOfferMsg{}
		}
		var body struct {
			Slug  string `json:"slug"`
			Prose string `json:"prose"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Slug == "" {
			return noOfferMsg{}
		}
		return bonusOfferMsg{slug: body.Slug, prose: body.Prose}
	}
}

// acceptOfferCmd posts the accept. The daemon synthesizes the bonus question
// at the session's current level, stores it already delivered to this session,
// and returns it — no second poll happens. Any failure is a silent defer: no
// decline is recorded and the serve-time defer stands.
func acceptOfferCmd(slug, repo, branch string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{"slug": slug, "repo": repo, "branch": branch})
		resp, err := client.Post(daemonBaseURL+"/recall/bonus/accept", "application/json", bytes.NewReader(body))
		if err != nil {
			return acceptFailedMsg{}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return acceptFailedMsg{}
		}
		var q struct {
			ID           int64    `json:"id"`
			QuestionType string   `json:"question_type"`
			Question     string   `json:"question"`
			Choices      []string `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&q); err != nil || q.ID == 0 {
			return acceptFailedMsg{}
		}
		return questionMsg{id: q.ID, questionType: q.QuestionType, question: q.Question, choices: q.Choices}
	}
}

// declineOfferCmd posts the explicit decline; the daemon increments the
// concept's decline count (third → declined-out). The session ends when the
// POST settles — an unanswered decline never silently drops.
func declineOfferCmd(slug string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{"slug": slug})
		resp, err := client.Post(daemonBaseURL+"/recall/bonus/decline", "application/json", bytes.NewReader(body))
		if err == nil {
			defer resp.Body.Close()
		}
		return declinePostedMsg{}
	}
}

func (m askModel) View() string {
	switch m.state {
	case stateThinking:
		if m.showCaughtUpMessage() {
			return "\r" + caughtUpMessage + "   "
		}
		return "\r" + "🤖🧠" + " " + animFrames[m.frame] + "   "
	case stateQuestion:
		return renderQuestion(m.question)
	case stateFeedback:
		return "\rEvaluating...   "
	case stateOffer:
		return renderOfferView(*m.offer, m.feedbackResult, m.awaitingBonus, m.frame)
	case stateDone:
		return "\n\nPress any key to continue...\n"
	}
	return ""
}

// renderOfferView renders the commit question's feedback inline (the offer
// comes after the feedback, per the session contract) followed by the framed
// retention-bonus offer. While the accepted bonus is synthesizing, the
// thinking animation covers the wait.
func renderOfferView(offer bonusOfferMsg, fr feedbackMsg, awaiting bool, frame int) string {
	var b strings.Builder
	if fr.correct {
		b.WriteString("✓ Correct.\n")
	} else {
		fmt.Fprintf(&b, "✗ The answer was: %s\n", fr.correctText)
	}
	if fr.feedback != "" {
		fmt.Fprintf(&b, "\n  %s\n", fr.feedback)
	}
	b.WriteString("\n🧠💗 Knowledge retention bonus\n")
	b.WriteString("──────────────────────────────────────\n")
	fmt.Fprintf(&b, "  %s\n", wordWrap(offer.prose, 60))
	b.WriteString("──────────────────────────────────────\n")
	if awaiting {
		fmt.Fprintf(&b, "\r%s   ", animFrames[frame])
	} else {
		fmt.Fprintf(&b, "%s", bonusOfferPrompt+"\n")
	}
	return b.String()
}

func (m askModel) showCaughtUpMessage() bool {
	if m.timeout <= caughtUpWindow {
		return true
	}
	return time.Since(m.started) >= m.timeout-caughtUpWindow
}

func renderQuestion(q questionMsg) string {
	var b strings.Builder
	b.WriteString("\n🧠🤖 Total-Recall Check\n")
	b.WriteString("──────────────────────────────────────\n")
	b.WriteString("  ")
	b.WriteString(wordWrap(q.question, 60))
	b.WriteString("\n")
	for i, c := range q.choices {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, c)
	}
	b.WriteString("──────────────────────────────────────\n")
	fmt.Fprintf(&b, "%s", choicePrompt(len(q.choices)))
	return b.String()
}

func parseChoiceSelection(key string, choiceCount int) (int, bool) {
	if choiceCount <= 0 || len(key) != 1 {
		return 0, false
	}
	selection, err := strconv.Atoi(key)
	if err != nil || selection < 1 || selection > choiceCount || selection > 9 {
		return 0, false
	}
	return selection - 1, true
}

func choicePrompt(choiceCount int) string {
	if choiceCount <= 0 {
		return "Press Enter to skip: "
	}
	upper := choiceCount
	if upper > 9 {
		upper = 9
	}
	return fmt.Sprintf("[1-%d] or Enter to skip: ", upper)
}

func wordWrap(s string, width int) string {
	if len(s) <= width {
		return s
	}
	var b strings.Builder
	words := strings.Fields(s)
	line := 0
	for i, w := range words {
		if i > 0 {
			if line+1+len(w) > width {
				b.WriteString("\n  ")
				line = 0
			} else {
				b.WriteString(" ")
				line++
			}
		}
		b.WriteString(w)
		line += len(w)
	}
	return b.String()
}

// renderPostAltScreen produces the terminal output printed after the Bubble Tea
// alt-screen closes. Returns "" when nothing should be printed (q/Esc exit).
func renderPostAltScreen(am askModel) string {
	switch {
	case am.advisory != "":
		return am.advisory + "\n"
	case am.offerShown && !am.bonusAnswered:
		// The commit question's feedback already rendered inline in the offer
		// view — repeating it here would double-print on exit.
		return ""
	case am.skipped:
		return "→ Question saved for later.\n"
	case am.feedbackResult.correctText != "":
		var b strings.Builder
		fr := am.feedbackResult
		if fr.correct {
			b.WriteString("✓ Correct.\n")
		} else {
			fmt.Fprintf(&b, "✗ The answer was: %s\n", fr.correctText)
		}
		if fr.feedback != "" {
			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s\n", fr.feedback)
		}
		return b.String()
	}
	return ""
}
