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
)

func startTestDaemon(t *testing.T) (*engine.Server, *cache.Store, string) {
	t.Helper()

	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

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
	if err := store.SaveQuestion(ctx, "What does DRY stand for?", []string{
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
	if err := store.SaveQuestion(ctx, "single question", []string{"a", "b"}, 0); err != nil {
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
	if err := store.SaveQuestion(ctx, "answerable question", []string{"choice a", "choice b"}, 0); err != nil {
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
	if err := store.SaveQuestion(ctx, "skippable question", []string{"x", "y"}, 0); err != nil {
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
