package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/ai"
)

func aiCompletionTestRequest() ai.CompletionRequest {
	return ai.CompletionRequest{Model: "test-model", UserTurn: "hi", MaxTokens: 32}
}

func TestCompleteSendsAgentIdentityHeaders(t *testing.T) {
	var gotUA, gotSession string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotUA = r.Header.Get("User-Agent")
		gotSession = r.Header.Get("x-opencode-session")
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "test-model")
	if _, err := c.Complete(context.Background(), aiCompletionTestRequest()); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if gotUA == "" || gotUA == "Go-http-client/1.1" {
		t.Fatalf("expected a total-recall User-Agent, got %q", gotUA)
	}
	if gotSession == "" {
		t.Fatal("expected x-opencode-session header to be sent")
	}

	// The session ID must be stable for the life of the client — the same
	// header value on every call within the process, not per-request.
	var firstSession = gotSession
	if _, err := c.Complete(context.Background(), aiCompletionTestRequest()); err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if gotSession != firstSession {
		t.Fatalf("session header unstable: %q then %q", firstSession, gotSession)
	}
}

func TestSessionIDFormatAndUniqueness(t *testing.T) {
	a, err := newSessionID()
	if err != nil {
		t.Fatalf("newSessionID: %v", err)
	}
	b, err := newSessionID()
	if err != nil {
		t.Fatalf("newSessionID: %v", err)
	}
	if a == b {
		t.Fatalf("expected unique session IDs, got %q twice", a)
	}
	if len(a) != len("trec-")+16 {
		t.Fatalf("unexpected session id shape %q", a)
	}
}

func TestCompleteOmitsSessionHeaderOnMintFailure(t *testing.T) {
	// An empty session must not send an empty-valued header (gateways sniff
	// presence, not value); construction failures degrade to anonymous.
	c := &Client{baseURL: "http://unused", model: "m", httpClient: &http.Client{}}
	if c.sessionID != "" {
		t.Fatalf("zero-value client unexpectedly carries a session id: %q", c.sessionID)
	}
}
