package main

import (
	"strings"
	"testing"
)

func TestBuildPostCommitHookScriptIncludesPowerShellHandoff(t *testing.T) {
	script := buildPostCommitHookScript(`D:\Tools\total-recall\tr.exe`)

	if !strings.Contains(script, `powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "& 'D:\Tools\total-recall\tr.exe' ask"`) {
		t.Fatalf("expected PowerShell handoff in script, got %q", script)
	}
	if !strings.Contains(script, `exec "D:\Tools\total-recall\tr.exe" ask`) {
		t.Fatalf("expected shell exec fallback in script, got %q", script)
	}
}

func TestBuildPostCommitHookScriptEscapesPowerShellQuotes(t *testing.T) {
	script := buildPostCommitHookScript(`D:\O'Connor\tr.exe`)

	if !strings.Contains(script, `& 'D:\O''Connor\tr.exe' ask`) {
		t.Fatalf("expected PowerShell-safe path escaping, got %q", script)
	}
}

func TestPostCommitHookTemplateIncludesSentinel(t *testing.T) {
	script := buildPostCommitHookScript("/usr/local/bin/tr")

	if !strings.Contains(script, "# total-recall managed") {
		t.Fatalf("expected sentinel comment in script, got:\n%s", script)
	}
}

func TestBuildPostCommitHookScriptHandlesUnixPath(t *testing.T) {
	script := buildPostCommitHookScript("/usr/local/bin/tr")

	if !strings.Contains(script, `/usr/local/bin/tr" ask`) {
		t.Fatalf("expected Unix path in exec fallback, got:\n%s", script)
	}
}