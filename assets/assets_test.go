package assets

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// resetCache clears the package-level cache between tests so each test sees a
// fresh Load resolution. Tests that mutate $TR_HOME or write override files
// must call this to avoid poisoning siblings.
func resetCache(t *testing.T) {
	t.Helper()
	cacheMu.Lock()
	entries = make(map[string]PromptAsset)
	cacheMu.Unlock()
}

// isolateHome points HOME/USERPROFILE at a temp dir and empties TR_HOME so
// dataDir() resolves into the temp dir — tests that read the default data dir
// never touch the real ~/.tr.
func isolateHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("TR_HOME", "")
}

func TestLoadEmbedded(t *testing.T) {
	isolateHome(t)
	resetCache(t)

	asset, err := Load("question-generation-policy")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if asset.Source != SourceEmbedded {
		t.Fatalf("expected Source=%q, got %q", SourceEmbedded, asset.Source)
	}
	if asset.Body == "" {
		t.Fatal("expected non-empty Body from embedded policy doc")
	}
	if !strings.Contains(asset.Body, "counterfactual debugging") {
		t.Fatalf("expected Body to contain known phrase from question-generation-policy.md; got: %.200s", asset.Body)
	}
	if asset.Name != "generate-quiz-question" {
		t.Fatalf("expected Name=%q from front matter, got %q", "generate-quiz-question", asset.Name)
	}
	if asset.Description == "" {
		t.Fatal("expected non-empty Description from front matter")
	}
}

func TestLoadOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	resetCache(t)

	overrideDir := filepath.Join(tmp, "prompts")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	overrideBody := "## Custom policy\n\nAlways ask about race conditions.\n"
	overrideContent := "---\nname: custom-override\ndescription: A custom override for testing.\n---\n" + overrideBody
	overridePath := filepath.Join(overrideDir, "question-generation-policy.md")
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	asset, err := Load("question-generation-policy")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if asset.Source != SourceOverride {
		t.Fatalf("expected Source=%q, got %q", SourceOverride, asset.Source)
	}
	if asset.Body != overrideBody {
		t.Fatalf("expected Body to match override body, got %q", asset.Body)
	}
	if asset.Name != "custom-override" {
		t.Fatalf("expected Name from override front matter, got %q", asset.Name)
	}
}

