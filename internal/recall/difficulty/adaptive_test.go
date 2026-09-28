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

func TestAdaptiveAIDelegationSignal(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.1, "code"),
			concept(0.1, "code"),
			concept(0.1, "code"),
		},
		CommitMsg:   "fix:",
		DiffSnippet: longSnippet(500),
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected hard from ai-delegation signal, got %q", got)
	}
}

func TestAdaptiveHighClusterSignal(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.9, "code"),
			concept(0.8, "code"),
			concept(0.7, "code"),
		},
		CommitMsg:   strings.Repeat("a", 200),
		DiffSnippet: "",
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected hard from high-cluster signal, got %q", got)
	}
}

func TestAdaptiveHighClusterNotEnoughConcepts(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.9, "code"),
			concept(0.9, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "intermediate" {
		t.Fatalf("expected intermediate (cluster needs >=3 concepts), got %q", got)
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
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "easy" {
		t.Fatalf("expected easy from dispersed signal, got %q", got)
	}
}

func TestAdaptiveAllCodeSourceEscalates(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.5, "code"),
			concept(0.5, "code"),
			concept(0.5, "code"),
			concept(0.5, "code"),
			concept(0.5, "code"),
		},
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected hard from all-code escalation, got %q", got)
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
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "intermediate" {
		t.Fatalf("expected intermediate fallback, got %q", got)
	}
}

func TestAdaptiveOrderingAIDelegationWinsOverCluster(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			concept(0.9, "code"),
			concept(0.9, "code"),
			concept(0.9, "code"),
		},
		CommitMsg:   "fix:",
		DiffSnippet: longSnippet(500),
	}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected hard (single escalation, no path bug), got %q", got)
	}
}

func TestAdaptiveEmptyCtxConceptsReturnsIntermediate(t *testing.T) {
	synth := SynthesisContext{Concepts: nil}
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "intermediate" {
		t.Fatalf("expected intermediate for nil concepts, got %q", got)
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
	if got := (Adaptive{}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected hard (high-cluster before dispersed), got %q", got)
	}
}
