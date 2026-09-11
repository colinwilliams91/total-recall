## Purpose

Resolve markdown prompt assets for the synthesis pipeline: ship canonical policy docs inside the binary via `//go:embed` and let a file dropped into the runtime override directory replace them without recompiling.

## Requirements

### Requirement: `assets.Load` resolves prompt-asset markdown files with `//go:embed` default and runtime `<data-dir>/prompts/` override
The `assets` package SHALL expose `Load(name string) (PromptAsset, error)` which resolves a named prompt asset (`<name>.md`) by checking the override slot at `<data-dir>/prompts/<name>.md` first — the Total Recall data dir is `$TR_HOME` when set to a non-empty value, else `~/.tr`, the same resolution rule as `config.UserConfigDir` and `cache.trDir`; if absent, returning the `//go:embed`-ed default `assets/prompts/<name>.md`; if both unavailable, returning a `Source == "fallback"` sentinel `PromptAsset` with empty `Body`. Loaded assets SHALL be cached keyed by name so per-invocation reads do not re-stat the filesystem (the caching mechanism is specified by the idempotency requirement below). The override mechanism SHALL NOT be gated on `TR_HOME` being set: a default install that never defines `TR_HOME` still reads `~/.tr/prompts/`. When the data dir cannot be resolved at all (no home directory available), `Load` SHALL fall through to the embedded default rather than failing.

#### Scenario: Embedded default returned when override is absent
- **WHEN** `Load("question-generation-policy")` is called and no override file exists in the data dir's `prompts/` directory
- **THEN** the returned `PromptAsset` has `Source == "embedded"`, `Body` populated from the `//go:embed` byte slice, and front-matter keys parsed into `Name`/`Description`

#### Scenario: Override returned when the slot file exists
- **WHEN** `Load("question-generation-policy")` is called with an override file at `<data-dir>/prompts/question-generation-policy.md`
- **THEN** the returned `PromptAsset` has `Source == "$TR_HOME"` and `Body` equal to the override file's post-front-matter content; the embedded default is not consulted

#### Scenario: Override in the default data dir
- **WHEN** `TR_HOME` is unset and `~/.tr/prompts/question-generation-policy.md` exists
- **THEN** `Load("question-generation-policy")` returns `Source == "$TR_HOME"` with the absolute `~/.tr/prompts/...` path

#### Scenario: Unresolvable data dir falls back to embedded
- **WHEN** neither `$TR_HOME` nor a home directory can be resolved
- **THEN** `Load` returns the embedded default (or the `fallback` sentinel when the asset is unknown), not an error

#### Scenario: Fallback when no source available
- **WHEN** `Load("<unknown-asset>")` is called and neither the override path nor the embedded map contains the asset
- **THEN** `Load` returns `PromptAsset{Source: "fallback", Body: ""}` and a non-nil error; the caller is expected to handle the fallback explicitly (log and continue with the inline template)

---

### Requirement: YAML front-matter is parsed via string splitting, no YAML dependency introduced
The `assets` package SHALL extract `name` and `description` keys from a leading `---\n...\n---` front-matter block by splitting on lines and matching `key: value` for the two known keys. The front-matter block is excluded from `Body`. A missing or malformed front-matter block is non-fatal — the asset is returned with empty `Name`/`Description` and `Body` containing the full file content. No YAML library is added as a dependency.

#### Scenario: Standard front matter parses correctly
- **WHEN** an asset file begins with `---\nname: generate-quiz-question\ndescription: Generate short quiz question...\n---\n## Goals\n` content
- **THEN** the returned `PromptAsset.Name == "generate-quiz-question"`, `Description` contains the description value, and `Body` begins with `## Goals`

#### Scenario: Missing front-matter treated as body-only
- **WHEN** an asset file begins directly with body content (no `---` opener)
- **THEN** `Name` and `Description` are empty strings, `Body` equals the full file content

#### Scenario: Malformed front-matter does not panic
- **WHEN** an asset file's front-matter section is incomplete or contains unknown keys
- **THEN** the parser does not panic; known keys are extracted, unknown keys ignored, and `Body` begins at the line after the closing `---` (or the full file if no closing `---` is found)

#### Scenario: Unknown keys ignored without error
- **WHEN** the front-matter contains `priority: high` alongside the known `name` and `description` keys
- **THEN** `priority` is ignored; no error is returned; `Name` and `Description` reflect only the known keys

