package main

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/engine"
)

func writePidForTest(t *testing.T, pid int) string {
	t.Helper()
	t.Setenv("TR_HOME", t.TempDir())
	path, err := config.DaemonPidPath()
	if err != nil {
		t.Fatalf("DaemonPidPath: %v", err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}
	return path
}

func pidFileGone(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func readPidFile(t *testing.T) (string, string) {
	t.Helper()
	path, err := config.DaemonPidPath()
	if err != nil {
		t.Fatalf("DaemonPidPath: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pidfile: %v", err)
	}
	return path, strings.TrimSpace(string(b))
}

// TestStopIdentityProbeTable exercises the identity probe directly: the test
// process itself (a torec test binary) must identify as this app's daemon.
func TestStopIdentityProbeTable(t *testing.T) {
	exists, isTorec, unknown := probeDaemonProcess(os.Getpid())
	if unknown || !exists || !isTorec {
		t.Fatalf("expected own process to exist and identify as the daemon, got exists=%t isTorec=%t unknown=%t", exists, isTorec, unknown)
	}

	// A process that exited long ago: exists=false, no signal implication.
	exists, isTorec, unknown = probeDaemonProcess(999_999_999)
	if unknown {
		t.Fatalf("missing pid should not be unknown on unix (deterministic absence)")
	}
	if exists || isTorec {
		t.Fatalf("expected absent pid to report !exists, got exists=%t isTorec=%t", exists, isTorec)
	}
}

// TestStopNoPidfile: stopping with no pidfile is an advisory + exit 0.
func TestStopNoPidfile(t *testing.T) {
	t.Setenv("TR_HOME", t.TempDir())
	if err := runStop(); err != nil {
		t.Fatalf("expected nil error (exit 0), got %v", err)
	}
}

// TestStopUnparsablePidfile: unparsable pidfile is advisory, cleaned, exit 0.
func TestStopUnparsablePidfile(t *testing.T) {
	t.Setenv("TR_HOME", t.TempDir())
	path, err := config.DaemonPidPath()
	if err != nil {
		t.Fatalf("DaemonPidPath: %v", err)
	}
	if err := os.WriteFile(path, []byte("not-a-pid"), 0o600); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}
	if err := runStop(); err != nil {
		t.Fatalf("expected nil error for unparsable pidfile, got %v", err)
	}
	if !pidFileGone(t, path) {
		t.Fatal("stale unparsable pidfile should have been removed")
	}
}

// TestStopStaleDeadPid: a pid that no longer exists is advisory + cleanup, no signal.
func TestStopStaleDeadPid(t *testing.T) {
	path := writePidForTest(t, 999_999_999)
	if err := runStop(); err != nil {
		t.Fatalf("expected nil for stale dead pid, got %v", err)
	}
	if !pidFileGone(t, path) {
		t.Fatal("stale dead-pid pidfile should have been removed")
	}
}

// TestStopPidRecycled: a pid belonging to a non-torec process must NOT be
// signalled; the unrelated process survives; pidfile removed; exit 0.
func TestStopPidRecycled(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep binary not available")
	}
	cmd := exec.Command("sleep", "3")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	path := writePidForTest(t, cmd.Process.Pid)
	if err := runStop(); err != nil {
		t.Fatalf("expected nil for recycled-pid pidfile, got %v", err)
	}
	if !pidFileGone(t, path) {
		t.Fatal("stale mismatched pidfile should have been removed")
	}
}

// TestStopHealthyDaemon drives the full stop flow against a live in-process
// daemon: pidfile written at bind, signal → drain → pidfile removed.
func TestStopHealthyDaemon(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a live bind on the daemon port")
	}
	t.Setenv("TR_HOME", t.TempDir())

	store, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer func() { store.Close() }()

	userCfg := config.DefaultUserConfig()
	cfg := config.Merge(&userCfg, nil)
	srv := engine.New(cfg, nil, store, nil)

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()

	pidPath, pid, err := readPidFileWait(t, 5*time.Second)
	if err != nil {
		t.Fatalf("pidfile never appeared: %v", err)
	}
	if pid <= 0 || pid != os.Getpid() {
		t.Fatalf("unexpected pidfile content %d", pid)
	}

	err = func() error {
		_, restore := captureStderrTo(t)
		defer restore()
		return runStop()
	}()
	if err != nil {
		t.Fatalf("runStop: %v", err)
	}

	select {
	case derr := <-done:
		if derr != nil {
			t.Fatalf("daemon exited with: %v", derr)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("daemon did not exit after stop")
	}
	if !pidFileGone(t, pidPath) {
		t.Fatal("pidfile not removed after successful stop")
	}
}

