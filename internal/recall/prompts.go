package recall

import (
	"fmt"
	"strings"

	"github.com/colinwilliams91/total-recall/internal/ai"
)

const (
	synthesisMaxTokens = 256

	synthesisSystemTmpl = `You are a technical recall assistant. Based on the list of concepts the developer has been working with, generate a single multiple-choice recall question to reinforce learning.

Difficulty level: %s

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Do not reference the specific codebase or project — make the question about the concept itself`
)

// SynthesisRequest builds the CompletionRequest used to synthesize a recall question
// from a list of concept names.
func SynthesisRequest(concepts []string, difficulty, model string) ai.CompletionRequest {
	system := fmt.Sprintf(synthesisSystemTmpl, difficulty)
	userTurn := "Concepts the developer has been working with:\n" + strings.Join(concepts, "\n")
	return ai.CompletionRequest{
		Model:     model,
		System:    system,
		UserTurn:  userTurn,
		MaxTokens: synthesisMaxTokens,
		JSON:      true,
	}
}
