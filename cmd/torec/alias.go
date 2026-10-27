package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// The alias surface is the concept-identity repair tool: users spot a split
// concept (two slugs for one idea) and merge it going forward with one
// command. All validation lives on the daemon's store route — this client
// passes arguments through and surfaces errors verbatim.

func aliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage concept identity aliases (the split-concept repair tool)",
		Long: `Manage concept identity aliases.

A concept identity is one canonical slug in the registry. When extraction
mints two different slugs for the same underlying idea (e.g.
go_interfaces and golang_interfaces), an alias redirects one name onto
the other for all future sightings:

  alias add <alias> <target>   merge alias onto an existing identity
  alias list                   show all alias mappings
  alias remove <alias>         delete an alias mapping

The merge is forward-looking: past sightings keep the slug they were
recorded with.`,
	}
	cmd.AddCommand(addAliasCmd(), listAliasesCmd(), removeAliasCmd())
	return cmd
}

func addAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <alias> <target>",
		Short: "Map alias onto an existing concept identity",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias, target := args[0], args[1]
			body := fmt.Sprintf(`{"alias":%q,"target":%q}`, alias, target)
			var out struct {
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}
			if err := aliasRequest(http.MethodPost, "/alias/add", body, &out); err != nil {
				return err
			}
			if out.Error != "" {
				return fmt.Errorf("alias add failed: %s", out.Error)
			}
			fmt.Printf("✓ alias %s -> %s (future sightings land on %s)\n", alias, target, target)
			return nil
		},
	}
}

func removeAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <alias>",
		Short: "Delete an alias mapping",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := fmt.Sprintf(`{"alias":%q}`, args[0])
			var out struct {
				OK      bool `json:"ok"`
				Removed bool `json:"removed"`
			}
			if err := aliasRequest(http.MethodPost, "/alias/remove", body, &out); err != nil {
				return err
			}
			if !out.Removed {
				fmt.Printf("no alias %s found — nothing to remove\n", args[0])
				return nil
			}
			fmt.Printf("✓ alias %s removed\n", args[0])
			return nil
		},
	}
}

func listAliasesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List alias mappings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var out struct {
				Aliases []struct {
					Alias  string `json:"alias"`
					Target string `json:"target"`
				} `json:"aliases"`
			}
			if err := aliasRequest(http.MethodGet, "/alias/list", "", &out); err != nil {
				return err
			}
			if len(out.Aliases) == 0 {
				fmt.Println("no aliases — registry identities resolve by exact slug")
				return nil
			}
			for _, a := range out.Aliases {
				fmt.Printf("%s\t%s\n", a.Alias, a.Target)
			}
			return nil
		},
	}
}

// aliasRequest posts/hits the daemon's alias routes, keeping the error
// message payload intact so validation errors (dangling target, malformed
// alias shape) surface verbatim from the store.
func aliasRequest(method, path, body string, out any) error {
	client := &http.Client{Timeout: 5 * time.Second}
	var req *http.Request
	var err error
	if body == "" {
		req, err = http.NewRequest(method, daemonBaseURL+path, nil)
	} else {
		req, err = http.NewRequest(method, daemonBaseURL+path, strings.NewReader(body))
	}
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("daemon not running on %s — start with 'torec serve'", daemonBaseURL)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("alias route error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("parsing alias response: %w", err)
	}
	if resp.StatusCode == http.StatusBadRequest {
		// The route wrapped the validation error as JSON; relay it.
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		return fmt.Errorf("alias add failed: %s", errBody.Error)
	}
	return nil
}
