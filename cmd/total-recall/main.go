package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/engine"
	"github.com/colinwilliams91/total-recall/internal/hooks"
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
			cfg, err := config.Load(quiet)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			return engine.New(cfg).Start()
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

	// ─── Hook installation ────────────────────────────────────────────────────
	repoRoot, repoErr := hooks.FindRepoRoot()
	if repoErr != nil {
		fmt.Println("\n  ⚠  Not inside a Git repository — skipping hook installation.")
		fmt.Println("     Run 'total-recall init' from a Git repository root to enable hooks.")
		return nil
	}

	// Load existing repo config to pre-populate selections on re-run.
	existingRepoCfg, _ := config.LoadRepoConfigFromDir(repoRoot)
	var preCommit, commitMsg, prePush bool
	if existingRepoCfg != nil {
		preCommit = existingRepoCfg.Hooks.PreCommit
		commitMsg = existingRepoCfg.Hooks.CommitMsg
		prePush = existingRepoCfg.Hooks.PrePush
	} else {
		// Sensible default: enable pre-commit, leave others off.
		preCommit = true
	}

	fmt.Println()
	hookForm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable pre-commit hook?").
				Description("Triggers a recall check on every commit. Highest signal — runs most frequently.").
				Value(&preCommit),
			huh.NewConfirm().
				Title("Enable commit-msg hook?").
				Description("Enriches recall with your commit intent. Runs silently after pre-commit.").
				Value(&commitMsg),
			huh.NewConfirm().
				Title("Enable pre-push hook?").
				Description("Architecture-level recall across all commits in the push. Less frequent.").
				Value(&prePush),
		),
	)

	if err := hookForm.Run(); err != nil {
		return fmt.Errorf("hook prompt: %w", err)
	}

	repoCfg := &config.RepoConfig{}
	if existingRepoCfg != nil {
		*repoCfg = *existingRepoCfg
	}
	repoCfg.Hooks = config.HooksConfig{
		PreCommit: preCommit,
		CommitMsg: commitMsg,
		PrePush:   prePush,
	}

	if err := config.WriteRepoConfig(repoRoot, repoCfg); err != nil {
		return fmt.Errorf("writing .tr.yaml: %w", err)
	}
	fmt.Printf("✓ Repo config saved to %s/.tr.yaml\n", repoRoot)

	installer := hooks.NewInstaller(repoRoot)
	installed, err := installer.InstallEnabled(repoCfg.Hooks)
	if err != nil {
		return fmt.Errorf("installing hooks: %w", err)
	}
	if len(installed) == 0 {
		fmt.Println("  No hooks enabled — skipping hook installation.")
	} else {
		fmt.Printf("✓ Installed %d hook(s) into %s/.git/hooks/\n", len(installed), repoRoot)
		fmt.Println("  Start the daemon with: total-recall serve")
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
			return runStatus()
		},
	}
}

func runStatus() error {
	const healthURL = "http://localhost:7331/health"
	client := &http.Client{Timeout: 1 * time.Second}

	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Println("✗ Daemon not running on localhost:7331")
		fmt.Println("  Start with: total-recall serve")
		os.Exit(1)
	}
	defer resp.Body.Close()

	var health struct {
		Status string `json:"status"`
	}
	if jsonErr := json.NewDecoder(resp.Body).Decode(&health); jsonErr != nil || health.Status != "ok" {
		fmt.Println("✗ Daemon returned unexpected health response")
		os.Exit(1)
	}

	fmt.Println("✓ Daemon running on localhost:7331")

	cfg, err := config.Load(quiet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  (could not load config: %v)\n", err)
		return nil
	}
	config.Show(cfg, os.Stdout)
	return nil
}