// The review follow-up fix: overrides load from the default data dir
// (~/.tr) when TR_HOME is unset — the override mechanism is not gated on the
// env var being set.
func TestLoadOverrideFromDefaultDataDir(t *testing.T) {
	isolateHome(t)
	resetCache(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	overridePath := filepath.Join(home, ".tr", "prompts", "question-generation-policy.md")
	if err := os.MkdirAll(filepath.Dir(overridePath), 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	overrideBody := "## Default-dir policy\n\nAlways ask about error handling.\n"
	if err := os.WriteFile(overridePath, []byte(overrideBody), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	asset, err := Load("question-generation-policy")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if asset.Source != SourceOverride {
		t.Fatalf("expected Source=%q from default data dir, got %q", SourceOverride, asset.Source)
	}
	if asset.Path != overridePath {
		t.Fatalf("expected Path=%q, got %q", overridePath, asset.Path)
	}
	if !strings.Contains(asset.Body, "Default-dir policy") {
		t.Fatalf("expected override body from default data dir, got: %.200s", asset.Body)
	}
}

// An unresolvable home dir means no override could exist; Load falls through
// to the embedded default instead of failing.
func TestLoadUnresolvableDataDirFallsBackToEmbedded(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("TR_HOME", "")
	resetCache(t)

	asset, err := Load("question-generation-policy")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if asset.Source != SourceEmbedded {
		t.Fatalf("expected Source=%q when data dir is unresolvable, got %q", SourceEmbedded, asset.Source)
	}
}

func TestLoadOverrideMissingFallsBackToEmbedded(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	resetCache(t)

	asset, err := Load("question-generation-policy")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if asset.Source != SourceEmbedded {
		t.Fatalf("expected Source=%q when override dir is empty, got %q", SourceEmbedded, asset.Source)
	}
}

func TestLoadMissing(t *testing.T) {
	isolateHome(t)
	resetCache(t)

	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	asset, err := Load("does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
	if asset.Source != SourceFallback {
		t.Fatalf("expected Source=%q, got %q", SourceFallback, asset.Source)
	}
	if asset.Body != "" {
		t.Fatalf("expected empty Body for fallback, got %q", asset.Body)
	}
	if !strings.Contains(buf.String(), `not found, falling back to inline synthesis template`) {
		t.Fatalf("expected fallback log line, got: %s", buf.String())
	}
}

func TestParseFrontMatterStandard(t *testing.T) {
	raw := "---\nname: generate-quiz-question\ndescription: Generate short quiz question.\n---\n## Goals\n\nBody text.\n"
	asset := parseAsset(raw)
	if asset.Name != "generate-quiz-question" {
		t.Fatalf("expected Name=%q, got %q", "generate-quiz-question", asset.Name)
	}
	if asset.Description != "Generate short quiz question." {
		t.Fatalf("expected Description %q, got %q", "Generate short quiz question.", asset.Description)
	}
	if !strings.HasPrefix(asset.Body, "## Goals") {
		t.Fatalf("expected Body to start after front matter, got %q", asset.Body)
	}
}

func TestParseFrontMatterMissing(t *testing.T) {
	raw := "## Goals\n\nNo front matter here.\n"
	asset := parseAsset(raw)
	if asset.Name != "" || asset.Description != "" {
		t.Fatalf("expected empty Name/Description when no front matter, got %+v", asset)
	}
	if asset.Body != raw {
		t.Fatalf("expected Body to equal full content when no front matter, got %q", asset.Body)
	}
}

func TestParseFrontMatterMalformedNoPanic(t *testing.T) {
	raw := "---\nname: broken\nthis is not key: value: properly\ndescription: ok\n---\n## Body\n"
	asset := parseAsset(raw)
	if asset.Name != "broken" {
		t.Fatalf("expected Name=%q extracted despite malformed line, got %q", "broken", asset.Name)
	}
	if !strings.Contains(asset.Body, "## Body") {
		t.Fatalf("expected Body to contain post-front-matter content, got %q", asset.Body)
	}
}

func TestParseFrontMatterUnknownKeysIgnored(t *testing.T) {
	raw := "---\nname: asset-x\npriority: high\ndescription: An asset.\n---\n## Body\n"
	asset := parseAsset(raw)
	if asset.Name != "asset-x" {
		t.Fatalf("expected Name=%q, got %q", "asset-x", asset.Name)
	}
	if asset.Description != "An asset." {
		t.Fatalf("expected Description %q, got %q", "An asset.", asset.Description)
	}
}

func TestLoadCachedAfterFirstCall(t *testing.T) {
	isolateHome(t)
	resetCache(t)

	first, err := Load("question-generation-policy")
	if err != nil {
		t.Fatalf("first Load error: %v", err)
	}
	for i := 0; i < 9; i++ {
		next, err := Load("question-generation-policy")
		if err != nil {
			t.Fatalf("iteration %d Load error: %v", i, err)
		}
		if next != first {
			t.Fatalf("iteration %d returned a different PromptAsset than the first call", i)
		}
	}
}

// setThreshold overrides the package drift threshold for the duration of the
// test and restores the previous value afterwards.
func setThreshold(t *testing.T, days int) {
	t.Helper()
	prev := driftThresholdDays
	t.Cleanup(func() { driftThresholdDays = prev })
	SetDriftThreshold(days)
}

// writeOverrideWithAge writes an override file under dir/prompts with its
// mtime set age before embeddedRefTime, so drift computations are exact.
func writeOverrideWithAge(t *testing.T, dir, name string, age time.Duration) string {
	t.Helper()
	overrideDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(overrideDir, 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	content := "---\nname: custom-override\ndescription: A custom override for testing.\n---\n## Custom policy\n\nAlways ask about race conditions.\n"
	path := filepath.Join(overrideDir, name+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}
	if age != 0 {
		past := embeddedRefTime().Add(-age)
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatalf("chtimes override: %v", err)
		}
	}
	return path
}

// captureLog redirects the standard logger into a buffer and returns a restore
// function; read the captured text only after calling it.
func captureLog(t *testing.T) func() string {
	t.Helper()
	var buf strings.Builder
	log.SetOutput(&buf)
	return func() string {
		log.SetOutput(os.Stderr)
		return buf.String()
	}
}

func TestLoadWarnsOnStaleOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	resetCache(t)
	setThreshold(t, 90)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 100*24*time.Hour)

	restore := captureLog(t)
	_, err := Load("question-generation-policy")
	out := restore()

	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !strings.Contains(out, "OVERRIDE WARNING") {
		t.Fatalf("expected OVERRIDE WARNING in log, got: %s", out)
	}
	if !strings.Contains(out, "100d older") {
		t.Fatalf("expected %q in warning, got: %s", "100d older", out)
	}
	if !strings.Contains(out, "re-sync with 'tr asset sync question-generation-policy'") {
		t.Fatalf("expected re-sync hint in warning, got: %s", out)
	}
}

func TestLoadNoWarningWhenRecent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	resetCache(t)
	setThreshold(t, 90)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 10*24*time.Hour)

	restore := captureLog(t)
	_, err := Load("question-generation-policy")
	out := restore()

	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if strings.Contains(out, "OVERRIDE WARNING") {
		t.Fatalf("expected no OVERRIDE WARNING for 10d-old override at 90d threshold, got: %s", out)
	}
	if !strings.Contains(out, "loaded from override at") {
		t.Fatalf("expected enriched override log line, got: %s", out)
	}
}

