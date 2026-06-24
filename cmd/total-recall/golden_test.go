package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func requireEqualGolden(t *testing.T, data []byte) {
	t.Helper()
	name := t.Name()
	path := filepath.Join("testdata", name+".golden")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("golden file %s does not exist.\nTo create it:\n  $env:UPDATE_GOLDEN=1; go test -run %s ./cmd/total-recall/...\n\nGot output:\n%s", path, name, string(data))
	}
	if err != nil {
		t.Fatalf("reading golden file %s: %v", path, err)
	}
	if !bytes.Equal(expected, data) {
		t.Fatalf("golden file mismatch: %s\n\nexpected (%d bytes):\n%q\n\ngot (%d bytes):\n%q\n\nTo update: $env:UPDATE_GOLDEN=1 go test -run %s ./cmd/total-recall/...",
			path, len(expected), string(expected), len(data), string(data), name)
	}
}

func TestGoldenAskThinkingView(t *testing.T) {
	m := newAskModel(10*time.Second, "")
	m.frame = 0
	view := m.View()
	requireEqualGolden(t, []byte(view))
}

func TestGoldenAskCaughtUpView(t *testing.T) {
	m := newAskModel(10*time.Second, "")
	m.started = time.Now().Add(-7 * time.Second)
	view := m.View()
	requireEqualGolden(t, []byte(view))
}

func TestGoldenAskQuestionView(t *testing.T) {
	m := newAskModel(10*time.Second, "")

	updated, _ := m.updateThinking(questionMsg{
		id:       42,
		question: "What does DRY stand for?",
		choices:  []string{"Don't Repeat Yourself", "Don't Run Yaks", "Digital Repository YAML", "Deferred Runtime Yielding"},
	})
	m2 := updated.(askModel)

	view := m2.View()
	requireEqualGolden(t, []byte(view))
}

func TestGoldenAskDoneView(t *testing.T) {
	m := newAskModel(10*time.Second, "")
	m.state = stateDone
	view := m.View()
	requireEqualGolden(t, []byte(view))
}

func TestGoldenAskFeedbackView(t *testing.T) {
	m := newAskModel(10 * time.Second, "")
	m.state = stateFeedback
	view := m.View()
	requireEqualGolden(t, []byte(view))
}
