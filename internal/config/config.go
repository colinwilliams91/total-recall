// Package config provides config schema, loading, and deep-merge logic for Total Recall.
package config

import (
	"os"
	"path/filepath"
)

// UserConfig is loaded from ~/.tr/config.yaml.
// It defines personal defaults that apply across all repositories.
type UserConfig struct {
	Privacy PrivacyConfig `yaml:"privacy"`
	AI      AIConfig      `yaml:"ai"`
	Recall  RecallConfig  `yaml:"recall"`
}

// PrivacyConfig controls opt-in data processing features.
type PrivacyConfig struct {
	ConversationAnalysis bool `yaml:"conversation_analysis"`
}

// AIConfig holds BYOK provider credentials and model selection.
// APIKey accepts either a raw value or an env var reference: env:<VAR_NAME>.
type AIConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api-key"`
	BaseURL  string `yaml:"base-url,omitempty"`
}

// KnownProviders maps user-facing provider names to their default base URLs.
// The adapter column is determined by which package handles each provider:
//
//	anthropic → internal/ai/anthropic (native Messages API)
//	openai    → internal/ai/openai    (OpenAI Chat Completions API)
//	ollama    → internal/ai/openai    (OpenAI-compatible)
//	groq      → internal/ai/openai    (OpenAI-compatible)
//  qwen      → internal/ai/openai    (OpenAI-compatible)
//  minimax   → internal/ai/openai    (OpenAI-compatible)
//  deepseek  → internal/ai/openai    (OpenAI-compatible)
//	lm-studio → internal/ai/openai    (OpenAI-compatible)
//	custom    → internal/ai/openai    (OpenAI-compatible; BaseURL required)
//
// For named presets, BaseURL in AIConfig is ignored — the registry value is used.
// For "custom", AIConfig.BaseURL is required; ai.New returns ErrNoProvider if empty.
var KnownProviders = map[string]string{
	"anthropic": "https://api.anthropic.com",
	"openai":    "https://api.openai.com/v1",
	"ollama":    "http://localhost:11434/v1",
	"groq":      "https://api.groq.com/openai/v1",
	"qwen":      "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"minimax":   "https://api.minimaxi.com/v1",
	"deepseek":  "https://api.deepseek.com/v1",
	"lm-studio": "http://localhost:1234/v1",
	"custom":    "", // user must supply BaseURL
}

// ResolvedAPIKey returns the actual API key value.
// If APIKey is in "env:<VAR_NAME>" format, it resolves the named environment variable.
// Returns ("", false) if the key is not set or the env var is empty.
func (a AIConfig) ResolvedAPIKey() (string, bool) {
	const prefix = "env:"
	if len(a.APIKey) > len(prefix) && a.APIKey[:len(prefix)] == prefix {
		val := os.Getenv(a.APIKey[len(prefix):])
		return val, val != ""
	}
	return a.APIKey, a.APIKey != ""
}

// RecallConfig holds recall engine defaults.
type RecallConfig struct {
	Difficulty   string `yaml:"difficulty"`
	MaxQuestions int    `yaml:"max_questions"`
}

// RepoConfig is loaded from .tr.yaml in the repository root.
// It defines project-specific settings and optional per-repo recall overrides.
// Privacy and AI keys are user-level only — they are discarded with a warning if present here.
type RepoConfig struct {
	Hooks        HooksConfig        `yaml:"hooks"`
	Mode         ModeConfig         `yaml:"mode"`
	Presentation PresentationConfig `yaml:"presentation"`
	Recall       *RecallConfig      `yaml:"recall,omitempty"`
	// User-level only. If present in .tr.yaml, discarded with a warning.
	Privacy *PrivacyConfig `yaml:"privacy,omitempty"`
	AI      *AIConfig      `yaml:"ai,omitempty"`
}

// HooksConfig controls which Git hooks are active for the repository.
type HooksConfig struct {
	PreCommit bool `yaml:"pre-commit"`
	CommitMsg bool `yaml:"commit-msg"`
	PrePush   bool `yaml:"pre-push"`
}

// ModeConfig controls daemon behaviour for the repository.
type ModeConfig struct {
	Blocking bool `yaml:"blocking"`
}

// PresentationConfig controls which output adapters are active.
type PresentationConfig struct {
	Terminal bool `yaml:"terminal"`
	MCP      bool `yaml:"mcp"`
}

// Config is the fully resolved configuration after deep-merging user and repo configs.
type Config struct {
	Privacy      PrivacyConfig
	AI           AIConfig
	Recall       RecallConfig
	Hooks        HooksConfig
	Mode         ModeConfig
	Presentation PresentationConfig
	Sources      ConfigSources
}

// ConfigSources records which config file each resolved key came from.
// Values are "user", "repo", or "default" (zero value, not explicitly configured).
type ConfigSources struct {
	PrivacyConversationAnalysis string
	AIProvider                  string
	AIModel                     string
	AIAPIKey                    string
	AIBaseURL                   string
	RecallDifficulty            string
	RecallMaxQuestions          string
	HooksPreCommit              string
	HooksCommitMsg              string
	HooksPrePush                string
	ModeBlocking                string
	PresentationTerminal        string
	PresentationMCP             string
}

// DefaultUserConfig returns the safe-default UserConfig written on first init or auto-creation.
func DefaultUserConfig() UserConfig {
	return UserConfig{
		Privacy: PrivacyConfig{ConversationAnalysis: false},
		AI: AIConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
			APIKey:   "env:ANTHROPIC_API_KEY",
			BaseURL:  "",
		},
		Recall: RecallConfig{
			Difficulty:   "adaptive",
			MaxQuestions: 1,
		},
	}
}

// UserConfigPath returns the absolute path to the config.yaml in the Total
// Recall data directory. When TR_HOME is set, it resolves to $TR_HOME/config.yaml;
// otherwise ~/.tr/config.yaml.
func UserConfigPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// UserConfigDir returns the absolute path to the Total Recall data directory.
// When the TR_HOME environment variable is set to a non-empty path, it is used
// (enabling test/CI isolation). Otherwise the default ~/.tr is used.
func UserConfigDir() (string, error) {
	if env := os.Getenv("TR_HOME"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tr"), nil
}
