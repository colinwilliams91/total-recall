package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDBFilename = "memory.db"
	recentLimit       = 50

	createConceptsTableSQL = `
CREATE TABLE IF NOT EXISTS concepts (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  concept   TEXT    NOT NULL,
  source    TEXT    NOT NULL DEFAULT 'code',
  weight    REAL    NOT NULL DEFAULT 1.0,
  repo      TEXT    NOT NULL,
  branch    TEXT    NOT NULL,
  seen_at   DATETIME NOT NULL
);
`

	createQuestionsTableSQL = `
CREATE TABLE IF NOT EXISTS questions (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    question      TEXT     NOT NULL,
    choices       TEXT     NOT NULL,
    correct_index INTEGER  NOT NULL DEFAULT 0,
    repo          TEXT     NOT NULL,
    branch        TEXT     NOT NULL,
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

// Open opens (or creates) the memory store at $TR_HOME/memory.db, or
// ~/.tr/memory.db when TR_HOME is unset. Only memory.db is supported — the
// legacy concepts.db migration is removed. Returns a non-nil *Store on success.
func Open() (*Store, error) {
	dir, err := trDir()
	if err != nil {
		return nil, err
	}

	memoryPath := filepath.Join(dir, defaultDBFilename)

	db, err := sql.Open("sqlite", memoryPath)
	if err != nil {
		return nil, fmt.Errorf("opening memory store at %s: %w", memoryPath, err)
	}

	// SQLite serializes writes internally, but the Go database/sql pool can open
	// multiple connections. When the async pipeline goroutine writes concurrently
	// with an HTTP handler read, the second connection gets SQLITE_BUSY. Limiting
	// to a single connection serializes all access through one handle, which is
	// the recommended pattern for SQLite in Go.
	db.SetMaxOpenConns(1)

	bg := context.Background()
	if _, err := db.ExecContext(bg, createConceptsTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing concepts schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createQuestionsTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing questions schema: %w", err)
	}

	if _, err := db.ExecContext(bg, `CREATE INDEX IF NOT EXISTS idx_concepts_repo_branch_seen ON concepts(repo, branch, seen_at DESC)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating idx_concepts_repo_branch_seen: %w", err)
	}
	if _, err := db.ExecContext(bg, `CREATE INDEX IF NOT EXISTS idx_questions_repo_branch_q ON questions(repo, branch, queued_at ASC) WHERE delivered_at IS NULL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating idx_questions_repo_branch_q: %w", err)
	}

	return &Store{db: db}, nil
}

// Save persists a batch of concept fingerprints to the cache, tagged with repo.
// Fingerprints are lightweight metadata only — no raw diff text.
type Fingerprint struct {
	Concept string
	Source  string
	Weight  float64
}

func (s *Store) Save(ctx context.Context, repo, branch string, concepts []Fingerprint) error {
	if repo == "" || branch == "" {
		log.Printf("[store] skipping insert: empty repo or branch")
		return nil
	}
	if len(concepts) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO concepts (concept, source, weight, repo, branch, seen_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, c := range concepts {
		if _, err := stmt.ExecContext(ctx, c.Concept, c.Source, c.Weight, repo, branch, now); err != nil {
			return fmt.Errorf("inserting concept %q: %w", c.Concept, err)
		}
	}
	return tx.Commit()
}

// Recent returns up to n concept rows for repo+branch, ordered by most recently
// seen. Both repo and branch are required; empty values return (nil, nil)
// without touching the store.
func (s *Store) Recent(ctx context.Context, repo, branch string, n int) ([]ConceptRow, error) {
	if repo == "" || branch == "" {
		log.Printf("[store] skipping recent: empty repo or branch")
		return nil, nil
	}
	if n <= 0 {
		n = recentLimit
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, concept, source, weight, seen_at FROM concepts WHERE repo = ? AND branch = ? ORDER BY seen_at DESC LIMIT ?`, repo, branch, n)
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

