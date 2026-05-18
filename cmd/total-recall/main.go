package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/spf13/cobra"
)

var version = "dev"

// quiet is set by the --quiet/-q persistent flag and suppresses advisory messages.
var quiet bool

func main() {
	root := &cobra.Command{
		Use:     "total-recall",
		Short:   "Total Recall — Preserve engineering cogitation in the age of AI-assisted coding",
		Version: version,
	}

	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false,
		"Suppress advisory messages (auto-created config notices, etc.)")

	root.AddCommand(
		serveCmd(),
		initCmd(),
		configCmd(),
		statusCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Total Recall daemon on localhost:7331",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.EnsureUserConfig(quiet); err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			fmt.Println("not implemented")
			return nil
		},
	}
}

// conversationAnalysisPrompt is the description shown in the init opt-in prompt.
// Simple, earnest language — no legal jargon.
const conversationAnalysisPrompt = `Would you like Total Recall to analyze your AI assistant
conversations to generate smarter, more relevant quiz questions?

When enabled, the model looks at what you and your AI discuss and extracts
only the concepts — nothing else is kept. Raw conversation text
is never stored.

You can change this anytime by editing ~/.tr/config.yaml.`

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Total Recall for this project and create user config",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit()
		},
	}
}

func runInit() error {
	cfgPath, err := config.UserConfigPath()
	if err != nil {
		return err
	}

	// Load existing config so we can preserve any values the user already has.
	// If no config exists yet, start from safe defaults.
	existing, loadErr := config.LoadUserConfig()
	if loadErr != nil {
		existing = nil
	}

	cfg := config.DefaultUserConfig()
	if existing != nil {
		cfg = *existing
	}

	// Prompt the user about conversation analysis opt-in.
	var enableConversationAnalysis bool
	if existing != nil {
		enableConversationAnalysis = existing.Privacy.ConversationAnalysis
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("🧠  Enable conversation analysis?").
				Description(conversationAnalysisPrompt).
				Value(&enableConversationAnalysis),
		),
	)

	if err := form.Run(); err != nil {
		return fmt.Errorf("prompt: %w", err)
	}

	cfg.Privacy.ConversationAnalysis = enableConversationAnalysis

	if err := config.WriteUserConfig(&cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("✓ User config saved to %s\n", cfgPath)
	if enableConversationAnalysis {
		fmt.Println("  Conversation analysis: enabled")
	} else {
		fmt.Println("  Conversation analysis: disabled (enable anytime in ~/.tr/config.yaml)")
	}
	return nil
}

func configCmd() *cobra.Command {
	var show bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and write Total Recall config values",
		RunE: func(cmd *cobra.Command, args []string) error {
			if show {
				return runConfigShow()
			}
			return cmd.Help()
		},
	}

	cmd.Flags().BoolVar(&show, "show", false,
		"Print the fully resolved config with source annotations (user / repo / default)")

	return cmd
}

func runConfigShow() error {
	cfg, err := config.Load(quiet)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	config.Show(cfg, os.Stdout)
	return nil
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status and active config",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented")
			return nil
		},
	}
}
