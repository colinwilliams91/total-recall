package difficulty

import (
	"context"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

func TestStaticVerbatim(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			{Concept: "c", Weight: 0.9, Source: "code", SeenAt: time.Now()},
		},
		CommitMsg:   "fix:",
		DiffSnippet: longSnippet(500),
	}
	if got := (Static{Value: "hard"}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected verbatim hard, got %q", got)
	}
	if got := (Static{Value: "easy"}).Resolve(context.Background(), synth); got != "easy" {
		t.Fatalf("expected verbatim easy, got %q", got)
	}
}

func TestStaticAdaptiveDelegatesToAdaptive(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{
			{Concept: "a", Weight: 0.9, Source: "code"},
			{Concept: "b", Weight: 0.8, Source: "code"},
			{Concept: "c", Weight: 0.7, Source: "code"},
		},
	}
	if got := (Static{Value: "adaptive"}).Resolve(context.Background(), synth); got != "hard" {
		t.Fatalf("expected adaptive delegation to return hard, got %q", got)
	}
}

func TestStaticEmptyStringReturnsEmpty(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{{Concept: "c", Weight: 0.9, Source: "code"}},
	}
	if got := (Static{Value: ""}).Resolve(context.Background(), synth); got != "" {
		t.Fatalf("expected empty string preserved, got %q", got)
	}
}
