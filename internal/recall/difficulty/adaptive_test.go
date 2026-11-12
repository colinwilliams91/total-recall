package difficulty

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

// longSnippet builds a synthetic diff snippet of exactly n characters.
func longSnippet(n int) string {
	return strings.Repeat("+x", n/2) + strings.Repeat("+", n%2)
}

// concept builds a ConceptRow with the given weight and source.
func concept(weight float64, source string) cache.ConceptRow {
	return cache.ConceptRow{Concept: "c", Weight: weight, Source: source, SeenAt: time.Now()}
}

func delegationShape() (string, string) {
	return "fix:", longSnippet(500)
}

func TestAdaptiveAIDelegationSignal(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.1, "code"),
			concept(0.1, "code"),
			concept(0.1, "code"),
		},
	}
	synth.CommitMsg, synth.DiffSnippet = delegationShape()
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "hard" {
		t.Fatalf("expected hard from ai-delegation signal, got %q", got.Difficulty)
	}
}

func TestAdaptiveHighClusterSignal(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.9, "code"),
			concept(0.8, "code"),
			concept(0.7, "code"),
		},
		CommitMsg: strings.Repeat("a", 200),
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "hard" {
		t.Fatalf("expected hard from high-cluster signal, got %q", got.Difficulty)
	}
}

func TestAdaptiveHighClusterNotEnoughConcepts(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.9, "code"),
			concept(0.9, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "intermediate" {
		t.Fatalf("expected intermediate (cluster needs >=3 concepts), got %q", got.Difficulty)
	}
}

func TestAdaptiveDispersedSignal(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.2, "code"),
			concept(0.2, "conversation"),
			concept(0.2, "code"),
			concept(0.2, "conversation"),
			concept(0.2, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "easy" {
		t.Fatalf("expected easy from dispersed signal, got %q", got.Difficulty)
	}
}

func TestAdaptiveDispersedFiresOnRealExtractionWeights(t *testing.T) {
	// Production extraction yields weights ~0.5–0.9 (model confidence), so an
	// all-floor-weight commit (every concept at 0.5) is the realistic easy
	// shape — the pre-retune 0.3 threshold could never fire on these values.
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.5, "code"),
			concept(0.5, "code"),
			concept(0.5, "code"),
			concept(0.5, "code"),
			concept(0.5, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "easy" {
		t.Fatalf("expected easy from realistic low-confidence spray, got %q", got.Difficulty)
	}
}

func TestAdaptiveDispersedBeatsDelegation(t *testing.T) {
	// Both the dispersed shape (5 low-weight concepts) and the delegation
	// shape fire; the reordered table resolves dispersed (easy) first —
	// delegation is the last commit-shape rule, not the first-match winner.
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.2, "code"),
			concept(0.2, "conversation"),
			concept(0.2, "code"),
			concept(0.2, "conversation"),
			concept(0.2, "code"),
		},
	}
	synth.CommitMsg, synth.DiffSnippet = delegationShape()
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "easy" {
		t.Fatalf("expected easy (dispersed before delegation), got %q", got.Difficulty)
	}
}

func TestAdaptiveAllCodeSourceEscalates(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.6, "code"),
			concept(0.6, "code"),
			concept(0.6, "code"),
			concept(0.6, "code"),
			concept(0.6, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "hard" {
		t.Fatalf("expected hard from all-code escalation, got %q", got.Difficulty)
	}
}

func TestAdaptiveFallbackNoSignals(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.5, "code"),
			concept(0.5, "conversation"),
			concept(0.5, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "intermediate" {
		t.Fatalf("expected intermediate fallback, got %q", got.Difficulty)
	}
}

func TestAdaptiveDelegationAloneStillEscalates(t *testing.T) {
	// Delegation is demoted, not removed: with no concept-structure signal
	// present it still resolves hard as the final commit-shape rule.
	synth := SynthesisContext{Concepts: nil}
	synth.CommitMsg, synth.DiffSnippet = delegationShape()
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "hard" {
		t.Fatalf("expected hard from delegation alone (demoted, not removed), got %q", got.Difficulty)
	}
}

func TestAdaptiveEmptyCtxConceptsReturnsIntermediate(t *testing.T) {
	synth := SynthesisContext{Concepts: nil}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "intermediate" {
		t.Fatalf("expected intermediate for nil concepts, got %q", got.Difficulty)
	}
}

func TestAdaptiveHighClusterWinsOverDispersed(t *testing.T) {
	// 5 concepts at avg 0.8: matches high-cluster AND the dispersed minimum
	// count — high-cluster is evaluated first and must win.
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.8, "code"),
			concept(0.8, "conversation"),
			concept(0.8, "code"),
			concept(0.8, "conversation"),
			concept(0.8, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "hard" {
		t.Fatalf("expected hard (high-cluster before dispersed), got %q", got.Difficulty)
	}
}

func TestAdaptiveNoDoubleEscalation(t *testing.T) {
	// Both high-cluster and the delegation shape fire; both produce hard, so
	// the reordered first-match table must not conflict or double-escalate.
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.9, "code"),
			concept(0.9, "code"),
			concept(0.9, "code"),
		},
	}
	synth.CommitMsg, synth.DiffSnippet = delegationShape()
	if got := (Adaptive{}).Resolve(context.Background(), synth); got.Difficulty != "hard" {
		t.Fatalf("expected hard (single escalation, no path bug), got %q", got.Difficulty)
	}
}
