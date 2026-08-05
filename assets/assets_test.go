package assets

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestLoadEmbedded(t *testing.T) {
	t.Setenv("TR_HOME", "")
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
	t.Setenv("TR_HOME", "")
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
	t.Setenv("TR_HOME", "")
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
