package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDBFilename = "memory.db"
	legacyDBFilename  = "concepts.db"
	recentLimit       = 50

	createConceptsTableSQL = `
CREATE TABLE IF NOT EXISTS concepts (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  concept   TEXT    NOT NULL,
  source    TEXT    NOT NULL DEFAULT 'code',
  weight    REAL    NOT NULL DEFAULT 1.0,
  seen_at   DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_concepts_seen_at ON concepts(seen_at DESC);
`

	createQuestionsTableSQL = `
CREATE TABLE IF NOT EXISTS questions (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    question      TEXT     NOT NULL,
    choices       TEXT     NOT NULL,
    correct_index INTEGER  NOT NULL DEFAULT 0,
    queued_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    delivered_at  DATETIME,
    claimed_by    TEXT,
    answer        TEXT,
    answer_index  INTEGER,
    correct       INTEGER,
    feedback      TEXT,
    answered_at   DATETIME
);
`
)

// ConceptRow is a persisted concept fingerprint from the cache.
type ConceptRow struct {
	ID      int64
	Concept string
	Source  string
	Weight  float64
	SeenAt  time.Time
}

// StoredQuestion is a recall question retrieved from the questions table.
type StoredQuestion struct {
	ID           int64
	Question     string
	Choices      []string
	CorrectIndex int
	QueuedAt     time.Time
	AnswerIndex  *int
	Correct      *bool
	Feedback     *string
}

// Store wraps the SQLite database used as the memory store.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the memory store at ~/.tr/memory.db.
// If memory.db does not exist but the legacy concepts.db does, it is copied
// to memory.db before opening (one-time migration).
// Returns a non-nil *Store on success.
func Open() (*Store, error) {
	dir, err := trDir()
	if err != nil {
		return nil, err
	}

	memoryPath := filepath.Join(dir, defaultDBFilename)
	legacyPath := filepath.Join(dir, legacyDBFilename)

	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {
			if copyErr := copyFile(legacyPath, memoryPath); copyErr == nil {
				log.Printf("[store] migrated %s → %s", legacyPath, memoryPath)
			}
		}
	}

	db, err := sql.Open("sqlite", memoryPath)
	if err != nil {
		return nil, fmt.Errorf("opening memory store at %s: %w", memoryPath, err)
	}

	bg := context.Background()
	if _, err := db.ExecContext(bg, createConceptsTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing concepts schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createQuestionsTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing questions schema: %w", err)
	}

	migrations := []struct{ table, column, def string }{
		{"questions", "correct_index", "INTEGER NOT NULL DEFAULT 0"},
		{"questions", "answer_index", "INTEGER"},
		{"questions", "correct", "INTEGER"},
		{"questions", "feedback", "TEXT"},
	}
	for _, m := range migrations {
		if err := addColumnIfMissing(bg, db, m.table, m.column, m.def); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating %s.%s: %w", m.table, m.column, err)
		}
	}

	return &Store{db: db}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// addColumnIfMissing executes `ALTER TABLE <table> ADD COLUMN <column> <definition>`.
// If the column already exists (SQLite returns a "duplicate column name" error),
// it returns nil so the call is idempotent. All other errors propagate.
func addColumnIfMissing(ctx context.Context, db *sql.DB, table, column, definition string) error {
	_, err := db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}
		return err
	}
	return nil
}

// Save persists a batch of concept fingerprints to the cache.
// Fingerprints are lightweight metadata only — no raw diff text.
type Fingerprint struct {
	Concept string
	Source  string
	Weight  float64
}

