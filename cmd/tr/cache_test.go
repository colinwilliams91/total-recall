package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/cache"
	_ "modernc.org/sqlite"
)

func setupCache(t *testing.T) *cache.Store {
	t.Helper()
	t.Setenv("TR_HOME", t.TempDir())

	s, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// buildChoices constructs a `[]cache.Choice` from raw text + a correct index.
// Position is assigned sequentially (0..N-1). Used by all save-then-claim
// tests below — the legacy []string signature is gone.
func buildChoices(texts []string, correctIdx int) []cache.Choice {
	out := make([]cache.Choice, len(texts))
	for i, text := range texts {
		out[i] = cache.Choice{Text: text, IsCorrect: i == correctIdx, Position: i}
	}
	return out
}

func TestOpenCreatesDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)
	t.Setenv("TR_HOME", "")

	s, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open failed: %v", err)
	}
	defer s.Close()

	dbPath := filepath.Join(tempDir, ".tr", "memory.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected memory.db at %s, but file not found", dbPath)
	}
}

func TestOpenUsesTRHomeWhenSet(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("TR_HOME", tempDir)

	s, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open failed: %v", err)
	}
	defer s.Close()

	dbPath := filepath.Join(tempDir, "memory.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected memory.db at %s (TR_HOME), but file not found", dbPath)
	}
}

func TestSaveAndRetrieveConcepts(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	concepts := []cache.Fingerprint{
		{Concept: "exponential-backoff", Source: "code", Weight: 1.0},
		{Concept: "circuit-breaker", Source: "code", Weight: 0.8},
	}
	if err := s.Save(ctx, "/repo/test", "main", concepts); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	recent, err := s.Recent(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("Recent failed: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 concepts, got %d", len(recent))
	}
	if recent[0].Concept != "exponential-backoff" && recent[0].Concept != "circuit-breaker" {
		t.Fatalf("unexpected first concept: %q", recent[0].Concept)
	}
}

func TestSaveMultipleConceptsAndOrderBySeenAt(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "first", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "second", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	recent, err := s.Recent(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("Recent failed: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 concepts, got %d", len(recent))
	}
	if recent[0].Concept != "second" {
		t.Fatalf("expected most recent concept first, got %q", recent[0].Concept)
	}
}

