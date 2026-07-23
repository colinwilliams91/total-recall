package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const pathWarningContext = "   (tr repo installs a post-commit hook that relies on PATH resolution at fire time.)"

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
	const prefix = "⚠  tr not found on PATH. Add $GOPATH/bin to PATH with: "
	switch s {
	case shellZsh:
		return prefix + `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc`
	case shellPowerShell:
		return prefix + `Add-Content $PROFILE 'set PATH="$PATH;$(go env GOPATH)/bin"'`
	default:
		// bash + other (fish, nushell, etc.) — bash form is the most portable.
		return prefix + `echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc`
	}
}
