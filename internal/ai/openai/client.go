package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/colinwilliams91/total-recall/internal/ai"
)

const defaultTimeout = 10 * time.Second

// Client implements ai.Provider using the OpenAI Chat Completions API.
// It also handles OpenAI-compatible providers (Ollama, Groq, LM Studio, custom).
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// New returns a Client pointed at baseURL, authenticated with apiKey.
// model is the default model; callers can override per-request via CompletionRequest.Model.
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// chatRequest is the OpenAI /chat/completions request body.
type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// chatResponse is the minimal OpenAI /chat/completions response we parse.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a chat completion request and returns the assistant content string.
func (c *Client) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	messages := make([]chatMessage, 0, 2)
	if req.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.UserTurn})

	body := chatRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
	}
	if req.JSON {
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai: http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai: read response: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("openai: parse response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("openai: api error: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: http %d: %s", resp.StatusCode, string(respBytes))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}

	return parsed.Choices[0].Message.Content, nil
}
