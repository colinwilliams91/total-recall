package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/colinwilliams91/total-recall/internal/cache"
	"github.com/colinwilliams91/total-recall/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const recallWorkflowInstructions = `After any git commit in this project, call recall_next to check for a pending recall question. Pass the current repository path as "repo" (the absolute path from git rev-parse --show-toplevel) AND the current branch as "branch" (from git rev-parse --abbrev-ref HEAD) so questions are scoped to active work. When unable to determine the branch (detached HEAD), the daemon returns no question — do not retry without a branch context. If a question is returned, present it to the user and record their selection with recall_select (user-side field is "selected_index", NOT the older "answer_index" name). The recall_next response does NOT include the correctness key — it is withheld until the user submits. After recall_select returns, the response includes "correct": true/false, "correct_index", and "correct_answer" (the text of the correct choice). If incorrect, provide a brief, direct explanation using your own knowledge — especially why the correct answer is right and why the chosen one doesn't fit. Do NOT call a separate AI tool for the explanation; the recall_select response gives you everything you need. If the queue is empty, continue normally.`

// repoFromToolInput dereferences an optional repo pointer, returning "" when
// absent. MCP clients that omit repo get no dequeue (the store layer requires
// both repo and branch).
func repoFromToolInput(r *string) string {
	if r != nil {
		return *r
	}
	return ""
}

// branchFromToolInput mirrors repoFromToolInput for the optional branch field.
func branchFromToolInput(b *string) string {
	if b != nil {
		return *b
	}
	return ""
}

// repoFromResourceURI extracts the repo query parameter from a resource URI
// like "recall://queue?repo=/path/to/repo". Returns "" when absent.
func repoFromResourceURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Query().Get("repo")
}

