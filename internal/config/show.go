package config

import (
	"fmt"
	"io"
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

	fmt.Fprintln(w, "recall:")
	fmt.Fprintf(w, "  difficulty: %s  # \033[0;34m%s\033[0m\n", cfg.Recall.Difficulty, s.RecallDifficulty)
	fmt.Fprintf(w, "  max_questions: %d  # \033[0;34m%s\033[0m\n", cfg.Recall.MaxQuestions, s.RecallMaxQuestions)

	fmt.Fprintln(w, "hooks:")
	fmt.Fprintf(w, "  pre-commit: %v  # \033[0;34m%s\033[0m\n", cfg.Hooks.PreCommit, s.HooksPreCommit)
	fmt.Fprintf(w, "  commit-msg: %v  # \033[0;34m%s\033[0m\n", cfg.Hooks.CommitMsg, s.HooksCommitMsg)
	fmt.Fprintf(w, "  pre-push: %v  # \033[0;34m%s\033[0m\n", cfg.Hooks.PrePush, s.HooksPrePush)

	fmt.Fprintln(w, "mode:")
	fmt.Fprintf(w, "  blocking: %v  # \033[0;34m%s\033[0m\n", cfg.Mode.Blocking, s.ModeBlocking)

	fmt.Fprintln(w, "presentation:")
	fmt.Fprintf(w, "  terminal: %v  # \033[0;34m%s\033[0m\n", cfg.Presentation.Terminal, s.PresentationTerminal)
	fmt.Fprintf(w, "  mcp: %v  # \033[0;34m%s\033[0m\n", cfg.Presentation.MCP, s.PresentationMCP)
}
