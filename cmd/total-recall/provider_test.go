package main

import (
	"testing"

	"github.com/colinwilliams91/total-recall/internal/ai/anthropic"
	"github.com/colinwilliams91/total-recall/internal/ai/openai"
	"github.com/colinwilliams91/total-recall/internal/config"
)

func TestNewProviderRejectsEmptyConfig(t *testing.T) {
	cfg := config.AIConfig{Provider: ""}
	provider, err := newProvider(cfg)
	if err == nil {
		t.Fatal("expected error for empty provider config")
	}
	if !isErrNoProvider(err) {
		t.Fatalf("expected ErrNoProvider, got %v", err)
	}
	if provider != nil {
		t.Fatal("expected nil provider on error")
	}
}

func TestNewProviderRejectsUnknownProvider(t *testing.T) {
	cfg := config.AIConfig{Provider: "nonexistent"}
	provider, err := newProvider(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !isErrNoProvider(err) {
		t.Fatalf("expected ErrNoProvider, got %v", err)
	}
	if provider != nil {
		t.Fatal("expected nil provider on error")
	}
}

func TestNewProviderRoutesAnthropic(t *testing.T) {
	cfg := config.AIConfig{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
		APIKey:   "env:ANTHROPIC_API_KEY",
	}
	provider, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := provider.(*anthropic.Client); !ok {
		t.Fatalf("expected *anthropic.Client, got %T", provider)
	}
}

func TestNewProviderRoutesOpenAIFallback(t *testing.T) {
	providers := []string{"openai", "ollama", "groq", "lm-studio", "qwen", "minimax", "deepseek", "openrouter"}
	for _, p := range providers {
		t.Run(p, func(t *testing.T) {
			cfg := config.AIConfig{
				Provider: p,
				Model:    "some-model",
				APIKey:   "env:TEST_KEY",
			}
			provider, err := newProvider(cfg)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", p, err)
			}
			if provider == nil {
				t.Fatalf("expected non-nil provider for %s", p)
			}
			if _, ok := provider.(*openai.Client); !ok {
				t.Fatalf("expected *openai.Client for %s, got %T", p, provider)
			}
		})
	}
}

func TestNewProviderCustomRequiresBaseURL(t *testing.T) {
	cfg := config.AIConfig{
		Provider: "custom",
		Model:    "my-model",
		APIKey:   "env:MY_KEY",
		BaseURL:  "",
	}
	provider, err := newProvider(cfg)
	if err == nil {
		t.Fatal("expected error for custom provider without base-url")
	}
	if !isErrNoProvider(err) {
		t.Fatalf("expected ErrNoProvider, got %v", err)
	}
	if provider != nil {
		t.Fatal("expected nil provider on error")
	}
}

func TestNewProviderCustomWithBaseURL(t *testing.T) {
	cfg := config.AIConfig{
		Provider: "custom",
		Model:    "my-model",
		APIKey:   "env:MY_KEY",
		BaseURL:  "http://localhost:8080/v1",
	}
	provider, err := newProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if _, ok := provider.(*openai.Client); !ok {
		t.Fatalf("expected *openai.Client for custom, got %T", provider)
	}
}
