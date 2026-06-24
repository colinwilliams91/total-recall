package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAskViewShowsCaughtUpMessageInFinalWindow(t *testing.T) {
	m := newAskModel(10*time.Second, "")
	m.started = time.Now().Add(-7 * time.Second)

	view := m.View()
	if !strings.Contains(view, caughtUpMessage) {
		t.Fatalf("expected caught-up message in view, got %q", view)
	}
}

func TestAskTimeoutSetsCaughtUpFeedback(t *testing.T) {
	m := newAskModel(2*time.Second, "")
	m.started = time.Now().Add(-3 * time.Second)

	updated, _ := m.updateThinking(tickMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.advisory != caughtUpMessage {
		t.Fatalf("expected advisory %q, got %q", caughtUpMessage, got.advisory)
	}
}

func TestRenderQuestionShowsAllChoices(t *testing.T) {
	view := renderQuestion(questionMsg{
		question: "What does the diff hunk header mean?",
		choices: []string{"first", "second", "third", "fourth"},
	})

	if !strings.Contains(view, "4. fourth") {
		t.Fatalf("expected fourth choice in view, got %q", view)
	}
	if !strings.Contains(view, "[1-4] or Enter to skip: ") {
		t.Fatalf("expected dynamic prompt in view, got %q", view)
	}
}

func TestParseChoiceSelectionSupportsFourthChoice(t *testing.T) {
	idx, ok := parseChoiceSelection("4", 4)
	if !ok {
		t.Fatalf("expected fourth choice to parse")
	}
	if idx != 3 {
		t.Fatalf("expected index 3, got %d", idx)
	}
}

func TestAskDaemonUnreachableShowsAdvisory(t *testing.T) {
	m := newAskModel(10*time.Second, "")

	updated, _ := m.updateThinking(daemonUnreachableMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.advisory != daemonUnavailableMessage {
		t.Fatalf("expected advisory %q, got %q", daemonUnavailableMessage, got.advisory)
	}
}

func TestAskProgramRunKeepsCaughtUpFeedbackOnTimeout(t *testing.T) {
	m := newAskModel(200*time.Millisecond, "")
	m.httpClient.Timeout = 50 * time.Millisecond
	m.started = time.Now().Add(-250 * time.Millisecond)

	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard))
	finalModel, err := p.Run()
	if err != nil {
		t.Fatalf("program run failed: %v", err)
	}

	got, ok := finalModel.(askModel)
	if !ok {
		t.Fatalf("expected askModel, got %T", finalModel)
	}
	if got.advisory != caughtUpMessage {
		t.Fatalf("expected advisory %q, got %q", caughtUpMessage, got.advisory)
	}
}

func TestAskModelSendsAnswerOnKeyPress(t *testing.T) {
	m := newAskModel(10*time.Second, "")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "What is the meaning of life?",
		choices:  []string{"42", "To crush your enemies", "To learn Go", "All of the above"},
	})
	m2 := updated.(askModel)

	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'1'}}))
	got := final.(askModel)

	if got.state != stateFeedback {
		t.Fatalf("expected stateFeedback after selecting choice, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (postAnswer)")
	}
}

func TestAskModelExitsOnCtrlC(t *testing.T) {
	m := newAskModel(10*time.Second, "")

	final, cmd := m.updateThinking(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	got := final.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone after Ctrl+C, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (Quit)")
	}
}

func TestAskModelEnterKeySkipsQuestion(t *testing.T) {
	m := newAskModel(10*time.Second, "")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "Skip me?",
		choices:  []string{"yes", "no"},
	})
	m2 := updated.(askModel)

	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	got := final.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone after Enter, got %v", got.state)
	}
	if !got.skipped {
		t.Fatal("expected skipped to be true after Enter")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (postSkip)")
	}
}

func TestAskModelIgnoresOutOfRangeChoice(t *testing.T) {
	m := newAskModel(10*time.Second, "")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "Pick one?",
		choices:  []string{"a", "b", "c", "d"},
	})
	m2 := updated.(askModel)

	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'9'}}))
	got := final.(askModel)

	if got.state != stateQuestion {
		t.Fatalf("expected stateQuestion for out-of-range choice, got %v", got.state)
	}
	if cmd != nil {
		t.Fatal("expected nil cmd for unrecognized key")
	}
}

