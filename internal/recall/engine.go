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

// Choice is one option of a multiple-choice recall question. IsCorrect is the
// engine's correctness key for this choice (per the AI contract, exactly one
// Choice per Question has IsCorrect=true after construction). The IsCorrect
// boolean travels with its row through shuffles — the prior `correctIdx == i`
// index-tracking arithmetic is gone.
type Choice struct {
	Text      string `json:"text"`
	IsCorrect bool   `json:"is_correct,omitempty"`
}

// Question is a synthesized recall question with multiple-choice answers.
// Choices are shuffled before delivery; CorrectIndex is derived from the
// post-shuffle position of the IsCorrect=true Choice (presentation aid only —
// the source of truth is the IsCorrect boolean on each row).
type Question struct {
	Question     string   `json:"question"`
	Choices      []Choice `json:"choices"`
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

// Synthesize loads recent concepts for repo+branch from the cache and asks the
// AI to generate a recall question. Both repo and branch are required; the
// method returns (nil, nil) (not an error) when either is empty. It also
// returns (nil, nil) (not an error) when the concept cache is empty for the
// (repo, branch) pair, or when the AI call or JSON parse fails.
func (e *Engine) Synthesize(ctx context.Context, repo, branch, difficulty, model string) (*Question, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	rows, err := e.store.Recent(ctx, repo, branch, 20)
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

	// AI contract returns {"question": "...", "choices": ["...","..."]} with
	// choices[0] = correct answer. Wrap into typed []Choice with IsCorrect
	// traveling on the row, then shuffle.
	var rawQ struct {
		Question string   `json:"question"`
		Choices  []string `json:"choices"`
	}
	if err := json.Unmarshal([]byte(raw), &rawQ); err != nil {
		log.Printf("[recall] synthesis parse failed (response: %.200s): %v", raw, err)
		return nil, nil
	}

	q := &Question{Question: rawQ.Question}
	if len(rawQ.Choices) >= 2 {
		q.Choices = make([]Choice, len(rawQ.Choices))
		for i, text := range rawQ.Choices {
			q.Choices[i] = Choice{Text: text, IsCorrect: i == 0}
		}
		// Shuffle mutates the slice order; the IsCorrect boolean stays attached
		// to its row, so we don't need to track "where did index 0 land."
		rand.Shuffle(len(q.Choices), func(i, j int) {
			q.Choices[i], q.Choices[j] = q.Choices[j], q.Choices[i]
		})
		// Derive CorrectIndex for the wire / caller convenience.
		for i, c := range q.Choices {
			if c.IsCorrect {
				q.CorrectIndex = i
				break
			}
		}
	} else {
		// 0 or 1 choice: no shuffle, no correct-index tracking (defensive).
		q.Choices = make([]Choice, len(rawQ.Choices))
		for i, text := range rawQ.Choices {
			q.Choices[i] = Choice{Text: text, IsCorrect: i == 0}
		}
	}

	return q, nil
}

// GenerateFeedback calls the configured AI provider to produce a short prose
// explanation of whether the developer answered correctly, why the correct
// answer is right, and (if applicable) why the chosen answer doesn't fit.
//
// On AI error, the failure is logged and an empty string is returned. The
// caller is expected to continue with empty feedback rather than fail the
// answer record — feedback failure must never block the answer from being
// stored.
func (e *Engine) GenerateFeedback(ctx context.Context, question string, choices []Choice, selectedIndex, correctIndex int, model string) string {
	req := FeedbackRequest(question, choices, selectedIndex, correctIndex, model)
	raw, err := e.provider.Complete(ctx, req)
	if err != nil {
		log.Printf("[recall] feedback AI call failed: %v", err)
		return ""
	}
	return raw
}
