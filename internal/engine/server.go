package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/pipeline"
	"github.com/colinwilliams91/total-recall/internal/presentation/terminal"
	"github.com/colinwilliams91/total-recall/internal/recall"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	daemonPort      = "7331"
	daemonAddr      = "localhost:" + daemonPort
	shutdownTimeout = 5 * time.Second
)

// HookResponse is the typed response body for all hook POST endpoints.
// In Phase 2, Recall is always nil. Phase 3 populates it with a recall question.
type HookResponse struct {
	Status string        `json:"status"`
	Recall *RecallPrompt `json:"recall,omitempty"`
}

// RecallPrompt carries the synthesized recall question delivered to hook clients.
// Defined here so Phase 3 can populate it without a breaking wire-format change.
type RecallPrompt struct {
	Question string   `json:"question"`
	Choices  []string `json:"choices,omitempty"`
}

// HookEnvelope is the JSON body expected in all hook POST requests.
type HookEnvelope struct {
	Hook      string          `json:"hook"`
	Repo      string          `json:"repo"`
	Branch    string          `json:"branch"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// HealthResponse is the JSON body for GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// Server is the Total Recall daemon HTTP server.
type Server struct {
	cfg          *config.Config
	provider     ai.Provider  // nil when AI is not configured
	store        *cache.Store // nil when AI is not configured
	recallEngine *recall.Engine
	dispatcher   Dispatcher
	mcpServer    *mcp.Server
	mux          *http.ServeMux
	httpSrv      *http.Server
	wg           sync.WaitGroup // tracks in-flight pipeline goroutines
}

// New creates a Server configured with the given resolved config and dependencies.
// provider, store, and recallEngine may be nil if AI is not configured — in that
// case hooks are still acknowledged but no recall questions are generated.
func New(cfg *config.Config, provider ai.Provider, store *cache.Store, recallEngine *recall.Engine) *Server {
	s := &Server{
		cfg:          cfg,
		provider:     provider,
		store:        store,
		recallEngine: recallEngine,
		mcpServer:    buildMCPServer(store, cfg),
		mux:          http.NewServeMux(),
	}
	if cfg.Presentation.Terminal {
		s.dispatcher = terminal.New()
	}
	s.httpSrv = &http.Server{
		Addr:    daemonAddr,
		Handler: s.mux,
	}
	s.RegisterRoutes()
	return s
}

// RegisterRoutes registers all HTTP routes on the server mux.
func (s *Server) RegisterRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /hooks/pre-commit", s.handleHook)
	s.mux.HandleFunc("POST /hooks/commit-msg", s.handleHook)
	s.mux.HandleFunc("POST /hooks/pre-push", s.handleHook)
	s.mux.HandleFunc("GET /recall/next", s.handleRecallNext)
	s.mux.HandleFunc("POST /recall/answer", s.handleRecallAnswer)
	s.mux.HandleFunc("GET /recall/stale", s.handleRecallStale)
	s.mux.Handle("/mcp/", mcpHandler(s.mcpServer))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
	var env HookEnvelope
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	log.Printf("[hook] %s  repo=%s  branch=%s", env.Hook, env.Repo, env.Branch)
	writeJSON(w, http.StatusAccepted, HookResponse{Status: "received"})

	if s.provider != nil {
		s.wg.Add(1)
		go s.runPipeline(env)
	}
}

// runPipeline extracts concepts from the hook payload, saves them to the cache,
// synthesizes a recall question, and dispatches it — all in the background.
func (s *Server) runPipeline(env HookEnvelope) {
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Extract the diff from the payload (pre-commit hook sends it as a string).
	var payload struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil || payload.Diff == "" {
		log.Printf("[pipeline] no diff in hook payload for %s — skipping", env.Hook)
		return
	}

	// Detached HEAD / non-git context: no branch means we can't scope concepts
	// to active work. Skip silently — the store layer also refuses empty
	// values, but logging here gives maintainers a clearer trail.
	if env.Branch == "" {
		log.Printf("[pipeline] skipping insert: detached HEAD (no branch)")
		return
	}

	concepts, err := pipeline.ExtractConcepts(ctx, s.provider, payload.Diff, s.cfg.AI.Model)
	if err != nil {
		log.Printf("[pipeline] extract error: %v", err)
		return
	}
	if len(concepts) == 0 {
		log.Printf("[pipeline] no concepts extracted for repo=%s branch=%s", env.Repo, env.Branch)
		return
	}

	fingerprints := make([]cache.Fingerprint, len(concepts))
	for i, c := range concepts {
		fingerprints[i] = cache.Fingerprint{
			Concept: c.Concept,
			Source:  c.Source,
			Weight:  c.Weight,
		}
	}
	if err := s.store.Save(ctx, env.Repo, env.Branch, fingerprints); err != nil {
		log.Printf("[pipeline] cache save error: %v", err)
	}

	if s.recallEngine == nil {
		return
	}
	q, err := s.recallEngine.Synthesize(ctx, env.Repo, env.Branch, "", s.cfg.AI.Model)
	if err != nil {
		log.Printf("[recall] synthesize error: %v", err)
		return
	}
	if q == nil {
		return
	}

	if err := s.store.SaveQuestion(ctx, env.Repo, env.Branch, q.Question, q.Choices, q.CorrectIndex); err != nil {
		log.Printf("[pipeline] save question: %v", err)
	}

	if err := s.mcpServer.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: "recall://queue"}); err != nil {
		log.Printf("[mcp] notify resource updated: %v", err)
	}

	if s.dispatcher != nil {
		if err := s.dispatcher.Dispatch(recall.Question{
			Question:     q.Question,
			Choices:      q.Choices,
			CorrectIndex: q.CorrectIndex,
		}); err != nil {
			log.Printf("[recall] dispatch error: %v", err)
		}
	}
}

func (s *Server) handleRecallNext(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	branch := r.URL.Query().Get("branch")
	if repo == "" || branch == "" {
		log.Printf("[recall] next called with empty repo or branch — client is outside git or detached HEAD")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	q, err := s.store.NextQuestion(r.Context(), repo, branch, "shell")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if q == nil {
		log.Printf("[recall] no pending questions for repo=%q branch=%q", repo, branch)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       q.ID,
		"question": q.Question,
		"choices":  q.Choices,
	})
}

func (s *Server) handleRecallAnswer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID          int64 `json:"id"`
		AnswerIndex *int  `json:"answer_index"`
		Skip        bool  `json:"skip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Skip {
		if err := s.store.SkipQuestion(r.Context(), body.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if body.AnswerIndex == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "answer_index is required"})
		return
	}
	q, err := s.store.GetQuestion(r.Context(), body.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if q == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "question not found"})
		return
	}
	if *body.AnswerIndex < 0 || *body.AnswerIndex >= len(q.Choices) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "answer_index out of range"})
		return
	}
	correct := *body.AnswerIndex == q.CorrectIndex
	answerText := q.Choices[*body.AnswerIndex]
	correctText := q.Choices[q.CorrectIndex]

	feedback := ""
	if r.URL.Query().Get("feedback") == "true" && s.recallEngine != nil {
		feedback = s.recallEngine.GenerateFeedback(r.Context(), q.Question, q.Choices, q.CorrectIndex, *body.AnswerIndex, s.cfg.AI.Model)
	}

	if err := s.store.AnswerQuestion(r.Context(), body.ID, *body.AnswerIndex, answerText, correct, feedback); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var feedbackOut any = nil
	if feedback != "" {
		feedbackOut = feedback
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"correct":      correct,
		"correct_text": correctText,
		"feedback":     feedbackOut,
	})
}

