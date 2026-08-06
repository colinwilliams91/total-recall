package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
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
	choices := []recall.Choice{
		{Text: "A"},
		{Text: "B"},
		{Text: "C"},
	}
	req := recall.FeedbackRequest("What is caching?", choices, 1, 1, "test-model")

	if !strings.Contains(req.UserTurn, "<- correct, chosen") {
		t.Fatalf("expected user turn to contain %q, got %q", "<- correct, chosen", req.UserTurn)
	}
	if !strings.Contains(req.UserTurn, "The developer answered correctly.") {
		t.Fatalf("expected user turn to end with correct-case sentence, got %q", req.UserTurn)
	}
}

func TestFeedbackRequestIncorrectCase(t *testing.T) {
	choices := []recall.Choice{
		{Text: "A"},
		{Text: "B"},
		{Text: "C"},
	}
	req := recall.FeedbackRequest("What is caching?", choices, 2, 0, "test-model")

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
	choices := []recall.Choice{{Text: "a"}, {Text: "b"}}
	req := recall.FeedbackRequest("q", choices, 0, 0, "m")

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

	choices := []recall.Choice{{Text: "a"}, {Text: "b"}}
	got := engine.GenerateFeedback(context.Background(), "q", choices, 0, 0, "m")
	if got != "" {
		t.Fatalf("expected empty string on AI error, got %q", got)
	}
}

func TestSynthesizeEmptyConceptsReturnsNil(t *testing.T) {
	s := setupCache(t)
	provider := &mockProvider{response: `{"question":"q","choices":["a","b"]}`}
	engine := recall.New(provider, s)

	synth := recall.SynthesisContext{}
	q, err := engine.Synthesize(context.Background(), "/repo", "main", "intermediate", "m", synth)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if q != nil {
		t.Fatalf("expected nil question for empty concepts, got %+v", q)
	}
	if provider.lastReq.System != "" {
		t.Fatal("expected provider not to be called for empty concepts")
	}
}

func TestSynthesizeEmptyRepoReturnsNil(t *testing.T) {
	s := setupCache(t)
	provider := &mockProvider{response: `{"question":"q","choices":["a","b"]}`}
	engine := recall.New(provider, s)

	synth := recall.SynthesisContext{
		Concepts: []cache.ConceptRow{{Concept: "c", Weight: 0.9, Source: "code", SeenAt: time.Now()}},
	}
	q, err := engine.Synthesize(context.Background(), "", "main", "intermediate", "m", synth)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if q != nil {
		t.Fatalf("expected nil question for empty repo, got %+v", q)
	}
}

