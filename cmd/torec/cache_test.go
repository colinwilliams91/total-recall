package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
		{Concept: "exponential-backoff", Slug: "exponential_backoff", Source: "code", Weight: 1.0},
		{Concept: "circuit-breaker", Slug: "circuit_breaker", Source: "code", Weight: 0.8},
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
		{Concept: "first", Slug: "first_concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "second", Slug: "second_concept", Source: "code", Weight: 1.0},
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
		{Concept: "x-concept", Slug: "x_concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("save for repo X failed: %v", err)
	}
	if err := s.Save(ctx, "/repo/y", "main", []cache.Fingerprint{
		{Concept: "y-concept", Slug: "y_concept", Source: "code", Weight: 1.0},
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
		{Concept: "x-branch-concept", Slug: "x_branch_concept", Source: "code", Weight: 1.0},
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

	concepts := []cache.Fingerprint{{Concept: "should-not-save", Slug: "should_not_save", Source: "code", Weight: 1.0}}

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
		{Concept: "real", Slug: "real_concept", Source: "code", Weight: 1.0},
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

	if err := s.SaveQuestion(ctx, "", "main", "q", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
		t.Fatalf("SaveQuestion with empty repo: expected nil error, got %v", err)
	}
	if err := s.SaveQuestion(ctx, "/r", "", "q", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "What is a goroutine?", buildChoices([]string{"a", "b", "c"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "single question", buildChoices([]string{"x", "y"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/x", "main", "X's question", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "test question", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "q1", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/x", "main", "q1", buildChoices([]string{"a"}, 0), "", nil); err != nil {
		t.Fatalf("SaveQuestion for repo X failed: %v", err)
	}
	if err := s.SaveQuestion(ctx, "/repo/y", "main", "q2", buildChoices([]string{"a"}, 0), "", nil); err != nil {
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
		buildChoices([]string{"a", "b", "c"}, 2), "", nil); err != nil {
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

// TestFreeTextQuestionTypeRejectedBySchema validates the free-text removal:
// 'free_text' is no longer a legal question_type value and the questions
// table no longer carries a correct_answer column.
func TestFreeTextQuestionTypeRejectedBySchema(t *testing.T) {
	setupCache(t)
	db := openRawDB(t)

	if _, err := db.Exec(`INSERT INTO questions (question_type, question, repo, branch)
VALUES ('free_text', 'q', '/repo/test', 'main')`); err == nil {
		t.Fatal("expected CHECK constraint violation for question_type='free_text', got nil error")
	} else if !strings.Contains(err.Error(), "CHECK") {
		t.Fatalf("expected CHECK constraint violation, got %v", err)
	}

	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('questions') WHERE name = 'correct_answer'`).
		Scan(&count); err != nil {
		t.Fatalf("pragma_table_info query: %v", err)
	}
	if count != 0 {
		t.Fatal("expected no correct_answer column on questions, found one")
	}
}

// TestUnknownQuestionTypeRejectedBySchema validates that any value outside
// the ('multiple_choice','multi_select') enum fails the CHECK constraint.
func TestUnknownQuestionTypeRejectedBySchema(t *testing.T) {
	setupCache(t)
	db := openRawDB(t)

	if _, err := db.Exec(`INSERT INTO questions (question_type, question, repo, branch)
VALUES ('poll', 'q', '/repo/test', 'main')`); err == nil {
		t.Fatal("expected CHECK constraint violation for question_type='poll', got nil error")
	} else if !strings.Contains(err.Error(), "CHECK") {
		t.Fatalf("expected CHECK constraint violation, got %v", err)
	}
}

func TestGetQuestionReturnsFullRow(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "full row question",
		buildChoices([]string{"x", "y", "z"}, 1), "", nil); err != nil {
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
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "skip me", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

func TestSubmitSelectionWritesUserActor(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "answer me", buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.SubmitSelection(ctx, claimed.ID, nil, ""); err != nil {
		t.Fatalf("SubmitSelection failed: %v", err)
	}

	db := openRawDB(t)
	var actor sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT actor FROM question_events WHERE question_id = ? AND event_type = 'answered'`,
		claimed.ID).Scan(&actor); err != nil {
		t.Fatalf("raw query answered event: %v", err)
	}
	if !actor.Valid || actor.String != "user" {
		t.Fatalf("expected answered event actor %q, got %q (valid=%v)", "user", actor.String, actor.Valid)
	}
}

func TestSubmitSelectionWithFeedback(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "feedback question",
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
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

// ── identity + provenance tests (concept-provenance change) ─────────────────

// TestOpenCreatesIdentityTables validates the two-layer schema reshape: the
// three new tables are present in sqlite_master on a fresh store.
func TestOpenCreatesIdentityTables(t *testing.T) {
	setupCache(t)
	db := openRawDB(t)

	for _, table := range []string{"concept_registry", "concept_aliases", "question_concepts"} {
		var name string
		err := db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name)
		if err == sql.ErrNoRows {
			t.Fatalf("table %s not found in sqlite_master", table)
		}
		if err != nil {
			t.Fatalf("querying sqlite_master for %s: %v", table, err)
		}
		if name != table {
			t.Fatalf("expected table %s, got %s", table, name)
		}
	}

	// The reshaped sightings table carries the registry FK column.
	var slugCol int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM pragma_table_info('concepts') WHERE name = 'slug'`).Scan(&slugCol); err != nil {
		t.Fatalf("pragma_table_info query: %v", err)
	}
	if slugCol != 1 {
		t.Fatal("expected concepts.slug column after reshape")
	}
}

// TestQuestionConceptsRoundTrip inserts provenance rows through SaveQuestion
// and reads them back at the column level (slug + weight snapshot).
func TestQuestionConceptsRoundTrip(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "go interfaces", Slug: "go_interfaces", Source: "code", Weight: 0.9},
		{Concept: "sql joins", Slug: "sql_joins", Source: "code", Weight: 0.8},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fed := []cache.ConceptRef{
		{Slug: "go_interfaces", Weight: 0.9},
		{Slug: "sql_joins", Weight: 0.8},
	}
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "which table does it join?",
		buildChoices([]string{"a", "b"}, 0), "", fed); err != nil {
		t.Fatalf("SaveQuestion: %v", err)
	}

	db := openRawDB(t)
	rows, err := db.QueryContext(ctx,
		`SELECT slug, weight FROM question_concepts ORDER BY slug ASC`)
	if err != nil {
		t.Fatalf("provenance query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []cache.ConceptRef
	for rows.Next() {
		var ref cache.ConceptRef
		if err := rows.Scan(&ref.Slug, &ref.Weight); err != nil {
			t.Fatalf("scan provenance row: %v", err)
		}
		got = append(got, ref)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("provenance rows err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 provenance rows, got %d", len(got))
	}
	// Ordered by slug: go_interfaces before sql_joins. Asserting on sorted
	// order rather than insertion order — the dedupe keeps values, not
	// positions, and map iteration is deliberately unordered.
	if got[0].Slug != "go_interfaces" || got[0].Weight != 0.9 {
		t.Fatalf("unexpected first provenance row: %+v", got[0])
	}
	if got[1].Slug != "sql_joins" || got[1].Weight != 0.8 {
		t.Fatalf("unexpected second provenance row: %+v", got[1])
	}
}

// TestSaveQuestionCollapsesDuplicateFedConcepts proves distinct-concept
// provenance: the sighting feed can present one concept several times, but
// the join persists exactly one row per (question, slug), keeping the MAXIMUM
// weight from the batch — order-independently.
func TestSaveQuestionCollapsesDuplicateFedConcepts(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "go interfaces", Slug: "go_interfaces", Source: "code", Weight: 0.9},
		{Concept: "sql joins", Slug: "sql_joins", Source: "code", Weight: 0.5},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// One slug 4× (weights scattered, two arriving at the 0.9 max) plus one
	// distinct slug — order scrambled on purpose (dedupe must not depend on
	// the batch being sorted).
	fed := []cache.ConceptRef{
		{Slug: "go_interfaces", Weight: 0.9},
		{Slug: "sql_joins", Weight: 0.5},
		{Slug: "go_interfaces", Weight: 0.4},
		{Slug: "go_interfaces", Weight: 0.2},
		{Slug: "go_interfaces", Weight: 0.9},
	}
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "collapsed provenance question",
		buildChoices([]string{"a", "b"}, 0), "", fed); err != nil {
		t.Fatalf("SaveQuestion: %v", err)
	}

	db := openRawDB(t)
	rows, err := db.QueryContext(ctx,
		`SELECT slug, weight FROM question_concepts ORDER BY slug ASC`)
	if err != nil {
		t.Fatalf("provenance query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []cache.ConceptRef
	for rows.Next() {
		var ref cache.ConceptRef
		if err := rows.Scan(&ref.Slug, &ref.Weight); err != nil {
			t.Fatalf("scan provenance row: %v", err)
		}
		got = append(got, ref)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("provenance rows err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows (one per DISTINCT fed concept), got %d: %+v", len(got), got)
	}
	// Ordered by slug: go_interfaces first.
	if got[0].Slug != "go_interfaces" || got[0].Weight != 0.9 {
		t.Fatalf("expected max-weight collapse for go_interfaces, got %+v", got[0])
	}
	if got[1].Slug != "sql_joins" || got[1].Weight != 0.5 {
		t.Fatalf("expected distinct concept stored row-per-slug, got %+v", got[1])
	}

	// The uniqueness invariant is load-bearing at the schema level too.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO question_concepts (question_id, slug, weight)
		 SELECT question_id, slug, weight FROM question_concepts LIMIT 1`); err == nil {
		t.Fatal("expected UNIQUE(question_id, slug) to reject a duplicate copy")
	}
}

// registryCount queries the concept_registry row count (and first_seen for
// a single slug) for identity-minting assertions.
func registryCount(t *testing.T, slug string) int {
	t.Helper()
	db := openRawDB(t)
	var n int
	var query string
	var args []any
	if slug == "" {
		query, args = `SELECT COUNT(*) FROM concept_registry`, nil
	} else {
		query, args = `SELECT COUNT(*) FROM concept_registry WHERE slug = ?`, []any{slug}
	}
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("registry count: %v", err)
	}
	return n
}

func firstSeenOf(t *testing.T, slug string) string {
	t.Helper()
	db := openRawDB(t)
	var ts string
	if err := db.QueryRowContext(context.Background(),
		`SELECT first_seen FROM concept_registry WHERE slug = ?`, slug).Scan(&ts); err != nil {
		t.Fatalf("reading first_seen for %q: %v", slug, err)
	}
	return ts
}

func registryProseOf(t *testing.T, slug string) string {
	t.Helper()
	db := openRawDB(t)
	var prose string
	if err := db.QueryRowContext(context.Background(),
		`SELECT prose FROM concept_registry WHERE slug = ?`, slug).Scan(&prose); err != nil {
		t.Fatalf("reading prose for %q: %v", slug, err)
	}
	return prose
}

// TestSaveIdentityResolution is the table-driven contract for identity
// validation at the write boundary: valid slugs mint on first sighting and
// reuse on repeat; malformed and missing slugs drop their element (with a
// logged warning) while the rest of the batch continues.
func TestSaveIdentityResolution(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// First sighting of two slugs mints exactly one registry row per slug.
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "go interfaces", Slug: "go_interfaces", Source: "code", Weight: 0.9},
		{Concept: "sql joins", Slug: "sql_joins", Source: "code", Weight: 0.8},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := registryCount(t, "go_interfaces"); got != 1 {
		t.Fatalf("expected registry to contain go_interfaces once, got %d rows", got)
	}
	if got := registryCount(t, "sql_joins"); got != 1 {
		t.Fatalf("expected registry to contain sql_joins once, got %d rows", got)
	}
	firstSeen := firstSeenOf(t, "go_interfaces")
	if prose := registryProseOf(t, "go_interfaces"); prose != "go interfaces" {
		t.Fatalf("expected registry prose to be the first-seen fingerprint, got %q", prose)
	}

	// Repeat sighting reuses the identity: registry row count unchanged, and
	// first_seen untouched.
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "go interfaces round 2", Slug: "go_interfaces", Source: "code", Weight: 0.7},
	}); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	if got := registryCount(t, "go_interfaces"); got != 1 {
		t.Fatalf("expected registry count 1 after repeat sighting, got %d", got)
	}
	if again := firstSeenOf(t, "go_interfaces"); again != firstSeen {
		t.Fatalf("first_seen changed on repeat sighting: %q -> %q", firstSeen, again)
	}
	// Repeat sighting prose does NOT overwrite the first-seen fingerprint.
	if prose := registryProseOf(t, "go_interfaces"); prose != "go interfaces" {
		t.Fatalf("expected first-seen prose untouched on repeat, got %q", prose)
	}

	// Malformed / missing slugs drop their element; the rest of the batch is
	// saved normally.
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "prose slug", Slug: "Handle HTTP Middleware Chaining", Source: "code", Weight: 0.9},
		{Concept: "no slug", Source: "code", Weight: 0.9},
		{Concept: "mixed case", Slug: "GoInterfaces", Source: "code", Weight: 0.9},
		{Concept: "sibling survives", Slug: "middleware_chain", Source: "code", Weight: 0.5},
	}); err != nil {
		t.Fatalf("Save with malformed slugs: %v", err)
	}
	if got := registryCount(t, ""); got != 3 { // go_interfaces, sql_joins, middleware_chain
		t.Fatalf("expected 3 registry rows after malformed batch, got %d", got)
	}
	recent, err := s.Recent(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	found := map[string]int{}
	for _, row := range recent {
		found[row.Slug]++
	}
	if n := found["middleware_chain"]; n != 1 {
		t.Fatalf("expected sibling element saved despite malformed neighbors, got %d", n)
	}
}

