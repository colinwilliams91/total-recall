package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func captureStderrTo(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	orig := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	var buf bytes.Buffer
	restore := func() {
		w.Close()
		buf.ReadFrom(r)
		os.Stderr = orig
	}
	return &buf, restore
}

// TestCheckTrOnPath_Found creates a temp dir with a fake `tr` binary and
// prepends it to PATH. The detection should find it and produce no output.
func TestCheckTrOnPath_Found(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test: PATH-based detection")
	}

	tmpDir := t.TempDir()
	fakeTr := filepath.Join(tmpDir, "tr")
	if err := os.WriteFile(fakeTr, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tr: %v", err)
	}

	t.Setenv("PATH", tmpDir)

	buf, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	if buf.Len() != 0 {
		t.Fatalf("expected no stderr output when tr is on PATH, got %q", buf.String())
	}
}

// TestCheckTrOnPath_NotFound_Bash: SHELL=/bin/bash, PATH=empty → bash warning.
func TestCheckTrOnPath_NotFound_Bash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test: bash advisory")
	}

	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", "/usr/bin:/bin") // /usr/bin has the Unix `tr` (translate), not the Total Recall binary

	buf, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	output := buf.String()
	if !strings.Contains(output, "~/.bashrc") {
		t.Fatalf("expected bash advisory targeting ~/.bashrc, got %q", output)
	}
	if !strings.Contains(output, "tr not found on PATH") {
		t.Fatalf("expected 'tr not found on PATH' in warning, got %q", output)
	}
}

// TestCheckTrOnPath_NotFound_Zsh: SHELL=/bin/zsh → zsh warning.
func TestCheckTrOnPath_NotFound_Zsh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test: zsh advisory")
	}

	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")

	buf, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	output := buf.String()
	if !strings.Contains(output, "~/.zshrc") {
		t.Fatalf("expected zsh advisory targeting ~/.zshrc, got %q", output)
	}
}

// TestCheckTrOnPath_NotFound_DefaultShell: SHELL unset on Linux → bash fallback.
func TestCheckTrOnPath_NotFound_DefaultShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test: default shell fallback")
	}

	t.Setenv("SHELL", "")
	t.Setenv("PATH", "/usr/bin:/bin")

	buf, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	output := buf.String()
	if !strings.Contains(output, "~/.bashrc") {
		t.Fatalf("expected bash fallback advisory targeting ~/.bashrc, got %q", output)
	}
}

// TestCheckTrOnPath_NotFound_OtherShell: SHELL=/usr/bin/fish → falls back to bash form.
func TestCheckTrOnPath_NotFound_OtherShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test: other-shell fallback")
	}

	t.Setenv("SHELL", "/usr/bin/fish")
	t.Setenv("PATH", "/usr/bin:/bin")

	buf, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	output := buf.String()
	// Unrecognized shells fall back to the bash form (most portable).
	if !strings.Contains(output, "~/.bashrc") {
		t.Fatalf("expected bash-form fallback for fish shell, got %q", output)
	}
}

// TestCheckTrOnPath_NotFound_PowerShell: Windows PATH empty → PowerShell advisory.
func TestCheckTrOnPath_NotFound_PowerShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test: PowerShell advisory")
	}

	// Set PATH to a dir with no `tr.exe` to force the not-found path.
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	buf, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	output := buf.String()
	if !strings.Contains(output, "$PROFILE") {
		t.Fatalf("expected PowerShell advisory targeting $PROFILE, got %q", output)
	}
}

// TestCheckTrOnPath_NeverWrites: ensures the function never writes to rc files
// even when PATH detection fails. The HOME env var is redirected to a temp dir
// and we verify no .bashrc/.zshrc/etc. files are created.
func TestCheckTrOnPath_NeverWrites(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only test: rc-file check")
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", "/usr/bin:/bin")

	_, restore := captureStderrTo(t)
	checkTrOnPath()
	restore()

	// List the contents of tmpHome; nothing should have been created.
	entries, err := os.ReadDir(tmpHome)
	if err != nil {
		t.Fatalf("read tmpHome: %v", err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected no files written to HOME, found: %v", names)
	}
}

// TestDetectShell exercises the shell-detection heuristic in isolation.
// Cross-platform: tests the in-process function, not external process invocation.
func TestDetectShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		got := detectShell()
		if got != shellPowerShell {
			t.Fatalf("Windows should always detect PowerShell, got %v", got)
		}
		return
	}

	cases := []struct {
		shellEnv string
		want     shellKind
	}{
		{"/bin/bash", shellBash},
		{"/usr/local/bin/bash", shellBash},
		{"/bin/zsh", shellZsh},
		{"/usr/local/bin/zsh", shellZsh},
		{"/usr/bin/fish", shellOther},
		{"", shellBash}, // empty SHELL on Linux defaults to bash
	}
	for _, tc := range cases {
		t.Setenv("SHELL", tc.shellEnv)
		got := detectShell()
		if got != tc.want {
			t.Errorf("SHELL=%q: got %v, want %v", tc.shellEnv, got, tc.want)
		}
	}
}

// TestShellWarning verifies the advisory strings for each shell kind.
func TestShellWarning(t *testing.T) {
	cases := []struct {
		kind     shellKind
		contains []string
	}{
		{shellBash, []string{"~/.bashrc", "export PATH"}},
		{shellZsh, []string{"~/.zshrc", "export PATH"}},
		{shellPowerShell, []string{"$PROFILE", "PATH"}},
		{shellOther, []string{"~/.bashrc"}}, // fallback to bash form
	}
	for _, tc := range cases {
		w := shellWarning(tc.kind)
		for _, s := range tc.contains {
			if !strings.Contains(w, s) {
				t.Errorf("kind=%d: warning %q missing %q", tc.kind, w, s)
			}
		}
	}
}
