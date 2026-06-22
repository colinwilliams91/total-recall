package recall

import (
	"fmt"
	"strings"

	"github.com/colinwilliams91/total-recall/internal/ai"
)

const (
	synthesisMaxTokens = 512
	feedbackMaxTokens  = 150

	synthesisSystemTmpl = `You are a technical recall assistant. Based on the list of concepts the developer has been working with, generate a single multiple-choice recall question to reinforce learning.

Difficulty level: %s

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Do not reference the specific codebase or project — make the question about the concept itself`

	feedbackSystemTmpl = `You are a technical recall assistant giving immediate feedback after a developer answers a quiz question. Be direct, concise, and informative. Do not use markdown, asterisks, bullet points, or headers. Write in plain prose. Maximum 3 sentences.

If the developer was correct: briefly confirm and add one sentence explaining why that answer is right — not just that it is right.

If the developer was incorrect: state the correct answer explicitly, explain why it is right, and briefly note why their chosen answer doesn't fit. Do not apologize or soften excessively.`
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

// FeedbackRequest builds the CompletionRequest used to generate post-answer feedback.
// The user turn lists every choice with `← correct, chosen` / `← correct` /
// `← chosen (incorrect)` annotations so the AI has full distractor context.
func FeedbackRequest(question string, choices []string, correctIndex, answerIndex int, model string) ai.CompletionRequest {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n\n", question)
	b.WriteString("Choices:\n")
	for i, c := range choices {
		annotation := ""
		switch {
		case i == correctIndex && i == answerIndex:
			annotation = "  <- correct, chosen"
		case i == correctIndex:
			annotation = "  <- correct"
		case i == answerIndex:
			annotation = "  <- chosen (incorrect)"
		}
		fmt.Fprintf(&b, "  [%d] %s%s\n", i+1, c, annotation)
	}
	if correctIndex == answerIndex {
		b.WriteString("\nThe developer answered correctly.\n")
	} else {
		fmt.Fprintf(&b, "\nThe developer chose option %d and was incorrect.\n", answerIndex+1)
	}
	return ai.CompletionRequest{
		Model:     model,
		System:    feedbackSystemTmpl,
		UserTurn:  b.String(),
		MaxTokens: feedbackMaxTokens,
		JSON:      false,
	}
}
