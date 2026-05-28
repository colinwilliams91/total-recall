package ai

import (
	"context"
	"errors"
)

// ErrNoProvider is returned by the provider factory when unconfigured or unknown.
// Callers (serveCmd) should log an advisory and proceed with a nil Provider.
var ErrNoProvider = errors.New("ai: no provider configured")

// CompletionRequest carries the inputs for a single AI completion call.
// Prompt construction is the responsibility of the calling layer (pipeline/, recall/).
type CompletionRequest struct {
	// Model is the model name to use. Empty string means the adapter uses its default.
	Model string
	// System is the system prompt.
	System string
	// UserTurn is the user-turn message content.
	UserTurn string `yaml:"user-turn"`
	// MaxTokens is the maximum tokens to generate. 0 means provider default.
	MaxTokens int
	// JSON hints that the caller wants structured JSON output.
	// OpenAI adapter: sets response_format={"type":"json_object"}.
	// Anthropic adapter: appends "\n\nRespond with valid JSON only." to System.
	JSON bool
}

// Provider is the thin AI abstraction used by the pipeline and recall engine.
// Adapters (internal/ai/openai, internal/ai/anthropic) implement this interface.
// Domain concerns (prompt construction, response parsing) live in calling layers.
type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (string, error)
}

// ProviderRegistry maps user-facing provider names to their default base URLs.
// The adapter to use for each preset is determined at the factory layer (cmd).
//
//	anthropic → internal/ai/anthropic (native Messages API)
//	openai    → internal/ai/openai    (Chat Completions API)
//	ollama    → internal/ai/openai    (OpenAI-compatible)
//	groq      → internal/ai/openai    (OpenAI-compatible)
//	lm-studio → internal/ai/openai    (OpenAI-compatible)
//	custom    → internal/ai/openai    (requires BaseURL)
var ProviderRegistry = map[string]string{
	"anthropic": "https://api.anthropic.com",
	"openai":    "https://api.openai.com/v1",
	"ollama":    "http://localhost:11434/v1",
	"groq":      "https://api.groq.com/openai/v1",
	"lm-studio": "http://localhost:1234/v1",
	"custom":    "", // user must supply BaseURL via AIConfig.BaseURL
}

