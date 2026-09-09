package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const pathWarningContext = "   (tr needs to be on PATH so you can run tr serve, tr repo, and tr ask from any terminal.)"

// checkTrOnPath detects whether the `tr` binary is reachable via PATH and
// prints a shell-specific warning to stderr if not. It NEVER writes to shell
// rc files — the user is expected to copy-paste the suggested command.
func checkTrOnPath() {
	if trOnPath() {
		return
	}
	shell := detectShell()
	warning := shellWarning(shell)
	fmt.Fprintln(os.Stderr, warning)
	fmt.Fprintln(os.Stderr, pathWarningContext)
}

func trOnPath() bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass",
			"-Command", "(Get-Command tr -ErrorAction SilentlyContinue).Name").Output()
		if err != nil {
			return false
		}
		return len(strings.TrimSpace(string(out))) > 0
	}
	_, err := exec.LookPath("tr")
	return err == nil
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
	prefix := fmt.Sprintf("⚠  tr not found on PATH. Add %s to PATH with: ", pathDir)
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
