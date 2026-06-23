package main

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAskViewShowsCaughtUpMessageInFinalWindow(t *testing.T) {
	m := newAskModel(10 * time.Second)
	m.started = time.Now().Add(-7 * time.Second)

	view := m.View()
	if !strings.Contains(view, caughtUpMessage) {
		t.Fatalf("expected caught-up message in view, got %q", view)
	}
}

func TestAskTimeoutSetsCaughtUpFeedback(t *testing.T) {
	m := newAskModel(2 * time.Second)
	m.started = time.Now().Add(-3 * time.Second)

	updated, _ := m.updateThinking(tickMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.feedback != caughtUpMessage {
		t.Fatalf("expected feedback %q, got %q", caughtUpMessage, got.feedback)
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
	m := newAskModel(10 * time.Second)

	updated, _ := m.updateThinking(daemonUnreachableMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.feedback != daemonUnavailableMessage {
		t.Fatalf("expected feedback %q, got %q", daemonUnavailableMessage, got.feedback)
	}
}

func TestAskProgramRunKeepsCaughtUpFeedbackOnTimeout(t *testing.T) {
	m := newAskModel(200 * time.Millisecond)
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
	if got.feedback != caughtUpMessage {
		t.Fatalf("expected feedback %q, got %q", caughtUpMessage, got.feedback)
	}
}

func TestAskModelSendsAnswerOnKeyPress(t *testing.T) {
	m := newAskModel(10 * time.Second)

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

	if got.state != stateDone {
		t.Fatalf("expected stateDone after selecting choice, got %v", got.state)
	}
	if !strings.Contains(got.feedback, "recorded") {
		t.Fatalf("expected feedback containing 'recorded', got %q", got.feedback)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (Quit)")
	}
}

func TestAskModelExitsOnCtrlC(t *testing.T) {
	m := newAskModel(10 * time.Second)

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
	m := newAskModel(10 * time.Second)

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
	if !strings.Contains(got.feedback, "skipped") {
		t.Fatalf("expected feedback containing 'skipped', got %q", got.feedback)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (Quit)")
	}
}

func TestAskModelIgnoresOutOfRangeChoice(t *testing.T) {
	m := newAskModel(10 * time.Second)

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