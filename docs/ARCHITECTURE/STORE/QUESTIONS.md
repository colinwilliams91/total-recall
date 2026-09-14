## Schema

```sql
CREATE TABLE questions (
     id             INTEGER  PRIMARY KEY AUTOINCREMENT,
     question       TEXT     NOT NULL,
     choices        TEXT     NOT NULL,       -- JSON array
     correct_index  INTEGER  NOT NULL,       -- 0-based; never sent to terminal client
     queued_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
     delivered_at   DATETIME,
     claimed_by     TEXT,                    -- "mcp" | "shell"
     answer_text    TEXT,                    -- chosen text or "skip"
     answer_index   INTEGER,                 -- 0-based, NULL if skipped
     correct        INTEGER,                 -- 1=correct 0=wrong NULL=skip/pending
     feedback       TEXT,                    -- AI explanation, NULL for MCP rows
     answered_at    DATETIME
 );
```