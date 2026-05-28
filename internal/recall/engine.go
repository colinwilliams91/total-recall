package recall

import (
	"context"
	"encoding/json"
	"log"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
)

const defaultDifficulty = "intermediate"

// Question is a synthesized recall question with multiple-choice answers.
// The first element in Choices is always the correct answer.
type Question struct {
	Question string   `json:"question"`
	Choices  []string `json:"choices"`
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

	return &q, nil
}