func TestSynthesizePopulatesUserTurnWithWeights(t *testing.T) {
	s := setupCache(t)
	provider := &mockProvider{response: `{"question":"q","choices":["a","b","c"]}`}
	engine := recall.New(provider, s)

	synth := recall.SynthesisContext{
		Concepts: []cache.ConceptRow{
			{Concept: "exponential backoff", Weight: 0.9, Source: "code", SeenAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
			{Concept: "jitter", Weight: 0.7, Source: "code", SeenAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
		},
		CommitMsg:   "fix: handle race in cache.Save",
		DiffSnippet: "+ func retry() {}",
	}
	q, err := engine.Synthesize(context.Background(), "/repo", "main", "hard", "m", synth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil question")
	}
	if !strings.Contains(provider.lastReq.UserTurn, "weight=0.9") {
		t.Fatalf("expected user turn to contain weight=0.9, got %q", provider.lastReq.UserTurn)
	}
	if !strings.Contains(provider.lastReq.UserTurn, "exponential backoff") {
		t.Fatalf("expected user turn to contain concept name, got %q", provider.lastReq.UserTurn)
	}
	if !strings.Contains(provider.lastReq.UserTurn, "Recent commit context") {
		t.Fatalf("expected user turn to contain commit context section, got %q", provider.lastReq.UserTurn)
	}
	if !strings.Contains(provider.lastReq.UserTurn, "fix: handle race in cache.Save") {
		t.Fatalf("expected user turn to contain commit message, got %q", provider.lastReq.UserTurn)
	}
}

func TestSynthesizeUserTurnOmitsCommitContextWhenEmpty(t *testing.T) {
	s := setupCache(t)
	provider := &mockProvider{response: `{"question":"q","choices":["a","b"]}`}
	engine := recall.New(provider, s)

	synth := recall.SynthesisContext{
		Concepts: []cache.ConceptRow{
			{Concept: "caching", Weight: 0.5, Source: "code", SeenAt: time.Now()},
		},
	}
	_, err := engine.Synthesize(context.Background(), "/repo", "main", "intermediate", "m", synth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(provider.lastReq.UserTurn, "Recent commit context") {
		t.Fatalf("expected NO commit context section when CommitMsg and DiffSnippet are empty, got %q", provider.lastReq.UserTurn)
	}
}

func TestSynthesisRequestEmbedsPolicy(t *testing.T) {
	concepts := []cache.ConceptRow{
		{Concept: "caching", Weight: 0.9, Source: "code", SeenAt: time.Now()},
	}
	policyBody := "## Custom policy\n\nFocus on counterfactual debugging.\n"
	req := recall.SynthesisRequest(concepts, "", "", policyBody, "hard", "m")

	if !strings.Contains(req.System, "counterfactual debugging") {
		t.Fatalf("expected system turn to contain policy body text, got %q", req.System)
	}
	if !strings.Contains(req.System, "## Format contract") {
		t.Fatalf("expected system turn to contain format contract section, got %q", req.System)
	}
	if !strings.Contains(req.System, `"choices"`) {
		t.Fatalf("expected system turn to contain JSON contract, got %q", req.System)
	}
	if !strings.Contains(req.System, "hard") {
		t.Fatalf("expected system turn to contain difficulty, got %q", req.System)
	}
}

func TestSynthesisRequestFallbackNoPolicy(t *testing.T) {
	concepts := []cache.ConceptRow{
		{Concept: "caching", Weight: 0.9, Source: "code", SeenAt: time.Now()},
	}
	req := recall.SynthesisRequest(concepts, "", "", "", "intermediate", "m")

	if !strings.Contains(req.System, "You are a technical recall assistant") {
		t.Fatalf("expected fallback template in system turn, got %q", req.System)
	}
	if strings.Contains(req.System, "## Format contract") {
		t.Fatalf("expected NO format contract section in fallback path, got %q", req.System)
	}
	if !strings.Contains(req.System, "intermediate") {
		t.Fatalf("expected difficulty in fallback template, got %q", req.System)
	}
}

func TestSynthesisRequestUserTurnEnriched(t *testing.T) {
	concepts := []cache.ConceptRow{
		{Concept: "backoff", Weight: 0.9, Source: "code", SeenAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
		{Concept: "jitter", Weight: 0.7, Source: "code", SeenAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
		{Concept: "circuit breaker", Weight: 0.5, Source: "code", SeenAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)},
	}
	req := recall.SynthesisRequest(concepts, "fix: race condition", "+ func retry() {}", "policy body", "hard", "m")

	for _, want := range []string{"backoff", "jitter", "circuit breaker", "weight=0.9", "weight=0.7", "weight=0.5", "Recent commit context", "fix: race condition", "+ func retry()"} {
		if !strings.Contains(req.UserTurn, want) {
			t.Fatalf("expected user turn to contain %q, got %q", want, req.UserTurn)
		}
	}
}

func TestSynthesisRequestUserTurnNoCommit(t *testing.T) {
	concepts := []cache.ConceptRow{
		{Concept: "caching", Weight: 0.5, Source: "code", SeenAt: time.Now()},
	}
	req := recall.SynthesisRequest(concepts, "", "", "policy", "intermediate", "m")

	if !strings.Contains(req.UserTurn, "caching") {
		t.Fatalf("expected user turn to list concept, got %q", req.UserTurn)
	}
	if strings.Contains(req.UserTurn, "Recent commit context") {
		t.Fatalf("expected NO commit context section, got %q", req.UserTurn)
	}
}

func TestSynthesisRequestMaxTokensAndJSON(t *testing.T) {
	concepts := []cache.ConceptRow{
		{Concept: "c", Weight: 0.5, Source: "code", SeenAt: time.Now()},
	}
	req := recall.SynthesisRequest(concepts, "", "", "", "intermediate", "m")

	if req.MaxTokens != 1024 {
		t.Fatalf("expected MaxTokens 1024, got %d", req.MaxTokens)
	}
	if !req.JSON {
		t.Fatal("expected JSON true for synthesis request")
	}
}
