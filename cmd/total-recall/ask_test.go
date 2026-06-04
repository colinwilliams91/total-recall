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

func TestAskDaemonUnreachableRemainsSilent(t *testing.T) {
	m := newAskModel(10 * time.Second)

	updated, _ := m.updateThinking(daemonUnreachableMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.feedback != "" {
		t.Fatalf("expected no feedback, got %q", got.feedback)
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