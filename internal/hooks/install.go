// Package hooks manages installation and dispatch of Total Recall Git hook scripts.
package hooks

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/colinwilliams91/total-recall/internal/config"
)

var hookNames = []string{"pre-commit", "commit-msg", "pre-push"}

// Installer manages installation of Total Recall hook scripts into .git/hooks/.
type Installer struct {
	repoRoot string
	hooksDir string
}

// NewInstaller creates an Installer for the given repository root and hooks directory.
// hooksDir should be the resolved absolute path to the git hooks directory
// (e.g., from ResolveHooksDir).
func NewInstaller(repoRoot string, hooksDir string) *Installer {
	return &Installer{
		repoRoot: repoRoot,
		hooksDir: hooksDir,
	}
}

// ResolveHooksDir runs `git rev-parse --git-path hooks` to resolve the actual
// git hooks directory. This is correct for linked worktrees where .git is a
// pointer file rather than a directory. Relative results are absolutized via
// filepath.Join(repoRoot, hooksDir).
func ResolveHooksDir(repoRoot string) (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return "", fmt.Errorf("resolving git hooks dir: %w", err)
	}
	hooksDir := strings.TrimSpace(string(out))
	if hooksDir == "" {
		return "", fmt.Errorf("git rev-parse --git-path hooks returned empty")
	}
	if !filepath.IsAbs(hooksDir) {
		hooksDir = filepath.Join(repoRoot, hooksDir)
	}
	return hooksDir, nil
}

// FindRepoRoot runs `git rev-parse --show-toplevel` to locate the Git repository root.
// Returns an error if not inside a Git repository or if git is unavailable.
// Implemented as a var so tests can stub it without spawning git.
var FindRepoRoot = findRepoRoot

func findRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or git is not available in PATH)")
	}
	return strings.TrimSpace(string(out)), nil
}

// IsManagedHook reports whether the file at path is a Total Recall managed hook
// by checking the first 5 lines for the sentinel comment.
// Returns (false, nil) if the file does not exist.
func IsManagedHook(path string) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 5 && scanner.Scan(); i++ {
		if strings.Contains(scanner.Text(), sentinel) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// scriptBody returns the sh hook script body (no shebang/sentinel) for hookName.
func scriptBody(hookName string) string {
	switch hookName {
	case "pre-commit":
		return preCommitBody
	case "commit-msg":
		return commitMsgBody
	case "pre-push":
		return prePushBody
	default:
		return ""
	}
}

// buildShScript produces the complete .sh hook file content.
// If existingContent is non-empty, the existing hook is chained before TR dispatch.
func buildShScript(hookName, existingContent string) string {
	var b strings.Builder
	b.WriteString(hookHeader)

	if existingContent != "" {
		b.WriteString("# ─── BEGIN existing hook (chained) ──────────────────────────────────────────\n")
		b.WriteString(existingContent)
		if !strings.HasSuffix(existingContent, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("# ─── END existing hook ──────────────────────────────────────────────────────\n\n")
		b.WriteString("# Total Recall dispatch follows. Runs only if the chained hook above exits 0.\n\n")
	}

	b.WriteString(scriptBody(hookName))
	return b.String()
}

// batContent returns the .bat hook content for hookName.
func batContent(hookName string) string {
	switch hookName {
	case "pre-commit":
		return preCommitBat
	case "commit-msg":
		return commitMsgBat
	case "pre-push":
		return prePushBat
	default:
		return ""
	}
}

// Install installs (or regenerates) the named hook in .git/hooks/.
// If an existing unmanaged hook is found its content is chained before TR dispatch.
// Returns chained=true when an existing unmanaged hook was preserved.
func (inst *Installer) Install(hookName string) (chained bool, err error) {
	if err := os.MkdirAll(inst.hooksDir, 0o755); err != nil {
		return false, fmt.Errorf("creating hooks dir: %w", err)
	}

	hookPath := filepath.Join(inst.hooksDir, hookName)
	managed, err := IsManagedHook(hookPath)
	if err != nil {
		return false, fmt.Errorf("checking %s: %w", hookName, err)
	}

	var existingContent string
	if !managed {
		data, readErr := os.ReadFile(hookPath)
		if readErr == nil && len(data) > 0 {
			existingContent = string(data)
			chained = true
		}
	}

	script := buildShScript(hookName, existingContent)
	if err := os.WriteFile(hookPath, []byte(script), 0o755); err != nil {
		return false, fmt.Errorf("writing %s hook: %w", hookName, err)
	}
	return chained, nil
}

// InstallBat installs the Windows .bat variant of the named hook alongside the .sh file.
func (inst *Installer) InstallBat(hookName string) error {
	batPath := filepath.Join(inst.hooksDir, hookName+".bat")
	content := batContent(hookName)
	if content == "" {
		return nil
	}
	return os.WriteFile(batPath, []byte(content), 0o644)
}

// InstallEnabled installs hooks for each hook enabled in hooks config.
// Reports which hooks were installed and whether any existing hooks were chained.
func (inst *Installer) InstallEnabled(hooks config.HooksConfig) (installed []string, err error) {
	for _, name := range hookNames {
		var enabled bool
		switch name {
		case "pre-commit":
			enabled = hooks.PreCommit
		case "commit-msg":
			enabled = hooks.CommitMsg
		case "pre-push":
			enabled = hooks.PrePush
		}
		if !enabled {
			continue
		}

		chained, installErr := inst.Install(name)
		if installErr != nil {
			return installed, installErr
		}
		if err := inst.InstallBat(name); err != nil {
			return installed, fmt.Errorf("installing %s.bat: %w", name, err)
		}
		installed = append(installed, name)

		if chained {
			fmt.Printf("  ↪ %s: installed (chained with existing hook)\n", name)
		} else {
			fmt.Printf("  ✓ %s: installed\n", name)
		}
	}
	return installed, nil
}
