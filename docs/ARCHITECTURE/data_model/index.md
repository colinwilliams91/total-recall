# Total Recall — Data Model

Source of truth: `internal/cache/store.go` (SQLite schema), `internal/config/config.go`, `internal/pipeline/extraction.go`, `internal/recall/engine.go`, `internal/engine/server.go`.

## ERD (persisted state — `~/.tr/memory.db`)

```mermaid
erDiagram
    %% ══════════ Core persistence (SQLite, modernc.org/sqlite, no CGo) ══════════

    concepts {
        INTEGER id PK "AUTOINCREMENT"
        TEXT concept "NOT NULL"
        TEXT source "NOT NULL, default 'code'"
        REAL weight "NOT NULL, default 1.0"
        TEXT repo "NOT NULL — scope key"
        TEXT branch "NOT NULL — scope key"
        DATETIME seen_at "NOT NULL, UTC"
    }

    questions {
        INTEGER id PK "AUTOINCREMENT"
        TEXT question_type "enum: multiple_choice|max multi_select, default multiple_choice"
        TEXT status "enum: queued|delivered|answered|skipped, default queued"
        TEXT question "NOT NULL — question body"
        TEXT repo "NOT NULL — scope key"
        TEXT branch "NOT NULL — scope key"
        TEXT feedback "NULL — AI-generated explanation"
    }

    choices {
        INTEGER id PK "AUTOINCREMENT"
        INTEGER question_id FK "> questions.id, ON DELETE CASCADE"
        INTEGER position "NOT NULL — display order"
        TEXT text "NOT NULL — option text"
        INTEGER is_correct "NOT NULL, default 0 — THE correctness key (no positional/index key)"
    }

    selections {
        INTEGER id PK "AUTOINCREMENT"
        INTEGER question_id FK "> questions.id, ON DELETE CASCADE"
        INTEGER choice_id FK "> choices.id, ON DELETE CASCADE"
    }

    question_events {
        INTEGER id PK "AUTOINCREMENT"
        INTEGER question_id FK "> questions.id, ON DELETE CASCADE"
        TEXT event_type "enum: queued|delivered|answered|skipped"
        DATETIME occurred_at "NOT NULL, default CURRENT_TIMESTAMP"
        TEXT actor "NULL — 'system' | 'user' | claimedBy"
        TEXT payload "NULL"
    }

    questions ||--o{ choices : "question_id"
    questions ||--o{ selections : "question_id"
    choices  ||--o{ selections : "choice_id"
    questions ||--o{ question_events : "question_id (event log)"

    %% ══════════ Conceptual (non-FK) relationships ══════════

    concepts }o..|| questions : "concepts feed synthesis (per repo+branch, no FK)"
```

**Notes on relationships:**

- `questions`, `concepts`, and their children are scoped by the `(repo, branch)` pair — no FK, no global pool. Empty repo/branch is refused at the store layer.
- `selections` is an association table; `UNIQUE(question_id, choice_id)` prevents duplicate picks and defensively blocks cross-question choice IDs. It carries **no timestamp** — the parent question's `answered` event is the authoritative "when".
- `is_correct` on `choices` is the sole correctness key. `CorrectIndex` in `StoredQuestion` is derived post-hoc and is a **presentation aid only** (sentinel `-1` = none correct).
- `questions.status` is a denormalized cache of lifecycle state; the **source of truth is the latest `question_events` row**. Queue order is derived from the `queued` event's `occurred_at` (partial index `idx_qe_qid_time WHERE event_type='queued'`).

## Enums & state machine

```mermaid
stateDiagram-v2
    direction LR
    [*] --> queued : SaveQuestion ("system" queued event)
    queued --> delivered : NextQuestion atomically claims ("delivered" event, actor=claimedBy)
    queued --> skipped : SkipQuestion ("skipped" event)
    delivered --> answered : SubmitSelection ("answered" event, actor='user')
    delivered --> skipped : SkipQuestion
    answered --> [*]
    skipped --> [*]

    note right of delivered
        Terminal states: answered, skipped.
        Transitions guarded — cannot leave terminal state.
        Events are append-only; never rewritten.
    end note
```

## In-memory / transient structures (never persisted as-is)

