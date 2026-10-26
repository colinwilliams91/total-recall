package cache

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "modernc.org/sqlite"
)

const (
	defaultDBFilename = "memory.db"
	recentLimit       = 50

	createConceptsTableSQL = `
CREATE TABLE IF NOT EXISTS concepts (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  slug      TEXT    NOT NULL REFERENCES concept_registry(slug),
  concept   TEXT    NOT NULL,
  source    TEXT    NOT NULL DEFAULT 'code',
  weight    REAL    NOT NULL DEFAULT 1.0,
  repo      TEXT    NOT NULL,
  branch    TEXT    NOT NULL,
  seen_at   DATETIME NOT NULL
);
`

	createConceptRegistryTableSQL = `
CREATE TABLE IF NOT EXISTS concept_registry (
  slug       TEXT PRIMARY KEY,
  prose      TEXT NOT NULL,
  first_seen DATETIME NOT NULL
);
`

	createConceptAliasesTableSQL = `
CREATE TABLE IF NOT EXISTS concept_aliases (
  alias  TEXT PRIMARY KEY,
  target TEXT NOT NULL REFERENCES concept_registry(slug)
);
`

	createQuestionConceptsTableSQL = `
CREATE TABLE IF NOT EXISTS question_concepts (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  slug        TEXT NOT NULL REFERENCES concept_registry(slug),
  weight      REAL NOT NULL DEFAULT 1.0
);
`

	createQuestionsTableSQL = `
CREATE TABLE IF NOT EXISTS questions (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    question_type   TEXT     NOT NULL DEFAULT 'multiple_choice'
                    CHECK (question_type IN ('multiple_choice','multi_select')),
    status          TEXT     NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued','delivered','answered','skipped')),
    question        TEXT     NOT NULL,
    repo            TEXT     NOT NULL,
    branch          TEXT     NOT NULL,
    feedback        TEXT
);
`

	createChoicesTableSQL = `
CREATE TABLE IF NOT EXISTS choices (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    text        TEXT    NOT NULL,
    is_correct  INTEGER NOT NULL DEFAULT 0
);
`

	createSelectionsTableSQL = `
CREATE TABLE IF NOT EXISTS selections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    choice_id   INTEGER NOT NULL REFERENCES choices(id)   ON DELETE CASCADE,
    UNIQUE (question_id, choice_id)
);
`

	createQuestionEventsTableSQL = `
CREATE TABLE IF NOT EXISTS question_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    event_type  TEXT    NOT NULL
                CHECK (event_type IN ('queued','delivered','answered','skipped')),
    occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor       TEXT,
    payload     TEXT
);
`
)

// ConceptRow is a persisted concept fingerprint from the cache. Slug is the
// resolved registry identity the sighting links to; Concept is the free-form
// prose of that sighting.
type ConceptRow struct {
	ID      int64
	Concept string
	Slug    string
	Source  string
	Weight  float64
	SeenAt  time.Time
}

// Choice is one option of a multiple-choice question. The engine's correctness
// key is the IsCorrect boolean on the row — never a positional reference into
// a serialized array. ID is the SQLite row id and is referenced by Selection.
type Choice struct {
	ID        int64
	Text      string
	IsCorrect bool
	Position  int
}

// Selection is one user pick: a (question, choice) pair. Multi-select
// questions produce multiple Selection rows per question. Selections carry no
// timestamp — the parent question's 'answered' question_events row is the
// authoritative "when" record.
type Selection struct {
	ID         int64
	QuestionID int64
	ChoiceID   int64
}

// StoredQuestion is a recall question retrieved from the questions table.
// Choices is populated by joining the choices table (in position order).
// Status is the denormalized cache column (the source of truth is the latest
// question_events row). Selections is populated by joining the selections
// table; nil for undelivered/skipped questions. CorrectIndex is derived from
// the IsCorrect boolean on the choice row — it is a presentation aid, not the
// source of truth.
type StoredQuestion struct {
	ID            int64
	QuestionType  string
	Status        string
	Question      string
	Repo          string
	Branch        string
	Choices       []Choice
	CorrectIndex  int
	Feedback      *string
	Selections    []Selection
}

