package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

// TestAliasCLIArgumentContracts covers cobra arg-count contracts for the
// alias surface: add takes exactly two arguments, remove exactly one, list
// none. Cases that pass arg-count validation are pointed at an unreachable
// daemon so the network error proves the path ran without touching anything.
func TestAliasCLIArgumentContracts(t *testing.T) {
	orig := daemonBaseURL
	daemonBaseURL = "http://localhost:0"
	defer func() { daemonBaseURL = orig }()

	cases := []struct {
		use     string
		args    []string
		wantArg bool
	}{
		{use: "add", args: []string{"only_alias"}, wantArg: true},
		{use: "add", args: []string{"only_alias", "extra", "third"}, wantArg: true},
		{use: "add", args: []string{"only_alias", "target"}, wantArg: false},
		{use: "remove", args: []string{}, wantArg: true},
		{use: "remove", args: []string{"one"}, wantArg: false},
		{use: "list", args: []string{"unexpected"}, wantArg: true},
	}
	for _, tc := range cases {
		t.Helper()
		cmd := aliasCmd()
		cmd.SetArgs(append([]string{tc.use}, tc.args...))
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		if tc.wantArg && err == nil {
			t.Fatalf("alias %s %v: expected arg-count error, got nil", tc.use, tc.args)
		}
		if !tc.wantArg && err == nil {
			t.Fatalf("alias %s %v: expected offline error (arg path ran), got nil", tc.use, tc.args)
		}
	}
}

// TestAliasCLISurfacesDaemonValidationErrors runs the thin client against a
// live test daemon: dangling targets and malformed alias shapes surface the
// store's error verbatim.
func TestAliasCLISurfacesDaemonValidationErrors(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	orig := daemonBaseURL
	daemonBaseURL = baseURL
	t.Cleanup(func() { daemonBaseURL = orig })

	// Dangling target.
	cmd := aliasCmd()
	cmd.SetArgs([]string{"add", "golang_interfaces", "never_minted"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected dangling-target error from CLI")
	}
	if !strings.Contains(err.Error(), "no registry identity") {
		t.Fatalf("expected verbatim store error, got %v", err)
	}

	// Malformed alias shape (needs a real target so shape validation fires).
	if err := store.Save(context.Background(), "/repo/test", "main", []cache.Fingerprint{
		{Concept: "real", Slug: "real_concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd = aliasCmd()
	cmd.SetArgs([]string{"add", "not a slug", "real_concept"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected malformed-alias error from CLI")
	} else if !strings.Contains(err.Error(), "[a-z0-9_]+") {
		t.Fatalf("expected verbatim shape error, got %v", err)
	}
}

// TestAliasCLIAddListRemoveFlow drives the full thin-client round trip
// against a live daemon: add → ordered, tab-separated list → remove →
// gone, including the no-op removal message.
func TestAliasCLIAddListRemoveFlow(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	orig := daemonBaseURL
	daemonBaseURL = baseURL
	t.Cleanup(func() { daemonBaseURL = orig })

	if err := store.Save(context.Background(), "/repo/test", "main", []cache.Fingerprint{
		{Concept: "beta", Slug: "b_target", Source: "code", Weight: 1.0},
		{Concept: "alpha", Slug: "a_target", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	runAlias := func(args ...string) (string, error) {
		cmd := aliasCmd()
		cmd.SetArgs(args)
		buf := &bytes.Buffer{}
		restore := captureStdout(buf)
		err := cmd.Execute()
		restore()
		return buf.String(), err
	}

	// Insert in reverse alphabetical order; list output must be sorted.
	if out, err := runAlias("add", "zz_alias", "b_target"); err != nil {
		t.Fatalf("alias add zz: %v", err)
	} else if !strings.Contains(out, "zz_alias -> b_target") {
		t.Fatalf("unexpected add output: %q", out)
	}
	if out, err := runAlias("add", "aa_alias", "a_target"); err != nil {
		t.Fatalf("alias add aa: %v", err)
	} else if !strings.Contains(out, "aa_alias -> a_target") {
		t.Fatalf("unexpected add output: %q", out)
	}

	out, err := runAlias("list")
	if err != nil {
		t.Fatalf("alias list: %v", err)
	}
	aa, zz := strings.Index(out, "aa_alias"), strings.Index(out, "zz_alias")
	if aa < 0 || zz < 0 {
		t.Fatalf("expected both aliases in list output, got %q", out)
	}
	if aa > zz {
		t.Fatalf("expected list ordered by alias, got %q", out)
	}
	for _, line := range []string{"aa_alias\ta_target", "zz_alias\tb_target"} {
		if !strings.Contains(out, line) {
			t.Fatalf("expected tab-separated mapping %q in list output %q", line, out)
		}
	}

	if out, err := runAlias("remove", "zz_alias"); err != nil {
		t.Fatalf("alias remove: %v", err)
	} else if !strings.Contains(out, "removed") {
		t.Fatalf("unexpected remove output: %q", out)
	}
	if out, err := runAlias("list"); err != nil {
		t.Fatalf("alias list after remove: %v", err)
	} else if strings.Contains(out, "zz_alias") {
		t.Fatalf("expected zz_alias gone, got %q", out)
	}

	if out, err := runAlias("remove", "zz_alias"); err != nil {
		t.Fatalf("alias remove unknown: %v", err)
	} else if !strings.Contains(out, "nothing to remove") {
		t.Fatalf("unexpected no-op output: %q", out)
	}
}

// TestAliasCLIUnavailableDaemon formats the offline hint naming 'torec
// serve' instead of a stack trace when the daemon is unreachable.
func TestAliasCLIUnavailableDaemon(t *testing.T) {
	orig := daemonBaseURL
	daemonBaseURL = "http://localhost:0"
	defer func() { daemonBaseURL = orig }()

	cmd := aliasCmd()
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected offline error")
	}
	if !strings.Contains(err.Error(), "torec serve") {
		t.Fatalf("expected offline hint, got %v", err)
	}
}