func (s *Store) Save(ctx context.Context, concepts []Fingerprint) error {
	if len(concepts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO concepts (concept, source, weight, seen_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, c := range concepts {
		if _, err := stmt.ExecContext(ctx, c.Concept, c.Source, c.Weight, now); err != nil {
			return fmt.Errorf("inserting concept %q: %w", c.Concept, err)
		}
	}
	return tx.Commit()
}

// Recent returns up to n concept rows ordered by most recently seen.
func (s *Store) Recent(ctx context.Context, n int) ([]ConceptRow, error) {
	if n <= 0 {
		n = recentLimit
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, concept, source, weight, seen_at FROM concepts ORDER BY seen_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, fmt.Errorf("querying recent concepts: %w", err)
	}
	defer rows.Close()

	var result []ConceptRow
	for rows.Next() {
		var r ConceptRow
		if err := rows.Scan(&r.ID, &r.Concept, &r.Source, &r.Weight, &r.SeenAt); err != nil {
			return nil, fmt.Errorf("scanning concept row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// SaveQuestion inserts a synthesized question into the questions table.
// Choices are JSON-marshalled into a TEXT column.
func (s *Store) SaveQuestion(ctx context.Context, question string, choices []string, correctIndex int) error {
	choicesJSON, err := json.Marshal(choices)
	if err != nil {
		return fmt.Errorf("marshalling choices: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO questions (question, choices, correct_index) VALUES (?, ?, ?)`,
		question, string(choicesJSON), correctIndex,
	)
	return err
}

// NextQuestion atomically claims and returns the oldest unclaimed question.
// It sets delivered_at and claimed_by in a single UPDATE ... RETURNING statement.
// Returns nil, nil when no unclaimed question exists.
func (s *Store) NextQuestion(ctx context.Context, claimedBy string) (*StoredQuestion, error) {
	row := s.db.QueryRowContext(ctx, `
UPDATE questions
SET delivered_at = datetime('now'), claimed_by = ?
WHERE id = (
  SELECT id FROM questions
  WHERE delivered_at IS NULL
  ORDER BY queued_at ASC
  LIMIT 1
)
RETURNING id, question, choices, correct_index, queued_at`, claimedBy)

	var sq StoredQuestion
	var choicesJSON string
	var queuedAt string
	if err := row.Scan(&sq.ID, &sq.Question, &choicesJSON, &sq.CorrectIndex, &queuedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("claiming next question: %w", err)
	}
	if err := json.Unmarshal([]byte(choicesJSON), &sq.Choices); err != nil {
		return nil, fmt.Errorf("unmarshalling choices: %w", err)
	}
	sq.QueuedAt = parseSQLiteTime(queuedAt)
	return &sq, nil
}

// AnswerQuestion records the user's answer for the question with the given ID.
func (s *Store) AnswerQuestion(ctx context.Context, id int64, answerIndex int, answerText string, correct bool, feedback string) error {
	var feedbackArg any
	if feedback == "" {
		feedbackArg = nil
	} else {
		feedbackArg = feedback
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE questions SET answer = ?, answer_index = ?, correct = ?, feedback = ?, answered_at = datetime('now') WHERE id = ?`,
		answerText, answerIndex, boolToInt(correct), feedbackArg, id,
	)
	return err
}

// GetQuestion fetches a single question by ID for answer evaluation.
// Returns (nil, nil) when the row does not exist.
func (s *Store) GetQuestion(ctx context.Context, id int64) (*StoredQuestion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, question, choices, correct_index, queued_at FROM questions WHERE id = ?`, id)
	var sq StoredQuestion
	var choicesJSON, queuedAt string
	if err := row.Scan(&sq.ID, &sq.Question, &choicesJSON, &sq.CorrectIndex, &queuedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("fetching question %d: %w", id, err)
	}
	if err := json.Unmarshal([]byte(choicesJSON), &sq.Choices); err != nil {
		return nil, fmt.Errorf("unmarshalling choices: %w", err)
	}
	sq.QueuedAt = parseSQLiteTime(queuedAt)
	return &sq, nil
}

// SkipQuestion records that the user skipped the question. It sets answer="skip"
// and answered_at but leaves answer_index, correct, and feedback NULL.
func (s *Store) SkipQuestion(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE questions SET answer = 'skip', answered_at = datetime('now') WHERE id = ?`,
		id,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseSQLiteTime parses a datetime string returned by the SQLite driver.
// modernc.org/sqlite returns timestamps in RFC 3339 form (e.g.
// "2026-06-24T06:51:03Z"); older space-separated "2006-01-02 15:04:05" values
// are also accepted for backward compatibility. A zero time is returned for
// unparseable input.
func parseSQLiteTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return t
}

// QueueDepth returns the number of unclaimed (undelivered) questions.
func (s *Store) QueueDepth(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM questions WHERE delivered_at IS NULL`).Scan(&n)
	return n, err
}

// RecentAnswered returns up to limit answered questions ordered by most recently answered.
func (s *Store) RecentAnswered(ctx context.Context, limit int) ([]StoredQuestion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, question, choices, correct_index, queued_at, answer_index, correct, feedback
		   FROM questions
		  WHERE answered_at IS NOT NULL
		  ORDER BY answered_at DESC
		  LIMIT ?`,
		limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent answered: %w", err)
	}
	defer rows.Close()

	var result []StoredQuestion
	for rows.Next() {
		var sq StoredQuestion
		var choicesJSON, queuedAt string
		var answerIndex sql.NullInt64
		var correct sql.NullInt64
		var feedback sql.NullString
		if err := rows.Scan(&sq.ID, &sq.Question, &choicesJSON, &sq.CorrectIndex, &queuedAt, &answerIndex, &correct, &feedback); err != nil {
			return nil, fmt.Errorf("scanning question row: %w", err)
		}
		if err := json.Unmarshal([]byte(choicesJSON), &sq.Choices); err != nil {
			return nil, fmt.Errorf("unmarshalling choices: %w", err)
		}
		sq.QueuedAt = parseSQLiteTime(queuedAt)
		if answerIndex.Valid {
			v := int(answerIndex.Int64)
			sq.AnswerIndex = &v
		}
		if correct.Valid {
			v := correct.Int64 != 0
			sq.Correct = &v
		}
		if feedback.Valid {
			v := feedback.String
			sq.Feedback = &v
		}
		result = append(result, sq)
	}
	return result, rows.Err()
}

// PeekNextQuestion returns the next unclaimed question without claiming it.
// Used by the recall://queue resource handler.
func (s *Store) PeekNextQuestion(ctx context.Context) (*StoredQuestion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, question, choices, correct_index, queued_at FROM questions WHERE delivered_at IS NULL ORDER BY queued_at ASC LIMIT 1`)

	var sq StoredQuestion
	var choicesJSON, queuedAt string
	if err := row.Scan(&sq.ID, &sq.Question, &choicesJSON, &sq.CorrectIndex, &queuedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("peeking next question: %w", err)
	}
	if err := json.Unmarshal([]byte(choicesJSON), &sq.Choices); err != nil {
		return nil, fmt.Errorf("unmarshalling choices: %w", err)
	}
	sq.QueuedAt = parseSQLiteTime(queuedAt)
	return &sq, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func trDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".tr")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("creating ~/.tr directory: %w", err)
	}
	return dir, nil
}
