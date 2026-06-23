# Spec: answer-schema

## Overview

Extends the `questions` table in `memory.db` to carry the full answer lifecycle: correct answer position at enqueue, user's chosen position and correctness at answer time, and AI-generated feedback for terminal users. Updates store methods and structs accordingly.

---

## Scenarios

### 1. Fresh install — questions table created with full schema

**Given** `memory.db` does not exist  
**When** `store.Open()` is called  
**Then** the `questions` table is created with columns: `id`, `question`, `choices`, `correct_index`, `queued_at`, `delivered_at`, `claimed_by`, `answer`, `answer_index`, `correct`, `feedback`, `answered_at`

---

### 2. Existing install — columns added idempotently

**Given** `memory.db` exists with the Phase 4A questions table (no `correct_index`, `answer_index`, `correct`, `feedback` columns)  
**When** `store.Open()` is called with the Phase 4C binary  
**Then** each missing column is added via `ALTER TABLE ADD COLUMN`  
**And** existing rows remain intact with `correct_index = 0`, `answer_index = NULL`, `correct = NULL`, `feedback = NULL`  
**And** calling `store.Open()` again (idempotent re-run) does not error

---

### 3. SaveQuestion persists correct_index

**Given** `recall.Synthesize` returns a `*recall.Question` with `CorrectIndex = 2` (after shuffling)  
**When** `runPipeline` calls `store.SaveQuestion(ctx, q.Question, q.Choices, q.CorrectIndex)`  
**Then** a row is inserted with `correct_index = 2`  
**And** `delivered_at`, `answer`, `answer_index`, `correct`, `feedback`, `answered_at` are all NULL

---

### 4. AnswerQuestion records all answer fields

**Given** question ID 1 exists with `correct_index = 0` and choices `["A", "B", "C"]`  
**When** `store.AnswerQuestion(ctx, 1, 1, "B", false, "A is correct because...")` is called  
**Then** the row is updated: `answer = "B"`, `answer_index = 1`, `correct = 0`, `feedback = "A is correct because..."`, `answered_at = now()`

---

### 5. AnswerQuestion with empty feedback (MCP path)

**Given** question ID 1 exists  
**When** `store.AnswerQuestion(ctx, 1, 0, "A", true, "")` is called  
**Then** `answer = "A"`, `answer_index = 0`, `correct = 1`, `feedback = NULL`, `answered_at = now()`

---

### 6. SkipQuestion records skip with no evaluation

**Given** question ID 1 exists  
**When** `store.SkipQuestion(ctx, 1)` is called  
**Then** `answer = "skip"`, `answered_at = now()`  
**And** `answer_index`, `correct`, `feedback` remain NULL

---

### 7. GetQuestion returns full StoredQuestion

**Given** question ID 1 exists with all fields populated  
**When** `store.GetQuestion(ctx, 1)` is called  
**Then** a `*StoredQuestion` is returned with `ID`, `Question`, `Choices`, `CorrectIndex`, `QueuedAt` populated  
**And** returns `nil, nil` when no row with the given ID exists

---

### 8. RecentAnswered returns enriched rows

**Given** three questions have been answered (two correct, one skipped)  
**When** `store.RecentAnswered(ctx, 10)` is called  
**Then** all three rows are returned ordered by `answered_at DESC`  
**And** each row includes `CorrectIndex`, `AnswerIndex` (nil for skip), `Correct` (nil for skip), `Feedback` (nil for skip and MCP rows)

---

### 9. StoredQuestion struct completeness

**Given** a `StoredQuestion` is scanned from the DB  
**Then** it has fields: `ID int64`, `Question string`, `Choices []string`, `CorrectIndex int`, `QueuedAt time.Time`, `AnswerIndex *int`, `Correct *bool`, `Feedback *string`  
**And** pointer fields are nil when the DB column is NULL
