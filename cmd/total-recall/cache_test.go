package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/cache"
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

	if err := s.SaveQuestion(ctx, "What is a goroutine?", []string{"a", "b", "c"}); err != nil {
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

	if err := s.SaveQuestion(ctx, "single question", []string{"x", "y"}); err != nil {
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

	if err := s.SaveQuestion(ctx, "test question", []string{"a", "b"}); err != nil {
		t.Fatalf("SaveQuestion failed: %v", err)
	}

	q, err := s.NextQuestion(ctx, "test")
	if err != nil {
		t.Fatalf("NextQuestion failed: %v", err)
	}
	if q == nil {
		t.Fatal("expected non-nil question")
	}

	if err := s.AnswerQuestion(ctx, q.ID, "a"); err != nil {
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

	if err := s.SaveQuestion(ctx, "q1", []string{"a", "b"}); err != nil {
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
