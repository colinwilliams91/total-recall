package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/spf13/cobra"
)

// stopWaitDeadline bounds how long `torec stop` awaits the daemon's graceful
// drain — slightly longer than the daemon's own shutdown drain so the CLI
// observes the exit rather than giving up first.
const stopWaitDeadline = 6 * time.Second

// stopHealthURL mirrors runStatus's /health probe.
const stopHealthURL = "http://localhost:7331/health"

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Gracefully stop the running Total Recall daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop()
		},
	}
}

func runStop() error {
	pidPath, err := config.DaemonPidPath()
	if err != nil {
		return fmt.Errorf("resolving daemon pid path: %w", err)
	}
	raw, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		fmt.Println("Daemon not running (no pidfile). Start with: torec serve")
		return nil
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil || pid <= 0 {
		fmt.Printf("Stale daemon.pid (unparsable) at %s — removing, nothing signalled.\n", pidPath)
		return cleanupPidFile(pidPath)
	}

	exists, isTorec, unknown := probeDaemonProcess(pid)
	if unknown || !exists || !isTorec {
		fmt.Printf("Stale daemon.pid at %s (pid %d is not a running torec daemon) — removing, no signal sent.\n", pidPath, pid)
		return cleanupPidFile(pidPath)
	}

	if err := basicStop(pid); err != nil {
		return fmt.Errorf("signalling daemon pid %d: %w", pid, err)
	}

	if !waitDaemonGone(pid, pidPath) {
		fmt.Printf("Daemon (%d) signalled and draining — did not fully stop within the wait window; check 'torec status'.\n", pid)
		return cleanupPidFile(pidPath)
	}

	if err := cleanupPidFile(pidPath); err != nil {
		return err
	}
	fmt.Printf("✓ Daemon stopped (pid %d)\n", pid)
	return nil
}

// cleanupPidFile removes the pidfile, treating a missing file as success.
func cleanupPidFile(pidPath string) error {
	if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", pidPath, err)
	}
	return nil
}

// daemonHealthDown reports whether /health has stopped responding.
func daemonHealthDown() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(stopHealthURL)
	if err != nil {
		return true
	}
	defer resp.Body.Close()
	return false
}

// waitDaemonGone polls until /health is down AND the daemon's lifecycle
// ended — the process exited OR the daemon itself removed its pidfile at
// drain completion (the in-process daemon case cannot observe process exit).
// Bounded by stopWaitDeadline.
func waitDaemonGone(pid int, pidPath string) bool {
	deadline := time.Now().Add(stopWaitDeadline)
	for time.Now().Before(deadline) {
		if daemonHealthDown() && (!daemonAlive(pid) || daemonPidFileGone(pidPath)) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return daemonHealthDown()
}

// daemonPidFileGone reports whether the daemon has removed its pidfile.
func daemonPidFileGone(pidPath string) bool {
	_, err := os.Stat(pidPath)
	return os.IsNotExist(err)
}

// daemonAlive reports whether the pid exists — existence only, no identity.
func daemonAlive(pid int) bool {
	if runtime.GOOS == "windows" {
		line, err := taskListRow(pid)
		return err == nil && !strings.Contains(strings.ToLower(line), "no tasks")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(sigZero()) == nil
}

// probeDaemonProcess asks the OS whether the pid is alive and whether it
// identifies as this application's daemon. unknown=true means "could not be
// identified" — always treated as stale by callers, never signalled.
func probeDaemonProcess(pid int) (exists bool, isTorec bool, unknown bool) {
	if runtime.GOOS == "windows" {
		line, err := taskListRow(pid)
		if err != nil {
			return false, false, true
		}
		return !strings.Contains(strings.ToLower(line), "no tasks"),
			strings.Contains(strings.ToLower(line), "torec"), false
	}
	name, alive, unknownErr := unixProcessName(pid)
	if unknownErr {
		return false, false, true
	}
	return alive, strings.HasPrefix(name, daemonName()), false
}

// taskListRow returns the raw tasklist CSV row for a pid (Windows).
func taskListRow(pid int) (string, error) {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "csv").Output()
	return string(out), err
}

// daemonName is the process name our binary presents on Unix (executable
// base name without extension or test suffix).
func daemonName() string {
	fallback := "torec"
	if runtime.GOOS == "windows" {
		fallback = "torec.exe"
	}
	if exe, err := os.Executable(); err == nil {
		base := filepath.Base(exe)
		base = strings.ReplaceAll(base, ".test", "")
		base = strings.TrimSuffix(base, ".exe")
		if strings.TrimSpace(base) != "" {
			return base
		}
	}
	return fallback
}

// basicStop delivers the stop: SIGTERM on Unix; best-effort taskkill on Windows.
func basicStop(pid int) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", strconv.Itoa(pid)).Run()
	}
	return exec.Command("kill", "-TERM", strconv.Itoa(pid)).Run()
}

// sigZero is POSIX kill(pid, 0): existence check without a real signal.
func sigZero() syscall.Signal {
	signal := syscall.Signal(0)
	return signal
}

// unixProcessName returns the command name for the pid. Linux reads /proc;
// macOS and other Unixes fall back to ps. exists=false with unknown=false
// means "pid not present".
// unixProcessName returns the command name for the pid. Linux reads /proc
// (fast, complete); other Unixes use ps directly — on macOS /proc does not
// exist, so a failed /proc read there must not be reported as "pid absent".
func unixProcessName(pid int) (name string, exists bool, unknown bool) {
	if runtime.GOOS == "linux" {
		if b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm")); err == nil {
			return strings.TrimSpace(string(b)), true, false
		} else if os.IsNotExist(err) {
			return "", false, false
		} else {
			return "", false, true // procfs present but unreadable — indeterminate
		}
	}
	// macOS and other Unixes: ps. macOS `ps -o comm=` prints a full path,
	// so the caller receives the basename.
	out, psErr := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	name = filepath.Base(strings.TrimSpace(string(out)))
	if psErr != nil || name == "" || name == "." {
		return "", false, false // ps found nothing for this pid
	}
	return name, true, false
}

// healthyDaemonPid returns the pidfile PID when it identifies as a live
// torec daemon's PID; (0, false) when the pidfile is absent/unusable, (pid,
// false) when the pid is present but not identifiable (recycled). Status
// prints the PID only when the pair resolves.
func healthyDaemonPid() (int, bool) {
	pid, _, ok := readPid()
	if !ok || pid < 0 {
		return 0, false
	}
	exists, isTorec, unknown := probeDaemonProcess(pid)
	if unknown || !exists || !isTorec {
		return 0, false
	}
	return pid, true
}

// staleDaemonPid reports the pidfile PID when the pidfile is present but the
// PID is not a running torec daemon (dead, recycled, or unparsable) — the
// status dead-end advisory condition.
func staleDaemonPid() (int, bool) {
	pid, path, ok := readPid()
	if !ok {
		return 0, false
	}
	exists, isTorec, unknown := probeDaemonProcess(pid)
	if unknown || !exists || !isTorec {
		_ = cleanupPidFile(path)
		return pid, true
	}
	return 0, false
}

// readPid loads the pidfile contents; ok=false when absent or unreadable.
// An unparsable present pidfile yields pid=-1 with ok=true (reported stale).
func readPid() (pid int, path string, ok bool) {
	pidPath, err := config.DaemonPidPath()
	if err != nil {
		return 0, "", false
	}
	raw, readErr := os.ReadFile(pidPath)
	if readErr != nil {
		return 0, pidPath, false
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil || pid <= 0 {
		return -1, pidPath, true
	}
	return pid, pidPath, true
}
