package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	daemonBaseURL  = "http://localhost:7331"
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
			m := newAskModel(time.Duration(timeout) * time.Second)
			p := tea.NewProgram(m, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				return err
			}
			// Print feedback on the main screen after the alt-screen has exited.
			if am, ok := finalModel.(askModel); ok && am.feedback != "" {
				fmt.Println(am.feedback)
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

// ── model ─────────────────────────────────────────────────────────────────────

type askModel struct {
	state      askState
	frame      int
	started    time.Time
	timeout    time.Duration
	polling    bool // true while an HTTP poll is in flight
	question   questionMsg
	feedback   string
	httpClient *http.Client
}

func newAskModel(timeout time.Duration) askModel {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return askModel{
		state:      stateThinking,
		started:    time.Now(),
		timeout:    timeout,
		httpClient: &http.Client{Timeout: 3 * time.Second},
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
	return func() tea.Msg {
		resp, err := client.Get(daemonBaseURL + "/recall/next")
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
	}
	return m, tea.Quit
}

func (m askModel) updateThinking(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.frame = (m.frame + 1) % len(animFrames)
		if time.Since(m.started) >= m.timeout {
			m.state = stateDone
			m.feedback = caughtUpMessage
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
		m.feedback = daemonUnavailableMessage
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

	answer := ""
	feedback := ""

	if idx, ok := parseChoiceSelection(k.String(), len(m.question.choices)); ok {
		if idx < len(m.question.choices) {
			answer = m.question.choices[idx]
			feedback = "✓ recorded"
		}
	} else {
		switch k.String() {
		case "enter":
		answer = "skip"
		feedback = "→ skipped"
		case "q", "esc":
		m.state = stateDone
		return m, tea.Quit
		case "ctrl+c":
		m.state = stateDone
		return m, tea.Quit
		default:
		return m, nil
		}
	}

	if answer != "" {
		_ = m.postAnswer(m.question.id, answer)
		// fmt.Println(feedback)
		m.feedback = feedback
	}
	m.state = stateDone
	return m, tea.Quit
}

func (m askModel) postAnswer(id int64, answer string) error {
	body, _ := json.Marshal(map[string]any{"id": id, "answer": answer})
	resp, err := m.httpClient.Post(daemonBaseURL+"/recall/answer", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
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
