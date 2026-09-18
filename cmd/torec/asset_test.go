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

// isolateHome points HOME/USERPROFILE at a temp dir and empties TR_HOME so the
// data-dir resolution lands in the temp dir — tests never touch the real ~/.tr.
func isolateHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("TR_HOME", "")
}

// Task 3.2: the asset long help documents the override loop — what an
// override is, list vs reset vs sync, and the restart caveat — reachable via
// both `asset --help` and the root `help asset` form.
func TestAssetLongHelpDocumentsOverrideLoop(t *testing.T) {
	for _, args := range [][]string{{"asset", "--help"}, {"help", "asset"}} {
		root := &cobra.Command{Use: "torec"}
		root.AddCommand(assetCmd())
		root.SetArgs(args)
		root.SetOut(nil)

		var buf bytes.Buffer
		restore := captureStdout(&buf)
		err := root.Execute()
		restore()

		if err != nil {
			t.Fatalf("args %v: execute error: %v", args, err)
		}
		out := buf.String()
		for _, want := range []string{
			"data dir's prompts/",
			"one policy, one file",
			"listed 'inactive'",
			"sync",
			"reset",
			"restart 'torec serve'",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("args %v: expected help to contain %q, got:\n%s", args, want, out)
			}
		}
		for _, stale := range []string{"--all", "reset [<name>]"} {
			if strings.Contains(out, stale) {
				t.Errorf("args %v: expected help to drop stale form %q, got:\n%s", args, stale, out)
			}
		}
	}
}

// Task 4.6.4's successor: the unresolvable-data-dir behavior is covered by
// TestAssetResetUnresolvableDataDirExits1. The name-validation table lives
// below (task 4.3).

// Task 2.1: soleAssetFrom resolves the no-argument target from an enumerated
// shipped-asset list — one name wins, zero or several refuse with the
// inventory named.
func TestSoleAssetFrom(t *testing.T) {
	name, err := soleAssetFrom([]string{"question-generation-policy"})
	if err != nil || name != "question-generation-policy" {
		t.Fatalf("one name → it; got (%q, %v)", name, err)
	}

	_, err = soleAssetFrom(nil)
	if err == nil || !strings.Contains(err.Error(), "no shipped prompt assets — nothing to sync to") {
		t.Fatalf("zero names → refusal; got: %v", err)
	}

	many := []string{"policy-a", "policy-b", "policy-c"}
	_, err = soleAssetFrom(many)
	if err == nil {
		t.Fatal("several names → refusal, got nil error")
	}
	for _, n := range many {
		if !strings.Contains(err.Error(), n) {
			t.Fatalf("expected refusal to list %q, got: %v", n, err)
		}
	}
	if !strings.Contains(err.Error(), "multiple shipped prompt assets — specify one of:") {
		t.Fatalf("expected multi-asset refusal wording, got: %v", err)
	}
}

// Task 2.2: no-arg sync resolves the sole shipped asset, creates the override
// with embedded bytes, and prints the advisory.
func TestAssetSyncNoArgTargetsSoleAsset(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)

	out, err := runAssetCmd(t, syncAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("no-arg sync error: %v", err)
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
	if !strings.Contains(out, "[assets] synced question-generation-policy to "+target+"; restart 'torec serve' to pick up the change") {
		t.Fatalf("expected sync advisory, got:\n%s", out)
	}
}

// Task 2.3: no-arg reset resolves the sole shipped asset — removes the
// override when present, no-ops (exit 0) when absent.
func TestAssetResetNoArgTargetsSoleAsset(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)

	out, err := runAssetCmd(t, resetAssetCmd(), nil, nil)
	if err != nil || !strings.Contains(out, "no override for question-generation-policy — nothing to reset") {
		t.Fatalf("expected absent-override no-op, got (%q, %v)", out, err)
	}

	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")
	out, err = runAssetCmd(t, resetAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("no-arg reset error: %v", err)
	}
	if _, statErr := os.Stat(overridePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected override removed, stat err: %v", statErr)
	}
	if !strings.Contains(out, "[assets] removed override at "+overridePath+"; restart 'torec serve' to pick up the change") {
		t.Fatalf("expected restart advisory, got:\n%s", out)
	}
}

