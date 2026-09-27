package pipeline

import (
	"context"
	"encoding/json"
	"log"

	"github.com/colinwilliams91/total-recall/internal/ai"
)

// ConceptFingerprint is the extract-and-discard output of concept analysis.
// Only concept metadata is written to the cache — raw diff text is never persisted.
type ConceptFingerprint struct {
	// Concept is the technical concept or skill identified (e.g., "Go interfaces", "SQL joins").
	Concept string `json:"concept"`

	// Source identifies where the concept was extracted from: "user", "agent", or "code".
	Source string `json:"source"`

	// Weight is a relative confidence score in [0.0, 1.0].
	Weight float64 `json:"weight"`
}

// parseConceptResponse parses a concept-extraction response body. The
// shipped contract is object-wrapped ({"concepts":[...]}, the shape most
// json_object-enforcing providers accept); bare arrays and single bare
// objects are accepted as legacy/terse-model tolerances. Anything else fails
// the parse and the caller degrades gracefully.
func parseConceptResponse(raw string) []ConceptFingerprint {
	// Only accept the wrapper when the literal "concepts" key is present —
	// unknown-field tolerance would otherwise let any object (including the
	// single-object legacy shape) parse "successfully" with zero concepts.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &envelope); err == nil {
		if conceptsRaw, ok := envelope["concepts"]; ok {
			var wrapped []ConceptFingerprint
			if uErr := json.Unmarshal(conceptsRaw, &wrapped); uErr == nil {
				return wrapped
			}
		}
	}

	var concepts []ConceptFingerprint
	if err := json.Unmarshal([]byte(raw), &concepts); err == nil {
		return concepts
	}

	var single ConceptFingerprint
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		return []ConceptFingerprint{single}
	}

	log.Printf("[pipeline] extraction parse failed (response: %.200s)", raw)
	return []ConceptFingerprint{}
}

// ExtractConcepts derives concept fingerprints from a staged Git diff using the AI provider.
// Pipeline logs error and continues gracefully on AI or parsing failure.
func ExtractConcepts(ctx context.Context, provider ai.Provider, diff, model string) ([]ConceptFingerprint, error) {
	req := ExtractionRequest(diff, model)
	raw, err := provider.Complete(ctx, req)
	if err != nil {
		log.Printf("[pipeline] extraction AI call failed: %v", err)
		return []ConceptFingerprint{}, nil
	}

	return parseConceptResponse(raw), nil
}
