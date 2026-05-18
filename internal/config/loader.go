package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// RepoConfigFile is the per-repository config filename, safe to commit.
	RepoConfigFile = ".tr.yaml"

	// AdvisoryMessage is emitted (to stderr) when ~/.tr/config.yaml is auto-created
	// because the user bypassed `total-recall init`. Suppressed by --quiet.
	AdvisoryMessage = "\n⚠  No Total Recall user config found.\n" +
		"   Created ~/.tr/config.yaml with safe defaults.\n" +
		"   Run 'total-recall init' to configure your preferences.\n\n"
)

// LoadUserConfig reads and parses ~/.tr/config.yaml.
// It warns (to stderr) if ai.api-key contains a raw value rather than an env var reference.
// Returns an error if the file does not exist or cannot be parsed.
func LoadUserConfig() (*UserConfig, error) {
	path, err := UserConfigPath()
	if err != nil {
		return nil, fmt.Errorf("resolving user config path: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	warnRawAPIKey(&cfg.AI)
	return &cfg, nil
}

// EnsureUserConfig loads ~/.tr/config.yaml, auto-creating it with safe defaults
// if it does not yet exist. The advisory message is suppressed when quiet is true.
// This is the entry point used by daemon startup and hook invocations.
func EnsureUserConfig(quiet bool) (*UserConfig, error) {
	path, err := UserConfigPath()
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		cfg := DefaultUserConfig()
		if err := writeUserConfig(path, &cfg); err != nil {
			return nil, fmt.Errorf("auto-creating user config: %w", err)
		}
		if !quiet {
			fmt.Fprint(os.Stderr, AdvisoryMessage)
		}
		return &cfg, nil
	}
	return LoadUserConfig()
}

// WriteUserConfig serializes cfg to ~/.tr/config.yaml, creating ~/.tr/ if needed.
// Used by `total-recall init` after the user completes the setup prompt.
func WriteUserConfig(cfg *UserConfig) error {
	path, err := UserConfigPath()
	if err != nil {
		return err
	}
	return writeUserConfig(path, cfg)
}

// writeUserConfig is the internal write helper. It creates parent directories
// with mode 0700 and writes the file with mode 0600.
func writeUserConfig(path string, cfg *UserConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializing user config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadRepoConfig reads and parses .tr.yaml from the current directory.
// Returns (nil, nil) if the file does not exist — the caller should treat
// a nil RepoConfig as "no per-repo overrides".
func LoadRepoConfig() (*RepoConfig, error) {
	data, err := os.ReadFile(RepoConfigFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", RepoConfigFile, err)
	}
	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", RepoConfigFile, err)
	}
	return &cfg, nil
}

// Load returns the fully resolved Config for the current context.
// It ensures the user config exists (auto-creating with safe defaults if absent),
// loads the per-repo config, and deep-merges them.
func Load(quiet bool) (*Config, error) {
	user, err := EnsureUserConfig(quiet)
	if err != nil {
		return nil, err
	}
	repo, err := LoadRepoConfig()
	if err != nil {
		return nil, err
	}
	return Merge(user, repo), nil
}

// warnRawAPIKey emits a stderr warning if ai.api-key is a non-empty value
// that does not use the recommended env:<VAR_NAME> pattern.
func warnRawAPIKey(ai *AIConfig) {
	if ai.APIKey == "" {
		return
	}
	if strings.HasPrefix(ai.APIKey, "env:") {
		return
	}
	fmt.Fprintf(os.Stderr,
		"⚠  ai.api-key in ~/.tr/config.yaml appears to be a raw value.\n"+
			"   Use the env:<VAR_NAME> pattern instead (e.g., env:ANTHROPIC_API_KEY).\n",
	)
}