---

### Requirement: Prompt-asset directory structure is `assets/prompts/` for embedded defaults and `<data-dir>/prompts/` for overrides
The embedded defaults SHALL live at `assets/prompts/<name>.md` in the repository, accessible to `//go:embed` from the `assets` package (located at the repository root so the embed path resolves correctly — Go's `//go:embed` directive does not permit `..` in paths, so the loader package lives at `assets/` rather than `internal/assets/`). The runtime override directory SHALL be `<data-dir>/prompts/<name>.md` (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`). Override files MUST be plain markdown; no subdirectory layout is enforced beyond the `prompts/` subdir.

#### Scenario: Embedded asset path resolves at build time
- **WHEN** the `assets` package is compiled
- **THEN** the `//go:embed` directive captures every `.md` file under `assets/prompts/` into the binary; `go test ./assets/...` confirms the bundle contains `question-generation-policy.md`

#### Scenario: Override directory honors `$TR_HOME`
- **WHEN** `TR_HOME=/scratch/.tr` is set and `/scratch/.tr/prompts/question-generation-policy.md` exists
- **THEN** `Load` returns the override from `/scratch/.tr/prompts/...`; the embedded default is not consulted

#### Scenario: Override directory missing but `$TR_HOME` set — falls back to embedded
- **WHEN** `TR_HOME=/scratch/.tr` is set but `/scratch/.tr/prompts/` does not exist
- **THEN** `Load` returns the embedded default with `Source == "embedded"`; no error is logged about the missing override directory (the absence is the common case)

---

### Requirement: Prompt-asset loading is idempotent and cached per process
The first `assets.Load(name)` call SHALL resolve and parse the asset; subsequent calls SHALL return the cached `PromptAsset` value without re-reading disk or re-parsing markdown. Caching MUST be implemented with a package-level mutex-guarded map keyed by asset name. The cache SHALL live for the lifetime of the process — there is no file-watching eviction; restarting the daemon is the cache invalidation mechanism.

#### Scenario: Repeated calls return the same value instance
- **WHEN** `Load("question-generation-policy")` is called 10 times in sequence
- **THEN** all 10 calls return the same `PromptAsset` value (or pointer-equal cached struct); the disk is read exactly once across the 10 calls

#### Scenario: Override taken at first call is preserved even if file is later removed
- **WHEN** the first `Load` call picks up an override at `$TR_HOME/prompts/`, and the override file is deleted before a subsequent `Load` call
- **THEN** the subsequent `Load` call still returns the cached override value; no re-stat is performed (restart to pick up disk changes)


---

---

### Requirement: `assets.Load` emits a structured startup log line for every resolved override
When `assets.Load` resolves to an override file in the data dir's `prompts/` directory (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`), the package SHALL emit a log line including the asset name, the resolved override path, the override file's mtime (RFC3339), and the embedded default's compile-time reference. The log line format SHALL be: `[assets] prompt asset %q loaded from override at %s (mtime=%s, embedded ref=%s)`. When no override file exists for the asset (the embedded path is used), no enriched log line is emitted — the embedded path is silent (the absence of an override is the common case and should not be noisy).

#### Scenario: Override log line contains mtime and embedded reference
- **WHEN** `Load("question-generation-policy")` resolves to an override file for that asset in the data dir's `prompts/` directory
- **THEN** the log line contains the absolute override path, the override's mtime in RFC3339 format, and the embedded-reference marker

#### Scenario: Embedded path is silent
- **WHEN** `Load("question-generation-policy")` resolves to the embedded default (no override file exists)
- **THEN** no override log line is emitted
---

---

### Requirement: Stale override above drift threshold emits an `OVERRIDE WARNING` line
After computing the mtime delta between the override file and the embedded default compile-time reference, when the delta exceeds the drift threshold (default 90 days, configurable via `PromptAssetConfig.DriftWarningDays`), the package SHALL emit an additional log line: `[assets] OVERRIDE WARNING: %s is %dd older than the embedded default — re-sync with 'tr asset sync %s'`. The warning is informational only — it SHALL NOT block daemon startup, SHALL NOT override the user's file, and SHALL NOT mutate any state. A user who deliberately keeps an old override MAY silence the warning by raising the configured threshold.

#### Scenario: Override 100 days old fires warning at default threshold
- **WHEN** `Load("question-generation-policy")` resolves to an override with mtime 100 days ago and `DriftWarningDays == 90`
- **THEN** the log line contains `OVERRIDE WARNING` and `100d older than the embedded default`

#### Scenario: Override 10 days old does not fire warning at default threshold
- **WHEN** `Load("question-generation-policy")` resolves to an override with mtime 10 days ago and `DriftWarningDays == 90`
- **THEN** no `OVERRIDE WARNING` log line is emitted

#### Scenario: Configurable threshold fires warning earlier
- **WHEN** `DriftWarningDays == 30` and the override is 35 days old
- **THEN** the `OVERRIDE WARNING` log line fires

#### Scenario: Threshold of 0 disables the warning entirely
- **WHEN** `DriftWarningDays == 0` and the override is 10 years old
- **THEN** no `OVERRIDE WARNING` log line is emitted (0 is the explicit disable sentinel)
---

---

### Requirement: `tr config show` includes a "prompt assets" section listing resolved sources and ages
`internal/config` `Show()` SHALL, after rendering the existing config sections, query `assets.Load` for every known embedded prompt name (today only `question-generation-policy`) and print one line per resolved asset under a `prompt assets:` section header. Each line SHALL be: `  <name>: <resolved-path>  # [<source-tag>], <age>`. The `<source-tag>` is one of `embedded`, `override`, or `fallback`; the `<age>` is a human-readable mtime-relative string (e.g., `embedded` for the no-override case, `6mo old` for an override). The section lives at the end of the `Show()` output.

#### Scenario: No override — section lists embedded default
- **WHEN** `Show(buf)` is called and no override file exists for the asset
- **THEN** the output contains a `prompt assets:` section with one line: `  question-generation-policy: <embedded>  # [embedded], embedded`

#### Scenario: Override present — section lists the override path and age
- **WHEN** `Show(buf)` is called and `$TR_HOME/prompts/question-generation-policy.md` exists with mtime 2 days ago
- **THEN** the output contains the section with one line: `  question-generation-policy: <absolute-override-path>  # [override], 2d old`

#### Scenario: Multiple assets each get their line
- **WHEN** multiple embedded assets exist (hypothetical future add of a `feedback-policy.md`)
- **THEN** the section contains one line per distinct asset name streamed alphabetically
---

---

### Requirement: `PromptAssetConfig.DriftWarningDays` defaults to 90 and lives in user-level config
`internal/config.UserConfig` SHALL carry a `PromptAsset PromptAssetConfig` field with `DriftWarningDays int yaml:"drift-warning-days,omitempty"`. `DefaultUserConfig()` SHALL set `DriftWarningDays` to `90`. The key is user-level only — when it appears in a repo-level `.tr.yaml`, it SHALL be silently discarded by the two-tier merge (consistent with the existing `Privacy`/`AI` discard rule for user-level-only keys). A value of `0` is the explicit-disable sentinel for the warning. Negative values SHALL be treated as 0 at merge time.

#### Scenario: Default is 90 days
- **WHEN** `DefaultUserConfig()` is called
- **THEN** `PromptAsset.DriftWarningDays == 90`

#### Scenario: User-level config overrides the threshold
- **WHEN** `~/.tr/config.yaml` contains `prompt-asset: { drift-warning-days: 30 }`
- **THEN** the merged `UserConfig.PromptAsset.DriftWarningDays == 30`

#### Scenario: Repo-level config is silently discarded
- **WHEN** a `.tr.yaml` in the repo root contains `prompt-asset: { drift-warning-days: 5 }`
- **THEN** the merge layer discards the key with the existing warning pattern used for `Privacy`/`AI`; the user-level value (or default 90) is preserved

#### Scenario: Zero disables the warning
- **WHEN** `DriftWarningDays == 0`
- **THEN** no `OVERRIDE WARNING` log line is emitted, regardless of the override's age

#### Scenario: Negative value is normalized to zero at merge
- **WHEN** the configured `DriftWarningDays == -10`
- **THEN** the merge layer normalizes to `0` and logs `[config] prompt-asset.drift-warning-days (-10) is negative — treated as 0 (warning disabled)`