// handleRecallStale returns per-branch counts of undelivered questions for the
// given repo. It backs the `tr status` advisory: the user is told which branch
// has pending questions so they can switch back and answer them.
//
// Query: ?repo=<path> (required).
// Response (200): {"repo":"<path>","branches":{"<branch>":<count>, ...}}.
// Branches with zero undelivered questions are omitted.
func (s *Server) handleRecallStale(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repo query parameter is required"})
		return
	}

	branches, err := s.store.StalePerBranch(r.Context(), repo)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repo":     repo,
		"branches": branches,
	})
}
// After Shutdown returns, Start() will return http.ErrServerClosed.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

// Serve starts the HTTP server on the given listener without signal handling.
// Useful for testing. For production use, call Start() instead.
func (s *Server) Serve(l net.Listener) error {
	return s.httpSrv.Serve(l)
}

// writeJSON sets Content-Type and encodes v as JSON.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Start binds to the daemon address and blocks until graceful shutdown.
// It listens for SIGTERM/SIGINT and drains in-flight requests before stopping.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", daemonAddr)
	if err != nil {
		return fmt.Errorf(
			"cannot bind to %s — another process may be using port %s\n  hint: run 'tr status' to check: %w",
			daemonAddr, daemonPort, err,
		)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("[daemon] shutdown signal received, draining in-flight requests...")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			log.Printf("[daemon] shutdown error: %v", err)
		}
	}()

	fmt.Printf("✓ Total Recall daemon running on %s\n", daemonAddr)

	// Warn if the configured API key is an env-var reference that resolved to nothing.
	// This happens when the user sets a User-scoped env var but launches the daemon in
	// a terminal session that predates the assignment — the process never inherited it.
	if key := s.cfg.AI.APIKey; len(key) > 4 && key[:4] == "env:" {
		if resolved, _ := s.cfg.AI.ResolvedAPIKey(); resolved == "" {
			log.Printf("[warn] AI provider key %q resolved to empty — AI features will be disabled.", key)
			log.Printf("[warn] Set the variable before starting the daemon, or open a new terminal after setting it.")
			log.Printf("[warn] To set for this session only: $env:%s = \"your-key\"  (PowerShell) / export %s=your-key  (bash)", key[4:], key[4:])
		}
	}

	if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("daemon error: %w", err)
	}
	s.wg.Wait()
	log.Println("[daemon] stopped")
	return nil
}