// isQuitCmd executes a tea.Cmd and reports whether it yields tea.QuitMsg.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestUpdateFeedbackReceivesFeedbackMsg(t *testing.T) {
	m := newAskModel(10 * time.Second, "")
	m.state = stateFeedback

	updated, cmd := m.updateFeedback(feedbackMsg{correct: true, correctText: "X", feedback: "Y"})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !got.feedbackResult.correct {
		t.Fatal("expected feedbackResult.correct true")
	}
	if got.feedbackResult.correctText != "X" {
		t.Fatalf("expected correctText %q, got %q", "X", got.feedbackResult.correctText)
	}
	if got.feedbackResult.feedback != "Y" {
		t.Fatalf("expected feedback %q, got %q", "Y", got.feedbackResult.feedback)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestUpdateFeedbackReceivesSkipMsg(t *testing.T) {
	m := newAskModel(10 * time.Second, "")
	m.state = stateFeedback

	updated, cmd := m.updateFeedback(skipMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !got.skipped {
		t.Fatal("expected skipped true")
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestUpdateFeedbackCtrlC(t *testing.T) {
	m := newAskModel(10 * time.Second, "")
	m.state = stateFeedback

	updated, cmd := m.updateFeedback(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestStateFeedbackView(t *testing.T) {
	m := newAskModel(10 * time.Second, "")
	m.state = stateFeedback

	view := m.View()
	if !strings.Contains(view, "Evaluating...") {
		t.Fatalf("expected view to contain %q, got %q", "Evaluating...", view)
	}
	if strings.Contains(view, "Thinking.") {
		t.Fatalf("expected view to NOT contain %q, got %q", "Thinking.", view)
	}
}

func TestPostAnswerParsesResponse(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	origURL := daemonBaseURL
	daemonBaseURL = baseURL
	t.Cleanup(func() { daemonBaseURL = origURL })

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "", "post-answer q", []string{"correct-a", "wrong-b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next")
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode next: %v", err)
	}
	resp.Body.Close()

	cmd := postAnswer(q.ID, 0, "", &http.Client{Timeout: 3 * time.Second})
	msg := cmd()

	fm, ok := msg.(feedbackMsg)
	if !ok {
		t.Fatalf("expected feedbackMsg, got %T", msg)
	}
	if !fm.correct {
		t.Fatal("expected correct true")
	}
	if fm.correctText != "correct-a" {
		t.Fatalf("expected correctText %q, got %q", "correct-a", fm.correctText)
	}
}

func TestPostSkipReturnsSkipMsg(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	origURL := daemonBaseURL
	daemonBaseURL = baseURL
	t.Cleanup(func() { daemonBaseURL = origURL })

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "", "post-skip q", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next")
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode next: %v", err)
	}
	resp.Body.Close()

	cmd := postSkip(q.ID, "", &http.Client{Timeout: 3 * time.Second})
	msg := cmd()

	if _, ok := msg.(skipMsg); !ok {
		t.Fatalf("expected skipMsg, got %T", msg)
	}
}

func TestPostAltScreenRendering(t *testing.T) {
	// Correct answer with feedback.
	correctOut := renderPostAltScreen(askModel{
		feedbackResult: feedbackMsg{correct: true, correctText: "X", feedback: "Great work!"},
	})
	if !strings.Contains(correctOut, "✓ Correct.") {
		t.Fatalf("correct case: expected %q in output, got %q", "✓ Correct.", correctOut)
	}
	if !strings.Contains(correctOut, "Great work!") {
		t.Fatalf("correct case: expected feedback in output, got %q", correctOut)
	}

	// Incorrect answer with feedback.
	incorrectOut := renderPostAltScreen(askModel{
		feedbackResult: feedbackMsg{correct: false, correctText: "The right answer", feedback: "Because..."},
	})
	if !strings.Contains(incorrectOut, "✗ The answer was: The right answer") {
		t.Fatalf("incorrect case: expected correctText in output, got %q", incorrectOut)
	}
	if !strings.Contains(incorrectOut, "Because...") {
		t.Fatalf("incorrect case: expected feedback in output, got %q", incorrectOut)
	}

	// Skip.
	skipOut := renderPostAltScreen(askModel{skipped: true})
	if !strings.Contains(skipOut, "→ Question saved for later.") {
		t.Fatalf("skip case: expected skip message, got %q", skipOut)
	}

	// Advisory.
	advisoryOut := renderPostAltScreen(askModel{advisory: "caught up message"})
	if advisoryOut != "caught up message\n" {
		t.Fatalf("advisory case: expected %q, got %q", "caught up message\n", advisoryOut)
	}

	// q/Esc exit — nothing printed.
	emptyOut := renderPostAltScreen(askModel{})
	if emptyOut != "" {
		t.Fatalf("q/Esc case: expected empty output, got %q", emptyOut)
	}
}

func TestAskModelQKeyExitsSilently(t *testing.T) {
	m := newAskModel(10 * time.Second, "")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "Silent exit?",
		choices:  []string{"a", "b"},
	})
	m2 := updated.(askModel)
	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}}))
	got := final.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone after q, got %v", got.state)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
	if got.skipped {
		t.Fatal("expected skipped false for q exit")
	}
	if got.advisory != "" {
		t.Fatalf("expected empty advisory for q exit, got %q", got.advisory)
	}
	if got.feedbackResult.correctText != "" {
		t.Fatal("expected empty feedbackResult for q exit")
	}
	if out := renderPostAltScreen(got); out != "" {
		t.Fatalf("expected no post-alt-screen output for q exit, got %q", out)
	}
}