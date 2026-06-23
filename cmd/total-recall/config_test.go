package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/config"
)

func TestDefaultUserConfigSanity(t *testing.T) {
	cfg := config.DefaultUserConfig()

	if cfg.AI.Provider != "anthropic" {
		t.Fatalf("expected provider anthropic, got %q", cfg.AI.Provider)
	}
	if cfg.AI.Model != "claude-sonnet-4-5" {
		t.Fatalf("expected model claude-sonnet-4-5, got %q", cfg.AI.Model)
	}
	if cfg.Privacy.ConversationAnalysis {
		t.Fatal("expected conversation_analysis to be false by default")
	}
	if cfg.Recall.Difficulty != "adaptive" {
		t.Fatalf("expected difficulty adaptive, got %q", cfg.Recall.Difficulty)
	}
	if cfg.Recall.MaxQuestions != 1 {
		t.Fatalf("expected max_questions 1, got %d", cfg.Recall.MaxQuestions)
	}
}

func TestMergeWithNilRepo(t *testing.T) {
	user := config.DefaultUserConfig()
	cfg := config.Merge(&user, nil)

	if cfg.AI.Provider != user.AI.Provider {
		t.Fatal("ai provider should come from user when repo is nil")
	}
	if cfg.Privacy.ConversationAnalysis != user.Privacy.ConversationAnalysis {
		t.Fatal("privacy should come from user when repo is nil")
	}
	if cfg.Hooks.PreCommit {
		t.Fatal("hooks should default to false when repo is nil")
	}
	if cfg.Sources.HooksPreCommit != "[default]" {
		t.Fatalf("expected [default] source for hooks, got %q", cfg.Sources.HooksPreCommit)
	}
}

func TestMergeRepoOverridesUser(t *testing.T) {
	user := config.DefaultUserConfig()
	repo := &config.RepoConfig{
		Hooks: config.HooksConfig{PreCommit: true, CommitMsg: true},
		Mode:  config.ModeConfig{Blocking: true},
		Presentation: config.PresentationConfig{
			Terminal: true,
		},
	}
	cfg := config.Merge(&user, repo)

	if !cfg.Hooks.PreCommit {
		t.Fatal("hooks.pre-commit should come from repo")
	}
	if !cfg.Hooks.CommitMsg {
		t.Fatal("hooks.commit-msg should come from repo")
	}
	if !cfg.Mode.Blocking {
		t.Fatal("mode.blocking should come from repo")
	}
	if !cfg.Presentation.Terminal {
		t.Fatal("presentation.terminal should come from repo")
	}

	if cfg.AI.Provider != user.AI.Provider {
		t.Fatal("ai provider should remain from user despite repo")
	}
	if cfg.Sources.HooksPreCommit != "[repo]" {
		t.Fatalf("expected [repo] source for hooks, got %q", cfg.Sources.HooksPreCommit)
	}
}

func TestPrivilegedKeysDiscardedInMerge(t *testing.T) {
	user := config.DefaultUserConfig()
	repo := &config.RepoConfig{
		Privacy: &config.PrivacyConfig{ConversationAnalysis: true},
		AI:      &config.AIConfig{Provider: "openai", Model: "gpt-4o"},
	}
	cfg := config.Merge(&user, repo)

	if cfg.Privacy.ConversationAnalysis != user.Privacy.ConversationAnalysis {
		t.Fatal("privacy should not be overridden by repo")
	}
	if cfg.AI.Provider != user.AI.Provider {
		t.Fatal("ai provider should not be overridden by repo")
	}
	if cfg.AI.Model != user.AI.Model {
		t.Fatal("ai model should not be overridden by repo")
	}
}

