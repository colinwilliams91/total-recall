package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
)

// stubProvider returns a canned body (and optional error). Moves past the
// canned body return empty responses.
type stubProvider struct {
	body string
	err  error
}

func (p *stubProvider) Complete(_ context.Context, _ ai.CompletionRequest) (string, error) {
	return p.body, p.err
}

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

func TestExtractConceptsParsesExtendedShape(t *testing.T) {
	raw := `{"concepts":[{"concept":"exponential backoff","slug":"exponential_backoff","source":"code","weight":0.9}]}`
	concepts, err := ExtractConcepts(context.Background(), &stubProvider{body: raw}, "diff", "model")
	if err != nil {
		t.Fatalf("ExtractConcepts: %v", err)
	}
	if len(concepts) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(concepts))
	}
	if concepts[0].Slug != "exponential_backoff" || concepts[0].Concept != "exponential backoff" {
		t.Fatalf("expected prose + slug preserved, got %+v", concepts[0])
	}
}

func TestExtractConceptsDropsSluglessElements(t *testing.T) {
	// Mixed batch: the slug-less element degrades, others process normally.
	raw := `{"concepts":[
		{"concept":"retry logic","source":"code","weight":0.8},
		{"concept":"jitter","slug":"jitter_backoff","source":"code","weight":0.8}
	]}`
	concepts, err := ExtractConcepts(context.Background(), &stubProvider{body: raw}, "diff", "model")
	if err != nil {
		t.Fatalf("ExtractConcepts: %v", err)
	}
	if len(concepts) != 1 || concepts[0].Slug != "jitter_backoff" {
		t.Fatalf("expected only the slugful element to survive, got %+v", concepts)
	}

	// All slug-less: empty result, no error (extraction failure degradation).
	raw = `[{"concept":"legacy shape","source":"code","weight":0.9}]`
	concepts, err = ExtractConcepts(context.Background(), &stubProvider{body: raw}, "diff", "model")
	if err != nil {
		t.Fatalf("ExtractConcepts all slugless: %v", err)
	}
	if len(concepts) != 0 {
		t.Fatalf("expected empty result when no element carries a slug, got %+v", concepts)
	}
}

// TestExtractionToStoreVerticalSlice is the first vertical-slice e2e
// validation: a provider-stub diff fixture flows ExtractConcepts →
// Store.Save, and the registry gains the slug with a sighting linked to it.
func TestExtractionToStoreVerticalSlice(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	provider := &stubProvider{body: `{"concepts":[
		{"concept":"interface satisfaction via implicit methods","slug":"go_interfaces","source":"code","weight":0.9}]}`}
	concepts, err := ExtractConcepts(ctx, provider, "+ type Foo interface { Bar() }", "model")
	if err != nil {
		t.Fatalf("ExtractConcepts: %v", err)
	}
	if len(concepts) != 1 {
		t.Fatalf("expected 1 fingerprint, got %d", len(concepts))
	}

	fingerprints := make([]cache.Fingerprint, len(concepts))
	for i, c := range concepts {
		fingerprints[i] = cache.Fingerprint{Concept: c.Concept, Slug: c.Slug, Source: c.Source, Weight: c.Weight}
	}
	if err := s.Save(ctx, "/repo/slice", "main", fingerprints); err != nil {
		t.Fatalf("Save: %v", err)
	}

	recent, err := s.Recent(ctx, "/repo/slice", "main", 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 sighting, got %d", len(recent))
	}
	if recent[0].Slug != "go_interfaces" || recent[0].Concept != "interface satisfaction via implicit methods" {
		t.Fatalf("expected registry link + sighting prose, got %+v", recent[0])
	}
}

// setupStore opens a scratch cache.Store under a temp TR_HOME.
func setupStore(t *testing.T) *cache.Store {
	t.Helper()
	t.Setenv("TR_HOME", t.TempDir())
	s, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestExtractConceptsProviderFailureDegrades keeps the invocation boundary
// honest: an AI error produces an empty batch and a nil error.
func TestExtractConceptsProviderFailureDegrades(t *testing.T) {
	concepts, err := ExtractConcepts(context.Background(), &stubProvider{err: errors.New("boom")}, "diff", "model")
	if err != nil {
		t.Fatalf("expected graceful degradation, got %v", err)
	}
	if len(concepts) != 0 {
		t.Fatalf("expected empty concepts on provider failure, got %+v", concepts)
	}
}
