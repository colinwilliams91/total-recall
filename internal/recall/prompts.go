package recall

import (
	"fmt"
	"strings"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/cache"
)

const (
	synthesisMaxTokens = 1024
	feedbackMaxTokens  = 412

	// formatContract is the trailing section appended after the policy doc body
	// in the composed system turn. It carries the JSON shape directive and the
	// choices[0] = correct contract — the pieces the policy doc itself does not
	// encode. When the policy body is empty (fallback), the legacy
	// synthesisSystemTmpl is used verbatim instead.
	formatContract = `## Format contract

Difficulty level: %s

## Output language (apply BEFORE anything else)

Before composing anything, look at the developer's commit message and code
comments in the user turn. If that context is English, EVERY part of your
output — question text and every choice — MUST be English. Never answer in
your training language, and never switch to Japanese, Chinese, or any other
language when the context is English. An English context with a non-English
question or choice is a hard failure.

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Protect the intellectual property of the developer's codebase; do not copy or expose any code snippets in the question or answers, but instead derive the question from the concept metadata and commit context to maximize relevancy.
- Generate the question and answer text in the same language as the commit message and code comments provided in the user turn. The match MUST be exact: English context → English output; Japanese context → Japanese output; Spanish → Spanish. Do NOT default to your training language. When no prose context is provided OR the language is ambiguous, generate in English.`

	// synthesisSystemTmpl is the legacy template used verbatim when the policy
	// asset fails to load. Preserves the pre-enrichment behavior so a missing
	// asset never blocks synthesis.
	synthesisSystemTmpl = `You are a software development quizzing assistant. Based on the list of concepts the developer has been working with, generate a single multiple-choice recall question to reinforce learning. Derive concepts from the provided commit diff and message context. The question should be concise, focused on one concept, and avoid exposing any code snippets.

Difficulty level: %s

## Output language (apply BEFORE anything else)

Before composing anything, look at the developer's commit message and code
comments in the user turn. If that context is English, EVERY part of your
output — question text and every choice — MUST be English. Never answer in
your training language, and never switch to Japanese, Chinese, or any other
language when the context is English. An English context with a non-English
question or choice is a hard failure.

Return ONLY a JSON object with no surrounding text:
{"question":"<question text>","choices":["<correct answer>","<wrong answer 1>","<wrong answer 2>","<wrong answer 3>"]}

Rules:
- The first choice must be the correct answer
- Wrong answers must be plausible but clearly incorrect to someone who understands the concept
- Keep the question concise and directly related to one of the provided concepts
- Protect the intellectual property of the developer's codebase; do not copy or expose any code snippets in the question or answers, but instead derive the question from the concept metadata and commit context to maximize relevancy.
- Generate the question and answer text in the same language as the commit message and code comments provided in the user turn. The match MUST be exact: English context → English output; Japanese context → Japanese output; Spanish → Spanish. Do NOT default to your training language. When no prose context is provided OR the language is ambiguous, generate in English.`

	feedbackSystemTmpl = `You are a software development quizzing assistant giving immediate, technical and conceptual feedback after a developer answers a quiz question. Be direct, concise, and informative. Do not use markdown, asterisks, bullet points, or headers. Write in plain prose. Maximum 3 short sentences. You MUST generate the feedback in the same language as the question text. Do NOT default to your training language.

If the developer was correct: briefly confirm and add one sentence explaining why that answer is right — not just that it is right. Add value to the learning experience by providing a short technical explanation or insight that reinforces the concept. Do not repeat the question or answer text in your feedback.

If the developer was incorrect: state the correct answer explicitly, explain why it is right, and briefly note why their chosen answer doesn't fit. Try to enrich the learning experience while remaining brief. Do not apologize or soften excessively.`
)

// SynthesisRequest builds the CompletionRequest used to synthesize a recall
// question from enriched concept rows, commit context, and a loaded policy doc.
//
// The system turn is composed from the policy body + format contract when the
// policy body is non-empty; otherwise the legacy synthesisSystemTmpl is used
// verbatim (fallback path). The user turn lists each concept with its weight,
// source, and seen-at timestamp, led by a focused-concepts section when the
// resolver's focus set matches rows in the concepts list, followed by a
// "Recent commit context" section when both commitMsg and diffSnippet are
// non-empty. Focus names that match no concept row are ignored — biasing
// reorders existing rows, it never invents feed content.
func SynthesisRequest(concepts []cache.ConceptRow, commitMsg, diffSnippet, policyBody, difficulty string, focus []string, model string) ai.CompletionRequest {
	system := composeSystemTurn(policyBody, difficulty)
	userTurn := composeUserTurn(concepts, focus, commitMsg, diffSnippet)
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
// optional commit context. When the focus set is non-empty and matches
// concept rows, those rows surface first under their own lead section
// ("Concepts the developer is currently working through") ahead of the
// standard list — weak-concept repetition exploits the policy's
// keep-the-question-on-a-provided-concept rule via ordering, not bespoke
// directives. When commitMsg and diffSnippet are both empty, the "Recent
// commit context" section is omitted entirely.
func composeUserTurn(concepts []cache.ConceptRow, focus []string, commitMsg, diffSnippet string) string {
	focused, rest := splitFocused(concepts, focus)
	var b strings.Builder
	if len(focused) > 0 {
		b.WriteString("Concepts the developer is currently working through:\n")
		for _, c := range focused {
			fmt.Fprintf(&b, "- %s (weight=%.1f, source=%s, seen=%s)\n", c.Concept, c.Weight, c.Source, c.SeenAt.Format("2006-01-02T15:04:05Z07:00"))
		}
		b.WriteString("\n")
	}
	b.WriteString("Concepts the developer has been working with:\n")
	for _, c := range rest {
		fmt.Fprintf(&b, "- %s (weight=%.1f, source=%s, seen=%s)\n", c.Concept, c.Weight, c.Source, c.SeenAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if commitMsg != "" && diffSnippet != "" {
		fmt.Fprintf(&b, "\nRecent commit context:\n%s\n```\n%s\n```\n", commitMsg, diffSnippet)
	}
	return b.String()
}

// splitFocused partitions concept rows into the resolver's focus-matched rows
// (in the focus list's ranking order) and the remainder (original order).
// Focus names matching no row are dropped silently — the feed stays 100%
// derivable from the user's own data.
func splitFocused(concepts []cache.ConceptRow, focus []string) (focused, rest []cache.ConceptRow) {
	bySlug := make(map[string]cache.ConceptRow, len(concepts))
	for _, c := range concepts {
		bySlug[c.Slug] = c
	}
	focusSet := make(map[string]bool, len(focus))
	for _, f := range focus {
		if c, ok := bySlug[f]; ok {
			focused = append(focused, c)
			focusSet[f] = true
		}
	}
	for _, c := range concepts {
		if !focusSet[c.Slug] {
			rest = append(rest, c)
		}
	}
	return focused, rest
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