// SaveQuestion inserts a synthesized question into the questions table, tagged
// with repo and branch. Choices are JSON-marshalled into a TEXT column. Both
// repo and branch are required; empty values are a no-op.
func (s *Store) SaveQuestion(ctx context.Context, repo, branch, question string, choices []string, correctIndex int) error {
	if repo == "" || branch == "" {
		log.Printf("[store] skipping savequestion: empty repo or branch")
		return nil
	}
	choicesJSON, err := json.Marshal(choices)
	if err != nil {
		return fmt.Errorf("marshalling choices: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO questions (question, choices, correct_index, repo, branch) VALUES (?, ?, ?, ?, ?)`,
		question, string(choicesJSON), correctIndex, repo, branch,
	)
	return err
}

// NextQuestion atomically claims and returns the oldest unclaimed question for
// the given repo and branch. It sets delivered_at and claimed_by in a single
// UPDATE ... RETURNING statement. Both repo and branch are required; empty
// values return (nil, nil) without touching the store. Returns nil, nil when
// no unclaimed question exists for the (repo, branch) pair.
func (s *Store) NextQuestion(ctx context.Context, repo, branch, claimedBy string) (*StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
UPDATE questions
SET delivered_at = datetime('now'), claimed_by = ?
WHERE id = (
  SELECT id FROM questions
  WHERE delivered_at IS NULL AND repo = ? AND branch = ?
  ORDER BY queued_at ASC
  LIMIT 1
)
RETURNING id, question, choices, correct_index, queued_at`, claimedBy, repo, branch)

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
// ID-keyed (globally unique) — no repo parameter needed.
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
// ID-keyed (globally unique) — no repo parameter needed.
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
// ID-keyed (globally unique) — no repo parameter needed.
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

// QueueDepth returns the number of unclaimed (undelivered) questions for the
// (repo, branch) pair. Both are required; empty values return (0, nil)
// without touching the store.
func (s *Store) QueueDepth(ctx context.Context, repo, branch string) (int, error) {
	if repo == "" || branch == "" {
		return 0, nil
	}
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? AND branch = ?`, repo, branch).Scan(&n)
	return n, err
}

// RecentAnswered returns up to limit answered questions for the (repo, branch)
// pair, ordered by most recently answered. Both are required; empty values
// return (nil, nil) without touching the store.
func (s *Store) RecentAnswered(ctx context.Context, repo, branch string, limit int) ([]StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, question, choices, correct_index, queued_at, answer_index, correct, feedback
		   FROM questions
		  WHERE answered_at IS NOT NULL AND repo = ? AND branch = ?
		  ORDER BY answered_at DESC
		  LIMIT ?`,
		repo, branch, limit)
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

// StalePerBranch returns a map of branch name → count of undelivered questions
// for that branch, restricted to the given repo. Branches with zero
// undelivered questions are omitted. Repo is required; an empty value returns
// an empty map. Used by GET /recall/stale to back the `tr status` advisory.
func (s *Store) StalePerBranch(ctx context.Context, repo string) (map[string]int, error) {
	out := make(map[string]int)
	if repo == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT branch, COUNT(*) FROM questions WHERE delivered_at IS NULL AND repo = ? AND branch != '' GROUP BY branch`,
		repo)
	if err != nil {
		return nil, fmt.Errorf("querying stale per-branch counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var branch string
		var count int
		if err := rows.Scan(&branch, &count); err != nil {
			return nil, fmt.Errorf("scanning stale per-branch row: %w", err)
		}
		out[branch] = count
	}
	return out, rows.Err()
}

// PeekNextQuestion returns the next unclaimed question for the (repo, branch)
// pair without claiming it. Both are required; empty values return
// (nil, nil) without touching the store. Used by the recall://queue resource.
func (s *Store) PeekNextQuestion(ctx context.Context, repo, branch string) (*StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, question, choices, correct_index, queued_at FROM questions WHERE delivered_at IS NULL AND repo = ? AND branch = ? ORDER BY queued_at ASC LIMIT 1`, repo, branch)

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

// trDir returns the Total Recall data directory. When the TR_HOME environment
// variable is set to a non-empty path, it is used (and created with 0700 if
// needed). Otherwise the default ~/.tr is used. TR_HOME enables test/CI
// isolation by redirecting memory.db and config.yaml away from the real home.
func trDir() (string, error) {
	if env := os.Getenv("TR_HOME"); env != "" {
		if err := os.MkdirAll(env, 0o700); err != nil {
			return "", fmt.Errorf("creating TR_HOME directory: %w", err)
		}
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".tr")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating ~/.tr directory: %w", err)
	}
	return dir, nil
}
