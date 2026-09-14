package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/colinwilliams91/total-recall/assets"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// assetNamePattern is the allowlist for user-supplied asset names: a single
// lowercase-hyphenated token, matching shipped asset naming. Validating at
// the CLI boundary (before any path construction) means traversal-shaped or
// odd-character arguments are rejected outright instead of being contained
// accidentally by other checks.
var assetNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// validateAssetName rejects asset names that are not a single
// lowercase-hyphenated token. The error names the offending argument and
// teaches the expected form.
func validateAssetName(name string) error {
	if !assetNamePattern.MatchString(strings.TrimSpace(name)) {
		return fmt.Errorf("invalid asset name '%s' — expected a single lowercase-hyphenated name, e.g. 'question-generation-policy'",
			name)
	}
	return nil
}

// promptsDir returns the override slot directory: the Total Recall data dir's
// prompts/ ($TR_HOME when set, else ~/.tr). The bool result is false when the
// data dir cannot be resolved (no override could exist).
func promptsDir() (string, bool) {
	dir, err := config.UserConfigDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(dir, "prompts"), true
}

func assetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "Inspect and manage prompt-asset overrides",
		Long: `Inspect and manage prompt-asset overrides.

Every quiz is shaped by a markdown policy doc shipped inside the binary.
A same-named file in the data dir's prompts/ directory (~/.tr/prompts,
or $TR_HOME/prompts when TR_HOME is set) replaces the shipped default —
edit it and restart the daemon, no recompile.

  list            every asset's resolved source, path, and age
  sync <name>     copy the shipped policy into your override slot as a
                  starting point for re-tuning
  reset [<name>]  remove an override so the shipped default takes effect

reset and sync only touch files — restart 'tr serve' to pick up the
change. A startup OVERRIDE WARNING fires when a stale override is older
than the shipped policy by more than prompt-asset.drift-warning-days.`,
	}
	cmd.AddCommand(listAssetCmd(), resetAssetCmd(), syncAssetCmd())
	return cmd
}

// sourceInactive marks slot files that shadow no shipped asset: present on
// disk, but never loaded by the engine. Presentation-only — the assets
// package's Source vocabulary stays strictly about load origin.
const sourceInactive = "inactive"

func listAssetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List resolved prompt assets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			shipped := make(map[string]struct{}, 8)
			for _, name := range assets.EmbeddedNames() {
				shipped[name] = struct{}{}
			}
			for _, asset := range assets.LoadAll() {
				if asset.Source == assets.SourceFallback || asset.Path == "" {
					continue
				}
				name := strings.TrimSuffix(filepath.Base(asset.Path), ".md")
				source := asset.Source
				if _, ok := shipped[name]; !ok {
					source = sourceInactive
				}
				path := asset.Path
				age := assets.Age(asset.ModTime)
				if asset.Source == assets.SourceEmbedded {
					path = "<embedded>"
					age = "embedded"
				}
				fmt.Printf("%s\t%s\t%s\t%s\n", name, source, path, age)
			}
			return nil
		},
	}
}

func resetAssetCmd() *cobra.Command {
	var all, force bool

	cmd := &cobra.Command{
		Use:   "reset [<name>]",
		Short: "Remove an override so the embedded default takes effect on next daemon restart",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, ok := promptsDir()
			if !ok {
				return fmt.Errorf("could not resolve the Total Recall data dir (~/.tr) — nothing to reset")
			}
			if len(args) == 1 {
				name := args[0]
				if err := validateAssetName(name); err != nil {
					return err
				}
				return removeOverride(filepath.Join(dir, name+".md"), name)
			}
			return resetAllOverrides(dir, all, force)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Remove every override in the data dir's prompts/ directory")
	cmd.Flags().BoolVar(&force, "force", false, "Skip the confirmation prompt")

	return cmd
}

func removeOverride(path, name string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("no override for %s — nothing to reset\n", name)
			return nil
		}
		return fmt.Errorf("removing %s: %w", path, err)
	}
	fmt.Printf("[assets] removed override at %s; restart 'tr serve' to pick up the change\n", path)
	return nil
}

// resetAllOverrides removes override files in batch. Multi-file or non-TTY
// batch removal requires --all; an interactive TTY additionally confirms
// before touching anything unless --force is passed.
func resetAllOverrides(dir string, all, force bool) error {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("no overrides to reset")
		return nil
	}

	var paths []string
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		paths = append(paths, filepath.Join(dir, de.Name()))
	}
	if len(paths) == 0 {
		fmt.Println("no overrides to reset")
		return nil
	}

	tty := term.IsTerminal(int(os.Stdin.Fd()))
	if !all && (!tty || len(paths) > 1) {
		return fmt.Errorf("refusing to remove %d override(s) without --all — pass --all --force for batch removal", len(paths))
	}
	if tty && !force && !confirm(fmt.Sprintf("Remove %d prompt-asset override(s)?", len(paths))) {
		return fmt.Errorf("aborted — no overrides removed")
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing %s: %w", path, err)
		}
		fmt.Printf("[assets] removed override at %s; restart 'tr serve' to pick up the change\n", path)
	}
	return nil
}

func syncAssetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "sync [<name>]",
		Short: "Refresh an override with the canonical embedded default content",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("sync requires an explicit asset name")
			}
			name := args[0]
			if err := validateAssetName(name); err != nil {
				return err
			}

			dir, ok := promptsDir()
			if !ok {
				return fmt.Errorf("could not resolve the Total Recall data dir (~/.tr) — nothing to sync to")
			}

			embeddedBytes, ok := assets.Embedded(name)
			if !ok {
				return fmt.Errorf("embedded asset '%s' not found — run 'tr asset list' to see the available asset names", name)
			}

			target := filepath.Join(dir, name+".md")
			if fi, err := os.Stat(target); err == nil && fi.Size() > 0 && !force {
				return fmt.Errorf("%s exists and is non-empty — pass --force to overwrite, or 'tr asset reset %s' to start from defaults", target, name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("creating prompts dir: %w", err)
			}
			if err := os.WriteFile(target, embeddedBytes, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", target, err)
			}

			fmt.Printf("[assets] synced %s to %s; restart 'tr serve' to pick up the change\n", name, target)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing non-empty override")

	return cmd
}

// confirm asks a yes/no question on an interactive TTY. Non-TTY callers are
// handled before reaching this; a non-affirmative read counts as a no.
func confirm(prompt string) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}
	fmt.Printf("%s [y/N]: ", prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