func readPidFileWait(t *testing.T, timeout time.Duration) (string, int, error) {
	t.Helper()
	path, err := config.DaemonPidPath()
	if err != nil {
		return "", 0, err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			return path, pid, err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return path, -1, err
}

// TestStopDrainsInFlight is task 2.4: while a hook POST is mid-flight, the
// stop signal arrives; the daemon completes the in-flight request within the
// drain window instead of dropping the connection, then clears the pidfile.
func TestStopDrainsInFlight(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a live bind on the daemon port")
	}
	t.Setenv("TR_HOME", t.TempDir())

	store, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer func() { store.Close() }()

	userCfg := config.DefaultUserConfig()
	cfg := config.Merge(&userCfg, nil)
	srv := engine.New(cfg, nil, store, nil)

	done := make(chan error, 1)
	go func() { done <- srv.Start() }()

	pidPath, _, err := readPidFileWait(t, 5*time.Second)
	if err != nil {
		t.Fatalf("pidfile never appeared: %v", err)
	}

	baseURL := "http://localhost:7331"
	type posted struct {
		code int
		err  error
	}
	results := make(chan posted, 1)
	go func() {
		resp, err := http.Post(baseURL+"/hooks/pre-commit", "application/json",
			strings.NewReader(`{"hook":"pre-commit","repo":"/x","branch":"b","payload":{}}`))
		if err != nil {
			results <- posted{-1, err}
			return
		}
		defer resp.Body.Close()
		results <- posted{resp.StatusCode, nil}
	}()

	time.Sleep(200 * time.Millisecond) // let the POST land in-flight

	if err := func() error {
		_, restore := captureStderrTo(t)
		defer restore()
		return runStop()
	}(); err != nil {
		t.Fatalf("runStop: %v", err)
	}

	res := <-results
	if res.err != nil {
		t.Fatalf("in-flight request was dropped: %v", res.err)
	}
	if res.code != http.StatusAccepted {
		t.Fatalf("in-flight hook request got %d, want 202", res.code)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("daemon exit: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("daemon did not exit")
	}

	if !pidFileGone(t, pidPath) {
		t.Fatal("pidfile survived stop")
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	if _, err := client.Get("http://localhost:7331/health"); err == nil {
		t.Fatal("/health still answers after stop")
	}
}

// TestStatusPidfileSurfacing exercises the pidfile-aware status branches
// (3.1): healthy + pidfile → "(pid N)"; dead + stale pidfile → stale note;
// dead with no pidfile → unchanged failure with exit-1 semantics preserved.
func TestStatusPidfileSurfacing(t *testing.T) {
	if testing.Short() {
		t.Skip("drives the real 7331 endpoint contract")
	}
	t.Setenv("TR_HOME", t.TempDir())
	pidPath, _ := config.DaemonPidPath()

	// The exit stub panics (the real os.Exit never returns; runStatus relies
	// on that) and captureStatus wraps recovery so both cases are observable.
	type exitSignal struct{ code int }
	origOsExit := osExit
	runStatusCaptured := func() (output string, exit int) {
		defer func() {
			if r := recover(); r != nil {
				if sig, isSig := r.(exitSignal); isSig {
					exit = sig.code
				} else {
					panic(r)
				}
			}
		}()
		var buf bytes.Buffer
		origStdout := os.Stdout
		rPipe, wPipe, _ := os.Pipe()
		os.Stdout = wPipe
		defer func() {
			wPipe.Close()
			buf.ReadFrom(rPipe)
			os.Stdout = origStdout
			output = buf.String()
		}()
		osExit = func(code int) { panic(exitSignal{code}) }
		runStatus()
		return "", 0
	}
	defer func() { osExit = origOsExit }()

	// Case 1: no daemon and no pidfile → unchanged failure, exit 1, no stale note.
	output, exit1 := runStatusCaptured()
	if exit1 != 1 {
		t.Fatalf("case1: expected exit 1 with daemon down, got %d", exit1)
	}
	if strings.Contains(output, "stale daemon.pid") {
		t.Fatalf("case1: unexpected stale note: %s", output)
	}

	// Case 2: daemon down + stale pidfile → failure + stale note (pidfile cleaned).
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(999_999_999)), 0o600); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}
	output, exit2 := runStatusCaptured()
	if exit2 != 1 {
		t.Fatalf("case2: expected exit 1, got %d", exit2)
	}
	if !strings.Contains(output, "stale daemon.pid") {
		t.Fatalf("case2: expected stale note, got: %s", output)
	}
}
