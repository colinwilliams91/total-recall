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
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	s, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenCreatesDatabase(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

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

func TestSaveAndRetrieveConcepts(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	concepts := []cache.Fingerprint{
		{Concept: "exponential-backoff", Source: "code", Weight: 1.0},
		{Concept: "circuit-breaker", Source: "code", Weight: 0.8},
	}
	if err := s.Save(ctx, concepts); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	recent, err := s.Recent(ctx, 10)
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

	if err := s.Save(ctx, []cache.Fingerprint{
		{Concept: "first", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	if err := s.Save(ctx, []cache.Fingerprint{
		{Concept: "second", Source: "code", Weight: 1.0},
	}); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	recent, err := s.Recent(ctx, 10)
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

func TestSaveQuestionAndClaim(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "What is a goroutine?", []string{"a", "b", "c"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "test")
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
}

func TestNextQuestionReturnsNilWhenEmpty(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	q, err := s.NextQuestion(ctx, "test")
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

	if err := s.SaveQuestion(ctx, "single question", []string{"x", "y"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q1, err := s.NextQuestion(ctx, "claimer-1")
	if err != nil {
		t.Fatalf("first NextQuestion failed: %v", err)
	}
	if q1 == nil {
		t.Fatal("expected first question")
	}

	q2, err := s.NextQuestion(ctx, "claimer-2")
	if err != nil {
		t.Fatalf("second NextQuestion failed: %v", err)
	}
	if q2 != nil {
		t.Fatal("expected nil on second claim (already claimed)")
	}
}

func TestAnswerQuestion(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "test question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.AnswerQuestion(ctx, q.ID, 0, "a", true, ""); err != nil {
		t.Fatalf("AnswerQuestion failed: %v", err)
	}
}

func TestQueueDepthEmpty(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	n, err := s.QueueDepth(ctx)
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

	n, err := s.QueueDepth(ctx)
	if err != nil {
		t.Fatalf("initial QueueDepth failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected initial depth 0, got %d", n)
	}

	if err := s.SaveQuestion(ctx, "q1", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	n, err = s.QueueDepth(ctx)
	if err != nil {
		t.Fatalf("QueueDepth after save failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected depth 1 after save, got %d", n)
	}

	if _, err := s.NextQuestion(ctx, "test"); err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	n, err = s.QueueDepth(ctx)
	if err != nil {
		t.Fatalf("QueueDepth after claim failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected depth 0 after claim, got %d", n)
	}
}

// rawDBPath resolves the memory.db path under the current test HOME.
func rawDBPath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	return filepath.Join(home, ".tr", "memory.db")
}

// openRawDB opens a direct database/sql handle to the test memory.db for
// column-level assertions not exposed by the cache.Store API (e.g. the raw
// `answer` column). The handle is closed automatically via t.Cleanup.
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", rawDBPath(t))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveQuestionPersistsCorrectIndex(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "correct-index question", []string{"a", "b", "c"}, 2); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	claimed, err := s.NextQuestion(ctx, "test")
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
	if q.CorrectIndex != 2 {
		t.Fatalf("expected CorrectIndex 2, got %d", q.CorrectIndex)
	}
	if len(q.Choices) != 3 || q.Choices[0] != "a" || q.Choices[1] != "b" || q.Choices[2] != "c" {
		t.Fatalf("expected choices [a b c], got %v", q.Choices)
	}
}

func TestGetQuestionReturnsFullRow(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "full row question", []string{"x", "y", "z"}, 1); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "test")
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
	if len(q.Choices) != 3 || q.Choices[0] != "x" || q.Choices[1] != "y" || q.Choices[2] != "z" {
		t.Fatalf("expected choices [x y z], got %v", q.Choices)
	}
	if q.CorrectIndex != 1 {
		t.Fatalf("expected CorrectIndex 1, got %d", q.CorrectIndex)
	}
	if q.QueuedAt.IsZero() {
		t.Fatal("expected non-zero QueuedAt")
	}

	missing, err := s.GetQuestion(ctx, 99999)
	if err != nil {
		t.Fatalf("GetQuestion missing ID: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for missing ID, got %+v", missing)
	}
}

func TestSkipQuestionLeavesNulls(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "skip me", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.SkipQuestion(ctx, claimed.ID); err != nil {
		t.Fatalf("SkipQuestion failed: %v", err)
	}

	// Verify answer == "skip" via a direct query (RecentAnswered doesn't expose the answer column).
	db := openRawDB(t)
	var answer sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT answer FROM questions WHERE id = ?`, claimed.ID).Scan(&answer); err != nil {
		t.Fatalf("raw query answer: %v", err)
	}
	if !answer.Valid || answer.String != "skip" {
		t.Fatalf("expected answer %q, got %v", "skip", answer)
	}

	// Verify nullable enriched fields via RecentAnswered.
	recent, err := s.RecentAnswered(ctx, 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 answered row, got %d", len(recent))
	}
	row := recent[0]
	if row.AnswerIndex != nil {
		t.Fatalf("expected AnswerIndex nil for skip, got %v", row.AnswerIndex)
	}
	if row.Correct != nil {
		t.Fatalf("expected Correct nil for skip, got %v", row.Correct)
	}
	if row.Feedback != nil {
		t.Fatalf("expected Feedback nil for skip, got %v", row.Feedback)
	}
}

func TestAnswerQuestionWithFeedback(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "feedback question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.AnswerQuestion(ctx, claimed.ID, 1, "b", false, "A is correct because..."); err != nil {
		t.Fatalf("AnswerQuestion failed: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 answered row, got %d", len(recent))
	}
	row := recent[0]
	if row.Correct == nil {
		t.Fatal("expected Correct non-nil")
	}
	if *row.Correct != false {
		t.Fatalf("expected Correct *false, got %v", *row.Correct)
	}
	if row.Feedback == nil {
		t.Fatal("expected Feedback non-nil")
	}
	if *row.Feedback != "A is correct because..." {
		t.Fatalf("expected feedback %q, got %q", "A is correct because...", *row.Feedback)
	}
}

func TestAnswerQuestionEmptyFeedbackStoresNull(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "empty feedback question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.AnswerQuestion(ctx, claimed.ID, 0, "a", true, ""); err != nil {
		t.Fatalf("AnswerQuestion failed: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, 10)
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

func TestRecentAnsweredEnrichedFields(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// q1: correct with feedback (terminal-style)
	if err := s.SaveQuestion(ctx, "terminal-style", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion q1: %v", err)
	}
	q1, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion q1: %v", err)
	}
	if err := s.AnswerQuestion(ctx, q1.ID, 0, "a", true, "Good job!"); err != nil {
		t.Fatalf("AnswerQuestion q1: %v", err)
	}

	// q2: incorrect without feedback (MCP-style)
	if err := s.SaveQuestion(ctx, "mcp-style", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion q2: %v", err)
	}
	q2, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion q2: %v", err)
	}
	if err := s.AnswerQuestion(ctx, q2.ID, 1, "b", false, ""); err != nil {
		t.Fatalf("AnswerQuestion q2: %v", err)
	}

	// q3: skipped
	if err := s.SaveQuestion(ctx, "skipped-one", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion q3: %v", err)
	}
	q3, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion q3: %v", err)
	}
	if err := s.SkipQuestion(ctx, q3.ID); err != nil {
		t.Fatalf("SkipQuestion q3: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, 10)
	if err != nil {
		t.Fatalf("RecentAnswered failed: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 answered rows, got %d", len(recent))
	}

	byQuestion := make(map[string]cache.StoredQuestion, len(recent))
	for _, r := range recent {
		byQuestion[r.Question] = r
	}

	for _, r := range recent {
		if r.CorrectIndex != 0 {
			t.Fatalf("expected CorrectIndex 0 for %q, got %d", r.Question, r.CorrectIndex)
		}
	}

	skip := byQuestion["skipped-one"]
	if skip.AnswerIndex != nil {
		t.Fatalf("expected AnswerIndex nil for skip, got %v", skip.AnswerIndex)
	}
	if skip.Correct != nil {
		t.Fatalf("expected Correct nil for skip, got %v", skip.Correct)
	}
	if skip.Feedback != nil {
		t.Fatalf("expected Feedback nil for skip, got %v", skip.Feedback)
	}

	mcp := byQuestion["mcp-style"]
	if mcp.Feedback != nil {
		t.Fatalf("expected Feedback nil for MCP-style, got %v", *mcp.Feedback)
	}
	if mcp.Correct == nil || *mcp.Correct != false {
		t.Fatalf("expected Correct *false for MCP-style, got %v", mcp.Correct)
	}

	term := byQuestion["terminal-style"]
	if term.Feedback == nil {
		t.Fatal("expected Feedback non-nil for terminal-style")
	}
	if *term.Feedback != "Good job!" {
		t.Fatalf("expected feedback %q, got %q", "Good job!", *term.Feedback)
	}
	if term.Correct == nil || !*term.Correct {
		t.Fatalf("expected Correct *true for terminal-style, got %v", term.Correct)
	}
}

func TestAddColumnIfMissingMigration(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	// Pre-create the memory.db with the old Phase 4A schema (no correct_index,
	// answer_index, correct, or feedback columns) and a surviving legacy row.
	trDir := filepath.Join(tempDir, ".tr")
	if err := os.MkdirAll(trDir, 0o700); err != nil {
		t.Fatalf("mkdir .tr: %v", err)
	}
	dbPath := filepath.Join(trDir, "memory.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open old db: %v", err)
	}
	oldSchema := `CREATE TABLE questions (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    question      TEXT     NOT NULL,
    choices       TEXT     NOT NULL,
    queued_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    delivered_at  DATETIME,
    claimed_by    TEXT,
    answer        TEXT,
    answered_at   DATETIME
);
CREATE TABLE IF NOT EXISTS concepts (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  concept   TEXT    NOT NULL,
  source    TEXT    NOT NULL DEFAULT 'code',
  weight    REAL    NOT NULL DEFAULT 1.0,
  seen_at   DATETIME NOT NULL
);`
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatalf("create old schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO questions (question, choices) VALUES (?, ?)`, "legacy q", `["a","b"]`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close old db: %v", err)
	}

	// cache.Open() must add all 4 new columns via addColumnIfMissing.
	s, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open migration failed: %v", err)
	}

	ctx := context.Background()
	q, err := s.GetQuestion(ctx, 1)
	if err != nil {
		t.Fatalf("GetQuestion after migration: %v", err)
	}
	if q == nil {
		t.Fatal("expected legacy row to survive migration, got nil")
	}
	if q.Question != "legacy q" {
		t.Fatalf("expected legacy question text, got %q", q.Question)
	}
	if q.CorrectIndex != 0 {
		t.Fatalf("expected default CorrectIndex 0, got %d", q.CorrectIndex)
	}

	// Exercise all 4 new columns via RecentAnswered (which selects them) by
	// answering the legacy row.
	if err := s.AnswerQuestion(ctx, 1, 0, "a", true, ""); err != nil {
		t.Fatalf("AnswerQuestion after migration: %v", err)
	}
	recent, err := s.RecentAnswered(ctx, 10)
	if err != nil {
		t.Fatalf("RecentAnswered after migration: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 answered row, got %d", len(recent))
	}

	// Idempotent: re-opening must not error (addColumnIfMissing no-ops on existing columns).
	if err := s.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}
	s2, err := cache.Open()
	if err != nil {
		t.Fatalf("cache.Open idempotent check failed: %v", err)
	}
	defer s2.Close()
}
