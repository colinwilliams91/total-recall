// Package config provides config schema, loading, and deep-merge logic for Total Recall.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// UserConfig is loaded from ~/.tr/config.yaml.
// It defines personal defaults that apply across all repositories.
type UserConfig struct {
	Privacy     PrivacyConfig     `yaml:"privacy"`
	AI          AIConfig          `yaml:"ai"`
	Recall      RecallConfig      `yaml:"recall"`
	PromptAsset PromptAssetConfig `yaml:"prompt-asset,omitempty"`
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
//		anthropic → internal/ai/anthropic (native Messages API)
//	 openai    → internal/ai/openai    (OpenAI Chat Completions API)
//		ollama    → internal/ai/openai    (OpenAI-compatible)
//		groq      → internal/ai/openai    (OpenAI-compatible)
//	 qwen      → internal/ai/openai    (OpenAI-compatible)
//	 minimax   → internal/ai/openai    (OpenAI-compatible)
//	 deepseek  → internal/ai/openai    (OpenAI-compatible)
//	 openrouter → internal/ai/openai   (OpenAI-compatible, unified model catalog)
//	 opencode   → internal/ai/openai   (OpenAI-compatible, Zen API / Go subscription)
//		lm-studio → internal/ai/openai    (OpenAI-compatible)
//		custom    → internal/ai/openai    (OpenAI-compatible; BaseURL required)
//
// For named presets, BaseURL in AIConfig is ignored — the registry value is used.
// For "custom", AIConfig.BaseURL is required; ai.New returns ErrNoProvider if empty.
var KnownProviders = map[string]string{
	"anthropic":  "https://api.anthropic.com",
	"openai":     "https://api.openai.com/v1",
	"ollama":     "http://localhost:11434/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"qwen":       "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"minimax":    "https://api.minimaxi.com/v1",
	"deepseek":   "https://api.deepseek.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"opencode":   "https://opencode.ai/zen/go/v1",
	"lm-studio":  "http://localhost:1234/v1",
	"custom":     "", // user must supply BaseURL
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

// PromptAssetConfig controls prompt-asset override observability.
// It is user-level only: a prompt-asset block in .tr.yaml is discarded with a
// warning, consistent with privacy.* and ai.*.
type PromptAssetConfig struct {
	DriftWarningDays int `yaml:"drift-warning-days"`
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
	Privacy     *PrivacyConfig     `yaml:"privacy,omitempty"`
	AI          *AIConfig          `yaml:"ai,omitempty"`
	PromptAsset *PromptAssetConfig `yaml:"prompt-asset,omitempty"`
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
	PromptAsset  PromptAssetConfig
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
	PromptAssetDriftWarningDays string
	HooksPreCommit              string
	HooksCommitMsg              string
	HooksPrePush                string
	ModeBlocking                string
	PresentationTerminal        string
	PresentationMCP             string
}

// defaultProvider is the zero-config AI provider. Anthropic stays the
// default: routing recall traffic through a free multi-model aggregator by
// default trades privacy for friction, and that is a product decision, not a
// safe default. See GTM.md Wave A2 / issue #34.
const defaultProvider = "anthropic"

// DefaultAIModel is the model paired with the default provider.
const DefaultAIModel = "claude-sonnet-4-5"

// DefaultAPIKeyEnv returns the zero-config api-key pattern for a provider:
// "env:" + upper-cased provider name + "_API_KEY". Hyphens become underscores
// so lm-studio resolves to env:LM_STUDIO_API_KEY. Keeping the pattern
// provider-derived (rather than a single hardcoded env var) means the default
// config is never pointing at the wrong provider's variable — issue #34.
func DefaultAPIKeyEnv(provider string) string {
	name := strings.ToUpper(strings.ReplaceAll(provider, "-", "_"))
	return "env:" + name + "_API_KEY"
}

// IsDefaultAPIKeyEnv reports whether s is exactly the unmodified default
// api-key placeholder of a known provider. Matching only known placeholders —
// not anything shaped like one — keeps init from re-keying a user's custom
// env var (e.g. env:MY_COMPANY_API_KEY) that happens to look like the
// pattern. Used to re-key when the user switches providers in init without
// touching their existing value; a custom env var name or raw key never
// matches.
func IsDefaultAPIKeyEnv(s string) bool {
	for provider := range KnownProviders {
		if s == DefaultAPIKeyEnv(provider) {
			return true
		}
	}
	return false
}

// DefaultUserConfig returns the safe-default UserConfig written on first init or auto-creation.
func DefaultUserConfig() UserConfig {
	return UserConfig{
		Privacy: PrivacyConfig{ConversationAnalysis: false},
		AI: AIConfig{
			Provider: defaultProvider,
			Model:    DefaultAIModel,
			APIKey:   DefaultAPIKeyEnv(defaultProvider),
			BaseURL:  "",
		},
		Recall: RecallConfig{
			Difficulty:   "adaptive",
			MaxQuestions: 1,
		},
		PromptAsset: PromptAssetConfig{
			DriftWarningDays: 90,
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

// DaemonPidPath returns the pidfile path in the data directory (same directory
// as config.yaml and memory.db). The daemon writes its PID there at startup
// and removes it at graceful exit; stop and status treat stale or missing
// files as advisory.
func DaemonPidPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.pid"), nil
}
