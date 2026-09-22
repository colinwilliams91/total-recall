package recall

import (
	"context"
	"encoding/json"
	"log"
	"math/rand/v2"

	"github.com/colinwilliams91/total-recall/assets"
	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/colinwilliams91/total-recall/internal/recall/difficulty"
)

// SynthesisContext carries the enriched inputs for a single synthesis call:
// the recent concept rows (with weights, sources, and timestamps), the commit
// message, and a short diff snippet. All fields are transient — they ride
// from runPipeline to Synthesize and never touch the SQLite store. It is the
// difficulty package's SynthesisContext: resolvers inspect the same inputs
// synthesis consumes, without importing the engine.
type SynthesisContext = difficulty.SynthesisContext

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

// Engine synthesizes recall questions by prompting the AI provider with
// concepts and context loaded from the cache and the hook envelope.
type Engine struct {
	provider ai.Provider
	store    *cache.Store
	policy   assets.PromptAsset
	resolver difficulty.Resolver
}

// warningQuestionPolicyFallback tells the operator the daemon is quizzing
// from the degraded legacy template rather than the policy doc. It names both
// candidate sources so the message fits whichever situation fired: a drop-in
// file that is absent or unparseable, or a binary whose embedded default is
// somehow missing/corrupt. The model-facing fallback kept parity with the
// traffic contract instead — no warning rides in the payload itself, because
// the answer flow parses responses as strict JSON and prose there fails the
// whole synthesis call.
func warningQuestionPolicyFallback(err error) {
	log.Printf("[recall] WARNING: question-synthesis policy fallback engaged — using the legacy inline template (load error: %v)", err)
	log.Printf("[recall] WARNING: check the drop-in policy at $TR_HOME/prompts/question-generation-policy.md (~/.tr/prompts/ by default) and re-serve if you touched the drop-in file, or assets/prompts/question-generation-policy.md in the source tree and rebuild if you did not enable the drop-in and suspect a broken shipped asset")
}

// New creates an Engine. Both provider and store must be non-nil. The
// difficulty resolver is constructed from the configured recall difficulty:
// a concrete value resolves verbatim via Static; "adaptive" (or an unset
// field, which preserves the prior default behavior) routes to the adaptive
// heuristics. The question-generation-policy prompt asset is loaded once at
// construction; a missing or corrupt asset falls back to SourceFallback and
// Synthesize uses the legacy inline template.
func New(provider ai.Provider, store *cache.Store, cfg *config.RecallConfig) *Engine {
	policy, err := assets.Load("question-generation-policy")
	if err != nil {
		warningQuestionPolicyFallback(err)
	}
	value := ""
	if cfg != nil {
		value = cfg.Difficulty
	}
	if value == "" {
		value = "adaptive"
	}
	return &Engine{
		provider: provider,
		store:    store,
		policy:   policy,
		resolver: difficulty.Static{Value: value},
	}
}

// Resolver exposes the engine's difficulty resolver. Synthesize accepts the
// resolver as a parameter (keeping difficulty selection injectable at the
// call boundary); the server passes the engine-constructed one through here.
func (e *Engine) Resolver() difficulty.Resolver {
	return e.resolver
}

// Synthesize asks the AI to generate a recall question from the enriched
// SynthesisContext. Both repo and branch are required. Difficulty selection
// is delegated to the resolver — empty contexts (no repo/branch/concepts)
// return before resolution so they never pay the resolver's cost.
func (e *Engine) Synthesize(ctx context.Context, repo, branch, model string, synth SynthesisContext, resolver difficulty.Resolver) (*Question, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	if len(synth.Concepts) == 0 {
		return nil, nil
	}

	difficulty := resolver.Resolve(ctx, synth)
	if difficulty == "" {
		difficulty = "intermediate"
	}

	req := SynthesisRequest(synth.Concepts, synth.CommitMsg, synth.DiffSnippet, e.policy.Body, difficulty, model)
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
	if len(rawQ.Choices) < 2 {
		log.Printf("[recall] synthesis returned %d choices (need >=2), skipping", len(rawQ.Choices))
		return nil, nil
	}
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
