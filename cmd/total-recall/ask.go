package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/colinwilliams91/total-recall/internal/hooks"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var daemonBaseURL = "http://localhost:7331"

const (
	defaultTimeout = 15 * time.Second
	animTick       = 400 * time.Millisecond
	caughtUpWindow = 4 * time.Second
)

var animFrames = []string{"Thinking.", "Thinking..", "Thinking..."}

const caughtUpMessage = "You're all caught up on your recall questions. Great job 🤖💗"

const daemonUnavailableMessage = "[total-recall] Daemon not running. Start with total-recall serve."

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
			repo, repoErr := hooks.FindRepoRoot()
			if repoErr != nil {
				repo = ""
				fmt.Fprintln(os.Stderr, "[ask] not inside a git repo — falling back to global recall queue")
			}
			m := newAskModel(time.Duration(timeout)*time.Second, repo)
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

	cmd.Flags().IntVar(&timeout, "timeout", 15, "Seconds to wait for a question before exiting")
	return cmd
}

// ── state machine ─────────────────────────────────────────────────────────────

type askState int

const (
	stateThinking askState = iota
	stateQuestion
	stateFeedback
	stateDone
)

// ── messages ──────────────────────────────────────────────────────────────────

type tickMsg struct{}
type questionMsg struct {
	id       int64
	question string
	choices  []string
}
type noQuestionMsg struct{}
type daemonUnreachableMsg struct{}
type answerPostedMsg struct{}
type feedbackMsg struct {
	correct     bool
	correctText string
	feedback    string
}
type skipMsg struct{}

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
	repo           string
}

func newAskModel(timeout time.Duration, repo string) askModel {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return askModel{
		state:      stateThinking,
		started:    time.Now(),
		timeout:    timeout,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		repo:       repo,
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
	return func() tea.Msg {
		u := daemonBaseURL + "/recall/next"
		if repo != "" {
			u += "?repo=" + url.QueryEscape(repo)
		}
		resp, err := client.Get(u)
		if err != nil {
			return daemonUnreachableMsg{}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			return noQuestionMsg{}
		}
		var body struct {
			ID       int64    `json:"id"`
			Question string   `json:"question"`
			Choices  []string `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return noQuestionMsg{}
		}
		return questionMsg{id: body.ID, question: body.Question, choices: body.Choices}
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
			m.state = stateFeedback
			return m, postAnswer(m.question.id, idx, m.repo, m.httpClient)
		}
		return m, nil
	}

	switch k.String() {
	case "enter":
		m.state = stateDone
		m.skipped = true
		return m, postSkip(m.question.id, m.repo, m.httpClient)
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
		}
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

func postAnswer(id int64, answerIndex int, repo string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{"id": id, "answer_index": answerIndex})
		u := daemonBaseURL + "/recall/answer?feedback=true"
		if repo != "" {
			u += "&repo=" + url.QueryEscape(repo)
		}
		resp, err := client.Post(u, "application/json", bytes.NewReader(body))
		if err != nil {
			return feedbackMsg{}
		}
		defer resp.Body.Close()
		var out struct {
			OK          bool   `json:"ok"`
			Correct     bool   `json:"correct"`
			CorrectText string `json:"correct_text"`
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

func postSkip(id int64, repo string, client *http.Client) tea.Cmd {
	return func() tea.Msg {
		body, _ := json.Marshal(map[string]any{"id": id, "skip": true})
		u := daemonBaseURL + "/recall/answer"
		if repo != "" {
			u += "?repo=" + url.QueryEscape(repo)
		}
		resp, err := client.Post(u, "application/json", bytes.NewReader(body))
		if err != nil {
			return skipMsg{}
		}
		defer resp.Body.Close()
		return skipMsg{}
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
	case stateDone:
		return "\n\nPress any key to continue...\n"
	}
	return ""
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
