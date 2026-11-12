package difficulty

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/colinwilliams91/total-recall/internal/cache"
)

// fakeStats is a canned LearnerStatsProvider for resolver-level tests.
type fakeStats struct {
	stats      cache.LearnerStats
	err        error
	gotWindow  int
	gotRepo    string
	gotBranch  string
}

func (f *fakeStats) LearnerStats(_ context.Context, repo, branch string, window int) (cache.LearnerStats, error) {
	f.gotWindow = window
	f.gotRepo = repo
	f.gotBranch = branch
	return f.stats, f.err
}

// captureLogs redirects the standard logger into a buffer for signal-line
// assertions, restoring the previous output and flags on cleanup.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	oldOut, oldFlags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
	})
	return buf
}

// fallbackSynth resolves to the commit-signal baseline's "intermediate"
// fallback (3 mixed-source concepts at avg 0.5: no heuristic fires).
func fallbackSynth() SynthesisContext {
	return SynthesisContext{
		Repo: "/repo/x",
		Branch: "main",
		Concepts: []cache.ConceptRow{
			concept(0.5, "code"),
			concept(0.5, "conversation"),
			concept(0.5, "code"),
		},
	}
}

// hardSynth resolves to "hard" via the (final) ai-delegation rule.
func hardSynth() SynthesisContext {
	synth := SynthesisContext{
		Repo: "/repo/x",
		Branch: "main",
		Concepts: []cache.ConceptRow{
			concept(0.5, "code"),
			concept(0.5, "conversation"),
			concept(0.5, "code"),
		},
	}
	synth.CommitMsg, synth.DiffSnippet = delegationShape()
	return synth
}

func answered(n int) cache.LearnerStats { return cache.LearnerStats{Answered: n} }

func TestPerformanceEscalatesHighPerformer(t *testing.T) {
	stats := cache.LearnerStats{Answered: 20, Correct: 19} // ratio 0.95
	f := &fakeStats{stats: stats}
	got := (&Performance{Baseline: Adaptive{}, Stats: f}).Resolve(context.Background(), fallbackSynth())
	if got.Difficulty != "hard" {
		t.Fatalf("expected escalation intermediate→hard, got %q", got.Difficulty)
	}
	if !contains(got.Signals, learnerTierSignal) {
		t.Fatalf("expected learner-tier signal, got %v", got.Signals)
	}
	if f.gotWindow != performanceWindow || f.gotRepo != "/repo/x" || f.gotBranch != "main" {
		t.Fatalf("stats read scoped wrong: window=%d repo=%q branch=%q", f.gotWindow, f.gotRepo, f.gotBranch)
	}
}

func TestPerformanceEscalationCapsAtHard(t *testing.T) {
	stats := cache.LearnerStats{Answered: 20, Correct: 19}
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), hardSynth())
	if got.Difficulty != "hard" {
		t.Fatalf("expected hard capped (no double escalation), got %q", got.Difficulty)
	}
	if !contains(got.Signals, learnerTierSignal) {
		t.Fatalf("expected learner-tier signal on the capped escalation, got %v", got.Signals)
	}
}

func TestPerformanceDeEscalatesStrugglingLearner(t *testing.T) {
	stats := cache.LearnerStats{Answered: 20, Correct: 7} // ratio 0.35
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), hardSynth())
	if got.Difficulty != "intermediate" {
		t.Fatalf("expected de-escalation hard→intermediate, got %q", got.Difficulty)
	}
	if !contains(got.Signals, learnerTierSignal) {
		t.Fatalf("expected learner-tier signal, got %v", got.Signals)
	}
}

func TestPerformanceDeEscalationFloorsAtEasy(t *testing.T) {
	stats := cache.LearnerStats{Answered: 20, Correct: 2}
	synth := SynthesisContext{
		Repo: "/repo/x", Branch: "main",
		Concepts: []cache.ConceptRow{
			concept(0.2, "code"),
			concept(0.2, "conversation"),
			concept(0.2, "code"),
			concept(0.2, "conversation"),
			concept(0.2, "code"),
		},
	}
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), synth)
	if got.Difficulty != "easy" {
		t.Fatalf("expected easy floored (no double de-escalation), got %q", got.Difficulty)
	}
}

