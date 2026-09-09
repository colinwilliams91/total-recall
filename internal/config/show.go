package config

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/colinwilliams91/total-recall/assets"
)

// Show writes the fully resolved config to w with inline source annotations.
// Each value is annotated with where it came from: "# [user]", "# [repo]", or "# [default]".
// The AI api-key is shown as its raw configured value (env: reference or empty), never resolved.
func Show(cfg *Config, w io.Writer) {
	s := cfg.Sources

	fmt.Fprintln(w, "🧠🔧YAML file values suffixed by scope, e.g. \033[0;34m [user] \033[0m level or \033[0;34m [repo] \033[0m level...")
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "privacy:")
	fmt.Fprintf(w, "  conversation_analysis: %v  # \033[0;34m%s\033[0m\n",
		cfg.Privacy.ConversationAnalysis, s.PrivacyConversationAnalysis)

	fmt.Fprintln(w, "ai:")
	fmt.Fprintf(w, "  provider: %s  # \033[0;34m%s\033[0m\n", cfg.AI.Provider, s.AIProvider)
	fmt.Fprintf(w, "  model: %s  # \033[0;34m%s\033[0m\n", cfg.AI.Model, s.AIModel)
	apiKeyDisplay := cfg.AI.APIKey
	if apiKeyDisplay == "" {
		apiKeyDisplay = "<not set>"
	}
	fmt.Fprintf(w, "  api-key: %s  # \033[0;34m%s\033[0m\n", apiKeyDisplay, s.AIAPIKey)
	if cfg.AI.BaseURL != "" {
		fmt.Fprintf(w, "  base-url: %s  # \033[0;34m%s\033[0m\n", cfg.AI.BaseURL, s.AIBaseURL)
	}

	fmt.Fprintln(w, "recall:")
	fmt.Fprintf(w, "  difficulty: %s  # \033[0;34m%s\033[0m\n", cfg.Recall.Difficulty, s.RecallDifficulty)
	fmt.Fprintf(w, "  max_questions: %d  # \033[0;34m%s\033[0m\n", cfg.Recall.MaxQuestions, s.RecallMaxQuestions)

	fmt.Fprintln(w, "prompt-asset:")
	fmt.Fprintf(w, "  drift-warning-days: %d  # \033[0;34m%s\033[0m\n", cfg.PromptAsset.DriftWarningDays, s.PromptAssetDriftWarningDays)

	fmt.Fprintln(w, "hooks:")
	fmt.Fprintf(w, "  pre-commit: %v  # \033[0;34m%s\033[0m\n", cfg.Hooks.PreCommit, s.HooksPreCommit)
	fmt.Fprintf(w, "  commit-msg: %v  # \033[0;34m%s\033[0m\n", cfg.Hooks.CommitMsg, s.HooksCommitMsg)
	fmt.Fprintf(w, "  pre-push: %v  # \033[0;34m%s\033[0m\n", cfg.Hooks.PrePush, s.HooksPrePush)

	fmt.Fprintln(w, "mode:")
	fmt.Fprintf(w, "  blocking: %v  # \033[0;34m%s\033[0m\n", cfg.Mode.Blocking, s.ModeBlocking)

	fmt.Fprintln(w, "presentation:")
	fmt.Fprintf(w, "  terminal: %v  # \033[0;34m%s\033[0m\n", cfg.Presentation.Terminal, s.PresentationTerminal)
	fmt.Fprintf(w, "  mcp: %v  # \033[0;34m%s\033[0m\n", cfg.Presentation.MCP, s.PresentationMCP)

	writePromptAssetsSection(w)
}

// writePromptAssetsSection prints one line per embedded-known prompt asset
// with its resolved source, path, and age, so a debugging user sees an active
// override in `tr config show` instead of digging through daemon logs. The
// section lives at the end of the show output to keep the config scan compact.
// Resolution is fresh (no asset cache), so the section always reflects disk.
func writePromptAssetsSection(w io.Writer) {
	resolved := make(map[string]assets.PromptAsset)
	for _, asset := range assets.LoadAll() {
		name := strings.TrimSuffix(filepath.Base(asset.Path), ".md")
		resolved[name] = asset
	}

	fmt.Fprintln(w, "prompt assets:")
	for _, name := range assets.EmbeddedNames() {
		asset, ok := resolved[name]
		if !ok {
			continue
		}
		tag := asset.Source
		path := asset.Path
		age := assets.Age(asset.ModTime)
		switch asset.Source {
		case assets.SourceEmbedded:
			tag = "embedded"
			path = "<embedded>"
			age = "embedded"
		case assets.SourceOverride:
			tag = "override"
		case assets.SourceFallback:
			tag = "fallback"
			path = "<embedded>"
		}
		fmt.Fprintf(w, "  %s: %s  # \033[0;34m[%s]\033[0m, %s\n", name, path, tag, age)
	}
}
