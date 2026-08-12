// Command promptgen composes the recall synthesis prompt exactly as the
// daemon builds it, so promptfoo evals exercise the real production prompt
// rather than a YAML copy that can drift. It performs no AI calls itself;
// provider.js (evals/synthesis/provider.js) invokes it and then calls the
// configured model with the composed system+user turns.
//
// Usage:
//
//	promptgen --concepts '[{"concept":"event sourcing pattern","source":"code","weight":0.9}]' \
//	          --commit-msg "feat: add event ledger" --diff "$(cat fixture.go)" \
//	          --difficulty intermediate [--policy evals/synthesis/policies/variant.md]
//
// Without --policy the embedded question-generation-policy asset is used,
// exactly as the daemon does (honoring $TR_HOME/prompts overrides via
// assets.Load). With --policy, the file's raw bytes become the policy body,
// enabling promptfoo A/B comparison of policy variants without a rebuild.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/colinwilliams91/total-recall/assets"
	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/recall"
)

// conceptIn mirrors the subset of cache.ConceptRow that fixtures supply.
type conceptIn struct {
	Concept string  `json:"concept"`
	Source  string  `json:"source"`
	Weight  float64 `json:"weight"`
}

// composedOut is the JSON emitted to stdout for provider.js to consume.
type composedOut struct {
	System    string `json:"system"`
	User      string `json:"user"`
	MaxTokens int    `json:"max_tokens"`
}

// synthesisDiffSnippetMaxChars mirrors internal/engine's truncation budget so
// evals see the same diff snippet the daemon would.
const synthesisDiffSnippetMaxChars = 500

func main() {
	conceptsJSON := flag.String("concepts", "[]", "JSON array of {concept,source,weight}")
	commitMsg := flag.String("commit-msg", "", "commit message")
	diff := flag.String("diff", "", "diff snippet (truncated to 500 chars like the daemon)")
	difficulty := flag.String("difficulty", "intermediate", "difficulty level")
	policyFile := flag.String("policy", "", "path to a policy markdown file; empty = embedded default")
	flag.Parse()

	var in []conceptIn
	if err := json.Unmarshal([]byte(*conceptsJSON), &in); err != nil {
		log.Fatalf("promptgen: parse --concepts: %v", err)
	}
	concepts := make([]cache.ConceptRow, len(in))
	for i, c := range in {
		concepts[i] = cache.ConceptRow{Concept: c.Concept, Source: c.Source, Weight: c.Weight}
	}

	policyBody := ""
	if *policyFile != "" {
		raw, err := os.ReadFile(*policyFile)
		if err != nil {
			log.Fatalf("promptgen: read policy %s: %v", *policyFile, err)
		}
		policyBody = string(raw)
	} else {
		asset, err := assets.Load("question-generation-policy")
		if err != nil {
			log.Fatalf("promptgen: load embedded policy: %v", err)
		}
		policyBody = asset.Body
	}

	snippet := *diff
	if len(snippet) > synthesisDiffSnippetMaxChars {
		snippet = snippet[:synthesisDiffSnippetMaxChars] + "\n[… truncated …]"
	}

	req := recall.SynthesisRequest(concepts, *commitMsg, snippet, policyBody, *difficulty, "")
	out := composedOut{System: req.System, User: req.UserTurn, MaxTokens: req.MaxTokens}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(out); err != nil {
		log.Fatalf("promptgen: encode: %v", err)
	}
}