func TestPerformanceMidRangeKeepsBaseline(t *testing.T) {
	stats := cache.LearnerStats{Answered: 20, Correct: 15} // ratio 0.75
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), fallbackSynth())
	if got.Difficulty != "intermediate" {
		t.Fatalf("expected baseline unchanged at 0.75, got %q", got.Difficulty)
	}
	if contains(got.Signals, learnerTierSignal) {
		t.Fatalf("expected no learner-tier signal when the tier holds, got %v", got.Signals)
	}
}

func TestPerformanceMinHistoryBoundary(t *testing.T) {
	// 4 answered: below the minimum → cold start, tier rules do not engage.
	cold := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: answered(4)}}).Resolve(context.Background(), fallbackSynth())
	if cold.Difficulty != "intermediate" || contains(cold.Signals, learnerTierSignal) {
		t.Fatalf("expected cold-start baseline below the minimum history, got %+v", cold)
	}
	if !contains(cold.Signals, coldStartSignal) {
		t.Fatalf("expected cold-start signal below the minimum history, got %v", cold.Signals)
	}

	// Exactly 5 answered: history is sufficient from the Nth answered onward.
	hot := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: cache.LearnerStats{Answered: 5, Correct: 5}}}).Resolve(context.Background(), fallbackSynth())
	if hot.Difficulty != "hard" || !contains(hot.Signals, learnerTierSignal) {
		t.Fatalf("expected tier rules engaged at exactly 5 answered, got %+v", hot)
	}
}

func TestPerformanceColdStartOnMissingStats(t *testing.T) {
	// Nil provider behaves as cold start.
	got := (&Performance{Baseline: Adaptive{}}).Resolve(context.Background(), fallbackSynth())
	if got.Difficulty != "intermediate" || !contains(got.Signals, coldStartSignal) || len(got.FocusConcepts) != 0 {
		t.Fatalf("expected cold-start baseline on nil provider, got %+v", got)
	}

	// An erroring provider must never block synthesis either.
	got = (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{err: context.DeadlineExceeded}}).Resolve(context.Background(), fallbackSynth())
	if got.Difficulty != "intermediate" || !contains(got.Signals, coldStartSignal) {
		t.Fatalf("expected cold-start baseline on provider error, got %+v", got)
	}
}

func tally(slug string, misses, hits int, last time.Time) cache.ConceptOutcome {
	return cache.ConceptOutcome{Slug: slug, Misses: misses, Hits: hits, LastAnsweredAt: last}
}

func TestPerformanceFocusRanking(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	stats := cache.LearnerStats{
		Answered: 20, Correct: 10,
		Concepts: []cache.ConceptOutcome{
			tally("jitter", 1, 2, base.Add(3*time.Hour)),       // on-surface, 1 miss
			tally("circuit_breaker", 2, 0, base.Add(4*time.Hour)), // on-surface, 2 misses (most recent)
			tally("retry", 2, 1, base.Add(1*time.Hour)),        // cross-surface, 2 misses (older)
			tally("all_hits", 0, 3, base.Add(5*time.Hour)),     // miss-free: never qualifies
		},
	}
	// Surface: circuit_breaker and jitter are in the current feed; retry is
	// answered on another surface and absent from the feed.
	synth := fallbackSynth()
	synth.Concepts = []cache.ConceptRow{
		{Concept: "circuit breaker", Slug: "circuit_breaker", Weight: 0.9, SeenAt: base},
		{Concept: "jitter", Slug: "jitter", Weight: 0.7, SeenAt: base},
	}
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), synth)
	want := []string{"circuit_breaker", "jitter", "retry"}
	if len(got.FocusConcepts) != len(want) {
		t.Fatalf("expected focus %v, got %v", want, got.FocusConcepts)
	}
	for i := range want {
		if got.FocusConcepts[i] != want[i] {
			t.Fatalf("expected focus %v, got %v", want, got.FocusConcepts)
		}
	}
}

