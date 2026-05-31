package recall

import (
	"context"
	"encoding/json"
	"log"
	"math/rand/v2"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
)

const defaultDifficulty = "intermediate"

// Question is a synthesized recall question with multiple-choice answers.
// Choices are shuffled before delivery; CorrectIndex indicates which element is correct.
type Question struct {
	Question     string   `json:"question"`
	Choices      []string `json:"choices"`
	CorrectIndex int      `json:"correct_index"`
}

// Engine synthesizes recall questions by pulling recent concepts from the
// cache and prompting the AI provider.
type Engine struct {
	provider ai.Provider
	store    *cache.Store
}

// New creates an Engine.  Both provider and store must be non-nil.
func New(provider ai.Provider, store *cache.Store) *Engine {
	return &Engine{provider: provider, store: store}
}

// Synthesize loads recent concepts from the cache and asks the AI to generate
// a recall question.  It returns nil, nil (not an error) if:
//   - the concept cache is empty, or
//   - the AI call or JSON parse fails (failure is logged instead).
func (e *Engine) Synthesize(ctx context.Context, difficulty, model string) (*Question, error) {
	rows, err := e.store.Recent(ctx, 20)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	if difficulty == "" {
		difficulty = defaultDifficulty
	}

	concepts := make([]string, len(rows))
	for i, r := range rows {
		concepts[i] = r.Concept
	}

	req := SynthesisRequest(concepts, difficulty, model)
	raw, err := e.provider.Complete(ctx, req)
	if err != nil {
		log.Printf("[recall] synthesis AI call failed: %v", err)
		return nil, nil
	}

	var q Question
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		log.Printf("[recall] synthesis parse failed (response: %.200s): %v", raw, err)
		return nil, nil
	}

	// Shuffle choices so the correct answer (index 0 per AI contract) lands at a
	// random position, and record that position in CorrectIndex.
	if len(q.Choices) >= 2 {
		correctIdx := 0
		rand.Shuffle(len(q.Choices), func(i, j int) {
			if correctIdx == i {
				correctIdx = j
			} else if correctIdx == j {
				correctIdx = i
			}
			q.Choices[i], q.Choices[j] = q.Choices[j], q.Choices[i]
		})
		q.CorrectIndex = correctIdx
	}

	return &q, nil
}
