package config

import (
	"fmt"
	"io"
)

// Show writes the fully resolved config to w with inline source annotations.
// Each value is annotated with where it came from: "# user", "# repo", or "# default".
// The AI api-key is shown as its raw configured value (env: reference or empty), never resolved.
func Show(cfg *Config, w io.Writer) {
	s := cfg.Sources

	fmt.Fprintln(w, "privacy:")
	fmt.Fprintf(w, "  conversation_analysis: %v  # %s\n",
		cfg.Privacy.ConversationAnalysis, s.PrivacyConversationAnalysis)

	fmt.Fprintln(w, "ai:")
	fmt.Fprintf(w, "  provider: %s  # %s\n", cfg.AI.Provider, s.AIProvider)
	fmt.Fprintf(w, "  model: %s  # %s\n", cfg.AI.Model, s.AIModel)
	apiKeyDisplay := cfg.AI.APIKey
	if apiKeyDisplay == "" {
		apiKeyDisplay = "<not set>"
	}
	fmt.Fprintf(w, "  api-key: %s  # %s\n", apiKeyDisplay, s.AIAPIKey)

	fmt.Fprintln(w, "recall:")
	fmt.Fprintf(w, "  difficulty: %s  # %s\n", cfg.Recall.Difficulty, s.RecallDifficulty)
	fmt.Fprintf(w, "  max_questions: %d  # %s\n", cfg.Recall.MaxQuestions, s.RecallMaxQuestions)

	fmt.Fprintln(w, "hooks:")
	fmt.Fprintf(w, "  pre-commit: %v  # %s\n", cfg.Hooks.PreCommit, s.HooksPreCommit)
	fmt.Fprintf(w, "  commit-msg: %v  # %s\n", cfg.Hooks.CommitMsg, s.HooksCommitMsg)
	fmt.Fprintf(w, "  pre-push: %v  # %s\n", cfg.Hooks.PrePush, s.HooksPrePush)

	fmt.Fprintln(w, "mode:")
	fmt.Fprintf(w, "  blocking: %v  # %s\n", cfg.Mode.Blocking, s.ModeBlocking)

	fmt.Fprintln(w, "presentation:")
	fmt.Fprintf(w, "  terminal: %v  # %s\n", cfg.Presentation.Terminal, s.PresentationTerminal)
	fmt.Fprintf(w, "  mcp: %v  # %s\n", cfg.Presentation.MCP, s.PresentationMCP)
}