func TestRecentConceptsScopedToRepo(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.Save(ctx, "/repo/x", "main", []cache.Fingerprint{
		{Concept: "x-concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("save for repo X failed: %v", err)
	}
	if err := s.Save(ctx, "/repo/y", "main", []cache.Fingerprint{
		{Concept: "y-concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("save for repo Y failed: %v", err)
	}

	xRecent, err := s.Recent(ctx, "/repo/x", "main", 10)
	if err != nil {
		t.Fatalf("Recent for repo X failed: %v", err)
	}
	if len(xRecent) != 1 || xRecent[0].Concept != "x-concept" {
		t.Fatalf("expected only x-concept for repo X, got %v", xRecent)
	}

	yRecent, err := s.Recent(ctx, "/repo/y", "main", 10)
	if err != nil {
		t.Fatalf("Recent for repo Y failed: %v", err)
	}
	if len(yRecent) != 1 || yRecent[0].Concept != "y-concept" {
		t.Fatalf("expected only y-concept for repo Y, got %v", yRecent)
	}
}

func TestRecentConceptsScopedToBranch(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.Save(ctx, "/r", "feature-X", []cache.Fingerprint{
		{Concept: "x-branch-concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("save for feature-X: %v", err)
	}

	mainRecent, err := s.Recent(ctx, "/r", "main", 10)
	if err != nil {
		t.Fatalf("Recent for main: %v", err)
	}
	if len(mainRecent) != 0 {
		t.Fatalf("expected 0 concepts for branch=main (cross-branch leak), got %d: %v", len(mainRecent), mainRecent)
	}

	xRecent, err := s.Recent(ctx, "/r", "feature-X", 10)
	if err != nil {
		t.Fatalf("Recent for feature-X: %v", err)
	}
	if len(xRecent) != 1 || xRecent[0].Concept != "x-branch-concept" {
		t.Fatalf("expected only x-branch-concept for branch=feature-X, got %v", xRecent)
	}
}

func TestSaveRefusesEmptyRepoOrBranch(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	concepts := []cache.Fingerprint{{Concept: "should-not-save", Source: "code", Weight: 1.0}}

	if err := s.Save(ctx, "", "main", concepts); err != nil {
		t.Fatalf("Save with empty repo: expected nil error, got %v", err)
	}
	if err := s.Save(ctx, "/r", "", concepts); err != nil {
		t.Fatalf("Save with empty branch: expected nil error, got %v", err)
	}
	if err := s.Save(ctx, "", "", concepts); err != nil {
		t.Fatalf("Save with both empty: expected nil error, got %v", err)
	}

	r, err := s.Recent(ctx, "/r", "main", 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(r) != 0 {
		t.Fatalf("expected 0 concepts after refusal, got %d: %v", len(r), r)
	}
}

func TestRecentRefusesEmptyRepoOrBranch(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.Save(ctx, "/r", "main", []cache.Fingerprint{
		{Concept: "real", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r1, err := s.Recent(ctx, "", "main", 10)
	if err != nil {
		t.Fatalf("Recent with empty repo: expected nil error, got %v", err)
	}
	if r1 != nil {
		t.Fatalf("Recent with empty repo: expected nil, got %v", r1)
	}
	r2, err := s.Recent(ctx, "/r", "", 10)
	if err != nil {
		t.Fatalf("Recent with empty branch: expected nil error, got %v", err)
	}
	if r2 != nil {
		t.Fatalf("Recent with empty branch: expected nil, got %v", r2)
	}
}

func TestSaveQuestionRefusesEmptyRepoOrBranch(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "", "main", "q", buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion with empty repo: expected nil error, got %v", err)
	}
	if err := s.SaveQuestion(ctx, "/r", "", "q", buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion with empty branch: expected nil error, got %v", err)
	}
	depth, err := s.QueueDepth(ctx, "/r", "main")
	if err != nil {
		t.Fatalf("QueueDepth: %v", err)
	}
	if depth != 0 {
		t.Fatalf("expected QueueDepth 0 after refusal, got %d", depth)
	}
}

func TestSaveQuestionAndClaim(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "What is a goroutine?", buildChoices([]string{"a", "b", "c"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil question")
	}
	if q.Question != "What is a goroutine?" {
		t.Fatalf("expected question text, got %q", q.Question)
	}
	if len(q.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(q.Choices))
	}
	if q.Status != "delivered" {
		t.Fatalf("expected status 'delivered', got %q", q.Status)
	}
}

func TestNextQuestionReturnsNilWhenEmpty(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	q, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if q != nil {
		t.Fatal("expected nil question when queue is empty")
	}
}

func TestNextQuestionIdempotent(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "single question", buildChoices([]string{"x", "y"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q1, err := s.NextQuestion(ctx, "/repo/test", "main", "claimer-1")
	if err != nil {
		t.Fatalf("first NextQuestion failed: %v", err)
	}
	if q1 == nil {
		t.Fatal("expected first question")
	}

	q2, err := s.NextQuestion(ctx, "/repo/test", "main", "claimer-2")
	if err != nil {
		t.Fatalf("second NextQuestion failed: %v", err)
	}
	if q2 != nil {
		t.Fatal("expected nil on second claim (already claimed)")
	}
}

func TestNextQuestionRepoIsolation(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/x", "main", "X's question", buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion for repo X failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "/repo/y", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion for repo Y failed: %v", err)
	}
	if q != nil {
		t.Fatal("expected nil — repo Y must not receive repo X's question")
	}

	q, err = s.NextQuestion(ctx, "/repo/x", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion for repo X failed: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil question for repo X")
	}
	if q.Question != "X's question" {
		t.Fatalf("expected X's question, got %q", q.Question)
	}
}

func TestSubmitSelection(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "test question", buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil question")
	}
	if len(q.Choices) == 0 {
		t.Fatal("expected at least one choice")
	}
	// Pick the first (correct) choice.
	if err := s.SubmitSelection(ctx, q.ID, []int64{q.Choices[0].ID}, ""); err != nil {
		t.Fatalf("SubmitSelection failed: %v", err)
	}
}

func TestQueueDepthEmpty(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	n, err := s.QueueDepth(ctx, "/repo/test", "main")
	if err != nil {
		t.Fatalf("QueueDepth failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected queue depth 0, got %d", n)
	}
}

func TestQueueDepthIncrementsOnSave(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	n, err := s.QueueDepth(ctx, "/repo/test", "main")
	if err != nil {
		t.Fatalf("initial QueueDepth failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected initial depth 0, got %d", n)
	}

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "q1", buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	n, err = s.QueueDepth(ctx, "/repo/test", "main")
	if err != nil {
		t.Fatalf("QueueDepth after save failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected depth 1 after save, got %d", n)
	}

	if _, err := s.NextQuestion(ctx, "/repo/test", "main", "test"); err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	n, err = s.QueueDepth(ctx, "/repo/test", "main")
	if err != nil {
		t.Fatalf("QueueDepth after claim failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected depth 0 after claim, got %d", n)
	}
}

func TestQueueDepthScopedToRepo(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/x", "main", "q1", buildChoices([]string{"a"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion for repo X failed: %v", err)
	}
	if err := s.SaveQuestion(ctx, "/repo/y", "main", "q2", buildChoices([]string{"a"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion for repo Y failed: %v", err)
	}

	xDepth, err := s.QueueDepth(ctx, "/repo/x", "main")
	if err != nil {
		t.Fatalf("QueueDepth for repo X failed: %v", err)
	}
	if xDepth != 1 {
		t.Fatalf("expected depth 1 for repo X, got %d", xDepth)
	}

	yDepth, err := s.QueueDepth(ctx, "/repo/y", "main")
	if err != nil {
		t.Fatalf("QueueDepth for repo Y failed: %v", err)
	}
	if yDepth != 1 {
		t.Fatalf("expected depth 1 for repo Y, got %d", yDepth)
	}
}

// rawDBPath resolves the memory.db path under the active test data directory.
// Honors TR_HOME when set (matching cache.trDir); otherwise falls back to
// ~/.tr/memory.db. Used by openRawDB for column-level assertions.
func rawDBPath(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("TR_HOME"); env != "" {
		return filepath.Join(env, "memory.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return filepath.Join(home, ".tr", "memory.db")
}

// openRawDB opens a direct database/sql handle to the test memory.db for
// column-level assertions not exposed by the cache.Store API. The handle is
// closed automatically via t.Cleanup.
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", rawDBPath(t))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestSaveQuestionPersistsIsCorrectOnChoice validates the new schema: the
// engine's correctness key is now a row property on the choices table, not a
// positional reference into a JSON array.
func TestSaveQuestionPersistsIsCorrectOnChoice(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// correct is index 2 ("c")
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "correct-index question",
		buildChoices([]string{"a", "b", "c"}, 2), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if claimed.CorrectIndex != 2 {
		t.Fatalf("expected derived CorrectIndex 2, got %d", claimed.CorrectIndex)
	}
	if len(claimed.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(claimed.Choices))
	}
	if claimed.Choices[0].Text != "a" || claimed.Choices[1].Text != "b" || claimed.Choices[2].Text != "c" {
		t.Fatalf("expected choices [a b c], got %v", claimed.Choices)
	}
	// Source of truth: IsCorrect on the row, not a position.
	if !claimed.Choices[2].IsCorrect {
		t.Fatal("expected choice[2].IsCorrect=true (the engine's key)")
	}
	if claimed.Choices[0].IsCorrect || claimed.Choices[1].IsCorrect {
		t.Fatal("expected choice[0] and choice[1].IsCorrect=false")
	}
}

func TestGetQuestionReturnsFullRow(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "full row question",
		buildChoices([]string{"x", "y", "z"}, 1), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	q, err := s.GetQuestion(ctx, claimed.ID)
	if err != nil {
		t.Fatalf("GetQuestion failed: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil from GetQuestion")
	}
	if q.ID != claimed.ID {
		t.Fatalf("expected ID %d, got %d", claimed.ID, q.ID)
	}
	if q.Question != "full row question" {
		t.Fatalf("expected question %q, got %q", "full row question", q.Question)
	}
	if len(q.Choices) != 3 {
		t.Fatalf("expected 3 choices, got %d", len(q.Choices))
	}
	if q.CorrectIndex != 1 {
		t.Fatalf("expected CorrectIndex 1, got %d", q.CorrectIndex)
	}

	missing, err := s.GetQuestion(ctx, 99999)
	if err != nil {
		t.Fatalf("GetQuestion missing ID: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for missing ID, got %+v", missing)
	}
}

func TestSetFeedback(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "feedback column question",
		buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	// Set feedback — column should be populated.
	if err := s.SetFeedback(ctx, claimed.ID, "the answer is A"); err != nil {
		t.Fatalf("SetFeedback (set): %v", err)
	}
	db := openRawDB(t)
	var fb sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT feedback FROM questions WHERE id = ?`, claimed.ID).Scan(&fb); err != nil {
		t.Fatalf("raw query feedback: %v", err)
	}
	if !fb.Valid {
		t.Fatal("expected feedback column non-NULL after SetFeedback")
	}
	if fb.String != "the answer is A" {
		t.Fatalf("expected feedback %q, got %q", "the answer is A", fb.String)
	}

	// Clear feedback by setting empty — column should be NULL.
	if err := s.SetFeedback(ctx, claimed.ID, ""); err != nil {
		t.Fatalf("SetFeedback (clear): %v", err)
	}
	var fb2 sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT feedback FROM questions WHERE id = ?`, claimed.ID).Scan(&fb2); err != nil {
		t.Fatalf("raw query feedback after clear: %v", err)
	}
	if fb2.Valid {
		t.Fatalf("expected feedback column NULL after empty SetFeedback, got %q", fb2.String)
	}
}

func TestSkipQuestionWritesEventAndStatus(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "skip me", buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.SkipQuestion(ctx, claimed.ID); err != nil {
		t.Fatalf("SkipQuestion failed: %v", err)
	}

	// Verify the question's status is 'skipped' via a direct query.
	db := openRawDB(t)
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM questions WHERE id = ?`, claimed.ID).Scan(&status); err != nil {
		t.Fatalf("raw query status: %v", err)
	}
	if status != "skipped" {
		t.Fatalf("expected status %q, got %q", "skipped", status)
	}

	// Verify a 'skipped' event row exists in question_events.
	var eventType string
	if err := db.QueryRowContext(ctx,
		`SELECT event_type FROM question_events WHERE question_id = ? AND event_type = 'skipped'`,
		claimed.ID).Scan(&eventType); err != nil {
		t.Fatalf("raw query skipped event: %v", err)
	}
	if eventType != "skipped" {
		t.Fatalf("expected event_type %q, got %q", "skipped", eventType)
	}

	// RecentAnswered MUST exclude skipped.
	recent, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(recent) != 0 {
		t.Fatalf("expected 0 answered rows (skipped excluded), got %d", len(recent))
	}
	// RecentSkipped MUST include it.
	skipped, err := s.RecentSkipped(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentSkipped failed: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped row, got %d", len(skipped))
	}
	if skipped[0].Status != "skipped" {
		t.Fatalf("expected status 'skipped', got %q", skipped[0].Status)
	}
}

func TestSubmitSelectionWithFeedback(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "feedback question",
		buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	// Pick the wrong choice (index 1)
	wrongChoice := claimed.Choices[1]
	if err := s.SubmitSelection(ctx, claimed.ID, []int64{wrongChoice.ID}, "A is correct because..."); err != nil {
		t.Fatalf("SubmitSelection failed: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 answered row, got %d", len(recent))
	}
	row := recent[0]
	if row.Feedback == nil {
		t.Fatal("expected Feedback non-nil")
	}
	if *row.Feedback != "A is correct because..." {
		t.Fatalf("expected feedback %q, got %q", "A is correct because...", *row.Feedback)
	}
	if len(row.Selections) != 1 || row.Selections[0].ChoiceID != wrongChoice.ID {
		t.Fatalf("expected single selection for wrongChoice.ID %d, got %+v", wrongChoice.ID, row.Selections)
	}
	if row.Status != "answered" {
		t.Fatalf("expected status 'answered', got %q", row.Status)
	}
}

func TestSubmitSelectionEmptyFeedbackStoresNull(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "empty feedback question",
		buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.SubmitSelection(ctx, claimed.ID, []int64{claimed.Choices[0].ID}, ""); err != nil {
		t.Fatalf("SubmitSelection failed: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 answered row, got %d", len(recent))
	}
	row := recent[0]
	if row.Feedback != nil {
		t.Fatalf("expected Feedback nil for empty feedback, got %v", *row.Feedback)
	}
}

// TestRecentAnsweredExcludesSkippedAndSkipsGetIt validates the three-way
// split between RecentAnswered, RecentSkipped, and RecentQuestions.
func TestRecentAnsweredExcludesSkippedAndSkipsGetIt(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// q1: correct, with feedback
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "terminal-style",
		buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion q1: %v", err)
	}
	q1, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion q1: %v", err)
	}
	if err := s.SubmitSelection(ctx, q1.ID, []int64{q1.Choices[0].ID}, "Good job!"); err != nil {
		t.Fatalf("SubmitSelection q1: %v", err)
	}

	// q2: incorrect, no feedback
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "mcp-style",
		buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion q2: %v", err)
	}
	q2, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion q2: %v", err)
	}
	if err := s.SubmitSelection(ctx, q2.ID, []int64{q2.Choices[1].ID}, ""); err != nil {
		t.Fatalf("SubmitSelection q2: %v", err)
	}

	// q3: skipped
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "skipped-one",
		buildChoices([]string{"a", "b"}, 0), ""); err != nil {
		t.Fatalf("SaveQuestion q3: %v", err)
	}
	q3, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion q3: %v", err)
	}
	if err := s.SkipQuestion(ctx, q3.ID); err != nil {
		t.Fatalf("SkipQuestion q3: %v", err)
	}

	answered, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(answered) != 2 {
		t.Fatalf("expected 2 answered rows (skip excluded), got %d", len(answered))
	}
	for _, r := range answered {
		if r.Status != "answered" {
			t.Fatalf("expected status 'answered', got %q for %q", r.Status, r.Question)
		}
		if r.Question == "skipped-one" {
			t.Fatal("RecentAnswered must not include skipped questions")
		}
	}

	skipped, err := s.RecentSkipped(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentSkipped failed: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped row, got %d", len(skipped))
	}
	if skipped[0].Status != "skipped" {
		t.Fatalf("expected status 'skipped', got %q", skipped[0].Status)
	}

	all, err := s.RecentQuestions(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("RecentQuestions failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 terminal rows, got %d", len(all))
	}
	// Verify Status field distinguishes answered from skipped.
	statusByQuestion := map[string]string{}
	for _, r := range all {
		statusByQuestion[r.Question] = r.Status
	}
	if statusByQuestion["terminal-style"] != "answered" {
		t.Fatal("terminal-style should be 'answered'")
	}
	if statusByQuestion["mcp-style"] != "answered" {
		t.Fatal("mcp-style should be 'answered'")
	}
	if statusByQuestion["skipped-one"] != "skipped" {
		t.Fatal("skipped-one should be 'skipped'")
	}
}

// TestRepoIndexesExist validates the indexes that cache.Open() creates. The
// prior idx_questions_repo_branch_q was tied to the implicit `delivered_at
// IS NULL` predicate; the new design filters on `status = 'queued'` (no
// covering index needed — `status` is a low-cardinality column and the repo
// + branch filter is selective enough for the in-process SQLite cost).
// The events index is the new hot path for queue ordering.
func TestRepoIndexesExist(t *testing.T) {
	s := setupCache(t)
	_ = s

	trHome := os.Getenv("TR_HOME")
	if trHome == "" {
		t.Fatal("TR_HOME not set by setupCache")
	}
	dbPath := filepath.Join(trHome, "memory.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	for _, idx := range []string{"idx_concepts_repo_branch_seen", "idx_choices_qid", "idx_qe_qid_time"} {
		var name string
		err := db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='index' AND name = ?`, idx).Scan(&name)
		if err == sql.ErrNoRows {
			t.Fatalf("index %s not found in sqlite_master", idx)
		}
		if err != nil {
			t.Fatalf("querying sqlite_master for %s: %v", idx, err)
		}
		if name != idx {
			t.Fatalf("expected index %s, got %s", idx, name)
		}
	}
}
