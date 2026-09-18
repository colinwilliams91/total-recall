package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const pathWarningContext = "   (torec needs to be on PATH so you can run torec serve, torec repo, and torec ask from any terminal.)"

// checkTorecOnPath detects whether the `torec` binary is reachable via PATH and
// prints a shell-specific warning to stderr if not. On Unix it additionally
// inspects any `tr` on PATH: if it identifies itself as a stale Total Recall
// binary it warns the user to delete it, because it shadows the coreutils
// translate utility of the same name. It NEVER writes to shell rc files —
// the user is expected to copy-paste the suggested command.
func checkTorecOnPath() {
	if !torecOnPath() {
		shell := detectShell()
		warning := shellWarning(shell)
		fmt.Fprintln(os.Stderr, warning)
		fmt.Fprintln(os.Stderr, pathWarningContext)
	}
	if stale := staleTrBinary(); stale {
		fmt.Fprintln(os.Stderr, "⚠  stale legacy 'tr' binary of this app found on PATH — it shadows the Unix translate utility and breaks other programs. Delete it (e.g. remove the file reported by 'command -v tr').")
	}
}

func torecOnPath() bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass",
			"-Command", "(Get-Command torec -ErrorAction SilentlyContinue).Name").Output()
		if err != nil {
			return false
		}
		return len(strings.TrimSpace(string(out))) > 0
	}
	_, err := exec.LookPath("torec")
	return err == nil
}

// staleTrBinary reports whether a `tr` binary on PATH is a stale install of
// this application (identified by its --version output). Unrelated `tr`
// programs — coreutils included — are never flagged. Unix only (Windows has
// no coreutils `tr`). Classify errors (missing, timeout) as "not ours".
func staleTrBinary() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	bin, err := exec.LookPath("tr")
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return false
	}
	return isRecallVersionOutput(string(out))
}

// isRecallVersionOutput matches this project's `--version` output format
// ("tr version dev" / "tr version <semver>"), which coreutils never prints
// (its format is "tr (GNU coreutils) X.Y", and it rejects --version flags
// translating text instead). The match is case-sensitive on the exact prefix.
func isRecallVersionOutput(s string) bool {
	fields := strings.Fields(s)
	return len(fields) >= 3 && fields[0] == "tr" && fields[1] == "version"
}

type shellKind int

const (
	shellBash shellKind = iota
	shellZsh
	shellPowerShell
	shellOther
)

func detectShell() shellKind {
	if runtime.GOOS == "windows" {
		return shellPowerShell
	}
	sh := os.Getenv("SHELL")
	switch {
	case strings.Contains(sh, "zsh"):
		return shellZsh
	case strings.Contains(sh, "bash"), sh == "":
		return shellBash
	default:
		return shellOther
	}
}

func shellWarning(s shellKind) string {
	pathDir := goInstallDir()
	const fallbackPathDir = "$(go env GOPATH)/bin"
	if pathDir == "" {
		pathDir = fallbackPathDir
	}
	prefix := fmt.Sprintf("⚠  torec not found on PATH. Add %s to PATH with: ", pathDir)
	switch s {
	case shellZsh:
		return prefix + fmt.Sprintf(`echo 'export PATH="$PATH:%s"' >> ~/.zshrc`, pathDir)
	case shellPowerShell:
		return prefix + fmt.Sprintf(`Add-Content $PROFILE '$env:Path = "$env:Path;%s"'`, pathDir)
	default:
		// bash + other (fish, nushell, etc.) — bash form is the most portable.
		return prefix + fmt.Sprintf(`echo 'export PATH="$PATH:%s"' >> ~/.bashrc`, pathDir)
	}
}

// goInstallDir resolves the directory where go install places binaries.
// GOBIN takes precedence; otherwise Go uses the first GOPATH entry's bin directory.
func goInstallDir() string {
	if dir := strings.TrimSpace(os.Getenv("GOBIN")); dir != "" {
		return dir
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		return ""
	}

	if output, err := exec.Command(goPath, "env", "GOBIN").Output(); err == nil {
		if dir := strings.TrimSpace(string(output)); dir != "" {
			return dir
		}
	}
	if output, err := exec.Command(goPath, "env", "GOPATH").Output(); err == nil {
		if paths := filepath.SplitList(strings.TrimSpace(string(output))); len(paths) > 0 && paths[0] != "" {
			return filepath.Join(paths[0], "bin")
		}
	}
	return ""
}