// Task 4.3: name validation accepts exactly single lowercase-hyphenated tokens.
func TestValidateAssetName(t *testing.T) {
	valid := []string{
		"question-generation-policy",
		"a",
		"policy-2",
		"question-generation-policy ", // surrounding whitespace is trimmed before the match
		"  policy-2  ",
	}
	for _, name := range valid {
		if err := validateAssetName(name); err != nil {
			t.Errorf("expected %q to be accepted, got: %v", name, err)
		}
	}

	invalid := map[string]string{
		"":                 "empty",
		"  ":               "whitespace only",
		"../../sensitive":  "traversal",
		"../prompts":       "parent traversal",
		"policy.md":        "user-supplied .md suffix",
		"Question Policy!": "spaces and punctuation",
		"question_policy":  "underscore",
		"question/policy":  "slash",
		"po licy":          "inner whitespace",
	}
	for name, why := range invalid {
		err := validateAssetName(name)
		if err == nil {
			t.Errorf("expected %q to be rejected (%s)", name, why)
			continue
		}
		if !strings.Contains(err.Error(), "invalid asset name '") || !strings.Contains(err.Error(), "e.g. 'question-generation-policy'") {
			t.Errorf("expected teaching error for %q, got: %v", name, err)
		}
	}
}

// Task 4.3: reset with a traversal-shaped name exits 1 without touching any
// path outside the override directory.
func TestAssetResetRejectsInvalidName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)

	_, err := runAssetCmd(t, resetAssetCmd(), []string{"../../sensitive"}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid asset name '../../sensitive'") {
		t.Fatalf("expected invalid-name refusal, got: %v", err)
	}
	// Nothing was removed: the prompts dir is untouched.
	if entries, _ := os.ReadDir(filepath.Join(tmp, "prompts")); len(entries) != 0 {
		t.Fatalf("expected no files touched, found: %v", entries)
	}
}

// Task 4.3: sync rejects odd names (user-supplied .md suffix, spaces) with
// exit 1 and writes nothing.
func TestAssetSyncRejectsInvalidName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)

	for _, name := range []string{"policy.md", "Question Policy!", "../prompts"} {
		_, err := runAssetCmd(t, syncAssetCmd(), []string{name}, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid asset name") {
			t.Errorf("expected %q to be refused, got: %v", name, err)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(tmp, "prompts")); len(entries) != 0 {
		t.Fatalf("expected no files written, found: %v", entries)
	}
}

// Task 2.3.1: slot files whose name matches no shipped asset list as
// `inactive`; shipped-name overrides keep `$TR_HOME`.
func TestAssetShowTagsUnmanagedAsInactive(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")
	orphanPath := writeOverrideFile(t, tmp, "my-experiment")

	out, err := runAssetCmd(t, showAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset show error: %v", err)
	}
	if !strings.Contains(out, "question-generation-policy\t$TR_HOME\t"+overridePath) {
		t.Fatalf("expected shipped-name override to keep $TR_HOME, got:\n%s", out)
	}
	if !strings.Contains(out, "my-experiment\tinactive\t"+orphanPath) {
		t.Fatalf("expected orphan tagged inactive, got:\n%s", out)
	}
}

// Task 2.3.2: no `inactive` tag appears when the slot holds nothing
// unmanaged.
func TestAssetShowNoUnmanagedTag(t *testing.T) {
	isolateHome(t)

	out, err := runAssetCmd(t, showAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset show error: %v", err)
	}
	if strings.Contains(out, "inactive") {
		t.Fatalf("expected no inactive tag without unmanaged files, got:\n%s", out)
	}
}

// Task 2.3.3: sync with an unknown name exits 1 and points at the inventory.
func TestAssetSyncUnknownNamePointsAtList(t *testing.T) {
	t.Setenv("TR_HOME", t.TempDir())

	_, err := runAssetCmd(t, syncAssetCmd(), []string{"ecs-policy"}, nil)
	if err == nil {
		t.Fatal("expected non-nil error for unknown asset")
	}
	if !strings.Contains(err.Error(), "embedded asset 'ecs-policy' not found") ||
		!strings.Contains(err.Error(), "run 'torec asset show' to see the available asset names") {
		t.Fatalf("expected teaching error, got: %v", err)
	}
}

// Task 2.3.4: reset on an unmanaged name still removes it — reset is the
// cleanup path for orphans.
func TestAssetResetUnmanagedFileStillWorks(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	orphanPath := writeOverrideFile(t, tmp, "my-experiment")

	out, err := runAssetCmd(t, resetAssetCmd(), []string{"my-experiment"}, nil)
	if err != nil {
		t.Fatalf("expected reset of unmanaged file to succeed, got: %v", err)
	}
	if _, statErr := os.Stat(orphanPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected orphan file removed, stat err: %v", statErr)
	}
	if !strings.Contains(out, "[assets] removed override at "+orphanPath+"; restart 'torec serve' to pick up the change") {
		t.Fatalf("expected restart advisory, got:\n%s", out)
	}
}

// ── torec asset list ─────────────────────────────────────────────────────────────

