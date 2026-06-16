package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const recallWorkflowInstructions = `After any git commit in this project, call recall_next to check for a pending recall question. If a question is returned, present it to the user and record their answer with recall_answer. If the queue is empty, continue normally.`

// buildMCPServer constructs and configures the MCP server with all tools,
// resources, and prompts for Total Recall. The returned *mcp.Server is
// mounted at /mcp/ on the existing HTTP listener.
func buildMCPServer(store *cache.Store, cfg *config.Config) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "total-recall", Version: "v1"}, &mcp.ServerOptions{
		SubscribeHandler: func(_ context.Context, _ *mcp.SubscribeRequest) error {
			return nil
		},
		UnsubscribeHandler: func(_ context.Context, _ *mcp.UnsubscribeRequest) error {
			return nil
		},
	})

	// recall_next — atomically dequeues the next pending question.
	type recallNextIn struct{}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall_next",
		Description: "Dequeue the next pending recall question. Returns null if the queue is empty.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ recallNextIn) (*mcp.CallToolResult, any, error) {
		if store == nil {
			return textResult(`{"question":null}`), nil, nil
		}
		q, err := store.NextQuestion(ctx, "mcp")
		if err != nil {
			log.Printf("[mcp] recall_next error: %v", err)
			return nil, nil, err
		}
		if q == nil {
			return textResult(`{"question":null}`), nil, nil
		}
		b, _ := json.Marshal(map[string]any{
			"id":       q.ID,
			"question": q.Question,
			"choices":  q.Choices,
		})
		return textResult(string(b)), nil, nil
	})

	// recall_answer — records the user's answer.
	type recallAnswerIn struct {
		ID     int64  `json:"id"`
		Answer string `json:"answer"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall_answer",
		Description: `Record the user's answer (or "skip") for a recall question.`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recallAnswerIn) (*mcp.CallToolResult, any, error) {
		if store == nil {
			return textResult(`{"ok":true}`), nil, nil
		}
		if err := store.AnswerQuestion(ctx, in.ID, in.Answer); err != nil {
			log.Printf("[mcp] recall_answer error: %v", err)
			return nil, nil, err
		}
		return textResult(`{"ok":true}`), nil, nil
	})

	// recall_status — daemon health and queue depth.
	type recallStatusIn struct{}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall_status",
		Description: "Return daemon health, AI configuration status, and pending question queue depth.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ recallStatusIn) (*mcp.CallToolResult, any, error) {
		aiConfigured := cfg.AI.Provider != ""
		depth := 0
		if store != nil {
			var err error
			depth, err = store.QueueDepth(ctx)
			if err != nil {
				log.Printf("[mcp] recall_status queueDepth error: %v", err)
			}
		}
		b, _ := json.Marshal(map[string]any{
			"daemon":        "ok",
			"ai_configured": aiConfigured,
			"queue_depth":   depth,
		})
		return textResult(string(b)), nil, nil
	})

	// recall://queue — subscribable live queue resource.
	srv.AddResource(&mcp.Resource{
		URI:         "recall://queue",
		Name:        "recall queue",
		Description: "Current pending question queue depth and next question preview.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		depth := 0
		var nextPayload any = nil
		if store != nil {
			var err error
			depth, err = store.QueueDepth(ctx)
			if err != nil {
				return nil, fmt.Errorf("queue depth: %w", err)
			}
			q, err := store.PeekNextQuestion(ctx)
			if err != nil {
				return nil, fmt.Errorf("peek question: %w", err)
			}
			if q != nil {
				nextPayload = map[string]any{
					"question": q.Question,
					"choices":  q.Choices,
				}
			}
		}
		b, _ := json.Marshal(map[string]any{
			"depth": depth,
			"next":  nextPayload,
		})
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      "recall://queue",
				MIMEType: "application/json",
				Text:     string(b),
			}},
		}, nil
	})

	// recall://recent — last 10 answered questions.
	srv.AddResource(&mcp.Resource{
		URI:         "recall://recent",
		Name:        "recent answers",
		Description: "Last 10 answered recall questions.",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		var answered []cache.StoredQuestion
		if store != nil {
			var err error
			answered, err = store.RecentAnswered(ctx, 10)
			if err != nil {
				return nil, fmt.Errorf("recent answered: %w", err)
			}
		}
		items := make([]map[string]any, 0, len(answered))
		for _, q := range answered {
			items = append(items, map[string]any{
				"id":       q.ID,
				"question": q.Question,
				"choices":  q.Choices,
			})
		}
		b, _ := json.Marshal(items)
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      "recall://recent",
				MIMEType: "application/json",
				Text:     string(b),
			}},
		}, nil
	})

	// recall_workflow — system prompt guiding the agent to use recall tools.
	srv.AddPrompt(&mcp.Prompt{
		Name:        "recall_workflow",
		Description: "System prompt instructing the AI agent to surface recall questions after commits.",
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{{
				Role:    mcp.Role("user"),
				Content: &mcp.TextContent{Text: recallWorkflowInstructions},
			}},
		}, nil
	})

	return srv
}

// mcpHandler returns an http.Handler that serves the MCP protocol at /mcp/.
func mcpHandler(srv *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return srv
	}, nil)
}

// textResult creates a CallToolResult with a single text content item.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