// Store wraps the SQLite database used as the memory store.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the memory store at $TR_HOME/memory.db, or
// ~/.tr/memory.db when TR_HOME is unset. Returns a non-nil *Store on success.
//
// MIGRATION: this version reshapes the `concepts` table into the two-layer
// identity model — `concepts` gains a `slug` FK into the new user-scoped
// `concept_registry`, and `concept_aliases` / `question_concepts` join the
// schema. This is on top of the prior reshapes of the `questions` table
// (dropped `correct_answer` and the 'free_text' CHECK value, and before that
// the multi-column reshape that added `choices`, `selections`, and
// `question_events`). There is no in-code migration path. If a prior
// `memory.db` exists, Open()'s `CREATE TABLE IF NOT EXISTS` is a no-op
// against the stale `concepts` table — sightings inserts reference
// `concepts.slug`, which cannot exist there, so saves fail loudly. The
// maintainer must delete the data file before the new build runs. See
// MIGRATION.md in this package.
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
	//
	// This also enforces exactly-once delivery in NextQuestion, whose
	// SELECT-then-UPDATE pattern depends on a single-connection pool. Do not
	// raise this value without first collapsing NextQuestion back to a single
	// atomic UPDATE…RETURNING statement.
	db.SetMaxOpenConns(1)

	bg := context.Background()
	if _, err := db.ExecContext(bg, createConceptRegistryTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing concept_registry schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createConceptsTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing concepts schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createConceptAliasesTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing concept_aliases schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createQuestionsTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing questions schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createChoicesTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing choices schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createSelectionsTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing selections schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createQuestionEventsTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing question_events schema: %w", err)
	}
	if _, err := db.ExecContext(bg, createQuestionConceptsTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing question_concepts schema: %w", err)
	}

	if _, err := db.ExecContext(bg, `CREATE INDEX IF NOT EXISTS idx_concepts_repo_branch_seen ON concepts(repo, branch, seen_at DESC)`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating idx_concepts_repo_branch_seen: %w", err)
	}
	if _, err := db.ExecContext(bg, `CREATE INDEX IF NOT EXISTS idx_choices_qid ON choices(question_id)`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating idx_choices_qid: %w", err)
	}
	if _, err := db.ExecContext(bg, `CREATE INDEX IF NOT EXISTS idx_qe_qid_time ON question_events(question_id, occurred_at DESC) WHERE event_type = 'queued'`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating idx_qe_qid_time: %w", err)
	}
	if _, err := db.ExecContext(bg, `CREATE INDEX IF NOT EXISTS idx_question_concepts_qid ON question_concepts(question_id)`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating idx_question_concepts_qid: %w", err)
	}

	return &Store{db: db}, nil
}

