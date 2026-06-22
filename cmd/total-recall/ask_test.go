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
	m := newAskModel(10 * time.Second)

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
	if got.advisory != caughtUpMessage {
		t.Fatalf("expected advisory %q, got %q", caughtUpMessage, got.advisory)
	}
}