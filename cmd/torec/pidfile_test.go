package main

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/engine"
)

// TestPidfileLifecycle exercises the pidfile write (after successful bind)
// and removal (on graceful shutdown — the same cleanup path SIGINT/SIGTERM
// take, since Shutdown drives the same drain).
func TestPidfileLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a live bind on the daemon port")
	}

	_, restoreLogs := captureStderrTo(t)
	t.Setenv("TR_HOME", t.TempDir())

	store, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer store.Close()

	userCfg := config.DefaultUserConfig()
	cfg := config.Merge(&userCfg, nil)
	srv := engine.New(cfg, nil, store, nil)

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()

	pidPath, err := config.DaemonPidPath()
	if err != nil {
		t.Fatalf("DaemonPidPath: %v", err)
	}

	// pidfile appears after bind (bounded wait for the daemon to come up).
	var pidFileBody string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		b, readErr := os.ReadFile(pidPath)
		if readErr == nil {
			pidFileBody = string(b)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pidFileBody == "" {
		t.Fatalf("pidfile %s was not written after daemon start", pidPath)
	}
	written, err := strconv.Atoi(pidFileBody)
	if err != nil {
		t.Fatalf("pidfile content %q is not an int: %v", pidFileBody, err)
	}
	if written != os.Getpid() {
		t.Fatalf("pidfile PID %d != test process PID %d", written, os.Getpid())
	}

	// Graceful shutdown (drain) must remove the pidfile.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("daemon did not finish shutdown in time")
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(pidPath); os.IsNotExist(statErr) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Fatalf("pidfile %s still exists after graceful shutdown", pidPath)
	}
	restoreLogs() // keep daemon logs out of test output
}