// Fingerprint are lightweight metadata only — no raw diff text.
// Save persists a batch of concept fingerprints to the cache, tagged with repo.
// Slug is the canonical identity key: it is resolved against concept_registry
// (aliases included) and minted on first sighting; Concept stays the free-form
// prose fingerprint.
type Fingerprint struct {
	Concept string
	Slug    string
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

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO concepts (slug, concept, source, weight, repo, branch, seen_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, c := range concepts {
		slug, ok := resolveIdentity(ctx, tx, c.Slug, c.Concept)
		if !ok {
			// Element-level skip: a malformed/missing slug degrades this
			// sighting only, never the batch or the pipeline.
			continue
		}
		if _, err := stmt.ExecContext(ctx, slug, c.Concept, c.Source, c.Weight, repo, branch, now); err != nil {
			return fmt.Errorf("inserting concept %q: %w", c.Concept, err)
		}
	}
	return tx.Commit()
}

// resolveIdentity resolves one fingerprint's canonical slug inside tx:
// alias lookup first (alias → target), then exact registry match, then mint.
// It returns ok=false (with a logged warning) for malformed or missing slugs —
// the caller skips that element and keeps the batch. The registry row is
// minted with the fingerprint's prose on first sighting (first_seen and prose
// are never touched on reuse).
func resolveIdentity(ctx context.Context, tx *sql.Tx, slug, prose string) (string, bool) {
	if slug == "" {
		log.Printf("[store] dropping sighting: empty slug")
		return "", false
	}
	if !IsValidSlug(slug) {
		log.Printf("[store] dropping sighting with malformed slug %q (want [a-z0-9_]+)", slug)
		return "", false
	}
	// Alias pressure valve: exact lookup only. An alias row redirects the
	// identity here; resolution stays non-fuzzy by construction.
	var target string
	err := tx.QueryRowContext(ctx, `SELECT target FROM concept_aliases WHERE alias = ?`, slug).Scan(&target)
	if err == nil {
		slug = target
	} else if err != sql.ErrNoRows {
		log.Printf("[store] alias lookup for %q: %v", slug, err)
		return "", false
	}

	// Mint on first sighting; reuse (first_seen + first-seen prose untouched)
	// on later ones.
	if _, err := tx.ExecContext(ctx, `
INSERT INTO concept_registry (slug, prose, first_seen)
VALUES (?, ?, COALESCE((SELECT first_seen FROM concept_registry WHERE slug = ?), ?))
ON CONFLICT(slug) DO NOTHING`,
		slug, prose, slug, time.Now().UTC()); err != nil {
		log.Printf("[store] registry mint for %q: %v", slug, err)
		return "", false
	}
	return slug, true
}

// slugValuePattern is the canonical slug shape: lowercase letters, digits,
// and underscores only. Prose-shaped text (spaces, mixed case, punctuation)
// never matches, so validation is a single comparison.
var slugValuePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// IsValidSlug reports whether the string conforms to the canonical slug shape.
func IsValidSlug(slug string) bool {
	return slugValuePattern.MatchString(slug)
}

// Alias is one alias → target mapping from the concept_aliases pressure
// valve. Resolve is exact-match at the write boundary; this type is the
// listing/management projection.
type Alias struct {
	Alias  string
	Target string
}

// AddAlias maps alias onto target after validating both the alias shape and
// that the target already exists in the registry. A dangling target is
// rejected with an explicit error (never created as a silent dangling row).
func (s *Store) AddAlias(ctx context.Context, alias, target string) error {
	if !IsValidSlug(alias) {
		return fmt.Errorf("invalid alias %q: want [a-z0-9_]+", alias)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM concept_registry WHERE slug = ?`, target).Scan(&exists); err != nil {
		return fmt.Errorf("reading registry for target %q: %w", target, err)
	}
	if exists == 0 {
		return fmt.Errorf("cannot add alias %q: target %q has no registry identity", alias, target)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO concept_aliases (alias, target) VALUES (?, ?)`, alias, target); err != nil {
		return fmt.Errorf("inserting alias %q: %w", alias, err)
	}
	return tx.Commit()
}

// RemoveAlias deletes the alias row by exact name and reports whether a row
// was removed. Unknown aliases are not an error — removal is a repair tool.
func (s *Store) RemoveAlias(ctx context.Context, alias string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM concept_aliases WHERE alias = ?`, alias)
	if err != nil {
		return false, fmt.Errorf("removing alias %q: %w", alias, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("reading rows affected for alias %q: %w", alias, err)
	}
	return n > 0, nil
}

// Aliases lists all alias → target pairs ordered by alias.
func (s *Store) Aliases(ctx context.Context) ([]Alias, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT alias, target FROM concept_aliases ORDER BY alias ASC`)
	if err != nil {
		return nil, fmt.Errorf("querying aliases: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Alias
	for rows.Next() {
		var a Alias
		if err := rows.Scan(&a.Alias, &a.Target); err != nil {
			return nil, fmt.Errorf("scanning alias row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
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
		`SELECT id, slug, concept, source, weight, seen_at FROM concepts WHERE repo = ? AND branch = ? ORDER BY seen_at DESC LIMIT ?`, repo, branch, n)
	if err != nil {
		return nil, fmt.Errorf("querying recent concepts: %w", err)
	}
	defer rows.Close()

	var result []ConceptRow
	for rows.Next() {
		var r ConceptRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.Concept, &r.Source, &r.Weight, &r.SeenAt); err != nil {
			return nil, fmt.Errorf("scanning concept row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ConceptRef is one fed concept as provenance input to SaveQuestion: the
// resolved registry slug plus the sighting weight at synthesis time. The
// weight is a per-write snapshot of what drove the question; the live weight
// lives on the sighting row.
type ConceptRef struct {
	Slug   string
	Weight float64
}

// SaveQuestion persists a synthesized question and its choices in one
// transaction, plus a 'queued' question_events row marking the question's
// creation, plus one question_concepts provenance row per fed concept
// (fedConcepts). Both repo and branch are required; empty values are a no-op.
// questionType defaults to 'multiple_choice' when empty. An empty concept
// batch persists the question with zero provenance rows rather than erroring.
func (s *Store) SaveQuestion(ctx context.Context, repo, branch, question string, choices []Choice, questionType string, fed []ConceptRef) error {
	if repo == "" || branch == "" {
		log.Printf("[store] skipping savequestion: empty repo or branch")
		return nil
	}
	if question == "" {
		return fmt.Errorf("refusing to save empty question text")
	}
	if questionType == "" {
		questionType = "multiple_choice"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO questions (question_type, status, question, repo, branch) VALUES (?, 'queued', ?, ?, ?)`,
		questionType, question, repo, branch)
	if err != nil {
		return fmt.Errorf("inserting question: %w", err)
	}
	questionID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("reading last insert id: %w", err)
	}

	choiceStmt, err := tx.PrepareContext(ctx, `INSERT INTO choices (question_id, position, text, is_correct) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing choice insert: %w", err)
	}
	defer choiceStmt.Close()
	for _, c := range choices {
		isCorrect := 0
		if c.IsCorrect {
			isCorrect = 1
		}
		if _, err := choiceStmt.ExecContext(ctx, questionID, c.Position, c.Text, isCorrect); err != nil {
			return fmt.Errorf("inserting choice: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO question_events (question_id, event_type, actor) VALUES (?, 'queued', 'system')`,
		questionID); err != nil {
		return fmt.Errorf("inserting queued event: %w", err)
	}

	// Provenance: one row per fed concept, same transaction. A rejected
	// SaveQuestion therefore leaves zero partial provenance rows.
	fedStmt, err := tx.PrepareContext(ctx, `INSERT INTO question_concepts (question_id, slug, weight) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing provenance insert: %w", err)
	}
	defer func() { _ = fedStmt.Close() }()
	for _, ref := range fed {
		if _, err := fedStmt.ExecContext(ctx, questionID, ref.Slug, ref.Weight); err != nil {
			return fmt.Errorf("inserting question_concepts (question %d, slug %q): %w", questionID, ref.Slug, err)
		}
	}

	return tx.Commit()
}

// NextQuestion atomically claims the oldest queued question for the (repo,
// branch) pair. It transitions status 'queued' → 'delivered' and inserts a
// 'delivered' question_events row in a single transaction.
func (s *Store) NextQuestion(ctx context.Context, repo, branch, claimedBy string) (*StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Pick the oldest queued question for this (repo, branch) pair, ordered by
	// the 'queued' event's occurred_at (source of truth for queue order).
	//
	// Exactly-once delivery relies on db.SetMaxOpenConns(1) (see Open()): the
	// SELECT-then-UPDATE pattern here is NOT a single atomic statement, so
	// raising the pool size would let two concurrent transactions SELECT the
	// same queued row and both claim it. Do not tune the pool size up without
	// first collapsing this back to a single atomic UPDATE…RETURNING.
	row := tx.QueryRowContext(ctx, `
SELECT q.id
FROM questions q
JOIN question_events e
  ON e.question_id = q.id AND e.event_type = 'queued'
WHERE q.status = 'queued' AND q.repo = ? AND q.branch = ?
ORDER BY e.occurred_at ASC
LIMIT 1`, repo, branch)
	var id int64
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("selecting next question: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE questions SET status = 'delivered' WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("updating status: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO question_events (question_id, event_type, actor) VALUES (?, 'delivered', ?)`,
		id, claimedBy); err != nil {
		return nil, fmt.Errorf("inserting delivered event: %w", err)
	}

	q, err := selectQuestionInTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing: %w", err)
	}
	return q, nil
}

// GetQuestion fetches a single question by ID joined with its choices.
func (s *Store) GetQuestion(ctx context.Context, id int64) (*StoredQuestion, error) {
	return selectQuestion(ctx, s.db, id)
}

// SubmitSelection records the user's picks for a question. In a single
// transaction it inserts one selections row per choiceID, transitions the
// question's status to 'answered', and inserts an 'answered' question_events
// row. The transaction is guarded by the current status (must be 'delivered');
// if the question is already in a terminal state, the submit is rejected and no rows are written.
// Feedback is the AI-generated explanation text (empty string → null in DB).
func (s *Store) SubmitSelection(ctx context.Context, questionID int64, selectedChoiceIDs []int64, feedback string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Guard: the question must currently be in 'delivered' state.
	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM questions WHERE id = ?`, questionID).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("question %d not found", questionID)
		}
		return fmt.Errorf("reading status: %w", err)
	}
	if status != "delivered" {
		return fmt.Errorf("question %d is in status %q, expected 'delivered'", questionID, status)
	}

	// Validate every choice ID belongs to this question (defense in depth —
	// the UNIQUE(question_id, choice_id) constraint catches cross-question IDs
	// anyway, but a clear error is friendlier).
	for _, choiceID := range selectedChoiceIDs {
		var ownerQID int64
		if err := tx.QueryRowContext(ctx,
			`SELECT question_id FROM choices WHERE id = ?`, choiceID).Scan(&ownerQID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("choice %d not found", choiceID)
			}
			return fmt.Errorf("reading choice %d: %w", choiceID, err)
		}
		if ownerQID != questionID {
			return fmt.Errorf("choice %d does not belong to question %d", choiceID, questionID)
		}
	}

	selStmt, err := tx.PrepareContext(ctx, `INSERT INTO selections (question_id, choice_id) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing selection insert: %w", err)
	}
	defer selStmt.Close()
	for _, choiceID := range selectedChoiceIDs {
		if _, err := selStmt.ExecContext(ctx, questionID, choiceID); err != nil {
			return fmt.Errorf("inserting selection (question %d, choice %d): %w", questionID, choiceID, err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE questions SET status = 'answered' WHERE id = ?`, questionID); err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO question_events (question_id, event_type, actor) VALUES (?, 'answered', 'user')`,
		questionID); err != nil {
		return fmt.Errorf("inserting answered event: %w", err)
	}

	if feedback != "" {
		if _, err := tx.ExecContext(ctx,
			`UPDATE questions SET feedback = ? WHERE id = ?`, feedback, questionID); err != nil {
			return fmt.Errorf("updating feedback: %w", err)
		}
	}

	return tx.Commit()
}

// SetFeedback updates the feedback column on a question. Used when feedback
// arrives asynchronously after the selection was already recorded. Empty
// feedback clears the column.
func (s *Store) SetFeedback(ctx context.Context, id int64, feedback string) error {
	var arg any
	if feedback == "" {
		arg = nil
	} else {
		arg = feedback
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE questions SET feedback = ? WHERE id = ?`, arg, id)
	return err
}

// SkipQuestion records a skip. In a single transaction it transitions status
// to 'skipped' and inserts a 'skipped' question_events row. The transaction
// is guarded by the current status (must be 'queued' or 'delivered'); if the
// question is already in a terminal state, the skip is rejected. No
// selections rows are inserted — "the user selected nothing" lives in
// status='skipped', not in absent selections rows. ID-keyed.
func (s *Store) SkipQuestion(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM questions WHERE id = ?`, id).Scan(&status); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("question %d not found", id)
		}
		return fmt.Errorf("reading status: %w", err)
	}
	if status != "queued" && status != "delivered" {
		return fmt.Errorf("question %d is in status %q, expected 'queued' or 'delivered'", id, status)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE questions SET status = 'skipped' WHERE id = ?`, id); err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO question_events (question_id, event_type) VALUES (?, 'skipped')`,
		id); err != nil {
		return fmt.Errorf("inserting skipped event: %w", err)
	}

	return tx.Commit()
}

// QueueDepth returns the number of queued (undelivered) questions for the
// (repo, branch) pair. Both are required; empty values return (0, nil) without
// touching the store.
func (s *Store) QueueDepth(ctx context.Context, repo, branch string) (int, error) {
	if repo == "" || branch == "" {
		return 0, nil
	}
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM questions WHERE status = 'queued' AND repo = ? AND branch = ?`, repo, branch).Scan(&n)
	return n, err
}

// RecentAnswered returns up to limit answered questions for the (repo, branch)
// pair, ordered by the 'answered' event's occurred_at descending. Skipped
// questions are excluded — RecentSkipped covers that case. Both repo and
// branch are required; empty values return (nil, nil) without touching the
// store. Each row is joined with its choices and its selections.
func (s *Store) RecentAnswered(ctx context.Context, repo, branch string, limit int) ([]StoredQuestion, error) {
	return recentQuestions(ctx, s.db, repo, branch, "answered", limit)
}

// RecentSkipped returns up to limit skipped questions for the (repo, branch)
// pair, ordered by the 'skipped' event's occurred_at descending. Both repo
// and branch are required; empty values return (nil, nil) without touching
// the store.
func (s *Store) RecentSkipped(ctx context.Context, repo, branch string, limit int) ([]StoredQuestion, error) {
	return recentQuestions(ctx, s.db, repo, branch, "skipped", limit)
}

// RecentQuestions returns up to limit terminal-state (answered or skipped)
// questions for the (repo, branch) pair, ordered by the terminal event's
// occurred_at descending. The returned rows carry their Status field, which
// distinguishes answered from skipped.
func (s *Store) RecentQuestions(ctx context.Context, repo, branch string, limit int) ([]StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT q.id, q.question_type, q.status, q.question, q.repo, q.branch, q.feedback,
       e_terminal.occurred_at
FROM questions q
JOIN question_events e_terminal
  ON e_terminal.question_id = q.id
 AND e_terminal.event_type  = q.status
WHERE q.status IN ('answered','skipped') AND q.repo = ? AND q.branch = ?
ORDER BY e_terminal.occurred_at DESC
LIMIT ?`, repo, branch, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent questions: %w", err)
	}
	defer rows.Close()

	var result []StoredQuestion
	for rows.Next() {
		var sq StoredQuestion
		var feedback sql.NullString
		var terminalAt string
		if err := rows.Scan(&sq.ID, &sq.QuestionType, &sq.Status, &sq.Question, &sq.Repo, &sq.Branch, &feedback, &terminalAt); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		if feedback.Valid {
			v := feedback.String
			sq.Feedback = &v
		}
		result = append(result, sq)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := hydrateChoicesAndSelections(ctx, s.db, result); err != nil {
		return nil, err
	}
	return result, nil
}

func recentQuestions(ctx context.Context, db *sql.DB, repo, branch, status string, limit int) ([]StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
SELECT q.id, q.question_type, q.status, q.question, q.repo, q.branch, q.feedback
FROM questions q
JOIN question_events e_terminal
  ON e_terminal.question_id = q.id
 AND e_terminal.event_type  = q.status
WHERE q.status = ? AND q.repo = ? AND q.branch = ?
ORDER BY e_terminal.occurred_at DESC
LIMIT ?`, status, repo, branch, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent %s: %w", status, err)
	}
	defer rows.Close()

	var result []StoredQuestion
	for rows.Next() {
		var sq StoredQuestion
		var feedback sql.NullString
		if err := rows.Scan(&sq.ID, &sq.QuestionType, &sq.Status, &sq.Question, &sq.Repo, &sq.Branch, &feedback); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		if feedback.Valid {
			v := feedback.String
			sq.Feedback = &v
		}
		result = append(result, sq)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := hydrateChoicesAndSelections(ctx, db, result); err != nil {
		return nil, err
	}
	return result, nil
}

// StalePerBranch returns a map of branch name → count of queued questions
// for that branch, restricted to the given repo. Branches with zero queued
// questions are omitted. Repo is required; an empty value returns an empty
// map. Used by GET /recall/stale to back the `torec status` advisory.
func (s *Store) StalePerBranch(ctx context.Context, repo string) (map[string]int, error) {
	out := make(map[string]int)
	if repo == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT branch, COUNT(*) FROM questions WHERE status = 'queued' AND repo = ? AND branch != '' GROUP BY branch`,
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

// PeekNextQuestion returns the next queued question for the (repo, branch)
// pair without claiming it. Both are required; empty values return
// (nil, nil) without touching the store. Used by the recall://queue resource.
func (s *Store) PeekNextQuestion(ctx context.Context, repo, branch string) (*StoredQuestion, error) {
	if repo == "" || branch == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
SELECT q.id
FROM questions q
JOIN question_events e
  ON e.question_id = q.id AND e.event_type = 'queued'
WHERE q.status = 'queued' AND q.repo = ? AND q.branch = ?
ORDER BY e.occurred_at ASC
LIMIT 1`, repo, branch)
	var id int64
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("peeking next question: %w", err)
	}
	return selectQuestion(ctx, s.db, id)
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

// ── query helpers ─────────────────────────────────────────────────────────────

// selectQuestion reads a single question by id joined with its choices (in
// position order). The returned StoredQuestion's CorrectIndex is derived from
// the choice with is_correct=1 — that field is a presentation aid only, the
// source of truth is the IsCorrect boolean on each choice row.
func selectQuestion(ctx context.Context, db *sql.DB, id int64) (*StoredQuestion, error) {
	row := db.QueryRowContext(ctx, `
SELECT id, question_type, status, question, repo, branch, feedback
FROM questions WHERE id = ?`, id)
	var sq StoredQuestion
	var feedback sql.NullString
	if err := row.Scan(&sq.ID, &sq.QuestionType, &sq.Status, &sq.Question, &sq.Repo, &sq.Branch, &feedback); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("fetching question %d: %w", id, err)
	}
	if feedback.Valid {
		v := feedback.String
		sq.Feedback = &v
	}

	choices, err := loadChoices(ctx, db, id)
	if err != nil {
		return nil, err
	}
	sq.Choices = choices
	sq.CorrectIndex = deriveCorrectIndex(choices)
	return &sq, nil
}

// selectQuestionInTx is the in-transaction variant of selectQuestion. Reads
// must happen on the same tx that wrote the 'delivered' event so the
// returned question reflects the just-claimed state.
func selectQuestionInTx(ctx context.Context, tx *sql.Tx, id int64) (*StoredQuestion, error) {
	row := tx.QueryRowContext(ctx, `
SELECT id, question_type, status, question, repo, branch, feedback
FROM questions WHERE id = ?`, id)
	var sq StoredQuestion
	var feedback sql.NullString
	if err := row.Scan(&sq.ID, &sq.QuestionType, &sq.Status, &sq.Question, &sq.Repo, &sq.Branch, &feedback); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("fetching question %d: %w", id, err)
	}
	if feedback.Valid {
		v := feedback.String
		sq.Feedback = &v
	}

	choices, err := loadChoicesInTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	sq.Choices = choices
	sq.CorrectIndex = deriveCorrectIndex(choices)
	return &sq, nil
}

// loadChoices reads the choice rows for a question in position order.
func loadChoices(ctx context.Context, db *sql.DB, questionID int64) ([]Choice, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, position, text, is_correct FROM choices
WHERE question_id = ? ORDER BY position ASC`, questionID)
	if err != nil {
		return nil, fmt.Errorf("loading choices: %w", err)
	}
	defer rows.Close()
	var result []Choice
	for rows.Next() {
		var c Choice
		var isCorrect int
		if err := rows.Scan(&c.ID, &c.Position, &c.Text, &isCorrect); err != nil {
			return nil, fmt.Errorf("scanning choice: %w", err)
		}
		c.IsCorrect = isCorrect != 0
		result = append(result, c)
	}
	return result, rows.Err()
}

func loadChoicesInTx(ctx context.Context, tx *sql.Tx, questionID int64) ([]Choice, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, position, text, is_correct FROM choices
WHERE question_id = ? ORDER BY position ASC`, questionID)
	if err != nil {
		return nil, fmt.Errorf("loading choices: %w", err)
	}
	defer rows.Close()
	var result []Choice
	for rows.Next() {
		var c Choice
		var isCorrect int
		if err := rows.Scan(&c.ID, &c.Position, &c.Text, &isCorrect); err != nil {
			return nil, fmt.Errorf("scanning choice: %w", err)
		}
		c.IsCorrect = isCorrect != 0
		result = append(result, c)
	}
	return result, rows.Err()
}

// hydrateChoicesAndSelections populates the Choices and Selections slices of
// each StoredQuestion in-place. One round-trip per slice per question; for
// typical small result sets (≤50) this is trivial.
func hydrateChoicesAndSelections(ctx context.Context, db *sql.DB, qs []StoredQuestion) error {
	for i := range qs {
		choices, err := loadChoices(ctx, db, qs[i].ID)
		if err != nil {
			return err
		}
		qs[i].Choices = choices
		qs[i].CorrectIndex = deriveCorrectIndex(choices)

		selRows, err := db.QueryContext(ctx, `
SELECT id, question_id, choice_id FROM selections
WHERE question_id = ? ORDER BY id ASC`, qs[i].ID)
		if err != nil {
			return fmt.Errorf("loading selections: %w", err)
		}
		var sels []Selection
		for selRows.Next() {
			var s Selection
			if err := selRows.Scan(&s.ID, &s.QuestionID, &s.ChoiceID); err != nil {
				selRows.Close()
				return fmt.Errorf("scanning selection: %w", err)
			}
			sels = append(sels, s)
		}
		selRows.Close()
		if err := selRows.Err(); err != nil {
			return err
		}
		qs[i].Selections = sels
	}
	return nil
}

// deriveCorrectIndex returns the position of the choice with IsCorrect=true,
// or -1 if no choice has IsCorrect set. The -1 sentinel is intentional:
// returning 0 for any "no correct row" case would silently identify
// choices[0] as the correct answer at every caller that reads
// CorrectIndex before checking for a valid index into Choices.
// Callers MUST check CorrectIndex >= 0 (or branch on QuestionType) before
// treating it as a valid index into Choices.
func deriveCorrectIndex(choices []Choice) int {
	for _, c := range choices {
		if c.IsCorrect {
			return c.Position
		}
	}
	return -1
}
