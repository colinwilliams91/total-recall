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