// TestSaveEmptyScopeNeverMintsIdentity proves scope-refusal ordering: an
// empty repo or branch performs NO registry insert — identity is never
// minted from scope-failed writes.
func TestSaveEmptyScopeNeverMintsIdentity(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	before := registryCount(t, "")
	if err := s.Save(ctx, "", "main", []cache.Fingerprint{
		{Concept: "leaked identity", Slug: "leaked_identity", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("Save with empty repo: %v", err)
	}
	if err := s.Save(ctx, "/r", "", []cache.Fingerprint{
		{Concept: "leaked identity", Slug: "leaked_identity", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("Save with empty branch: %v", err)
	}
	if after := registryCount(t, ""); after != before {
		t.Fatalf("expected registry unchanged after scope refusal (%d -> %d)", before, after)
	}
	if got := registryCount(t, "leaked_identity"); got != 0 {
		t.Fatalf("expected no leaked_identity registry row, got %d", got)
	}
}

// TestAliasResolutionOnSave covers the alias pressure valve on the Save
// path: sightings under an alias land on the target identity, and a
// mint-time slug colliding with an existing alias resolves to the alias
// target (open question #3's mandatory edge case).
func TestAliasResolutionOnSave(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// Mint the target identity first (register requires an existing target).
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "SQL joins", Slug: "sql_joins", Source: "code", Weight: 0.9},
	}); err != nil {
		t.Fatalf("Save target: %v", err)
	}
	if err := s.AddAlias(ctx, "joining_tables_in_sql", "sql_joins"); err != nil {
		t.Fatalf("AddAlias: %v", err)
	}

	// A sighting arriving under the alias lands on the target identity.
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "joining tables", Slug: "joining_tables_in_sql", Source: "code", Weight: 0.8},
	}); err != nil {
		t.Fatalf("Save under alias: %v", err)
	}
	if got := registryCount(t, "joining_tables_in_sql"); got != 0 {
		t.Fatalf("expected no registry row minted for the alias, got %d", got)
	}
	if got := registryCount(t, "sql_joins"); got != 1 {
		t.Fatalf("expected alias sighting to land on sql_joins, got %d rows", got)
	}

	// Mint-time collision: a NEW fingerprint whose slug IS an existing alias
	// resolves to the alias target too (no second registry row).
	if err := s.Save(ctx, "/repo/other", "feat", []cache.Fingerprint{
		{Concept: "join semantics from another repo", Slug: "joining_tables_in_sql", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("Save mint-time collision: %v", err)
	}
	if got := registryCount(t, "sql_joins"); got != 1 {
		t.Fatalf("expected still exactly one sql_joins registry row, got %d", got)
	}
	crossRepo, err := s.Recent(ctx, "/repo/other", "feat", 10)
	if err != nil {
		t.Fatalf("Recent cross-repo: %v", err)
	}
	if len(crossRepo) != 1 || crossRepo[0].Slug != "sql_joins" {
		t.Fatalf("expected cross-repo sighting resolved to sql_joins, got %+v", crossRepo)
	}
}

// TestAddAliasValidation covers the alias-write-time rejections: dangling
// targets and non-slug-shaped aliases are refused explicitly.
func TestAddAliasValidation(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// Dangling target rejected at alias-write time.
	err := s.AddAlias(ctx, "dangling_alias", "never_minted")
	if err == nil {
		t.Fatal("expected explicit error adding alias with nonexistent target")
	}
	if !strings.Contains(err.Error(), "no registry identity") {
		t.Fatalf("expected dangling-target error, got %v", err)
	}
	if got := registryCount(t, ""); got != 0 {
		t.Fatalf("expected dangling alias write to leave no registry rows, got %d", got)
	}

	// Non-slug-shaped alias rejected.
	if err := s.Save(ctx, "/r", "main", []cache.Fingerprint{
		{Concept: "real", Slug: "real_concept", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.AddAlias(ctx, "not a slug", "real_concept"); err == nil {
		t.Fatal("expected error for malformed alias shape")
	}
	if err := s.AddAlias(ctx, "real_concept_alias", "real_concept"); err != nil {
		t.Fatalf("valid alias add: %v", err)
	}
	list, err := s.Aliases(ctx)
	if err != nil {
		t.Fatalf("Aliases: %v", err)
	}
	if len(list) != 1 || list[0].Alias != "real_concept_alias" || list[0].Target != "real_concept" {
		t.Fatalf("unexpected alias listing: %+v", list)
	}
	removed, err := s.RemoveAlias(ctx, "real_concept_alias")
	if err != nil || !removed {
		t.Fatalf("RemoveAlias: removed=%v err=%v", removed, err)
	}
	if removedAgain, err := s.RemoveAlias(ctx, "real_concept_alias"); err != nil || removedAgain {
		t.Fatalf("expected second remove to report no-op, got removed=%v err=%v", removedAgain, err)
	}
}

// TestSaveQuestionProvenanceAtomicity proves the provenance insert rides the
// same transaction as the question: any rejected SaveQuestion leaves zero
// partial provenance rows.
func TestSaveQuestionProvenanceAtomicity(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	fed := []cache.ConceptRef{{Slug: "go_interfaces", Weight: 0.9}, {Slug: "sql_joins", Weight: 0.8}}

	// Rejected save (empty question text) → explicit error, nothing written.
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "", buildChoices([]string{"a", "b"}, 0), "", fed); err == nil {
		t.Fatal("expected error refusing empty question text")
	}
	// Rejected save (scope refusal) → nil error, nothing written.
	if err := s.SaveQuestion(ctx, "", "main", "q", buildChoices([]string{"a", "b"}, 0), "", fed); err != nil {
		t.Fatalf("SaveQuestion scope refusal: %v", err)
	}

	db := openRawDB(t)
	var questionRows, provenanceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM questions`).Scan(&questionRows); err != nil {
		t.Fatalf("count questions: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM question_concepts`).Scan(&provenanceRows); err != nil {
		t.Fatalf("count question_concepts: %v", err)
	}
	if questionRows != 0 || provenanceRows != 0 {
		t.Fatalf("expected zero partial rows after rejected saves (questions=%d, provenance=%d)",
			questionRows, provenanceRows)
	}

	// Empty concept batch persists the question with zero provenance rows
	// rather than erroring (spec: bypass paths, e.g., no feed context).
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "no context question",
		buildChoices([]string{"a", "b"}, 0), "", nil); err != nil {
		t.Fatalf("SaveQuestion with empty batch: %v", err)
	}
	var provenance int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM question_concepts`).Scan(&provenance); err != nil {
		t.Fatalf("count provenance: %v", err)
	}
	if provenance != 0 {
		t.Fatalf("expected zero provenance rows for empty fed batch, got %d", provenance)
	}
}

// TestRecentCarriesResolvedSlug proves Recent reads surface the resolved
// registry slug next to the sighting prose — including alias-resolved
// sightings (the read always carries the target).
func TestRecentCarriesResolvedSlug(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "go interfaces", Slug: "go_interfaces", Source: "code", Weight: 0.9},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	recent, err := s.Recent(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recent) != 1 || recent[0].Slug != "go_interfaces" || recent[0].Concept != "go interfaces" {
		t.Fatalf("expected slug round-trip, got %+v", recent)
	}

	if err := s.AddAlias(ctx, "golang_interfaces", "go_interfaces"); err != nil {
		t.Fatalf("AddAlias: %v", err)
	}
	if err := s.Save(ctx, "/repo/test", "main", []cache.Fingerprint{
		{Concept: "alias sighting", Slug: "golang_interfaces", Source: "code", Weight: 0.5},
	}); err != nil {
		t.Fatalf("Save under alias: %v", err)
	}
	recent, err = s.Recent(ctx, "/repo/test", "main", 10)
	if err != nil {
		t.Fatalf("Recent after alias: %v", err)
	}
	var sawAliasTarget bool
	for _, row := range recent {
		if row.Concept == "alias sighting" && row.Slug == "go_interfaces" {
			sawAliasTarget = true
		}
	}
	if !sawAliasTarget {
		t.Fatalf("expected alias sighting to read back with target slug, got %+v", recent)
	}
}

// TestRecentFeedExcludesOutOfSurfaceConcepts is the working-surface-first
// regression: even when the user-scoped registry links sightings across
// repos, Recent for a (repo, branch) surface never widens to include them.
func TestRecentFeedExcludesOutOfSurfaceConcepts(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// Two surfaces mint the SAME identity — the registry row is shared.
	if err := s.Save(ctx, "/repo/a", "main", []cache.Fingerprint{
		{Concept: "go interfaces", Slug: "go_interfaces", Source: "code", Weight: 0.9},
	}); err != nil {
		t.Fatalf("Save repo A: %v", err)
	}
	if err := s.Save(ctx, "/repo/b", "feat-db", []cache.Fingerprint{
		{Concept: "go interfaces elsewhere", Slug: "go_interfaces", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("Save repo B: %v", err)
	}
	if got := registryCount(t, "go_interfaces"); got != 1 {
		t.Fatalf("expected one shared registry identity across repos, got %d", got)
	}

	feedA, err := s.Recent(ctx, "/repo/a", "main", 20)
	if err != nil {
		t.Fatalf("Recent repo A: %v", err)
	}
	if len(feedA) != 1 || feedA[0].Concept != "go interfaces" {
		t.Fatalf("repo A feed must contain only its own sighting, got %+v", feedA)
	}
	feedB, err := s.Recent(ctx, "/repo/b", "feat-db", 20)
	if err != nil {
		t.Fatalf("Recent repo B: %v", err)
	}
	if len(feedB) != 1 || feedB[0].Concept != "go interfaces elsewhere" {
		t.Fatalf("repo B feed must contain only its own sighting, got %+v", feedB)
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
