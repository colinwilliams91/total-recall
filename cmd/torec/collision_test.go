package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNoTrShadowingOnPath reproduces (on Linux, where GNU coreutils is a
// standard dependency) the upstream incident documented in
// docs/handoff/Upstream-Bug-Fix-Handoff_ Linux-tr-Command-Name-Collision.md:
// with our binary directory first on PATH, `tr` must still resolve to the
// system translate utility and the executable name must not collide.
func TestNoTrShadowingOnPath(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only regression test for the coreutils tr collision")
	}
	if _, err := exec.LookPath("tr"); err != nil {
		t.Skip("coreutils tr not available in this environment")
	}

	// A stub binary dir standing in for our install dir. The guard works
	// because the real install only ever exposes `torec` — never `tr`.
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "torec"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write stub torec: %v", err)
	}

	usrBinTr := exec.Command("sh", "-c", "PATH=/usr/bin:/bin command -v tr")
	var baseline bytes.Buffer
	usrBinTr.Stdout = &baseline
	if err := usrBinTr.Run(); err != nil {
		t.Fatalf("locate system tr: %v", err)
	}
	wantTr := strings.TrimSpace(baseline.String())
	if wantTr == "" {
		t.Skip("system coreutils tr not found at /usr/bin or /bin")
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin:/bin")

	commandV := exec.Command("sh", "-c", "command -v tr")
	var out bytes.Buffer
	commandV.Stdout = &out
	if err := commandV.Run(); err != nil {
		t.Fatalf("command -v tr failed: %v", err)
	}
	if resolved := strings.TrimSpace(out.String()); resolved != wantTr {
		t.Fatalf("system tr was shadowed: `command -v tr` resolved to %q, want %q", resolved, wantTr)
	}

	translate := exec.Command("sh", "-c", "echo hello | tr '[:lower:]' '[:upper:]'")
	out.Reset()
	translate.Stdout = &out
	if err := translate.Run(); err != nil {
		t.Fatalf("system tr invocation failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "HELLO" {
		t.Fatalf("system tr misbehaved with our bin dir first on PATH: got %q", got)
	}
}
