package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/engine"
	"github.com/colinwilliams91/total-recall/internal/hooks"
	"github.com/colinwilliams91/total-recall/internal/recall"
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

// scriptedProvider implements ai.Provider with a sequence of canned responses.
// Used by pipeline integration tests where ExtractConcepts and Synthesize each
// consume one response. Each call to Complete returns the next response in order
// (meticulously tracked with a mutex for concurrent-access safety).
type scriptedProvider struct {
	mu        sync.Mutex
	responses []string
	calls     int
	lastReq   ai.CompletionRequest
}

func (m *scriptedProvider) Complete(_ context.Context, req ai.CompletionRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastReq = req
	if m.calls >= len(m.responses) {
		return "", nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}

// startTestDaemonWithPipeline starts a daemon wired with a provider (and
// optionally a recallEngine built from the store the daemon will use), enabling
// the async pipeline to fire on hook POSTs. makeRecall receives the store and
// returns the recall engine; pass nil for no recall engine (concepts saved but
// no question synthesized).
func startTestDaemonWithPipeline(t *testing.T, provider ai.Provider, makeRecall func(*cache.Store) *recall.Engine) (*engine.Server, *cache.Store, string) {
	t.Helper()

	t.Setenv("TR_HOME", t.TempDir())

	store, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	userCfg := config.DefaultUserConfig()
	cfg := config.Merge(&userCfg, nil)

	var recallEngine *recall.Engine
	if makeRecall != nil {
		recallEngine = makeRecall(store)
	}

	srv := engine.New(cfg, provider, store, recallEngine)

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

	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestRecallNextWithQuestion(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "What does DRY stand for?",
		[]cache.Choice{
		{Text: "Don't Repeat Yourself", IsCorrect: true, Position: 0},
		{Text: "Don't Run Yaks", IsCorrect: false, Position: 1},
		{Text: "Digital Repository YAML", IsCorrect: false, Position: 2},
		{Text: "Deferred Runtime Yielding", IsCorrect: false, Position: 3},
		}, ""); err != nil {
		t.Fatalf("seed question: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
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
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "single question",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r1 := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first call, got %d", r1.StatusCode)
	}
	r1.Body.Close()

	r2 := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
	defer r2.Body.Close()

	if r2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on second call, got %d", r2.StatusCode)
	}
}

func TestRecallAnswer(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "answerable question",
		[]cache.Choice{
		{Text: "choice a", IsCorrect: true, Position: 0},
		{Text: "choice b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
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

	answerBody := fmt.Sprintf(`{"id":%d,"selected_index":0}`, question.ID)
	r2 := mustPOST(t, baseURL, "/recall/select", []byte(answerBody))
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
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "skippable question",
		[]cache.Choice{
		{Text: "x", IsCorrect: true, Position: 0},
		{Text: "y", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
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
	r2 := mustPOST(t, baseURL, "/recall/select", []byte(skipBody))
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

	resp := mustPOST(t, baseURL, "/recall/select", []byte(`not json`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRecallNextRepoScoped(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/x", "main", "X's question",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed for repo X: %v", err)
	}

	rY := mustGET(t, baseURL, "/recall/next?repo=/repo/y&branch=main")
	defer rY.Body.Close()
	if rY.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 for repo Y (no questions), got %d", rY.StatusCode)
	}

	rX := mustGET(t, baseURL, "/recall/next?repo=/repo/x&branch=main")
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

// ── 10. Pipeline integration tests (repo-tagging invariants) ─────────────────

// hookBody builds a hook envelope JSON string with the given repo and diff.
func hookBody(repo, diff string) string {
	return fmt.Sprintf(`{"hook":"pre-commit","repo":%q,"branch":"main","timestamp":"2026-01-01T00:00:00Z","payload":{"diff":%q}}`, repo, diff)
}

// waitForConcepts polls store.Recent until it returns at least n concepts for repo
// or fails the test after a 3s deadline. The pipeline runs asynchronously so the
// store write completes after the hook POST returns 202.
func waitForConcepts(t *testing.T, store *cache.Store, repo string, n int) []cache.ConceptRow {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recent, err := store.Recent(ctx, repo, "main", 10)
		if err != nil {
			t.Fatalf("Recent poll: %v", err)
		}
		if len(recent) >= n {
			return recent
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d concepts for repo %q", n, repo)
	return nil
}

// waitForUndeliveredQuestion polls store.PeekNextQuestion until it returns a
// non-nil question for repo or fails after 3s.
func waitForUndeliveredQuestion(t *testing.T, store *cache.Store, repo string) *cache.StoredQuestion {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		q, err := store.PeekNextQuestion(ctx, repo, "main")
		if err != nil {
			t.Fatalf("Peek poll: %v", err)
		}
		if q != nil {
			return q
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for undelivered question for repo %q", repo)
	return nil
}

// Task 10.1: runPipeline persists concepts tagged with env.Repo.
func TestPipelineSavesConceptsTaggedWithRepo(t *testing.T) {
	provider := &scriptedProvider{responses: []string{
		`[{"concept":"cached-connection","source":"code","weight":1.0}]`,
	}}
	_, store, baseURL := startTestDaemonWithPipeline(t, provider, nil)

	resp := mustPOST(t, baseURL, "/hooks/pre-commit", []byte(hookBody("/repo/test", "+ func foo() {}")))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	recent := waitForConcepts(t, store, "/repo/test", 1)
	if recent[0].Concept != "cached-connection" {
		t.Fatalf("expected cached-connection, got %q", recent[0].Concept)
	}

	wrongRecent, err := store.Recent(context.Background(), "/wrong/repo", "main", 10)
	if err != nil {
		t.Fatalf("Recent for wrong repo: %v", err)
	}
	if len(wrongRecent) != 0 {
		t.Fatalf("expected 0 concepts for /wrong/repo (cross-repo leak), got %d", len(wrongRecent))
	}
}

// Task 10.2: runPipeline synthesizes and saves questions tagged with env.Repo.
func TestPipelineSavesQuestionsTaggedWithRepo(t *testing.T) {
	provider := &scriptedProvider{responses: []string{
		`[{"concept":"retry-pattern","source":"code","weight":0.9}]`,
		`{"question":"What is a retry pattern?","choices":["Option A","Option B"],"correct_index":0}`,
	}}
	_, store, baseURL := startTestDaemonWithPipeline(t, provider, func(s *cache.Store) *recall.Engine {
		return recall.New(provider, s)
	})

	resp := mustPOST(t, baseURL, "/hooks/pre-commit", []byte(hookBody("/repo/test", "+ func retry() {}")))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	q := waitForUndeliveredQuestion(t, store, "/repo/test")
	if q.Question != "What is a retry pattern?" {
		t.Fatalf("expected synthesized question, got %q", q.Question)
	}

	wrongQ, err := store.PeekNextQuestion(context.Background(), "/wrong/repo", "main")
	if err != nil {
		t.Fatalf("Peek for wrong repo: %v", err)
	}
	if wrongQ != nil {
		t.Fatalf("expected no question for /wrong/repo (cross-repo leak), got %+v", wrongQ)
	}
}

// Task 10.3: repo-move advisory is logged on empty dequeue for non-empty repo.
func TestRecallNextRepoMoveAdvisory(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	var logBuf bytes.Buffer
	origOut := log.Writer()
	log.SetOutput(&logBuf)
	defer log.SetOutput(origOut)

	resp := mustGET(t, baseURL, "/recall/next?repo=/moved/repo&branch=main")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	if !strings.Contains(logBuf.String(), "no pending questions for repo") {
		t.Fatalf("expected repo-move advisory in log, got:\n%s", logBuf.String())
	}
	if !strings.Contains(logBuf.String(), "/moved/repo") {
		t.Fatalf("expected repo path in advisory, got:\n%s", logBuf.String())
	}
}

// Task 10.4: POST /recall/answer?repo=… is accepted without error (symmetry).
func TestRecallAnswerAcceptsRepoParam(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "repo-param-symmetry q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode next: %v", err)
	}
	resp.Body.Close()

	body := fmt.Sprintf(`{"id":%d,"selected_index":0}`, q.ID)
	r := mustPOST(t, baseURL, "/recall/select?repo=/any/repo&feedback=false", []byte(body))
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with repo param, got %d", r.StatusCode)
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.OK {
		t.Fatal("expected ok:true")
	}
}

// TestRecallNextBranchIsolation adds an analog to TestRecallNextRepoScoped
// for branch-isolation: queueing under (repo, branch-A) and dequeueing under
// (repo, branch-B) must return 204.
func TestRecallNextBranchIsolation(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/x", "feature-A", "feature-A's q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rWrongBranch := mustGET(t, baseURL, "/recall/next?repo="+url.QueryEscape("/repo/x")+"&branch=main")
	defer rWrongBranch.Body.Close()
	if rWrongBranch.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 for wrong branch, got %d", rWrongBranch.StatusCode)
	}

	rRight := mustGET(t, baseURL, "/recall/next?repo="+url.QueryEscape("/repo/x")+"&branch=feature-A")
	defer rRight.Body.Close()
	if rRight.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for correct branch, got %d", rRight.StatusCode)
	}
	var result struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(rRight.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Question != "feature-A's q" {
		t.Fatalf("expected feature-A's q, got %q", result.Question)
	}
}

// TestPipelineSavesConceptsTaggedWithBranch extends the repo-tagging test to
// also assert that the branch column is populated by the pipeline.
func TestPipelineSavesConceptsTaggedWithBranch(t *testing.T) {
	provider := &scriptedProvider{responses: []string{
		`[{"concept":"branch-isolation-concept","source":"code","weight":1.0}]`,
	}}
	_, store, baseURL := startTestDaemonWithPipeline(t, provider, nil)

	resp := mustPOST(t, baseURL, "/hooks/pre-commit", []byte(hookBody("/repo/branch-test", "+ func branchIsolation() {}")))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	// waitForConcepts takes the legacy 3-arg (repo) signature; assert branch
	// isolation directly via store.Recent.
	ctx := context.Background()
	deadline := time.Now().Add(3 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		recent, err := store.Recent(ctx, "/repo/branch-test", "main", 10)
		if err != nil {
			t.Fatalf("Recent: %v", err)
		}
		if len(recent) >= 1 {
			found = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !found {
		t.Fatalf("timed out waiting for concept on (repo=/repo/branch-test, branch=main)")
	}

	// Cross-branch leak check: querying the same repo with a different branch
	// must return zero rows.
	wrongBranch, err := store.Recent(ctx, "/repo/branch-test", "feature-X", 10)
	if err != nil {
		t.Fatalf("Recent for wrong branch: %v", err)
	}
	if len(wrongBranch) != 0 {
		t.Fatalf("expected 0 concepts for branch=feature-X (cross-branch leak), got %d", len(wrongBranch))
	}
}

// TestRecallStaleEndpoint exercises GET /recall/stale?repo=<path>. The
// handler returns 400 on missing repo, 200 with an empty map when no stale
// questions, and 200 with per-branch counts when there are pending
// undelivered questions.
func TestRecallStaleEndpoint(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)

	// 400 when repo is missing.
	rBad := mustGET(t, baseURL, "/recall/stale")
	defer rBad.Body.Close()
	if rBad.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 with missing repo, got %d", rBad.StatusCode)
	}

	// 200 with empty branches when no pending questions.
	rEmpty := mustGET(t, baseURL, "/recall/stale?repo="+url.QueryEscape("/repo/stale-test"))
	defer rEmpty.Body.Close()
	if rEmpty.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with empty branches, got %d", rEmpty.StatusCode)
	}
	var emptyResult struct {
		Repo     string         `json:"repo"`
		Branches map[string]int `json:"branches"`
	}
	if err := json.NewDecoder(rEmpty.Body).Decode(&emptyResult); err != nil {
		t.Fatalf("decode empty: %v", err)
	}
	if emptyResult.Repo != "/repo/stale-test" {
		t.Fatalf("expected repo %q, got %q", "/repo/stale-test", emptyResult.Repo)
	}
	if len(emptyResult.Branches) != 0 {
		t.Fatalf("expected empty branches, got %v", emptyResult.Branches)
	}

	// Seed pending questions on two branches.
	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/repo/stale-test", "feature-A", "stale A1",
		[]cache.Choice{
		{Text: "x", IsCorrect: true, Position: 0},
		}, ""); err != nil {
		t.Fatalf("seed A1: %v", err)
	}
	if err := store.SaveQuestion(ctx, "/repo/stale-test", "feature-A", "stale A2",
		[]cache.Choice{
		{Text: "x", IsCorrect: true, Position: 0},
		}, ""); err != nil {
		t.Fatalf("seed A2: %v", err)
	}
	if err := store.SaveQuestion(ctx, "/repo/stale-test", "bugfix-B", "stale B1",
		[]cache.Choice{
		{Text: "x", IsCorrect: true, Position: 0},
		}, ""); err != nil {
		t.Fatalf("seed B1: %v", err)
	}

	// 200 with per-branch counts.
	rFull := mustGET(t, baseURL, "/recall/stale?repo="+url.QueryEscape("/repo/stale-test"))
	defer rFull.Body.Close()
	if rFull.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with stale counts, got %d", rFull.StatusCode)
	}
	var fullResult struct {
		Repo     string         `json:"repo"`
		Branches map[string]int `json:"branches"`
	}
	if err := json.NewDecoder(rFull.Body).Decode(&fullResult); err != nil {
		t.Fatalf("decode full: %v", err)
	}
	if fullResult.Branches["feature-A"] != 2 {
		t.Fatalf("expected feature-A count 2, got %d (all: %v)", fullResult.Branches["feature-A"], fullResult.Branches)
	}
	if fullResult.Branches["bugfix-B"] != 1 {
		t.Fatalf("expected bugfix-B count 1, got %d (all: %v)", fullResult.Branches["bugfix-B"], fullResult.Branches)
	}
	if _, ok := fullResult.Branches["main"]; ok {
		t.Fatalf("expected no entry for main (zero count omitted), got branches: %v", fullResult.Branches)
	}

	// Cross-repo isolation: a different repo must not see these counts.
	rOther := mustGET(t, baseURL, "/recall/stale?repo="+url.QueryEscape("/other/repo"))
	defer rOther.Body.Close()
	var otherResult struct {
		Branches map[string]int `json:"branches"`
	}
	if err := json.NewDecoder(rOther.Body).Decode(&otherResult); err != nil {
		t.Fatalf("decode other: %v", err)
	}
	if len(otherResult.Branches) != 0 {
		t.Fatalf("expected empty branches for unrelated repo, got %v", otherResult.Branches)
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
	cacheChoices := make([]cache.Choice, len(choices))
	for i, text := range choices {
		cacheChoices[i] = cache.Choice{
			Text:      text,
			IsCorrect: i == correctIndex,
			Position:  i,
		}
	}
	if err := store.SaveQuestion(ctx, "/repo/test", "main", question, cacheChoices, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
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

	body := fmt.Sprintf(`{"id":%d,"selected_index":0}`, id)
	resp := mustPOST(t, baseURL, "/recall/select?feedback=true", []byte(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, field := range []string{"correct", "correct_answer", "feedback"} {
		if _, ok := result[field]; !ok {
			t.Fatalf("expected %q field in response, got %v", field, result)
		}
	}
}

func TestRecallAnswerCorrectEvaluation(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	id := seedAndClaim(t, store, baseURL, "correct eval q", []string{"right", "wrong"}, 0)

	body := fmt.Sprintf(`{"id":%d,"selected_index":0}`, id)
	resp := mustPOST(t, baseURL, "/recall/select?feedback=true", []byte(body))
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

	body := fmt.Sprintf(`{"id":%d,"selected_index":1}`, id)
	resp := mustPOST(t, baseURL, "/recall/select?feedback=true", []byte(body))
	defer resp.Body.Close()

	var result struct {
		Correct     bool   `json:"correct"`
		CorrectText string `json:"correct_answer"`
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

	body := fmt.Sprintf(`{"id":%d,"selected_index":99}`, id)
	resp := mustPOST(t, baseURL, "/recall/select?feedback=true", []byte(body))
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
	if result.Error != "selected_index out of range" {
		t.Fatalf("expected error %q, got %q", "selected_index out of range", result.Error)
	}
}

func TestRecallAnswerUnknownID(t *testing.T) {
	_, _, baseURL := startTestDaemon(t)

	body := `{"id":99999,"selected_index":0}`
	resp := mustPOST(t, baseURL, "/recall/select?feedback=true", []byte(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ── 4C MCP integration tests ─────────────────────────────────────────────────

func TestMCPRecallNextReturnsCorrectIndex(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/test", "main", "mcp next q",
		[]cache.Choice{
		{Text: "a", IsCorrect: false, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		{Text: "c", IsCorrect: true, Position: 2},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
        Arguments: map[string]any{
            "repo":   "/repo/test",
            "branch": "main",
        },
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, result)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// The correctness key MUST NOT be in the delivery response — it is
	// withheld until after the user submits via recall_select (delivery leak
	// fix; see design.md Decision "wire contract — full break").
	if _, present := m["correct_index"]; present {
		t.Fatal("correct_index MUST NOT be in the delivery response (delivery leak)")
	}
	// The question_type discriminator is now delivered.
	if m["question_type"] != "multiple_choice" {
		t.Fatalf("expected question_type=multiple_choice, got %v", m["question_type"])
	}
	// The choices are still delivered (the user needs them to pick).
	choices, ok := m["choices"].([]any)
	if !ok || len(choices) != 3 {
		t.Fatalf("expected 3 choices in delivery, got %v", m["choices"])
	}
}

func TestMCPRecallAnswerReturnsCorrectness(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/test", "main", "mcp answer q",
		[]cache.Choice{
		{Text: "right", IsCorrect: true, Position: 0},
		{Text: "wrong", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)

	nextResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
        Arguments: map[string]any{
            "repo":   "/repo/test",
            "branch": "main",
        },
	})
	if err != nil {
		t.Fatalf("CallTool recall_next: %v", err)
	}
	var next map[string]any
	json.Unmarshal([]byte(toolText(t, nextResult)), &next)
	id := int64(next["id"].(float64))

	answerResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_select",
		Arguments: map[string]any{"id": id, "selected_index": 0, "skip": false},
	})
	if err != nil {
		t.Fatalf("CallTool recall_select: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, answerResult)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, field := range []string{"ok", "correct", "correct_index", "correct_answer"} {
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

	if err := store.SaveQuestion(ctx, "/repo/test", "main", "mcp skip q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)

	nextResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
        Arguments: map[string]any{
            "repo":   "/repo/test",
            "branch": "main",
        },
	})
	if err != nil {
		t.Fatalf("CallTool recall_next: %v", err)
	}
	var next map[string]any
	json.Unmarshal([]byte(toolText(t, nextResult)), &next)
	id := int64(next["id"].(float64))

	skipResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_select",
		Arguments: map[string]any{"id": id, "selected_index": 0, "skip": true},
	})
	if err != nil {
		t.Fatalf("CallTool recall_select skip: %v", err)
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
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "recent-correct",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed q1: %v", err)
	}
	q1, err := store.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("claim q1: %v", err)
	}
	if err := store.SubmitSelection(ctx, q1.ID, []int64{q1.Choices[0].ID}, "Nice!"); err != nil {
		t.Fatalf("answer q1: %v", err)
	}

	// Skip one question.
	if err := store.SaveQuestion(ctx, "/repo/test", "main", "recent-skip",
		[]cache.Choice{
		{Text: "x", IsCorrect: true, Position: 0},
		{Text: "y", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed q2: %v", err)
	}
	q2, err := store.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("claim q2: %v", err)
	}
	if err := store.SkipQuestion(ctx, q2.ID); err != nil {
		t.Fatalf("skip q2: %v", err)
	}

	session := mcpSession(t, baseURL)
	rr, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "recall://recent?repo=" + url.QueryEscape("/repo/test") + "&branch=main"})
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
	// recall://recent uses RecentAnswered — skip is EXCLUDED. The new schema
	// separates RecentAnswered (terminal answered only) from RecentSkipped.
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (skip excluded by RecentAnswered), got %d", len(rows))
	}

	row := rows[0]
	byQuestion := row["question"].(string)
	if byQuestion != "recent-correct" {
		t.Fatalf("expected recent-correct row, got %q", byQuestion)
	}
	// New wire shape: correct_index is the engine's key, selected_indices is
	// the user's pick (plural for forward-compat with multi-select).
	if row["correct_index"] == nil {
		t.Fatal("expected correct_index in recent-correct row")
	}
	selIdx, ok := row["selected_indices"].([]any)
	if !ok || len(selIdx) != 1 || selIdx[0] != float64(0) {
		t.Fatalf("expected selected_indices=[0], got %v", row["selected_indices"])
	}
	if row["feedback"] == nil {
		t.Fatal("expected feedback non-nil in recent-correct row")
	}
	if row["status"] != "answered" {
		t.Fatalf("expected status=answered, got %v", row["status"])
	}
}

// ── 10. MCP repo-scoped tests ─────────────────────────────────────────────────

// ptrStr returns a pointer to s, for populating *string fields in tool arguments.
func ptrStr(s string) *string { return &s }

// Task 10.5: recall_next with Repo field returns repo X's question and skips Y.
func TestMCPRecallNextRepoScoped(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/x", "main", "X's MCP question",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed X: %v", err)
	}
	if err := store.SaveQuestion(ctx, "/repo/y", "main", "Y's MCP question",
		[]cache.Choice{
		{Text: "c", IsCorrect: true, Position: 0},
		{Text: "d", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed Y: %v", err)
	}

	session := mcpSession(t, baseURL)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
		Arguments: map[string]any{"repo": "/repo/x", "branch": "main"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, result)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["question"] != "X's MCP question" {
		t.Fatalf("expected X's question, got %v", m["question"])
	}

	yDepth, err := store.QueueDepth(ctx, "/repo/y", "main")
	if err != nil {
		t.Fatalf("QueueDepth for Y: %v", err)
	}
	if yDepth != 1 {
		t.Fatalf("expected Y depth 1 (untouched), got %d", yDepth)
	}
}

// Task 10.6: recall_status with Repo field scopes QueueDepth.
func TestMCPRecallStatusRepoScoped(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/x", "main", "X depth q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		}, ""); err != nil {
		t.Fatalf("seed X: %v", err)
	}
	if err := store.SaveQuestion(ctx, "/repo/y", "main", "Y depth q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		}, ""); err != nil {
		t.Fatalf("seed Y: %v", err)
	}
	if err := store.SaveQuestion(ctx, "/repo/y", "main", "Y depth q2",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		}, ""); err != nil {
		t.Fatalf("seed Y2: %v", err)
	}

	session := mcpSession(t, baseURL)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_status",
		Arguments: map[string]any{"repo": "/repo/x", "branch": "main"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, result)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["queue_depth"] != float64(1) {
		t.Fatalf("expected queue_depth 1 for repo X, got %v", m["queue_depth"])
	}
}

// Task 10.7: recall_answer accepts the optional Repo field (ID-keyed, but plumbing asserted).
func TestMCPRecallAnswerAcceptsRepoField(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/x", "main", "MCP repo answer q",
		[]cache.Choice{
		{Text: "right", IsCorrect: true, Position: 0},
		{Text: "wrong", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session := mcpSession(t, baseURL)

	nextResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_next",
		Arguments: map[string]any{"repo": "/repo/x", "branch": "main"},
	})
	if err != nil {
		t.Fatalf("CallTool recall_next: %v", err)
	}
	var next map[string]any
	json.Unmarshal([]byte(toolText(t, nextResult)), &next)
	id := int64(next["id"].(float64))

	answerResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "recall_select",
		Arguments: map[string]any{"id": id, "selected_index": 0, "skip": false, "repo": "/repo/x", "branch": "main"},
	})
	if err != nil {
		t.Fatalf("CallTool recall_select with repo: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(toolText(t, answerResult)), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["ok"] != true {
		t.Fatalf("expected ok true, got %v", m["ok"])
	}
	if m["correct"] != true {
		t.Fatalf("expected correct true, got %v", m["correct"])
	}
}

// Task 10.8: recall://queue?repo=/repo/x scopes QueueDepth and PeekNextQuestion.
func TestMCPQueueResourceRepoScoped(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/x", "main", "X queue q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed X: %v", err)
	}
	if err := store.SaveQuestion(ctx, "/repo/y", "main", "Y queue q",
		[]cache.Choice{
		{Text: "c", IsCorrect: true, Position: 0},
		{Text: "d", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed Y: %v", err)
	}

	session := mcpSession(t, baseURL)
	rr, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "recall://queue?repo=" + url.QueryEscape("/repo/x") + "&branch=main"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(rr.Contents) == 0 {
		t.Fatal("expected non-empty resource contents")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rr.Contents[0].Text), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["depth"] != float64(1) {
		t.Fatalf("expected depth 1 for repo X, got %v", m["depth"])
	}
	next, ok := m["next"].(map[string]any)
	if !ok {
		t.Fatalf("expected next object, got %v", m["next"])
	}
	if next["question"] != "X queue q" {
		t.Fatalf("expected X queue q, got %v", next["question"])
	}
}

// Task 10.9: recall://recent?repo=/repo/x scopes RecentAnswered.
func TestMCPRecentResourceRepoScoped(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	// Answer one question for repo X.
	if err := store.SaveQuestion(ctx, "/repo/x", "main", "X answered q",
		[]cache.Choice{
		{Text: "a", IsCorrect: true, Position: 0},
		{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed X: %v", err)
	}
	qX, err := store.NextQuestion(ctx, "/repo/x", "main", "test")
	if err != nil {
		t.Fatalf("claim X: %v", err)
	}
	if err := store.SubmitSelection(ctx, qX.ID, []int64{qX.Choices[0].ID}, ""); err != nil {
		t.Fatalf("answer X: %v", err)
	}

	// Answer one question for repo Y.
	if err := store.SaveQuestion(ctx, "/repo/y", "main", "Y answered q",
		[]cache.Choice{
		{Text: "c", IsCorrect: true, Position: 0},
		{Text: "d", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed Y: %v", err)
	}
	qY, err := store.NextQuestion(ctx, "/repo/y", "main", "test")
	if err != nil {
		t.Fatalf("claim Y: %v", err)
	}
	if err := store.SubmitSelection(ctx, qY.ID, []int64{qY.Choices[0].ID}, ""); err != nil {
		t.Fatalf("answer Y: %v", err)
	}

	session := mcpSession(t, baseURL)
	rr, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "recall://recent?repo=" + url.QueryEscape("/repo/x") + "&branch=main"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(rr.Contents) == 0 {
		t.Fatal("expected non-empty resource contents")
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(rr.Contents[0].Text), &rows); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for repo X, got %d", len(rows))
	}
	if rows[0]["question"] != "X answered q" {
		t.Fatalf("expected X answered q, got %v", rows[0]["question"])
	}
}

func TestRepoInstallsIntoCommonGitdirInWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main-repo")
	worktreeDir := filepath.Join(tmpDir, "worktree")

	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatalf("mkdir main: %v", err)
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@test"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "initial", "-q")
	cmd.Dir = mainDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "worktree", "add", worktreeDir, "-b", "worktree-branch")
	cmd.Dir = mainDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	origDir, _ := os.Getwd()
	os.Chdir(worktreeDir)
	defer os.Chdir(origDir)

	hooksDir, err := hooks.ResolveHooksDir(worktreeDir)
	if err != nil {
		t.Fatalf("ResolveHooksDir: %v", err)
	}

	installer := hooks.NewInstaller(worktreeDir, hooksDir)
	repoCfg := config.HooksConfig{PreCommit: true}
	installed, err := installer.InstallEnabled(repoCfg)
	if err != nil {
		t.Fatalf("InstallEnabled: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 hook installed, got %d", len(installed))
	}

	mainHooksDir := filepath.Join(mainDir, ".git", "hooks")
	hookPath := filepath.Join(mainHooksDir, "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Fatalf("expected pre-commit hook in main repo's .git/hooks/ (%s), not found", hookPath)
	}

	postCommitPath := filepath.Join(hooksDir, "post-commit")
	if err := os.WriteFile(postCommitPath, []byte(buildPostCommitHookScript("/usr/local/bin/tr")), 0o755); err != nil {
		t.Fatalf("write post-commit: %v", err)
	}
	mainPostCommit := filepath.Join(mainHooksDir, "post-commit")
	if _, err := os.Stat(mainPostCommit); os.IsNotExist(err) {
		t.Fatalf("expected post-commit hook in main repo's .git/hooks/ (%s), not found", mainPostCommit)
	}
}


// ── 11. Schema-refactor specific tests ─────────────────────────────────────────

// TestRecallNextDeliveryOmitsCorrectnessKey validates the delivery leak fix:
// the GET /recall/next response MUST NOT contain the correctness key
// (correct_index) — it is withheld until the user submits. A client that
// inspects delivery JSON must not learn the answer.
func TestRecallNextDeliveryOmitsCorrectnessKey(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	ctx := context.Background()

	if err := store.SaveQuestion(ctx, "/repo/test", "main", "delivery-leak test",
		[]cache.Choice{
			{Text: "wrong-a", IsCorrect: false, Position: 0},
			{Text: "right", IsCorrect: true, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo=/repo/test&branch=main")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)

	if strings.Contains(string(body), "correct_index") {
		t.Fatalf("delivery response must not contain correct_index: %s", body)
	}
	if strings.Contains(string(body), "correct_answer") {
		t.Fatalf("delivery response must not contain correct_answer: %s", body)
	}
	_ = ctx
}

// TestSubmitSelectionAndSkipAtomicity exercises the terminal-state guards on
// SubmitSelection and SkipQuestion. Once a question is in a terminal state
// (answered or skipped), subsequent submits and skips must be rejected — no
// rows are written, the status is unchanged.
func TestSubmitSelectionAndSkipAtomicity(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "atomicity test",
		[]cache.Choice{
			{Text: "a", IsCorrect: true, Position: 0},
			{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil || q == nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}

	if err := s.SubmitSelection(ctx, q.ID, []int64{q.Choices[0].ID}, ""); err != nil {
		t.Fatalf("first submit: %v", err)
	}

	if err := s.SubmitSelection(ctx, q.ID, []int64{q.Choices[1].ID}, ""); err == nil {
		t.Fatal("expected second submit to be rejected, got nil error")
	}

	if err := s.SkipQuestion(ctx, q.ID); err == nil {
		t.Fatal("expected skip on answered question to be rejected, got nil error")
	}

	answered, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentAnswered: %v", err)
	}
	if len(answered) != 1 {
		t.Fatalf("expected 1 answered, got %d", len(answered))
	}
	if len(answered[0].Selections) != 1 {
		t.Fatalf("expected 1 selection, got %d", len(answered[0].Selections))
	}
	if answered[0].Selections[0].ChoiceID != q.Choices[0].ID {
		t.Fatal("selection should reference the first (originally chosen) choice, not the late attempt")
	}
}

// TestSkipTerminalGuardSimilarForSkipped verifies the symmetric guard: once
// skipped, a question cannot be answered afterwards.
func TestSkipTerminalGuardSimilarForSkipped(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "skip-then-answer test",
		[]cache.Choice{
			{Text: "a", IsCorrect: true, Position: 0},
			{Text: "b", IsCorrect: false, Position: 1},
		}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	q, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil || q == nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}

	if err := s.SkipQuestion(ctx, q.ID); err != nil {
		t.Fatalf("skip: %v", err)
	}

	if err := s.SubmitSelection(ctx, q.ID, []int64{q.Choices[0].ID}, ""); err == nil {
		t.Fatal("expected submit-after-skip to be rejected, got nil error")
	}

	skipped, err := s.RecentSkipped(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentSkipped: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(skipped))
	}
	if len(skipped[0].Selections) != 0 {
		t.Fatalf("expected zero selections on skipped question, got %d", len(skipped[0].Selections))
	}
}
