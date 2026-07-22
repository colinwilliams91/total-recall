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

// TestRecentConceptsScopedToBranch verifies the branch-isolation invariant
// (analogous to TestRecentConceptsScopedToRepo). Saving under (repo, "feature-X")
// and querying (repo, "main") must return zero rows.
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

// TestSaveRefusesEmptyRepoOrBranch exercises the Decision 3 store guard:
// saving concepts with an empty repo or branch is a no-op (returns nil,
// inserts zero rows).
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

// TestRecentRefusesEmptyRepoOrBranch exercises the Decision 3 store guard
// for reads: querying with an empty repo or branch returns (nil, nil).
func TestRecentRefusesEmptyRepoOrBranch(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// Seed a real row first so we can assert the empty-arg call doesn't see it.
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

// TestSaveQuestionRefusesEmptyRepoOrBranch exercises the Decision 3 store
// guard for SaveQuestion.
func TestSaveQuestionRefusesEmptyRepoOrBranch(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "", "main", "q", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion with empty repo: expected nil error, got %v", err)
	}
	if err := s.SaveQuestion(ctx, "/r", "", "q", []string{"a", "b"}, 0); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "What is a goroutine?", []string{"a", "b", "c"}, 0); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "single question", []string{"x", "y"}, 0); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/x", "main", "X's question", []string{"a", "b"}, 0); err != nil {
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

func TestAnswerQuestion(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "test question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "q1", []string{"a", "b"}, 0); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/x", "main", "q1", []string{"a"}, 0); err != nil {
		t.Fatalf("SaveQuestion for repo X failed: %v", err)
	}
	if err := s.SaveQuestion(ctx, "/repo/y", "main", "q2", []string{"a"}, 0); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "correct-index question", []string{"a", "b", "c"}, 2); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "full row question", []string{"x", "y", "z"}, 1); err != nil {
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "skip me", []string{"a", "b"}, 0); err != nil {
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
	recent, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "feedback question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.AnswerQuestion(ctx, claimed.ID, 1, "b", false, "A is correct because..."); err != nil {
		t.Fatalf("AnswerQuestion failed: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
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

	if err := s.SaveQuestion(ctx, "/repo/test", "main", "empty feedback question", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}
	claimed, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.AnswerQuestion(ctx, claimed.ID, 0, "a", true, ""); err != nil {
		t.Fatalf("AnswerQuestion failed: %v", err)
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

func TestRecentAnsweredEnrichedFields(t *testing.T) {
	s := setupCache(t)
	ctx := context.Background()

	// q1: correct with feedback (terminal-style)
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "terminal-style", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion q1: %v", err)
	}
	q1, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion q1: %v", err)
	}
	if err := s.AnswerQuestion(ctx, q1.ID, 0, "a", true, "Good job!"); err != nil {
		t.Fatalf("AnswerQuestion q1: %v", err)
	}

	// q2: incorrect without feedback (MCP-style)
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "mcp-style", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion q2: %v", err)
	}
	q2, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion q2: %v", err)
	}
	if err := s.AnswerQuestion(ctx, q2.ID, 1, "b", false, ""); err != nil {
		t.Fatalf("AnswerQuestion q2: %v", err)
	}

	// q3: skipped
	if err := s.SaveQuestion(ctx, "/repo/test", "main", "skipped-one", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("SaveQuestion q3: %v", err)
	}
	q3, err := s.NextQuestion(ctx, "/repo/test", "main", "test")
	if err != nil {
		t.Fatalf("NextQuestion q3: %v", err)
	}
	if err := s.SkipQuestion(ctx, q3.ID); err != nil {
		t.Fatalf("SkipQuestion q3: %v", err)
	}

	recent, err := s.RecentAnswered(ctx, "/repo/test", "main", 10)
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

// TestAddColumnIfMissingMigration is removed in Y1 (cache-tenant-isolation).
// The in-code migration path it tested is gone — fresh schemas are created
// with the full final layout, and existing databases must be wiped manually
// (see task 10.5 in the change). The test's "purge un-tagged legacy row"
// behavior is no longer a thing.

// Task 10.18: Cover the covering indexes idx_concepts_repo_branch_seen and
// idx_questions_repo_branch_q exist after cache.Open(), defending against a
// regression that drops the CREATE INDEX calls.
func TestRepoIndexesExist(t *testing.T) {
	s := setupCache(t)
	_ = s // store is kept open; SQLite allows concurrent connections

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
	for _, idx := range []string{"idx_concepts_repo_branch_seen", "idx_questions_repo_branch_q"} {
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
