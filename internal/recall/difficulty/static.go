package difficulty

import "context"

// Static returns the configured difficulty verbatim. A configured value of
// "adaptive" routes to the Adaptive heuristics instead — the user's expressed
// intent lives here, the implementation lives in Adaptive.
type Static struct {
	Value string
}

// Resolve returns s.Value as-is unless it equals "adaptive", in which case
// the Adaptive resolver produces the concrete level. An empty value is
// preserved: the synthesis caller substitutes its own safety-net default.
func (s Static) Resolve(ctx context.Context, synth SynthesisContext) string {
	if s.Value == "adaptive" {
		return Adaptive{}.Resolve(ctx, synth)
	}
	return s.Value
}
