package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colinwilliams91/total-recall/assets"
	"github.com/spf13/cobra"
)

// runAssetCmd invokes a command's RunE with stdout captured, applying the
// given flags first. Returns the captured stdout and the RunE error (nil error
// = exit 0 through cobra; non-nil = non-zero exit).
func runAssetCmd(t *testing.T, cmd *cobra.Command, args []string, flags map[string]string) (string, error) {
	t.Helper()
	for k, v := range flags {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("setting flag %s=%s: %v", k, v, err)
		}
	}
	var buf bytes.Buffer
	restore := captureStdout(&buf)
	err := cmd.RunE(cmd, args)
	restore()
	return buf.String(), err
}

// writeOverrideFile creates an override file under $TR_HOME/prompts with
// content distinct from the embedded default.
func writeOverrideFile(t *testing.T, trHome, name string) string {
	t.Helper()
	path := filepath.Join(trHome, "prompts", name+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir prompts dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("## Custom policy\n\nAlways ask about race conditions.\n"), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}
	return path
}

// ── tr asset list ─────────────────────────────────────────────────────────────

// Task 3.4.1: with TR_HOME unset, the canonical asset is listed as embedded.
func TestAssetListNoOverrides(t *testing.T) {
	t.Setenv("TR_HOME", "")

	out, err := runAssetCmd(t, listAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset list error: %v", err)
	}
	if !strings.Contains(out, "question-generation-policy\tembedded\t<embedded>\tembedded") {
		t.Fatalf("expected embedded asset line, got:\n%s", out)
	}
}

// Task 3.4.2: with one override, the line carries the $TR_HOME source, the
// absolute override path, and an age column.
func TestAssetListWithOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")

	out, err := runAssetCmd(t, listAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset list error: %v", err)
	}

	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "question-generation-policy\t") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("expected a line for question-generation-policy, got:\n%s", out)
	}
	fields := strings.Split(line, "\t")
	if len(fields) != 4 {
		t.Fatalf("expected 4 tab-separated columns, got %q", line)
	}
	if fields[1] != "$TR_HOME" {
		t.Fatalf("expected source column %q, got %q", "$TR_HOME", fields[1])
	}
	if fields[2] != overridePath {
		t.Fatalf("expected resolved path %q, got %q", overridePath, fields[2])
	}
	if fields[3] == "" {
		t.Fatal("expected non-empty age column")
	}
}

// Task 3.4.3: with two overrides, both appear — one shadowing the embedded
// default, one orphaned beyond the embedded set.
func TestAssetListMultipleOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")
	orphanPath := writeOverrideFile(t, tmp, "orphan-policy")

	out, err := runAssetCmd(t, listAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset list error: %v", err)
	}
	if !strings.Contains(out, "question-generation-policy\t$TR_HOME\t"+overridePath) {
		t.Fatalf("expected overridden asset line, got:\n%s", out)
	}
	if !strings.Contains(out, "orphan-policy\t$TR_HOME\t"+orphanPath) {
		t.Fatalf("expected orphan override line, got:\n%s", out)
	}
	if strings.Count(strings.TrimRight(out, "\n"), "\n") != 1 {
		t.Fatalf("expected exactly two asset lines, got:\n%s", out)
	}
}

// ── tr asset reset ────────────────────────────────────────────────────────────

// Task 4.6.1: reset <name> removes the override and prints the restart advisory.
func TestAssetResetSingleOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")

	out, err := runAssetCmd(t, resetAssetCmd(), []string{"question-generation-policy"}, nil)
	if err != nil {
		t.Fatalf("asset reset error: %v", err)
	}
	if _, statErr := os.Stat(overridePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected override file to be removed, stat err: %v", statErr)
	}
	if !strings.Contains(out, "[assets] removed override at "+overridePath+"; restart 'tr serve' to pick up the change") {
		t.Fatalf("expected restart advisory, got:\n%s", out)
	}
}

// Task 4.6.2: reset <name> on a missing override is a no-op with exit 0.
func TestAssetResetNoOverrideIsNoOp(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)

	out, err := runAssetCmd(t, resetAssetCmd(), []string{"question-generation-policy"}, nil)
	if err != nil {
		t.Fatalf("expected exit 0 for missing override, got error: %v", err)
	}
	if !strings.Contains(out, "no override for question-generation-policy — nothing to reset") {
		t.Fatalf("expected nothing-to-reset message, got:\n%s", out)
	}
}