```mermaid
classDiagram
    direction LR

    class ConceptFingerprint {
        <<pipeline — extract-and-discard output>>
        +Concept string
        +Source string
        +Weight float64 "[0.0, 1.0]"
    }
    class Fingerprint {
        <<cache.Store batch input>>
        +Concept string
        +Source string
        +Weight float64
    }
    class ConceptRow {
        <<cache read model>>
        +ID int64
        +Concept string
        +Source string
        +Weight float64
        +SeenAt time
    }
    class SynthesisContext {
        <<transient — rides runPipeline → Synthesize>>
        +Concepts []ConceptRow
        +CommitMsg string
        +DiffSnippet string
    }
    class StoredQuestion {
        <<hydrated read model>>
        +ID int64
        +QuestionType string
        +Status string
        +Question string
        +Repo string
        +Branch string
        +Choices []Choice
        +CorrectIndex int "-1 = none correct"
        +Feedback *string
        +Selections []Selection
    }
    class Choice {
        +ID int64
        +Text string
        +IsCorrect bool
        +Position int
    }
    class Selection {
        +ID int64
        +QuestionID int64
        +ChoiceID int64
    }

    ConceptFingerprint ..> Fingerprint : structurally identical\npipeline → store
    SynthesisContext o-- ConceptRow : enriched input
    StoredQuestion o-- Choice : joined, position order
    StoredQuestion o-- Selection : joined, insertion order
```

**Flow:** `git diff` → `pipeline.ExtractConcepts` → `ConceptFingerprint`(s) → `Store.Save` as `Fingerprint` → `concepts` rows → `recall.Engine` reads `Recent(repo, branch)` → `SynthesisContext` → AI provider → shuffled `Question` → `Store.SaveQuestion` → `questions` + `choices` + `queued` event.

## Config model (two-tier YAML deep-merge)

```mermaid
classDiagram
    direction TB

    class UserConfig {
        <<~/.tr/config.yaml — cross-repo personal defaults>>
        +Privacy PrivacyConfig
        +AI AIConfig
        +Recall RecallConfig
        +PromptAsset PromptAssetConfig
    }
    class RepoConfig {
        <<.tr.yaml — project settings>>
        +Hooks HooksConfig
        +Mode ModeConfig
        +Presentation PresentationConfig
        +Recall RecallConfig "optional per-repo override"
        +Privacy PrivacyConfig "USER-LEVEL ONLY — discarded with warning"
        +AI AIConfig "USER-LEVEL ONLY — discarded with warning"
        +PromptAsset PromptAssetConfig "USER-LEVEL ONLY — discarded with warning"
    }
    class Config {
        <<resolved after deep-merge>>
        +Privacy PrivacyConfig
        +AI AIConfig
        +Recall RecallConfig
        +PromptAsset PromptAssetConfig
        +Hooks HooksConfig
        +Mode ModeConfig
        +Presentation PresentationConfig
        +Sources ConfigSources
    }
    class ConfigSources {
        <<provenance map: user | repo | default>>
    }
    class AIConfig {
        <<provider enum: anthropic | openai | ollama | groq | qwen | minimax | deepseek | lm-studio | custom>>
        +Provider string
        +Model string
        +APIKey string "raw or env:VAR_NAME"
        +BaseURL string "required for custom"
    }
    class HooksConfig {
        +PreCommit bool
        +CommitMsg bool
        +PrePush bool
    }
    class RecallConfig {
        +Difficulty string "default 'adaptive'"
        +MaxQuestions int "default 1"
    }

    UserConfig --> Config : deep-merge (user wins only where repo unset... user values fill gaps)
    RepoConfig --> Config : deep-merge (repo keys win)
    Config o-- AIConfig
    Config o-- RecallConfig
    Config o-- ConfigSources
```

## Storage layout

```
~/.tr/ (or $TR_HOME)          ← single data dir, 0700
├── memory.db                 ← SQLite: concepts, questions, choices, selections, question_events
└── config.yaml               ← UserConfig
```

Key invariants:

- `db.SetMaxOpenConns(1)` — single-connection pool serializes all access; `NextQuestion`'s SELECT-then-UPDATE exactly-once claim depends on it.
- No `ALTER TABLE` migration path — schema changes are done by deleting `memory.db` (see `internal/cache/MIGRATION.md`).
- Concepts and questions are always repo+branch-scoped; the store refuses empty scope keys.
