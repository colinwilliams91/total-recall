package pipeline

import (
	"fmt"
	"log"

	"github.com/colinwilliams91/total-recall/internal/ai"
)

const (
	extractionMaxDiffChars = 8000
	extractionMaxTokens    = 512

	extractionSystem = `You are a technical concept extractor. Analyze the provided Git diff and identify the most important technical concepts, patterns, or skills demonstrated by the changes.

Return ONLY a JSON array with no surrounding text. Each element must have exactly these fields:
- "concept": a concise name for the technical concept (e.g., "exponential backoff", "SQL joins", "Go interfaces")
- "source": always the string "code"  
- "weight": a confidence score between 0.0 and 1.0 (higher = more central to the change)

Aim for 3-7 concepts. Focus on what a developer should understand and remember about these changes.

Example output:
[{"concept":"exponential backoff","source":"code","weight":0.9},{"concept":"jitter for retry synchronization","source":"code","weight":0.8}]`
)

// ExtractionRequest builds the CompletionRequest used to extract concepts from a staged diff.
// If the diff exceeds the character limit it is truncated with a marker appended.
func ExtractionRequest(diff, model string) ai.CompletionRequest {
	userTurn := diff
	if len(diff) > extractionMaxDiffChars {
		log.Printf("[pipeline] diff truncated from %d to %d chars for extraction", len(diff), extractionMaxDiffChars)
		userTurn = fmt.Sprintf("%s\n[... diff truncated for context limit ...]", diff[:extractionMaxDiffChars])
	}
	return ai.CompletionRequest{
		Model:     model,
		System:    extractionSystem,
		UserTurn:  userTurn,
		MaxTokens: extractionMaxTokens,
		JSON:      true,
	}
}