func TestPerformanceFocusCapAndSurfacePriority(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	// Four weak on-surface concepts plus one weaker-history cross-surface
	// concept: the cap fills from the surface first; the cross-surface
	// concept only enters if a slot is left.
	stats := cache.LearnerStats{
		Answered: 20, Correct: 5,
		Concepts: []cache.ConceptOutcome{
			tally("w1", 5, 0, base),
			tally("w2", 4, 0, base),
			tally("w3", 3, 0, base),
			tally("w4", 2, 0, base),
			tally("cross", 9, 0, base), // more misses, but off-surface
		},
	}
	synth := fallbackSynth()
	synth.Concepts = []cache.ConceptRow{
		{Concept: "w1", Slug: "w1", SeenAt: base},
		{Concept: "w2", Slug: "w2", SeenAt: base},
		{Concept: "w3", Slug: "w3", SeenAt: base},
		{Concept: "w4", Slug: "w4", SeenAt: base},
	}
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), synth)
	if strings.Join(got.FocusConcepts, ",") != "w1,w2,w3" {
		t.Fatalf("expected cap 3 filled from the working surface, got %v", got.FocusConcepts)
	}

	// Drop one surface concept: the freed slot goes to the cross-surface
	// concept, appended after the remaining surface concepts.
	synth.Concepts = []cache.ConceptRow{
		{Concept: "w2", Slug: "w2", SeenAt: base},
		{Concept: "w3", Slug: "w3", SeenAt: base},
		{Concept: "w4", Slug: "w4", SeenAt: base},
	}
	got = (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), synth)
	if strings.Join(got.FocusConcepts, ",") != "w2,w3,w4" {
		t.Fatalf("expected surface-first fill w2,w3,w4 (cap full, cross concept excluded), got %v", got.FocusConcepts)
	}

	// Drop two: w3, w4 stay on-surface, and the leftover slot goes to the
	// cross-surface concept appended last.
	synth.Concepts = []cache.ConceptRow{
		{Concept: "w3", Slug: "w3", SeenAt: base},
		{Concept: "w4", Slug: "w4", SeenAt: base},
	}
	got = (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), synth)
	if strings.Join(got.FocusConcepts, ",") != "w3,w4,cross" {
		t.Fatalf("expected cross-surface concept in the leftover slot only, got %v", got.FocusConcepts)
	}
}

func TestPerformanceEmptyFocusWhenAllCorrect(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	stats := cache.LearnerStats{
		Answered: 20, Correct: 20,
		Concepts: []cache.ConceptOutcome{
			tally("a", 0, 3, base),
			tally("b", 0, 2, base),
		},
	}
	got := (&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), fallbackSynth())
	if len(got.FocusConcepts) != 0 {
		t.Fatalf("expected empty focus when all recent outcomes are correct, got %v", got.FocusConcepts)
	}
	if got.Difficulty != "hard" || !contains(got.Signals, learnerTierSignal) {
		t.Fatalf("expected perfect record to escalate, got %+v", got)
	}
}

func TestPerformanceLogSignals(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	fallbackSynth()

	// Baseline (no history) case: cold-start logged.
	buf := captureLogs(t)
	(&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: answered(4)}}).Resolve(context.Background(), fallbackSynth())
	if !strings.Contains(buf.String(), "cold-start") {
		t.Fatalf("expected cold-start in the log line, got %q", buf.String())
	}

	// learner-tier escalation and de-escalation cases.
	buf = captureLogs(t)
	(&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: cache.LearnerStats{Answered: 20, Correct: 19}}}).Resolve(context.Background(), fallbackSynth())
	if !strings.Contains(buf.String(), "learner-tier") || !strings.Contains(buf.String(), `selected "hard"`) {
		t.Fatalf("expected learner-tier escalation log line, got %q", buf.String())
	}

	buf = captureLogs(t)
	(&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: cache.LearnerStats{Answered: 20, Correct: 7}}}).Resolve(context.Background(), hardSynth())
	if !strings.Contains(buf.String(), "learner-tier") || !strings.Contains(buf.String(), `selected "intermediate"`) {
		t.Fatalf("expected learner-tier de-escalation log line, got %q", buf.String())
	}

	// Focus contents logged when non-empty.
	buf = captureLogs(t)
	stats := cache.LearnerStats{Answered: 20, Correct: 10, Concepts: []cache.ConceptOutcome{tally("circuit_breaker", 2, 0, base)}}
	(&Performance{Baseline: Adaptive{}, Stats: &fakeStats{stats: stats}}).Resolve(context.Background(), fallbackSynth())
	if !strings.Contains(buf.String(), "performance focus concepts: circuit_breaker") {
		t.Fatalf("expected focus concepts logged, got %q", buf.String())
	}
}

// contains reports whether the slice holds the exact string.
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