// branchFromResourceURI extracts the branch query parameter from a resource
// URI like "recall://queue?repo=/path/to/repo&branch=feature-X". Returns "".
func branchFromResourceURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Query().Get("branch")
}

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

	// recall_next — atomically dequeues the next pending question for repo+branch.
	// The correctness key is WITHHELD at delivery (no `correct_index` in the
	// response) — it arrives only in the recall_select response after the
	// user submits a selection.
	type recallNextIn struct {
		Repo   *string `json:"repo,omitempty"`
		Branch *string `json:"branch,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall_next",
		Description: "Dequeue the next pending recall question. Pass \"repo\" (absolute repo path) and \"branch\" (current git branch) to scope to the current repository and branch; both are required — the daemon returns null if either is missing. No global pool exists. The response does NOT include the correctness key; it is withheld until the user submits via recall_select.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recallNextIn) (*mcp.CallToolResult, any, error) {
		if store == nil {
			return textResult(`{"question":null}`), nil, nil
		}
		repo := repoFromToolInput(in.Repo)
		branch := branchFromToolInput(in.Branch)
		q, err := store.NextQuestion(ctx, repo, branch, "mcp")
		if err != nil {
			log.Printf("[mcp] recall_next error: %v", err)
			return nil, nil, err
		}
		if q == nil {
			return textResult(`{"question":null}`), nil, nil
		}
		choices := make([]string, len(q.Choices))
		for i, c := range q.Choices {
			choices[i] = c.Text
		}
		b, _ := json.Marshal(map[string]any{
			"id":            q.ID,
			"question_type": q.QuestionType,
			"question":      q.Question,
			"choices":       choices,
		})
		return textResult(string(b)), nil, nil
	})

	// recall_select — records the user's selection (renamed from recall_answer).
	// User-side field is "selected_index" (NOT "answer_index"). Response
	// includes both `correct_index` and `correct_answer` (the text of the
	// correct choice) for stateless MCP consumer feedback generation.
	type recallSelectIn struct {
		ID            int64   `json:"id"`
		SelectedIndex *int    `json:"selected_index"`
		Skip          bool    `json:"skip"`
		Repo          *string `json:"repo,omitempty"`
		Branch        *string `json:"branch,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall_select",
		Description: `Record the user's selection (or "skip") for a recall question. User-side field is "selected_index" (NOT the older "answer_index" name). The optional "repo" and "branch" are accepted for symmetry but the operation is ID-keyed.`,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recallSelectIn) (*mcp.CallToolResult, any, error) {
		if store == nil {
			return textResult(`{"ok":true,"skipped":false}`), nil, nil
		}
		if in.Skip {
			if err := store.SkipQuestion(ctx, in.ID); err != nil {
				log.Printf("[mcp] recall_select skip error: %v", err)
				return nil, nil, err
			}
			return textResult(`{"ok":true,"skipped":true}`), nil, nil
		}
		if in.SelectedIndex == nil {
			return nil, nil, fmt.Errorf("selected_index is required")
		}
		q, err := store.GetQuestion(ctx, in.ID)
		if err != nil {
			log.Printf("[mcp] recall_select get error: %v", err)
			return nil, nil, err
		}
		if q == nil {
			return nil, nil, fmt.Errorf("question %d not found", in.ID)
		}
		if *in.SelectedIndex < 0 || *in.SelectedIndex >= len(q.Choices) {
			return nil, nil, fmt.Errorf("selected_index out of range")
		}
		selectedChoice := q.Choices[*in.SelectedIndex]
		correct := selectedChoice.IsCorrect
		correctIndex := q.CorrectIndex
		correctText := ""
		if correctIndex >= 0 && correctIndex < len(q.Choices) {
			correctText = q.Choices[correctIndex].Text
		}
		if err := store.SubmitSelection(ctx, in.ID, []int64{selectedChoice.ID}, ""); err != nil {
			log.Printf("[mcp] recall_select error: %v", err)
			return nil, nil, err
		}
		b, _ := json.Marshal(map[string]any{
			"ok":             true,
			"correct":        correct,
			"correct_index":  correctIndex,
			"correct_answer": correctText,
		})
		return textResult(string(b)), nil, nil
	})

	// recall_status — daemon health and queue depth.
	type recallStatusIn struct {
		Repo   *string `json:"repo,omitempty"`
		Branch *string `json:"branch,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recall_status",
		Description: "Return daemon health, AI configuration status, and pending question queue depth. Pass both \"repo\" and \"branch\" to scope queue_depth; otherwise it returns 0 (no global pool).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recallStatusIn) (*mcp.CallToolResult, any, error) {
		aiConfigured := cfg.AI.Provider != ""
		depth := 0
		if store != nil {
			var err error
			depth, err = store.QueueDepth(ctx, repoFromToolInput(in.Repo), branchFromToolInput(in.Branch))
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
	}, queueResourceHandler(store))

	// Template matches recall://queue?repo=<path>&branch=<branch> so scoped
	// reads reach the handler. AddResource only matches exact URIs; the
	// template handles the query-param variant. Both share the same handler
	// which parses repo and branch from the URI.
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "recall://queue{?repo}{&branch}",
		Name:        "recall queue (repo-and-branch-scoped)",
		Description: "Current pending question queue depth and next question preview, scoped by repo and branch.",
		MIMEType:    "application/json",
	}, queueResourceHandler(store))

	// recall://recent — last 10 answered questions.
	srv.AddResource(&mcp.Resource{
		URI:         "recall://recent",
		Name:        "recent answers",
		Description: "Last 10 answered recall questions.",
		MIMEType:    "application/json",
	}, recentResourceHandler(store))

	// Template for repo+branch-scoped recent reads:
	// recall://recent?repo=<path>&branch=<branch>
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "recall://recent{?repo}{&branch}",
		Name:        "recent answers (repo-and-branch-scoped)",
		Description: "Last 10 answered recall questions, scoped by repo and branch.",
		MIMEType:    "application/json",
	}, recentResourceHandler(store))

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

// queueResourceHandler returns a ReadResource handler for the recall://queue
// resource that scopes QueueDepth and PeekNextQuestion by the (repo, branch)
// pair extracted from the request URI. Shared by the plain resource and the
// URI template. When either is empty, the store returns 0 / nil automatically
// and the response shows {"depth":0,"next":null}.
func queueResourceHandler(store *cache.Store) func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		repo := repoFromResourceURI(req.Params.URI)
		branch := branchFromResourceURI(req.Params.URI)
		depth := 0
		var nextPayload any = nil
		if store != nil {
			var err error
			depth, err = store.QueueDepth(ctx, repo, branch)
			if err != nil {
				return nil, fmt.Errorf("queue depth: %w", err)
			}
			q, err := store.PeekNextQuestion(ctx, repo, branch)
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
	}
}

// recentResourceHandler returns a ReadResource handler for the recall://recent
// resource that scopes RecentAnswered by the (repo, branch) pair extracted
// from the request URI. Shared by the plain resource and the URI template.
// When either is empty, the store returns nil and the response shows [].
func recentResourceHandler(store *cache.Store) func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		repo := repoFromResourceURI(req.Params.URI)
		branch := branchFromResourceURI(req.Params.URI)
		var answered []cache.StoredQuestion
		if store != nil {
			var err error
			answered, err = store.RecentAnswered(ctx, repo, branch, 10)
			if err != nil {
				return nil, fmt.Errorf("recent answered: %w", err)
			}
		}
		items := make([]map[string]any, 0, len(answered))
		for _, q := range answered {
			choiceTexts := make([]string, len(q.Choices))
			for i, c := range q.Choices {
				choiceTexts[i] = c.Text
			}
			selectedIndices := make([]int, 0, len(q.Selections))
			for _, s := range q.Selections {
				for i, c := range q.Choices {
					if c.ID == s.ChoiceID {
						selectedIndices = append(selectedIndices, i)
						break
					}
				}
			}
			item := map[string]any{
				"id":              q.ID,
				"question_type":   q.QuestionType,
				"status":          q.Status,
				"question":        q.Question,
				"choices":         choiceTexts,
				"correct_index":   q.CorrectIndex,
				"selected_indices": selectedIndices,
				"feedback":        q.Feedback,
			}
			items = append(items, item)
		}
		b, _ := json.Marshal(items)
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      "recall://recent",
				MIMEType: "application/json",
				Text:     string(b),
			}},
		}, nil
	}
}
