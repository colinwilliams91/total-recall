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

// ExtractConcepts derives concept fingerprints from a staged Git diff using the AI provider.
// Pipeline logs error and continues gracefully on AI or parsing failure.
func ExtractConcepts(ctx context.Context, provider ai.Provider, diff, model string) ([]ConceptFingerprint, error) {
	req := ExtractionRequest(diff, model)
	raw, err := provider.Complete(ctx, req)
	if err != nil {
		log.Printf("[pipeline] extraction AI call failed: %v", err)
		return []ConceptFingerprint{}, nil
	}

	var concepts []ConceptFingerprint
	if err := json.Unmarshal([]byte(raw), &concepts); err != nil {
		var single ConceptFingerprint
		if err2 := json.Unmarshal([]byte(raw), &single); err2 == nil {
			return []ConceptFingerprint{single}, nil
		}
		log.Printf("[pipeline] extraction parse failed (response: %.200s): %v", raw, err)
		return []ConceptFingerprint{}, nil
	}

	return concepts, nil
}
