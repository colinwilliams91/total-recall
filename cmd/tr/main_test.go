package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/hooks"
)

func TestPostCommitHookScriptContainsSentinel(t *testing.T) {
	script := buildPostCommitHookScript("/usr/local/bin/tr")
	if !strings.Contains(script, "# total-recall managed") {
		t.Fatalf("expected sentinel comment in script, got:\n%s", script)
	}
}

func TestPostCommitHookScriptContainsPowerShellHandoff(t *testing.T) {
	script := buildPostCommitHookScript(`C:\Users\colin\go\bin\tr.exe`)
	if !strings.Contains(script, `powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "& 'C:\Users\colin\go\bin\tr.exe' ask"`) {
		t.Fatalf("expected PowerShell handoff with baked path in script, got:\n%s", script)
	}
}

func TestPostCommitHookScriptContainsExecFallback(t *testing.T) {
	script := buildPostCommitHookScript(`C:\Users\colin\go\bin\tr.exe`)
	if !strings.Contains(script, `exec "C:/Users/colin/go/bin/tr.exe" ask`) {
		t.Fatalf("expected exec with forward-slash baked path in script, got:\n%s", script)
	}
}

func TestPostCommitHookScriptBakesPathWithForwardSlashesForSh(t *testing.T) {
	script := buildPostCommitHookScript(`C:\Users\colin\go\bin\tr.exe`)
	// The sh fallback must receive the path with forward slashes so MSYS sh
	// (the shell Git uses to run hooks on Windows) can exec the .exe directly.
	if !strings.Contains(script, `exec "C:/Users/colin/go/bin/tr.exe" ask`) {
		t.Fatalf("expected forward-slash path in sh fallback, got:\n%s", script)
	}
	// The PowerShell branch must keep the native backslash path (PowerShell
	// accepts both, but the path came from os.Executable() verbatim).
	if !strings.Contains(script, `'C:\Users\colin\go\bin\tr.exe'`) {
		t.Fatalf("expected backslash path in PowerShell branch, got:\n%s", script)
	}
}

func TestPostCommitHookScriptTemplateHasPercentSPlaceholders(t *testing.T) {
	// Guard against someone reverting to PATH-based lookup (the bare `exec tr
	// ask` form). The template MUST contain two %%s placeholders — one for the
	// PowerShell branch, one for the sh fallback — so the baked binary path is
	// always injected at tr repo time. See buildPostCommitHookScript docs.
	if strings.Count(postCommitHookScriptTmpl, "%s") != 2 {
		t.Fatalf("expected exactly 2 %%s placeholders in template, got:\n%s", postCommitHookScriptTmpl)
	}
}

func TestPostCommitHookScriptReferencesTRRepo(t *testing.T) {
	script := buildPostCommitHookScript("/usr/local/bin/tr")
	if !strings.Contains(script, "tr repo") {
		t.Fatalf("expected 'tr repo' reference in generated-by comment, got:\n%s", script)
	}
}

func TestRunRepoOutsideGitFailsWithExactMessage(t *testing.T) {
	origFindRepoRoot := hooks.FindRepoRoot
	hooks.FindRepoRoot = func() (string, error) {
		return "", os.ErrNotExist
	}
	defer func() { hooks.FindRepoRoot = origFindRepoRoot }()

	origOsExit := osExit
	var exitCode int
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = origOsExit }()

	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runRepo()

	w.Close()
	buf.ReadFrom(r)
	os.Stdout = origStdout

	output := buf.String()
	expected := "Total Recall only works with git projects. cd into a project and run tr repo."
	if !strings.Contains(output, expected) {
		t.Fatalf("expected exact message %q in output, got %q", expected, output)
	}
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

func TestResolveHooksDirAbsolutizesRelative(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	hooksDir, err := hooks.ResolveHooksDir(repoDir)
	if err != nil {
		t.Fatalf("ResolveHooksDir: %v", err)
	}

	if !filepath.IsAbs(hooksDir) {
		t.Fatalf("expected absolute path, got %q", hooksDir)
	}
	normalized := filepath.ToSlash(hooksDir)
	if !strings.HasSuffix(normalized, ".git/hooks") {
		t.Fatalf("expected path ending in .git/hooks, got %q", hooksDir)
	}
}

func TestResolveHooksDirWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	mainDir := filepath.Join(tmpDir, "main-repo")
	worktreeDir := filepath.Join(tmpDir, "worktree")

	// Resolve symlinks so git invocations agree on a canonical path
	// (macOS: /var -> /private/var; Windows: 8.3 short names).
	if resolved, err := filepath.EvalSymlinks(tmpDir); err == nil {
		tmpDir = resolved
		mainDir = filepath.Join(tmpDir, "main-repo")
		worktreeDir = filepath.Join(tmpDir, "worktree")
	}

	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatalf("mkdir main: %v", err)
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@test"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "initial", "-q")
	cmd.Dir = mainDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "worktree", "add", worktreeDir, "-b", "worktree-branch")
	cmd.Dir = mainDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	origDir, _ := os.Getwd()
	os.Chdir(worktreeDir)
	defer os.Chdir(origDir)

	hooksDir, err := hooks.ResolveHooksDir(worktreeDir)
	if err != nil {
		t.Fatalf("ResolveHooksDir: %v", err)
	}

	if !filepath.IsAbs(hooksDir) {
		t.Fatalf("expected absolute path, got %q", hooksDir)
	}

	// Compare against git's own output from the main repo, so both paths are
	// in canonical form (avoids macOS /var -> /private/var symlink mismatches
	// and Windows 8.3 short-name mismatches like RUNNER~1 vs runneradmin).
	mainCmd := exec.Command("git", "rev-parse", "--git-path", "hooks")
	mainCmd.Dir = mainDir
	mainOut, err := mainCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse from main: %v", err)
	}
	mainHooksDir := strings.TrimSpace(string(mainOut))
	if !filepath.IsAbs(mainHooksDir) {
		mainHooksDir = filepath.Join(mainDir, mainHooksDir)
	}
	normalizedMainHooks := filepath.ToSlash(mainHooksDir)
	normalizedHooks := filepath.ToSlash(hooksDir)
	if normalizedHooks != normalizedMainHooks {
		t.Fatalf("expected hooks dir %q, got %q", normalizedMainHooks, normalizedHooks)
	}
}
