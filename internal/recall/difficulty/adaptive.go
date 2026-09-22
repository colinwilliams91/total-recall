package difficulty

import (
	"context"
	"log"
	"strings"
)

const (
	adaptiveHighClusterWeight         = 0.7
	adaptiveDispersedWeight           = 0.3
	adaptiveMinHighClusterConcepts    = 3
	adaptiveMinDispersedConcepts      = 5
	adaptiveMinAllCodeSourceConcepts  = 5
	adaptiveDelegationMsgMaxChars     = 50
	adaptiveDelegationSnippetMinChars = 400
)

const (
	aiDelegationSignal = "ai-delegation"
	highClusterSignal  = "high-cluster"
	dispersedSignal    = "dispersed"
	allCodeSignal      = "all-code"
	fallbackSignal     = "fallback"
)

// Adaptive selects the difficulty from deterministic signals over the
// SynthesisContext — a large AI-generated-looking diff with a terse commit
// message, a tight cluster of high-weight concepts, a broad spray of
// low-weight ones, or a uniformly code-sourced concept mix. Heuristics are
// first-match; the fallback is "intermediate".
type Adaptive struct{}

// Resolve applies the heuristic table in first-match order and returns one
// of "easy", "intermediate", or "hard". A per-call log line names the matched
// signal so a developer can see why their questions came back easy or hard.
func (Adaptive) Resolve(_ context.Context, synth SynthesisContext) string {
	level, signals := resolveAdaptive(synth)
	log.Printf("[recall] adaptive resolver selected %q (signals: %s)", level, strings.Join(signals, ", "))
	return level
}

// resolveAdaptive evaluates the heuristic table in order: AI-delegation wins
// outright, then high-cluster, then dispersed, then the all-code escalation
// of the fallback, then the bare fallback. Returns the level and the names of
// the heuristics that fired (exactly one, given first-match ordering).
func resolveAdaptive(synth SynthesisContext) (string, []string) {
	if len(synth.CommitMsg) < adaptiveDelegationMsgMaxChars && len(synth.DiffSnippet) > adaptiveDelegationSnippetMinChars {
		return "hard", []string{aiDelegationSignal}
	}

	total := 0.0
	allCode := len(synth.Concepts) > 0
	for _, c := range synth.Concepts {
		total += c.Weight
		if c.Source != "code" {
			allCode = false
		}
	}
	avg := 0.0
	if len(synth.Concepts) > 0 {
		avg = total / float64(len(synth.Concepts))
	}

	if len(synth.Concepts) >= adaptiveMinHighClusterConcepts && avg > adaptiveHighClusterWeight {
		return "hard", []string{highClusterSignal}
	}
	if len(synth.Concepts) >= adaptiveMinDispersedConcepts && avg < adaptiveDispersedWeight {
		return "easy", []string{dispersedSignal}
	}
	if len(synth.Concepts) >= adaptiveMinAllCodeSourceConcepts && allCode {
		return "hard", []string{allCodeSignal}
	}
	return "intermediate", []string{fallbackSignal}
}