func TestLoadWarnsAtConfigurableThreshold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	resetCache(t)
	setThreshold(t, 30)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 35*24*time.Hour)

	restore := captureLog(t)
	_, err := Load("question-generation-policy")
	out := restore()

	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !strings.Contains(out, "OVERRIDE WARNING") {
		t.Fatalf("expected OVERRIDE WARNING for 35d-old override at 30d threshold, got: %s", out)
	}
}

func TestLoadWarnDisabledAtZeroThreshold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	resetCache(t)
	setThreshold(t, 0)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 10*365*24*time.Hour)

	restore := captureLog(t)
	_, err := Load("question-generation-policy")
	out := restore()

	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if strings.Contains(out, "OVERRIDE WARNING") {
		t.Fatalf("expected zero threshold to disable the warning, got: %s", out)
	}
}

func TestLoadEmbeddedIsSilent(t *testing.T) {
	isolateHome(t)
	resetCache(t)

	restore := captureLog(t)
	_, err := Load("question-generation-policy")
	out := restore()

	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if strings.Contains(out, "[assets]") {
		t.Fatalf("expected no asset log lines for the embedded path, got: %s", out)
	}
}

func TestLoadAllEnumeratesEmbeddedAndOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	overridePath := writeOverrideWithAge(t, tmp, "question-generation-policy", 2*24*time.Hour)

	all := LoadAll()
	if len(all) != 1 {
		t.Fatalf("expected one distinct asset across embedded + override, got %d: %+v", len(all), all)
	}
	asset := all[0]
	if asset.Source != SourceOverride {
		t.Fatalf("expected override to win on name collision, got Source=%q", asset.Source)
	}
	if asset.Path != overridePath {
		t.Fatalf("expected resolved override path %q, got %q", overridePath, asset.Path)
	}
	if asset.ModTime.IsZero() {
		t.Fatal("expected non-zero ModTime for override resolution")
	}
	if !strings.Contains(asset.Body, "Always ask about race conditions") {
		t.Fatalf("expected override body, got: %.200s", asset.Body)
	}
}

