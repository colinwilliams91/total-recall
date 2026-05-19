package pipeline

// ConceptFingerprint is the extract-and-discard output of conversation analysis.
// Only concept metadata is written to the cache — raw conversation text is never persisted.
type ConceptFingerprint struct {
	// Concept is the technical concept or skill identified (e.g., "Go interfaces", "SQL joins").
	Concept string

	// Source identifies where the concept was extracted from: "user", "agent", or "code".
	Source string

	// Weight is a relative confidence score in [0.0, 1.0].
	Weight float64
}

// ExtractConcepts derives concept fingerprints from a conversation turn.
// input is the raw text to analyze (user message or agent response).
// source identifies the origin signal: "user" (Signal 1) or "agent" (Signal 2).
//
// This is a stub — AI-powered extraction will be implemented in the recall engine phase.
func ExtractConcepts(input, source string) []ConceptFingerprint {
	// TODO(recall-engine): implement AI-powered concept extraction.
	return nil
}
