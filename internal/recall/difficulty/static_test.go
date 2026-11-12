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
	got := (Static{Value: "hard"}).Resolve(context.Background(), synth)
	if got.Difficulty != "hard" {
		t.Fatalf("expected verbatim hard, got %q", got.Difficulty)
	}
	if len(got.FocusConcepts) != 0 {
		t.Fatalf("expected empty focus from verbatim path, got %v", got.FocusConcepts)
	}
	if got = (Static{Value: "easy"}).Resolve(context.Background(), synth); got.Difficulty != "easy" {
		t.Fatalf("expected verbatim easy, got %q", got.Difficulty)
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
	got := (Static{Value: "adaptive"}).Resolve(context.Background(), synth)
	if got.Difficulty != "hard" {
		t.Fatalf("expected adaptive delegation to return hard, got %q", got.Difficulty)
	}
	if len(got.FocusConcepts) != 0 {
		t.Fatalf("expected empty focus from adaptive delegation, got %v", got.FocusConcepts)
	}
}

func TestStaticEmptyStringReturnsEmpty(t *testing.T) {
	synth := SynthesisContext{
		Concepts: []cache.ConceptRow{{Concept: "c", Weight: 0.9, Source: "code"}},
	}
	got := (Static{Value: ""}).Resolve(context.Background(), synth)
	if got.Difficulty != "" {
		t.Fatalf("expected empty string preserved, got %q", got.Difficulty)
	}
	if len(got.FocusConcepts) != 0 {
		t.Fatalf("expected empty focus on empty value, got %v", got.FocusConcepts)
	}
}
