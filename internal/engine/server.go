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
	"syscall"
	"time"

	"github.com/colinwilliams91/total-recall/internal/config"
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
	cfg     *config.Config
	mux     *http.ServeMux
	httpSrv *http.Server
}

// New creates a Server configured with the given resolved config.
func New(cfg *config.Config) *Server {
	s := &Server{
		cfg: cfg,
		mux: http.NewServeMux(),
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
	// MCP route group — placeholder for Phase 3+
	s.mux.HandleFunc("/mcp/", s.handleMCPStub)
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
}

func (s *Server) handleMCPStub(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "MCP not yet implemented"})
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
			"cannot bind to %s — another process may be using port %s\n  hint: run 'total-recall status' to check: %w",
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
	if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("daemon error: %w", err)
	}
	log.Println("[daemon] stopped")
	return nil
}