func TestLoadAllIncludesOrphanOverrides(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 0)
	writeOverrideWithAge(t, tmp, "orphan-policy", 0)

	all := LoadAll()
	names := make([]string, 0, len(all))
	for _, a := range all {
		names = append(names, strings.TrimSuffix(filepath.Base(a.Path), ".md"))
	}
	if len(all) != 2 || names[0] != "orphan-policy" || names[1] != "question-generation-policy" {
		t.Fatalf("expected sorted union [orphan-policy question-generation-policy], got %+v (%v)", all, names)
	}
}

func TestEmbeddedNamesAndBytes(t *testing.T) {
	names := EmbeddedNames()
	if len(names) == 0 || names[0] != "question-generation-policy" {
		t.Fatalf("expected embedded names to include question-generation-policy, got %v", names)
	}
	b, ok := Embedded("question-generation-policy")
	if !ok || len(b) == 0 {
		t.Fatal("expected embedded bytes for question-generation-policy")
	}
	if _, ok := Embedded("does-not-exist"); ok {
		t.Fatal("expected ok=false for unknown embedded name")
	}
}

// Task 1.4: UnmanagedNames classifies slot files against shipped asset names.
func TestUnmanagedNamesClassification(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 0)
	writeOverrideWithAge(t, tmp, "my-experiment", 0)

	got := UnmanagedNames()
	if len(got) != 1 || got[0] != "my-experiment" {
		t.Fatalf("expected [my-experiment], got %v", got)
	}
}

// Task 1.4: the startup warning logs only orphan files, not shipped-name
// overrides.
func TestWarnUnmanagedOverridesLogsOrphans(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TR_HOME", tmp)
	writeOverrideWithAge(t, tmp, "question-generation-policy", 0)
	writeOverrideWithAge(t, tmp, "my-experiment", 0)

	restore := captureLog(t)
	WarnUnmanagedOverrides()
	out := restore()

	if !strings.Contains(out, `unmanaged file "my-experiment.md"`) {
		t.Fatalf("expected unmanaged warning for the orphan, got: %s", out)
	}
	if strings.Contains(out, "question-generation-policy") {
		t.Fatalf("expected no warning for the shipped-name override, got: %s", out)
	}
	if !strings.Contains(out, "run 'tr asset list'") {
		t.Fatalf("expected the list pointer in the warning, got: %s", out)
	}
}

// Task 1.4: empty, absent, and unresolvable slots are silent.
func TestWarnUnmanagedOverridesSilent(t *testing.T) {
	cases := []struct {
		name string
		env  func(t *testing.T)
	}{
		{"no slot dir", func(t *testing.T) { t.Setenv("TR_HOME", t.TempDir()) }},
		{"unset data dir", func(t *testing.T) { isolateHome(t) }},
		{"unresolvable home", func(t *testing.T) {
			t.Setenv("HOME", "")
			t.Setenv("USERPROFILE", "")
			t.Setenv("TR_HOME", "")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.env(t)
			// An empty slot dir exists but holds nothing.
			restore := captureLog(t)
			WarnUnmanagedOverrides()
			out := restore()
			if strings.Contains(out, "unmanaged file") {
				t.Fatalf("expected silence, got: %s", out)
			}
		})
	}
}

func TestAgeHumanized(t *testing.T) {
	now := time.Now()
	cases := []struct {
		age  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m old"},
		{3 * time.Hour, "3h old"},
		{48 * time.Hour, "2d old"},
		{10 * 24 * time.Hour, "10d old"},
		{180 * 24 * time.Hour, "6mo old"},
		{800 * 24 * time.Hour, "2y old"},
	}
	for _, tc := range cases {
		if got := Age(now.Add(-tc.age)); got != tc.want {
			t.Errorf("Age(%s ago) = %q, want %q", tc.age, got, tc.want)
		}
	}
	if got := Age(time.Time{}); got != "unknown" {
		t.Errorf("Age(zero) = %q, want %q", got, "unknown")
	}
}
