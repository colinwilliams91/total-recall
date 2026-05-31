package main

// wire.go provides the newProvider factory function — the cmd-layer wiring point
// that imports all AI adapter packages. This file exists separately so that
// internal/ai (which defines the interface) does not need to import its own
// sub-packages (which would create an import cycle).

import (
	"errors"
	"fmt"

	"github.com/colinwilliams91/total-recall/internal/ai"
	"github.com/colinwilliams91/total-recall/internal/ai/anthropic"
	"github.com/colinwilliams91/total-recall/internal/ai/openai"
	"github.com/colinwilliams91/total-recall/internal/config"
)

// newProvider constructs an ai.Provider from the resolved AIConfig.
// Returns (nil, ai.ErrNoProvider) when provider is unconfigured or unknown.
// Callers should log an advisory and pass nil to engine.New.
func newProvider(cfg config.AIConfig) (ai.Provider, error) {
	if cfg.Provider == "" {
		return nil, ai.ErrNoProvider
	}

	baseURL, ok := ai.ProviderRegistry[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("%w: unknown provider %q (known: anthropic, openai, ollama, groq, lm-studio, custom)", ai.ErrNoProvider, cfg.Provider)
	}

	if cfg.Provider == "custom" {
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("%w: provider is \"custom\" but base-url is empty — run 'total-recall init' to configure", ai.ErrNoProvider)
		}
		baseURL = cfg.BaseURL
	}

	apiKey, _ := cfg.ResolvedAPIKey()

	switch cfg.Provider {
	case "anthropic":
		return anthropic.New(baseURL, apiKey, cfg.Model), nil
	default:
		return openai.New(baseURL, apiKey, cfg.Model), nil
	}
}

// isErrNoProvider reports whether err wraps ai.ErrNoProvider.
func isErrNoProvider(err error) bool {
	return errors.Is(err, ai.ErrNoProvider)
}
