package anthropic

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

const (
	defaultTimeout    = 10 * time.Second
	anthropicVersion  = "2023-06-01"
	messagesEndpoint  = "/v1/messages"
	jsonOnlyInstruct  = "\n\nRespond with valid JSON only."
)

// Client implements ai.Provider using the Anthropic Messages API.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// New returns a Client pointed at baseURL, authenticated with apiKey.
func New(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// messagesRequest is the Anthropic /v1/messages request body.
type messagesRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	System    string         `json:"system,omitempty"`
	Messages  []anthropicMsg `json:"messages"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// messagesResponse is the minimal Anthropic response we parse.
type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a messages request and returns the assistant text content.
func (c *Client) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024 // Anthropic requires max_tokens; use a safe default
	}

	system := req.System
	if req.JSON {
		system += jsonOnlyInstruct
	}

	body := messagesRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages: []anthropicMsg{
			{Role: "user", Content: req.UserTurn},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+messagesEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("anthropic: read response: %w", err)
	}

	var parsed messagesResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("anthropic: parse response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("anthropic: api error (%s): %s", parsed.Error.Type, parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, string(respBytes))
	}
	for _, block := range parsed.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic: no text content in response")
}