// Task 4.6.3: batch reset without --all refuses (non-zero); with --all --force
// both overrides are removed.
func TestAssetResetMultipleRequiresAllFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	pathA := writeOverrideFile(t, tmp, "question-generation-policy")
	pathB := writeOverrideFile(t, tmp, "orphan-policy")

	cmd := resetAssetCmd()
	_, err := runAssetCmd(t, cmd, nil, nil)
	if err == nil {
		t.Fatal("expected refusal (non-zero exit) for batch reset without --all")
	}

	out, err := runAssetCmd(t, resetAssetCmd(), nil, map[string]string{"all": "true", "force": "true"})
	if err != nil {
		t.Fatalf("expected --all --force batch removal to succeed, got error: %v", err)
	}
	for _, path := range []string{pathA, pathB} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be removed", path)
		}
	}
	if !strings.Contains(out, "[assets] removed override at "+pathA) || !strings.Contains(out, "[assets] removed override at "+pathB) {
		t.Fatalf("expected advisory per removed override, got:\n%s", out)
	}
}

// Task 4.6.4: reset with TR_HOME unset exits 1.
func TestAssetResetNoTrHomeExits1(t *testing.T) {
	t.Setenv("TR_HOME", "")

	_, err := runAssetCmd(t, resetAssetCmd(), []string{"question-generation-policy"}, nil)
	if err == nil || !strings.Contains(err.Error(), "TR_HOME is not set; nothing to reset") {
		t.Fatalf("expected TR_HOME unset error, got: %v", err)
	}
}

// ── tr asset sync ─────────────────────────────────────────────────────────────

// Task 5.7.1: sync <name> writes the embedded bytes to the override slot.
func TestAssetSyncCreatesNewOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)

	out, err := runAssetCmd(t, syncAssetCmd(), []string{"question-generation-policy"}, nil)
	if err != nil {
		t.Fatalf("asset sync error: %v", err)
	}

	target := filepath.Join(tmp, "prompts", "question-generation-policy.md")
	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading synced override: %v", err)
	}
	embeddedBytes, ok := assets.Embedded("question-generation-policy")
	if !ok {
		t.Fatal("expected embedded bytes to exist")
	}
	if !bytes.Equal(written, embeddedBytes) {
		t.Fatal("expected synced file to equal the embedded bytes")
	}
	if !strings.Contains(out, "[assets] synced question-generation-policy to "+target+"; restart 'tr serve' to pick up the change") {
		t.Fatalf("expected sync advisory, got:\n%s", out)
	}
}

// Task 5.7.2: sync onto an existing non-empty override refuses without
// --force and overwrites with it.
func TestAssetSyncOverwritesWithForce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	existingPath := writeOverrideFile(t, tmp, "question-generation-policy")
	existing, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read existing override: %v", err)
	}

	_, err = runAssetCmd(t, syncAssetCmd(), []string{"question-generation-policy"}, nil)
	if err == nil || !strings.Contains(err.Error(), "exists and is non-empty — pass --force to overwrite, or 'tr asset reset question-generation-policy' to start from defaults") {
		t.Fatalf("expected refuse-without-force error, got: %v", err)
	}
	unchanged, err := os.ReadFile(existingPath)
	if err != nil || !bytes.Equal(unchanged, existing) {
		t.Fatalf("expected existing override untouched, err: %v", err)
	}

	_, err = runAssetCmd(t, syncAssetCmd(), []string{"question-generation-policy"}, map[string]string{"force": "true"})
	if err != nil {
		t.Fatalf("expected --force overwrite to succeed, got error: %v", err)
	}
	embeddedBytes, ok := assets.Embedded("question-generation-policy")
	if !ok {
		t.Fatal("expected embedded bytes to exist")
	}
	overwritten, err := os.ReadFile(existingPath)
	if err != nil || !bytes.Equal(overwritten, embeddedBytes) {
		t.Fatalf("expected override overwritten with embedded bytes, err: %v", err)
	}
}

// Task 5.7.3: sync with no name refuses (sync-all is too easy to misread).
func TestAssetSyncEmptyNoNameRefuses(t *testing.T) {
	t.Setenv("TR_HOME", t.TempDir())

	_, err := runAssetCmd(t, syncAssetCmd(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "sync requires an explicit asset name") {
		t.Fatalf("expected explicit-name error, got: %v", err)
	}
}

// Task 5.7.4: sync with TR_HOME unset exits 1.
func TestAssetSyncNoTrHomeExits1(t *testing.T) {
	t.Setenv("TR_HOME", "")

	_, err := runAssetCmd(t, syncAssetCmd(), []string{"question-generation-policy"}, nil)
	if err == nil || !strings.Contains(err.Error(), "TR_HOME is not set; nothing to sync to") {
		t.Fatalf("expected TR_HOME unset error, got: %v", err)
	}
}
