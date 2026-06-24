package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/recall"
)

// mockProvider implements ai.Provider with canned responses for testing
// recall.Engine behavior without real AI calls. It records the last request
// so tests can assert on prompt construction.
type mockProvider struct {
	response string
	err      error
	lastReq  ai.CompletionRequest
}

func (m *mockProvider) Complete(_ context.Context, req ai.CompletionRequest) (string, error) {
	m.lastReq = req
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func TestFeedbackRequestCorrectCase(t *testing.T) {
	req := recall.FeedbackRequest("What is caching?", []string{"A", "B", "C"}, 1, 1, "test-model")

	if !strings.Contains(req.UserTurn, "<- correct, chosen") {
		t.Fatalf("expected user turn to contain %q, got %q", "<- correct, chosen", req.UserTurn)
	}
	if !strings.Contains(req.UserTurn, "The developer answered correctly.") {
		t.Fatalf("expected user turn to end with correct-case sentence, got %q", req.UserTurn)
	}
}

func TestFeedbackRequestIncorrectCase(t *testing.T) {
	req := recall.FeedbackRequest("What is caching?", []string{"A", "B", "C"}, 0, 2, "test-model")

	if !strings.Contains(req.UserTurn, "[1] A  <- correct") {
		t.Fatalf("expected correct-choice annotation, got %q", req.UserTurn)
	}
	if !strings.Contains(req.UserTurn, "[3] C  <- chosen (incorrect)") {
		t.Fatalf("expected chosen-incorrect annotation, got %q", req.UserTurn)
	}
	if !strings.Contains(req.UserTurn, "The developer chose option 3 and was incorrect.") {
		t.Fatalf("expected incorrect-case sentence referencing option 3, got %q", req.UserTurn)
	}
}

func TestFeedbackRequestTokenBudget(t *testing.T) {
	req := recall.FeedbackRequest("q", []string{"a", "b"}, 0, 0, "m")

	if req.MaxTokens != 150 {
		t.Fatalf("expected MaxTokens 150, got %d", req.MaxTokens)
	}
	if req.JSON {
		t.Fatal("expected JSON false for feedback request")
	}
}

func TestGenerateFeedbackDegradation(t *testing.T) {
	s := setupCache(t)
	provider := &mockProvider{err: errors.New("timeout")}
	engine := recall.New(provider, s)

	got := engine.GenerateFeedback(context.Background(), "q", []string{"a", "b"}, 0, 0, "m")
	if got != "" {
		t.Fatalf("expected empty string on AI error, got %q", got)
	}
}
