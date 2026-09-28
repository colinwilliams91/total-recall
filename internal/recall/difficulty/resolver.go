// Package difficulty selects the recall question difficulty level passed to
// synthesis prompt construction. Selection is a separable concern from prompt
// building: the resolver inspects the SynthesisContext signals and returns a
// concrete level (or "" for "no preference", which the caller defaults).
package difficulty

import (
	"context"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

// SynthesisContext carries the inputs a resolver may inspect: recent concept
// rows (with weights and sources) plus the commit message and diff snippet
// from the hook event. It mirrors the synthesis inputs so resolvers need no
// dependency on the recall engine.
type SynthesisContext struct {
	Concepts    []cache.ConceptRow
	CommitMsg   string
	DiffSnippet string
}

// Resolver selects a difficulty level for one synthesis call.
type Resolver interface {
	Resolve(ctx context.Context, synth SynthesisContext) string
}
