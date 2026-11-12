package difficulty

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

const (
	// performanceWindow bounds both learner windows: the correctness-ratio
	// window (scoped to the synthesis repo+branch) and the user-wide tally
	// window the focus ranking reads.
	performanceWindow = 20
	// performanceMinHistory gates the tier rules: below this many answered
	// questions in scope the resolution is the baseline alone (cold start) —
	// thresholds before the window fills would chase noise.
	performanceMinHistory = 5
	// performanceEscalateRatio is the user-specified example adopted verbatim:
	// a recent correctness ratio at or above it earns one harder level.
	performanceEscalateRatio = 0.92
	// performanceDeEscalateRatio is the struggling-learner floor: below it,
	// the learner is eased one level rather than hammered.
	performanceDeEscalateRatio = 0.50
	// performanceFocusCap bounds the weakness-targeted focus set.
	performanceFocusCap = 3
)

const (
	learnerTierSignal = "learner-tier"
	coldStartSignal   = "cold-start"
)

// LearnerStatsProvider supplies windowed learner outcome evidence for one
// synthesis scope. The narrow seam keeps the resolver off the raw store: it
// reads aggregates, never rows, and can never write.
type LearnerStatsProvider interface {
	LearnerStats(ctx context.Context, repo, branch string, window int) (cache.LearnerStats, error)
}

// Performance composes learner evidence over the commit-signal baseline:
// with insufficient learner history it IS the baseline (cold start = the
// commit-signal behavior); with sufficient history, the windowed correctness
// ratio moves the baseline at most one tier, and demonstrated weaknesses
// rank a focus set the synthesis feed reorders around. Commit-shape signals
// stay second-class by construction — the baseline (Adaptive, reordered)
// resolves first and the learner only modulates.
type Performance struct {
	// Baseline is the commit-signal resolver Performance wraps. Nil defaults
	// to the Adaptive heuristics.
	Baseline Resolver
	// Stats supplies learner outcomes; nil (or an erroring read) behaves as
	// cold start — a missing evidence source must never block synthesis.
	Stats LearnerStatsProvider
}

// Resolve resolves the baseline, then applies the learner tier and focus
// ranking. The returned Resolution names every fired signal: the baseline's,
// plus learner-tier when the tier rule moved the level, or cold-start when
// the baseline resolved alone.
func (p *Performance) Resolve(ctx context.Context, synth SynthesisContext) Resolution {
	baselineResolver := p.Baseline
	if baselineResolver == nil {
		baselineResolver = Adaptive{}
	}
	baseline := baselineResolver.Resolve(ctx, synth)

	if p.Stats == nil {
		return p.coldStart(baseline)
	}
	stats, err := p.Stats.LearnerStats(ctx, synth.Repo, synth.Branch, performanceWindow)
	if err != nil {
		log.Printf("[recall] learner stats unavailable (%v) — resolving from the commit-signal baseline", err)
		return p.coldStart(baseline)
	}
	if stats.Answered < performanceMinHistory {
		return p.coldStart(baseline)
	}

	signals := append([]string{}, baseline.Signals...)
	ratio := float64(stats.Correct) / float64(stats.Answered)
	level := baseline.Difficulty
	switch {
	case ratio >= performanceEscalateRatio:
		level = escalateTier(level)
		signals = append(signals, learnerTierSignal)
	case ratio < performanceDeEscalateRatio:
		level = deEscalateTier(level)
		signals = append(signals, learnerTierSignal)
	}

	focus := rankFocus(stats.Concepts, synth)
	log.Printf("[recall] performance resolver selected %q (signals: %s)", level, strings.Join(signals, ", "))
	if len(focus) > 0 {
		log.Printf("[recall] performance focus concepts: %s", strings.Join(focus, ", "))
	}
	return Resolution{Difficulty: level, FocusConcepts: focus, Signals: signals}
}

// coldStart returns the baseline resolution verbatim (no tier move, no
// focus), marked with the cold-start signal.
func (p *Performance) coldStart(baseline Resolution) Resolution {
	signals := append([]string{}, baseline.Signals...)
	signals = append(signals, coldStartSignal)
	log.Printf("[recall] performance resolver selected %q (signals: %s)", baseline.Difficulty, strings.Join(signals, ", "))
	return Resolution{Difficulty: baseline.Difficulty, Signals: signals}
}

// escalateTier moves one level up, capping at hard. Unknown levels pass
// through unchanged so a resolver electing "" keeps the caller's default.
func escalateTier(level string) string {
	switch level {
	case "easy":
		return "intermediate"
	case "intermediate":
		return "hard"
	default:
		return level
	}
}

// deEscalateTier moves one level down, floored at easy. Unknown levels pass
// through unchanged.
func deEscalateTier(level string) string {
	switch level {
	case "hard":
		return "intermediate"
	case "intermediate":
		return "easy"
	default:
		return level
	}
}

// rankFocus selects the weakness-targeted focus concepts: concepts with
// recent misses, ranked by miss count descending with most-recently-answered
// as the tiebreak. Concepts on the current working surface (present in the
// synthesis feed) rank first; cross-surface weak concepts fill only leftover
// slots — working-surface relevance stays first-class, tutoring
// second-class. Zero-outcome and miss-free concepts never qualify.
func rankFocus(concepts []cache.ConceptOutcome, synth SynthesisContext) []string {
	surface := make(map[string]bool, len(synth.Concepts))
	for _, c := range synth.Concepts {
		surface[c.Slug] = true
	}

	var onSurface, crossSurface []cache.ConceptOutcome
	for _, c := range concepts {
		if c.Misses <= 0 {
			continue
		}
		if surface[c.Slug] {
			onSurface = append(onSurface, c)
		} else {
			crossSurface = append(crossSurface, c)
		}
	}
	byMisses := func(a, b cache.ConceptOutcome) bool {
		if a.Misses != b.Misses {
			return a.Misses > b.Misses
		}
		return a.LastAnsweredAt.After(b.LastAnsweredAt)
	}
	sort.Slice(onSurface, func(i, j int) bool { return byMisses(onSurface[i], onSurface[j]) })
	sort.Slice(crossSurface, func(i, j int) bool { return byMisses(crossSurface[i], crossSurface[j]) })

	pool := append(append([]cache.ConceptOutcome{}, onSurface...), crossSurface...)
	var focus []string
	for _, c := range pool {
		if len(focus) >= performanceFocusCap {
			break
		}
		focus = append(focus, c.Slug)
	}
	return focus
}