func TestMergeRepoRecallDeepMerge(t *testing.T) {
	user := config.DefaultUserConfig()
	repo := &config.RepoConfig{
		Recall: &config.RecallConfig{
			Difficulty: "hard",
		},
	}
	cfg := config.Merge(&user, repo)

	if cfg.Recall.Difficulty != "hard" {
		t.Fatalf("expected difficulty hard from repo, got %q", cfg.Recall.Difficulty)
	}
	if cfg.Recall.MaxQuestions != user.Recall.MaxQuestions {
		t.Fatalf("max_questions should remain from user, got %d", cfg.Recall.MaxQuestions)
	}
	if cfg.Sources.RecallDifficulty != "[repo]" {
		t.Fatalf("expected [repo] source for difficulty, got %q", cfg.Sources.RecallDifficulty)
	}
	if cfg.Sources.RecallMaxQuestions != "[user]" {
		t.Fatalf("expected [user] source for max_questions, got %q", cfg.Sources.RecallMaxQuestions)
	}
}

func TestResolvedAPIKeyReturnsEnvVar(t *testing.T) {
	t.Setenv("TR_TEST_API_KEY", "sk-my-secret-key")
	ai := config.AIConfig{APIKey: "env:TR_TEST_API_KEY"}
	key, ok := ai.ResolvedAPIKey()
	if !ok {
		t.Fatal("expected ok=true for set env var")
	}
	if key != "sk-my-secret-key" {
		t.Fatalf("expected sk-my-secret-key, got %q", key)
	}
}

func TestResolvedAPIKeyReturnsRawForNonEnv(t *testing.T) {
	ai := config.AIConfig{APIKey: "sk-raw-key"}
	key, ok := ai.ResolvedAPIKey()
	if !ok {
		t.Fatal("expected ok=true for non-empty raw key")
	}
	if key != "sk-raw-key" {
		t.Fatalf("expected sk-raw-key, got %q", key)
	}
}

func TestResolvedAPIKeyReturnsEmptyForEmpty(t *testing.T) {
	ai := config.AIConfig{APIKey: ""}
	_, ok := ai.ResolvedAPIKey()
	if ok {
		t.Fatal("expected ok=false for empty key")
	}
}

func TestResolvedAPIKeyReturnsEmptyForMissingEnvVar(t *testing.T) {
	t.Setenv("TR_TEST_MISSING_KEY", "")
	ai := config.AIConfig{APIKey: "env:TR_TEST_MISSING_KEY"}
	_, ok := ai.ResolvedAPIKey()
	if ok {
		t.Fatal("expected ok=false for empty env var")
	}
}

func TestShowPrintsSourceTags(t *testing.T) {
	user := config.DefaultUserConfig()
	cfg := config.Merge(&user, nil)

	var buf bytes.Buffer
	config.Show(cfg, &buf)

	output := buf.String()
	if !strings.Contains(output, "[user]") && !strings.Contains(output, "[default]") {
		t.Fatalf("expected output to contain source tags ([user] or [default]), got:\n%s", output)
	}
	if !strings.Contains(output, "provider:") {
		t.Fatalf("expected provider field in Show output, got:\n%s", output)
	}
}

func TestAutoCreateUserConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	cfg, err := config.EnsureUserConfig(true)
	if err != nil {
		t.Fatalf("EnsureUserConfig failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.AI.Provider != "anthropic" {
		t.Fatalf("expected default provider, got %q", cfg.AI.Provider)
	}

	cfgPath, _ := config.UserConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file was not auto-created")
	}
}

func TestAutoCreateUserConfigEmitsAdvisory(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	var stderr bytes.Buffer
	restore := captureStderr(&stderr)
	defer restore()

	_, err := config.EnsureUserConfig(false)
	restore()

	if err != nil {
		t.Fatalf("EnsureUserConfig failed: %v", err)
	}

	if !strings.Contains(stderr.String(), "No Total Recall user config found") {
		t.Fatalf("expected advisory message on stderr, got:\n%s", stderr.String())
	}
}

func TestLoadReturnsMergedConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	// First ensure user config exists with defaults
	_, err := config.EnsureUserConfig(true)
	if err != nil {
		t.Fatalf("EnsureUserConfig failed: %v", err)
	}

	cfg, err := config.Load(true)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.AI.Provider != "anthropic" {
		t.Fatalf("expected default provider anthropic, got %q", cfg.AI.Provider)
	}
}
