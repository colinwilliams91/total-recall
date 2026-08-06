package recall

import (
	"fmt"
	"strings"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
)

const (
	synthesisMaxTokens = 1024
	feedbackMaxTokens  = 150

	// formatContract is the trailing section appended after the policy doc body
	// in the composed system turn. It carries the JSON shape directive and the
	// choices[0] = correct contract — the pieces the policy doc itself does not
	// encode. When the policy body is empty (fallback), the legacy
	// synthesisSystemTmpl is used verbatim instead.
	formatContract = `## Format contract

Difficulty level: %s

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Protect the intellectual property of the developer's codebase; do not copy or expose any code snippets in the question or answers, but instead derive the question from the concept metadata and commit context to maximize relevancy.`

	// synthesisSystemTmpl is the legacy template used verbatim when the policy
	// asset fails to load. Preserves the pre-enrichment behavior so a missing
	// asset never blocks synthesis.
	synthesisSystemTmpl = `You are a technical recall assistant. Based on the list of concepts the developer has been working with, generate a single multiple-choice recall question to reinforce learning.

Difficulty level: %s

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Protect the intellectual property of the developer's codebase; do not copy or expose any code snippets in the question or answers, but instead derive the question from the concept metadata and commit context to maximize relevancy.`

	feedbackSystemTmpl = `You are a technical recall assistant giving immediate feedback after a developer answers a quiz question. Be direct, concise, and informative. Do not use markdown, asterisks, bullet points, or headers. Write in plain prose. Maximum 3 sentences.

If the developer was correct: briefly confirm and add one sentence explaining why that answer is right — not just that it is right.

If the developer was incorrect: state the correct answer explicitly, explain why it is right, and briefly note why their chosen answer doesn't fit. Do not apologize or soften excessively.`
)

// SynthesisRequest builds the CompletionRequest used to synthesize a recall
// question from enriched concept rows, commit context, and a loaded policy doc.
//
// The system turn is composed from the policy body + format contract when the
// policy body is non-empty; otherwise the legacy synthesisSystemTmpl is used
// verbatim (fallback path). The user turn lists each concept with its weight,
// source, and seen-at timestamp, followed by a "Recent commit context" section
// when both commitMsg and diffSnippet are non-empty.
func SynthesisRequest(concepts []cache.ConceptRow, commitMsg, diffSnippet, policyBody, difficulty, model string) ai.CompletionRequest {
	system := composeSystemTurn(policyBody, difficulty)
	userTurn := composeUserTurn(concepts, commitMsg, diffSnippet)
	return ai.CompletionRequest{
		Model:     model,
		System:    system,
		UserTurn:  userTurn,
		MaxTokens: synthesisMaxTokens,
		JSON:      true,
	}
}

// composeSystemTurn builds the system prompt from the policy doc body and the
// format contract. When policyBody is empty, the legacy template is used
// verbatim so a missing asset never blocks synthesis.
func composeSystemTurn(policyBody, difficulty string) string {
	if policyBody == "" {
		return fmt.Sprintf(synthesisSystemTmpl, difficulty)
	}
	return policyBody + "\n\n" + fmt.Sprintf(formatContract, difficulty)
}

// composeUserTurn builds the user-turn message from enriched concept rows and
// optional commit context. When commitMsg and diffSnippet are both empty, the
// "Recent commit context" section is omitted entirely.
func composeUserTurn(concepts []cache.ConceptRow, commitMsg, diffSnippet string) string {
	var b strings.Builder
	b.WriteString("Concepts the developer has been working with:\n")
	for _, c := range concepts {
		fmt.Fprintf(&b, "- %s (weight=%.1f, source=%s, seen=%s)\n", c.Concept, c.Weight, c.Source, c.SeenAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if commitMsg != "" && diffSnippet != "" {
		fmt.Fprintf(&b, "\nRecent commit context:\n%s\n```\n%s\n```\n", commitMsg, diffSnippet)
	}
	return b.String()
}

// FeedbackRequest builds the CompletionRequest used to generate post-answer feedback.
// The user turn lists every choice with `← correct, chosen` / `← correct` /
// `← chosen (incorrect)` annotations so the AI has full distractor context.
// selectedIndex is the user's pick (0-based into the post-shuffle choices
// slice); correctIndex is the post-shuffle position of the IsCorrect=true row.
func FeedbackRequest(question string, choices []Choice, selectedIndex, correctIndex int, model string) ai.CompletionRequest {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n\n", question)
	b.WriteString("Choices:\n")
	for i, c := range choices {
		annotation := ""
		switch {
		case i == correctIndex && i == selectedIndex:
			annotation = "  <- correct, chosen"
		case i == correctIndex:
			annotation = "  <- correct"
		case i == selectedIndex:
			annotation = "  <- chosen (incorrect)"
		}
		fmt.Fprintf(&b, "  [%d] %s%s\n", i+1, c.Text, annotation)
	}
	if correctIndex == selectedIndex {
		b.WriteString("\nThe developer answered correctly.\n")
	} else {
		fmt.Fprintf(&b, "\nThe developer chose option %d and was incorrect.\n", selectedIndex+1)
	}
	return ai.CompletionRequest{
		Model:     model,
		System:    feedbackSystemTmpl,
		UserTurn:  b.String(),
		MaxTokens: feedbackMaxTokens,
		JSON:      false,
	}
}
