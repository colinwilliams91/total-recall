// Package assets loads prompt-asset markdown files shipped under assets/prompts/
// and overlays runtime overrides from $TR_HOME/prompts/. The canonical asset
// today is question-generation-policy.md; future prompt assets follow the same
// loader pattern.
//
// Embedded defaults ship inside the binary via //go:embed; a developer iterating
// on question style drops a replacement at $TR_HOME/prompts/<name>.md and
// restarts the daemon — no recompile. Loading is cached per-process so the
// synthesis hot path pays no per-call IO.
package assets

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed prompts/*.md
var embedFS embed.FS

// Source identifies where a PromptAsset was loaded from.
const (
	SourceEmbedded = "embedded" // shipped inside the binary via //go:embed
	SourceOverride = "$TR_HOME" // loaded from $TR_HOME/prompts/<name>.md
	SourceFallback = "fallback" // no source available; caller falls back to inline template
)

// PromptAsset is a parsed prompt-asset markdown file.
type PromptAsset struct {
	Name        string    // front-matter `name` value (best-effort; empty when absent)
	Description string    // front-matter `description` value (best-effort; empty when absent)
	Body        string    // post-front-matter markdown; empty when no source available
	Source      string    // one of SourceEmbedded, SourceOverride, SourceFallback
	Path        string    // resolved location: embed FS path for embedded, absolute file path for override; empty for fallback
	ModTime     time.Time // override file mtime for drift/age reporting; zero for embedded/fallback
}

// driftThresholdDays is the override-staleness threshold in days above which
// Load logs an OVERRIDE WARNING. It defaults to 90 so the package is useful
// with no wiring; SetDriftThreshold lets the daemon apply the user's
// prompt-asset.drift-warning-days. Zero disables the warning.
var (
	driftThresholdDays = 90

	// processStart is the fallback compile-time reference when the running
	// binary's own mtime cannot be determined.
	processStart = time.Now()
)

// SetDriftThreshold sets the override-staleness warning threshold in days.
// Values below zero are clamped to zero, which disables the warning.
func SetDriftThreshold(days int) {
	if days < 0 {
		days = 0
	}
	driftThresholdDays = days
}

// cache holds parsed assets keyed by name so repeated Load calls do not re-read
// disk or re-parse markdown. The cache lives for the process lifetime — restart
// the daemon to pick up disk changes.
var (
	cacheMu sync.Mutex
	entries = make(map[string]PromptAsset)
)

// Load resolves a named prompt asset (<name>.md) by checking $TR_HOME/prompts/
// first when $TR_HOME is set, then falling back to the //go:embed-ed default,
// then returning a SourceFallback sentinel when neither is available. Results
// are cached per-name so the first call pays the parse cost and subsequent calls
// return the cached PromptAsset value.
//
// A non-nil error is returned alongside the fallback sentinel when no source is
// available; callers are expected to log and continue with their inline fallback
// template rather than aborting.
func Load(name string) (PromptAsset, error) {
	cacheMu.Lock()
	if cached, ok := entries[name]; ok {
		cacheMu.Unlock()
		return cached, nil
	}
	cacheMu.Unlock()

	asset, err := resolve(name)

	cacheMu.Lock()
	entries[name] = asset
	cacheMu.Unlock()

	return asset, err
}

// resolve performs the actual disk/embed read and front-matter parse. It is
// called once per name on the first Load; subsequent calls hit the cache.
func resolve(name string) (PromptAsset, error) {
	asset := resolveQuiet(name)

	switch asset.Source {
	case SourceOverride:
		log.Printf("[assets] prompt asset %q loaded from override at %s (mtime=%s, embedded ref=%s)",
			name, asset.Path, asset.ModTime.UTC().Format(time.RFC3339), embeddedRefTime().UTC().Format(time.RFC3339))
		warnOnStaleOverride(asset.Path, name, asset.ModTime)
		return asset, nil
	case SourceEmbedded:
		return asset, nil
	default:
		log.Printf("[assets] prompt asset %q not found, falling back to inline synthesis template", name)
		return asset, fmt.Errorf("assets: prompt asset %q not found in override or embedded defaults", name)
	}
}

// resolveQuiet resolves an asset without logging and without touching the
// package cache. LoadAll uses it so inspection surfaces (`tr asset list`,
// `tr config show`) read fresh disk state on every invocation.
func resolveQuiet(name string) PromptAsset {
	if trHome, ok := os.LookupEnv("TR_HOME"); ok && trHome != "" {
		overridePath := filepath.Join(trHome, "prompts", name+".md")
		if fi, err := os.Stat(overridePath); err == nil && !fi.IsDir() {
			if b, err := os.ReadFile(overridePath); err == nil {
				asset := parseAsset(string(b))
				asset.Source = SourceOverride
				asset.Path = overridePath
				asset.ModTime = fi.ModTime()
				return asset
			}
		}
	}

	embeddedPath := "prompts/" + name + ".md"
	if b, err := embedFS.ReadFile(embeddedPath); err == nil {
		asset := parseAsset(string(b))
		asset.Source = SourceEmbedded
		asset.Path = embeddedPath
		return asset
	}

	return PromptAsset{Source: SourceFallback}
}

// LoadAll returns one resolved PromptAsset per distinct asset name across the
// embedded defaults and the $TR_HOME/prompts/ override directory (when
// TR_HOME is set). Overrides win on name collisions. Resolution is fresh on
// every call — no package cache — so inspection surfaces always reflect the
// current on-disk state.
func LoadAll() []PromptAsset {
	nameSet := make(map[string]struct{})
	for _, name := range EmbeddedNames() {
		nameSet[name] = struct{}{}
	}
	if trHome, ok := os.LookupEnv("TR_HOME"); ok && trHome != "" {
		if dirEntries, err := os.ReadDir(filepath.Join(trHome, "prompts")); err == nil {
			for _, de := range dirEntries {
				if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
					continue
				}
				nameSet[strings.TrimSuffix(de.Name(), ".md")] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]PromptAsset, 0, len(names))
	for _, name := range names {
		out = append(out, resolveQuiet(name))
	}
	return out
}

// EmbeddedNames lists the prompt-asset names compiled into the binary, sorted.
func EmbeddedNames() []string {
	matches, err := fs.Glob(embedFS, "prompts/*.md")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, strings.TrimSuffix(filepath.Base(m), ".md"))
	}
	sort.Strings(names)
	return names
}

// Embedded returns the raw embedded bytes for a named prompt asset. The bool
// result is false when the name is not part of the shipped defaults.
func Embedded(name string) ([]byte, bool) {
	b, err := embedFS.ReadFile("prompts/" + name + ".md")
	if err != nil {
		return nil, false
	}
	return b, true
}

// embeddedRefTime approximates the compile-time reference of the embedded
// defaults. embed.FS entries carry no usable mtime at runtime, so the running
// binary's own mtime is used — the binary is built from the embedded assets,
// which makes it exactly "the build the override predates". Drift reported
// against it answers the user-facing question: "is my override older than the
// Total Recall I am now running?"
func embeddedRefTime() time.Time {
	if exe, err := os.Executable(); err == nil {
		if fi, err := os.Stat(exe); err == nil && !fi.ModTime().IsZero() {
			return fi.ModTime()
		}
	}
	return processStart
}

// warnOnStaleOverride emits the OVERRIDE WARNING log line when an override's
// mtime is older than the embedded default's build reference by more than the
// configured threshold. Purely informational — never blocks, never mutates.
func warnOnStaleOverride(overridePath, name string, modTime time.Time) {
	if driftThresholdDays <= 0 {
		return
	}
	daysOld := int(math.Round(embeddedRefTime().Sub(modTime).Hours() / 24))
	if daysOld <= driftThresholdDays {
		return
	}
	log.Printf("[assets] OVERRIDE WARNING: %s is %dd older than the embedded default — re-sync with 'tr asset sync %s'",
		overridePath, daysOld, name)
}

// Age renders a human-readable mtime-relative age ("2d old", "6mo old") for
// display in `tr asset list` and `tr config show`. A zero mtime renders as
// "unknown" — callers substitute "embedded" when the asset's source is the
// embedded default.
func Age(modTime time.Time) string {
	if modTime.IsZero() {
		return "unknown"
	}
	d := time.Since(modTime)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm old", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh old", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd old", int(d.Hours()/24))
	case d < 730*24*time.Hour:
		return fmt.Sprintf("%dmo old", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy old", int(d.Hours()/24/365))
	}
}

// parseAsset splits a raw markdown file into front-matter metadata and body.
// Front matter is a leading `---\n...\n---` block; the two known keys are
// `name` and `description`. Missing or malformed front matter is non-fatal —
// the asset is returned with empty Name/Description and Body containing the
// full file content. No YAML library is introduced; the parse is a line-by-line
// scan for the two known keys.
func parseAsset(raw string) PromptAsset {
	const opener = "---"
	lines := strings.Split(raw, "\n")

	if len(lines) < 2 || strings.TrimSpace(lines[0]) != opener {
		return PromptAsset{Body: raw}
	}

	asset := PromptAsset{}
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == opener {
			endIdx = i
			break
		}
		key, value, ok := splitKeyValue(lines[i])
		if !ok {
			continue
		}
		switch key {
		case "name":
			asset.Name = value
		case "description":
			asset.Description = value
		}
	}

	if endIdx == -1 {
		return PromptAsset{Body: raw}
	}

	asset.Body = strings.Join(lines[endIdx+1:], "\n")
	return asset
}

// splitKeyValue parses a single `key: value` line from front matter. Returns
// ok=false when the line is not in that form. Trims whitespace around both
// halves; value may be empty.
func splitKeyValue(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}
