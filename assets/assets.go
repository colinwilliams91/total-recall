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
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	Name        string // front-matter `name` value (best-effort; empty when absent)
	Description string // front-matter `description` value (best-effort; empty when absent)
	Body        string // post-front-matter markdown; empty when no source available
	Source      string // one of SourceEmbedded, SourceOverride, SourceFallback
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
	if trHome, ok := os.LookupEnv("TR_HOME"); ok && trHome != "" {
		overridePath := filepath.Join(trHome, "prompts", name+".md")
		if b, err := os.ReadFile(overridePath); err == nil {
			asset := parseAsset(string(b))
			asset.Source = SourceOverride
			log.Printf("[assets] prompt asset %q loaded from override at %s", name, overridePath)
			return asset, nil
		}
	}

	embeddedBytes, err := embedFS.ReadFile("prompts/" + name + ".md")
	if err == nil {
		asset := parseAsset(string(embeddedBytes))
		asset.Source = SourceEmbedded
		return asset, nil
	}

	log.Printf("[assets] prompt asset %q not found, falling back to inline synthesis template", name)
	return PromptAsset{Source: SourceFallback}, fmt.Errorf("assets: prompt asset %q not found in override or embedded defaults", name)
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
