package pipeline

import (
	"testing"
)

func TestExtractConceptsObjectWrappedContract(t *testing.T) {
	// The shipped contract: JSON object root enforced by most providers that
	// accept response_format=json_object. The wrapper is unwrapped against the
	// provider's raw body before any legacy tolerance.
	raw := `{"concepts":[{"concept":"exp backoff","source":"code","weight":0.9},{"concept":"jitter","source":"code","weight":0.7}]}`
	concepts := parseConceptResponse(raw)
	if len(concepts) != 2 {
		t.Fatalf("expected 2 concepts from object-wrapped contract, got %d", len(concepts))
	}
	if concepts[0].Concept != "exp backoff" || concepts[0].Weight != 0.9 {
		t.Fatalf("unexpected first concept: %+v", concepts[0])
	}
}

func TestExtractConceptsBareArrayLegacy(t *testing.T) {
	// Older prompts/hosts of this pipeline shipped the bare-array contract.
	raw := `[{"concept":"retry pattern","source":"code","weight":0.9}]`
	concepts := parseConceptResponse(raw)
	if len(concepts) != 1 || concepts[0].Concept != "retry pattern" {
		t.Fatalf("expected legacy bare array to parse, got %+v", concepts)
	}
}

func TestExtractConceptsSingleObjectTolerance(t *testing.T) {
	// Terse models sometimes emit one bare object instead of a list.
	raw := `{"concept":"git rebase","source":"code","weight":0.8}`
	concepts := parseConceptResponse(raw)
	if len(concepts) != 1 || concepts[0].Concept != "git rebase" {
		t.Fatalf("expected single-object tolerance, got %+v", concepts)
	}
	if concepts[0].Concept != "git rebase" {
		t.Fatalf("unexpected concept, got %q", concepts[0].Concept)
	}
}

func TestExtractConceptsGarbageReturnsEmpty(t *testing.T) {
	if got := parseConceptResponse(`<!doctype html>`); len(got) != 0 {
		t.Fatalf("expected empty result for garbage response, got %+v", got)
	}
}
