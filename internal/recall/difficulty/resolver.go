// Package difficulty selects the recall question difficulty level passed to
// synthesis prompt construction. Selection is a separable concern from prompt
// building: the resolver inspects the SynthesisContext signals and returns a
// concrete level (or "" for "no preference", which the caller defaults).
package difficulty

import (
	"context"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

// SynthesisContext carries the inputs a resolver may inspect: the repo and
// branch the synthesis is scoped to (learner-stats reads are scoped per
// surface), recent concept rows (with weights, sources, and slugs) plus the
// commit message and diff snippet from the hook event. It mirrors the
// synthesis inputs so resolvers need no dependency on the recall engine.
type SynthesisContext struct {
	Repo        string
	Branch      string
	Concepts    []cache.ConceptRow
	CommitMsg   string
	DiffSnippet string
}

// Resolution carries one resolver's outcome for a synthesis call: the
// difficulty level ("" = no preference, which the caller substitutes a
// default for), the focus concepts to bias the synthesis feed toward
// demonstrated weaknesses (empty when the resolver has no content steer),
// and the fired signal names for the queued-event audit payload (empty when
// the resolver reports none).
type Resolution struct {
	Difficulty    string
	FocusConcepts []string
	Signals       []string
}

// Resolver selects the resolution for one synthesis call: a difficulty level
// plus, optionally, focus concepts for content targeting.
type Resolver interface {
	Resolve(ctx context.Context, synth SynthesisContext) Resolution
}
