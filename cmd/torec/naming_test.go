package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestExecutableNamingBuildConfig guards the coreutils-collision fix: the
// project must build and release only a `torec` executable, never `tr`
// (which would shadow /usr/bin/tr on Unix). It introspects the build config.
func TestExecutableNamingBuildConfig(t *testing.T) {
	mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	if !strings.Contains(string(mk), "BINARY_NAME = torec") {
		t.Error("Makefile must set BINARY_NAME = torec")
	}
	if !strings.Contains(string(mk), "./cmd/torec") {
		t.Error("Makefile must build ./cmd/torec")
	}

	gr, err := os.ReadFile(filepath.Join("..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yaml: %v", err)
	}
	if !strings.Contains(string(gr), "binary: torec") {
		t.Error(".goreleaser.yaml must set binary: torec")
	}
	if !strings.Contains(string(gr), "main: ./cmd/torec") {
		t.Error(".goreleaser.yaml must point main at ./cmd/torec")
	}

	if _, err := os.Stat(filepath.Join("..", "tr")); runtime.GOOS != "windows" && err == nil {
		t.Error("cmd/tr must not exist — the package directory name becomes the go install binary name")
	}
}
