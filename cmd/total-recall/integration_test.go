package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/engine"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func startTestDaemon(t *testing.T) (*engine.Server, *cache.Store, string) {
	t.Helper()

	t.Setenv("TR_HOME", t.TempDir())

	store, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	userCfg := config.DefaultUserConfig()
	cfg := config.Merge(&userCfg, nil)

	srv := engine.New(cfg, nil, store, nil)

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	go srv.Serve(ln)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})

	waitForDaemon(t, baseURL)
	return srv, store, baseURL
}

func waitForDaemon(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("daemon did not start within 3s")
}

func mustGET(t *testing.T, baseURL, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func mustPOST(t *testing.T, baseURL, path string, body []byte) *http.Response {
	t.Helper()
	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func TestDaemonHealth(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustGET(t, baseURL, "/health")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("expected status ok, got %q", health.Status)
	}
}

func TestDaemonHookPreCommit(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	body := `{"hook":"pre-commit","repo":"/tmp/test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ test"}}`
	resp := mustPOST(t, baseURL, "/hooks/pre-commit", []byte(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var hookResp engine.HookResponse
	if err := json.NewDecoder(resp.Body).Decode(&hookResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if hookResp.Status != "received" {
		t.Fatalf("expected status received, got %q", hookResp.Status)
	}
}

func TestDaemonHookInvalidJSON(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustPOST(t, baseURL, "/hooks/pre-commit", []byte(`not json`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRecallNextEmpty(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustGET(t, baseURL, "/recall/next")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestRecallNextWithQuestion(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "", "What does DRY stand for?", []string{
		"Don't Repeat Yourself",
		"Don't Run Yaks",
		"Digital Repository YAML",
		"Deferred Runtime Yielding",
	}, 0); err != nil {
		t.Fatalf("seed question: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		ID       int64    `json:"id"`
		Question string   `json:"question"`
		Choices  []string `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if result.Question != "What does DRY stand for?" {
		t.Fatalf("expected DRY question, got %q", result.Question)
	}
	if len(result.Choices) != 4 {
		t.Fatalf("expected 4 choices, got %d", len(result.Choices))
	}
}

func TestRecallNextIdempotent(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "", "single question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r1 := mustGET(t, baseURL, "/recall/next")
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first call, got %d", r1.StatusCode)
	}
	r1.Body.Close()

	r2 := mustGET(t, baseURL, "/recall/next")
	defer r2.Body.Close()

	if r2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on second call, got %d", r2.StatusCode)
	}
}

func TestRecallAnswer(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "", "answerable question", []string{"choice a", "choice b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var question struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&question); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	answerBody := fmt.Sprintf(`{"id":%d,"answer_index":0}`, question.ID)
	r2 := mustPOST(t, baseURL, "/recall/answer", []byte(answerBody))
	defer r2.Body.Close()

	if r2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", r2.StatusCode)
	}
	var answerResp struct {
		Ok bool `json:"ok"`
	}
	if err := json.NewDecoder(r2.Body).Decode(&answerResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !answerResp.Ok {
		t.Fatal("expected ok:true")
	}
}

func TestRecallAnswerSkip(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "", "skippable question", []string{"x", "y"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode: %v", err)
	}
	resp.Body.Close()

	skipBody := fmt.Sprintf(`{"id":%d,"skip":true}`, q.ID)
	r2 := mustPOST(t, baseURL, "/recall/answer", []byte(skipBody))
	defer r2.Body.Close()

	if r2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", r2.StatusCode)
	}
	var answerResp struct {
		Ok bool `json:"ok"`
	}
	if err := json.NewDecoder(r2.Body).Decode(&answerResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !answerResp.Ok {
		t.Fatal("expected ok:true")
	}
}

func TestMCPEndpoint(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustGET(t, baseURL, "/mcp/")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("MCP endpoint returned 404 — expected 200 or 400")
	}
}

func TestMCPEndpointContentType(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustGET(t, baseURL, "/mcp/")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Fatal("expected Content-Type header on MCP response")
	}
}

func TestDaemonAllHookEndpoints(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	hookTypes := []string{"/hooks/pre-commit", "/hooks/commit-msg", "/hooks/pre-push"}
	body := `{"hook":"test","repo":"/tmp/test","branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":"+ test"}}`
	for _, hook := range hookTypes {
		resp := mustPOST(t, baseURL, hook, []byte(body))
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("%s: expected 202, got %d", hook, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestDaemonHealthJSONContentType(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustGET(t, baseURL, "/health")
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json Content-Type, got %q", ct)
	}
}

func TestRecallAnswerInvalidBody(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	resp := mustPOST(t, baseURL, "/recall/answer", []byte(`not json`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRecallNextRepoScoped(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/x", "X's question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("seed for repo X: %v", err)
	}

	rY := mustGET(t, baseURL, "/recall/next?repo=/repo/y")
	defer rY.Body.Close()
	if rY.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 for repo Y (no questions), got %d", rY.StatusCode)
	}

	rX := mustGET(t, baseURL, "/recall/next?repo=/repo/x")
	defer rX.Body.Close()
	if rX.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for repo X, got %d", rX.StatusCode)
	}
	var result struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(rX.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Question != "X's question" {
		t.Fatalf("expected X's question, got %q", result.Question)
	}
}

// ── MCP helpers ──────────────────────────────────────────────────────────────

// mcpSession connects an MCP client to the test daemon's /mcp/ endpoint and
// returns the initialized session. The session is closed via t.Cleanup.
func mcpSession(t *testing.T, baseURL string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             baseURL + "/mcp/",
		HTTPClient:           &http.Client{Timeout: 5 * time.Second},
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// toolText extracts the text content from a CallToolResult.
func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content from tool call")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// seedAndClaim saves a question, claims it via GET /recall/next, and returns the ID.
func seedAndClaim(t *testing.T, store *cache.Store, baseURL, question string, choices []string, correctIndex int) int64 {
	t.Helper()
	ctx := context.Background()
	if err := store.SaveQuestion(ctx, question, choices, correctIndex); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resp := mustGET(t, baseURL, "/recall/next")
	defer resp.Body.Close()
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode next: %v", err)
	}
	if q.ID == 0 {
		t.Fatal("expected non-zero question ID")
	}
	return q.ID
}

// ── 4C HTTP integration tests ────────────────────────────────────────────────

func TestRecallAnswerWithFeedbackTrue(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	id := seedAndClaim(t, store, baseURL, "feedback-true q", []string{"a", "b"}, 0)

	body := fmt.Sprintf(`{"id":%d,"answer_index":0}`, id)
	resp := mustPOST(t, baseURL, "/recall/answer?feedback=true", []byte(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"correct", "correct_text", "feedback"} {
		if _, ok := result[field]; !ok {
			t.Fatalf("expected %q field in response, got %v", field, result)
		}
	}
}

func TestRecallAnswerCorrectEvaluation(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	id := seedAndClaim(t, store, baseURL, "correct eval q", []string{"right", "wrong"}, 0)

	body := fmt.Sprintf(`{"id":%d,"answer_index":0}`, id)
	resp := mustPOST(t, baseURL, "/recall/answer?feedback=true", []byte(body))
	defer resp.Body.Close()

	var result struct {
		Correct bool `json:"correct"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.Correct {
		t.Fatal("expected correct true")
	}
}

func TestRecallAnswerIncorrectEvaluation(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	id := seedAndClaim(t, store, baseURL, "incorrect eval q", []string{"right", "wrong"}, 0)

	body := fmt.Sprintf(`{"id":%d,"answer_index":1}`, id)
	resp := mustPOST(t, baseURL, "/recall/answer?feedback=true", []byte(body))
	defer resp.Body.Close()

	var result struct {
		Correct     bool   `json:"correct"`
		CorrectText string `json:"correct_text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Correct {
		t.Fatal("expected correct false")
	}
	if result.CorrectText != "right" {
		t.Fatalf("expected correct_text %q, got %q", "right", result.CorrectText)
	}
}

func TestRecallAnswerOutOfRange(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	id := seedAndClaim(t, store, baseURL, "out-of-range q", []string{"a", "b", "c"}, 0)

	body := fmt.Sprintf(`{"id":%d,"answer_index":99}`, id)
	resp := mustPOST(t, baseURL, "/recall/answer?feedback=true", []byte(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	var result struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Error != "answer_index out of range" {
		t.Fatalf("expected error %q, got %q", "answer_index out of range", result.Error)
	}
}

func TestRecallAnswerUnknownID(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	body := `{"id":99999,"answer_index":0}`
	resp := mustPOST(t, baseURL, "/recall/answer?feedback=true", []byte(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ── 4C MCP integration tests ─────────────────────────────────────────────────

func TestMCPRecallNextReturnsCorrectIndex(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "mcp next q", []string{"a", "b", "c"}, 2); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, result)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	idx, ok := m["correct_index"]
	if !ok {
		t.Fatal("expected correct_index field in MCP response")
	}
	if idx != float64(2) {
		t.Fatalf("expected correct_index 2, got %v", idx)
	}
}

func TestMCPRecallAnswerReturnsCorrectness(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "mcp answer q", []string{"right", "wrong"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)

	nextResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool recall_next: %v", err)
	}
	var next map[string]any
	json.Unmarshal([]byte(toolText(t, nextResult)), &next)
	id := int64(next["id"].(float64))

	answerResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_answer",
		Arguments: map[string]any{"id": id, "answer_index": 0, "skip": false},
	})
	if err != nil {
		t.Fatalf("CallTool recall_answer: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, answerResult)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, field := range []string{"ok", "correct", "correct_index", "correct_text"} {
		if _, ok := m[field]; !ok {
			t.Fatalf("expected %q field in MCP response, got %v", field, m)
		}
	}
	if m["correct"] != true {
		t.Fatalf("expected correct true, got %v", m["correct"])
	}
}

func TestMCPRecallAnswerSkip(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "mcp skip q", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)

	nextResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool recall_next: %v", err)
	}
	var next map[string]any
	json.Unmarshal([]byte(toolText(t, nextResult)), &next)
	id := int64(next["id"].(float64))

	skipResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_answer",
		Arguments: map[string]any{"id": id, "answer_index": 0, "skip": true},
	})
	if err != nil {
		t.Fatalf("CallTool recall_answer skip: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, skipResult)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok true, got %v", m["ok"])
	}
}

func TestRecallRecentEnriched(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	// Answer one question correctly (terminal-style with feedback).
	if err := store.SaveQuestion(ctx, "recent-correct", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("seed q1: %v", err)
	}
	q1, err := store.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("claim q1: %v", err)
	}
	if err := store.AnswerQuestion(ctx, q1.ID, 0, "a", true, "Nice!"); err != nil {
		t.Fatalf("answer q1: %v", err)
	}

	// Skip one question.
	if err := store.SaveQuestion(ctx, "recent-skip", []string{"x", "y"}, 0); err != nil {
		t.Fatalf("seed q2: %v", err)
	}
	q2, err := store.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("claim q2: %v", err)
	}
	if err := store.SkipQuestion(ctx, q2.ID); err != nil {
		t.Fatalf("skip q2: %v", err)
	}

	session := mcpSession(t, baseURL)
	rr, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "recall://recent"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(rr.Contents) == 0 {
		t.Fatal("expected non-empty resource contents")
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(rr.Contents[0].Text), &rows); err != nil {
		t.Fatalf("parse recent: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	byQuestion := make(map[string]map[string]any, len(rows))
	for _, r := range rows {
		byQuestion[r["question"].(string)] = r
	}

	// Correct row: correct_index, answer_index, correct, feedback all populated.
	correct := byQuestion["recent-correct"]
	if correct["correct_index"] == nil {
		t.Fatal("expected correct_index in recent-correct row")
	}
	if correct["answer_index"] == nil {
		t.Fatal("expected answer_index in recent-correct row")
	}
	if correct["correct"] == nil {
		t.Fatal("expected correct in recent-correct row")
	}
	if correct["feedback"] == nil {
		t.Fatal("expected feedback non-nil in recent-correct row")
	}

	// Skip row: answer_index, correct, feedback nil; correct_index populated.
	skip := byQuestion["recent-skip"]
	if skip["correct_index"] == nil {
		t.Fatal("expected correct_index in recent-skip row")
	}
	if skip["answer_index"] != nil {
		t.Fatalf("expected answer_index nil in skip row, got %v", skip["answer_index"])
	}
	if skip["correct"] != nil {
		t.Fatalf("expected correct nil in skip row, got %v", skip["correct"])
	}
	if skip["feedback"] != nil {
		t.Fatalf("expected feedback nil in skip row, got %v", skip["feedback"])
	}
}