// Task 3.4.1: with no overrides in the data dir, the canonical asset is listed
// as embedded.
func TestAssetShowNoOverrides(t *testing.T) {
	isolateHome(t)

	out, err := runAssetCmd(t, showAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset show error: %v", err)
	}
	if !strings.Contains(out, "question-generation-policy\tembedded\t<embedded>\tembedded") {
		t.Fatalf("expected embedded asset line, got:\n%s", out)
	}
}

// Task 3.4.2: with one override, the line carries the $TR_HOME source, the
// absolute override path, and an age column.
func TestAssetShowWithOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")

	out, err := runAssetCmd(t, showAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset show error: %v", err)
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

// Task 3.4.3: with two slot files, both appear — one shadowing the embedded
// default ($TR_HOME), one orphaned beyond the embedded set (inactive, per the
// asset-slot-ux-clarity change).
func TestAssetShowMultipleOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideFile(t, tmp, "question-generation-policy")
	orphanPath := writeOverrideFile(t, tmp, "orphan-policy")

	out, err := runAssetCmd(t, showAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset show error: %v", err)
	}
	if !strings.Contains(out, "question-generation-policy\t$TR_HOME\t"+overridePath) {
		t.Fatalf("expected overridden asset line, got:\n%s", out)
	}
	if !strings.Contains(out, "orphan-policy\tinactive\t"+orphanPath) {
		t.Fatalf("expected orphan tagged inactive, got:\n%s", out)
	}
	if strings.Count(strings.TrimRight(out, "\n"), "\n") != 1 {
		t.Fatalf("expected exactly two asset lines, got:\n%s", out)
	}
}

// ── torec asset reset ────────────────────────────────────────────────────────────

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
	if !strings.Contains(out, "[assets] removed override at "+overridePath+"; restart 'torec serve' to pick up the change") {
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

// The review follow-up: with TR_HOME unset, the commands operate on the
// default data dir (~/.tr) — sync lands there and list sees it, proving the
// CLI's data-dir resolution agrees with the assets package's.
func TestAssetSyncUsesDefaultDataDir(t *testing.T) {
	isolateHome(t)

	out, err := runAssetCmd(t, syncAssetCmd(), []string{"question-generation-policy"}, nil)
	if err != nil {
		t.Fatalf("asset sync error: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	expected := filepath.Join(home, ".tr", "prompts", "question-generation-policy.md")
	if !strings.Contains(out, "synced question-generation-policy to "+expected) {
		t.Fatalf("expected sync target under ~/.tr, got:\n%s", out)
	}
	if _, statErr := os.Stat(expected); statErr != nil {
		t.Fatalf("expected override file at %s: %v", expected, statErr)
	}

	out, err = runAssetCmd(t, showAssetCmd(), nil, nil)
	if err != nil {
		t.Fatalf("asset show error: %v", err)
	}
	if !strings.Contains(out, "question-generation-policy\t$TR_HOME\t"+expected) {
		t.Fatalf("expected show to show the default-dir override, got:\n%s", out)
	}

	_, err = runAssetCmd(t, resetAssetCmd(), []string{"question-generation-policy"}, nil)
	if err != nil {
		t.Fatalf("asset reset error: %v", err)
	}
	if _, statErr := os.Stat(expected); !os.IsNotExist(statErr) {
		t.Fatalf("expected reset to remove %s", expected)
	}
}

// Reset/sync exit 1 when the data dir cannot be resolved at all (no home dir).
func TestAssetResetUnresolvableDataDirExits1(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("TR_HOME", "")

	_, err := runAssetCmd(t, resetAssetCmd(), []string{"question-generation-policy"}, nil)
	if err == nil || !strings.Contains(err.Error(), "could not resolve the Total Recall data dir") {
		t.Fatalf("expected data-dir resolution error, got: %v", err)
	}
}

// ── torec asset sync ─────────────────────────────────────────────────────────────

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
	if !strings.Contains(out, "[assets] synced question-generation-policy to "+target+"; restart 'torec serve' to pick up the change") {
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
	if err == nil || !strings.Contains(err.Error(), "exists and is non-empty — pass --force to overwrite, or 'torec asset reset question-generation-policy' to start from defaults") {
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

// Task 5.7.4: sync exits 1 when the data dir cannot be resolved at all.
func TestAssetSyncUnresolvableDataDirExits1(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("TR_HOME", "")

	_, err := runAssetCmd(t, syncAssetCmd(), []string{"question-generation-policy"}, nil)
	if err == nil || !strings.Contains(err.Error(), "could not resolve the Total Recall data dir") {
		t.Fatalf("expected data-dir resolution error, got: %v", err)
	}
}
