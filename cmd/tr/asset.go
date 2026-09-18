package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/colinwilliams91/total-recall/assets"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/spf13/cobra"
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

// soleAssetFrom resolves the no-argument target when the shipped asset set
// is enumerated: exactly one name wins; zero or several are refused with a
// message listing the actual inventory. Decision by enumeration, not a
// hardcoded constant — the rule survives a future second asset shipping.
func soleAssetFrom(names []string) (string, error) {
	switch len(names) {
	case 0:
		return "", fmt.Errorf("no shipped prompt assets — nothing to sync to")
	case 1:
		return names[0], nil
	default:
		return "", fmt.Errorf("multiple shipped prompt assets — specify one of: %s",
			strings.Join(names, ", "))
	}
}

// soleShippedAsset is soleAssetFrom against the binary's embedded inventory.
func soleShippedAsset() (string, error) {
	return soleAssetFrom(assets.EmbeddedNames())
}

func assetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "asset",
		Short: "Inspect and manage prompt-asset overrides",
		Long: `Inspect and manage your prompt-asset override.

Every quiz is shaped by a markdown policy doc shipped inside the binary —
one policy, one file. A same-named file in the data dir's prompts/
directory (~/.tr/prompts, or $TR_HOME/prompts when TR_HOME is set)
replaces the shipped default — that slot is the deployment target for
your own tuning. You never type an asset name: 'sync' copies the shipped
policy into the slot, names the file, and you edit it from there.
Anything else you drop in the slot is listed 'inactive' — present on
disk, ignored by quizzes — and 'reset <name>' cleans up strays.

  show            what is loaded: resolved source, path, and age
  sync            copy the shipped policy into your slot as a starting
                  point for re-tuning  (or: sync <name>)
  reset           remove the override so the shipped default takes
                  effect on next daemon restart  (or: reset <name>)

reset and sync only touch files — restart 'tr serve' to pick up the
change. A startup OVERRIDE WARNING fires when a stale override is older
than the shipped policy by more than prompt-asset.drift-warning-days.`,
	}
	cmd.AddCommand(showAssetCmd(), resetAssetCmd(), syncAssetCmd())
	return cmd
}

// sourceInactive marks slot files that shadow no shipped asset: present on
// disk, but never loaded by the engine. Presentation-only — the assets
// package's Source vocabulary stays strictly about load origin.
const sourceInactive = "inactive"

func showAssetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show resolved prompt assets",
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
			name, err := soleShippedAsset()
			if err != nil {
				return err
			}
			return removeOverride(filepath.Join(dir, name+".md"), name)
		},
	}

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

func syncAssetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "sync [<name>]",
		Short: "Refresh an override with the canonical embedded default content",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) == 1 {
				name = args[0]
				if err := validateAssetName(name); err != nil {
					return err
				}
			} else {
				resolved, err := soleShippedAsset()
				if err != nil {
					return err
				}
				name = resolved
			}

			dir, ok := promptsDir()
			if !ok {
				return fmt.Errorf("could not resolve the Total Recall data dir (~/.tr) — nothing to sync to")
			}

			embeddedBytes, ok := assets.Embedded(name)
			if !ok {
				return fmt.Errorf("embedded asset '%s' not found — run 'tr asset show' to see the available asset names", name)
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
