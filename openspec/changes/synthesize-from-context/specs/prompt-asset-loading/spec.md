## ADDED Requirements

### Requirement: `internal/assets.Load` resolves prompt-asset markdown files with `//go:embed` default and runtime `$TR_HOME/prompts/` override
The `internal/assets` package SHALL expose `Load(name string) (PromptAsset, error)` which resolves a named prompt asset (`<name>.md`) by checking `$TR_HOME/prompts/<name>.md` first when `$TR_HOME` is set; if absent, returning the `//go:embed`-ed default `assets/prompts/<name>.md`; if both unavailable, returning a `Source == "fallback"` sentinel `PromptAsset` with empty `Body`. Loaded assets SHALL be cached via `sync.Once` keyed by name so per-invocation reads do not re-stat the filesystem.

#### Scenario: Embedded default returned when override is absent
- **WHEN** `Load("question-generation-policy")` is called and `$TR_HOME` is unset
- **THEN** the returned `PromptAsset` has `Source == "embedded"`, `Body` populated from the `//go:embed` byte slice, and front-matter keys parsed into `Name`/`Description`

#### Scenario: Override returned when `$TR_HOME/prompts/<name>.md` exists
- **WHEN** `Load("question-generation-policy")` is called with `$TR_HOME` pointing at a directory containing `prompts/question-generation-policy.md`
- **THEN** the returned `PromptAsset` has `Source == "$TR_HOME"` and `Body` equal to the override file's post-front-matter content; the embedded default is not consulted

#### Scenario: Fallback when no source available
- **WHEN** `Load("<unknown-asset>")` is called and neither the override path nor the embedded map contains the asset
- **THEN** `Load` returns `PromptAsset{Source: "fallback", Body: ""}` and a non-nil error; the caller is expected to handle the fallback explicitly (log and continue with the inline template)

---

### Requirement: YAML front-matter is parsed via string splitting, no YAML dependency introduced
`internal/assets` SHALL extract `name` and `description` keys from a leading `---\n...\n---` front-matter block by splitting on lines and matching `key: value` for the two known keys. The front-matter block is excluded from `Body`. A missing or malformed front-matter block is non-fatal — the asset is returned with empty `Name`/`Description` and `Body` containing the full file content. No YAML library is added as a dependency.

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

### Requirement: Prompt-asset directory structure is `assets/prompts/` for embedded defaults and `$TR_HOME/prompts/` for overrides
The embedded defaults SHALL live at `assets/prompts/<name>.md` in the repository, accessible to `//go:embed` from the `internal/assets` package. The runtime override directory SHALL be `$TR_HOME/prompts/<name>.md` when `TR_HOME` is set (honoring the existing `TR_HOME` test-isolation convention from the `tr-home-override` capability). Override files MUST be plain markdown; no subdirectory layout is enforced beyond the `prompts/` subdir.

#### Scenario: Embedded asset path resolves at build time
- **WHEN** `internal/assets` is compiled
- **THEN** the `//go:embed` directive captures every `.md` file under `assets/prompts/` into the binary; `go test ./internal/assets/...` confirms the bundle contains `question-generation-policy.md`

#### Scenario: Override directory honors `$TR_HOME`
- **WHEN** `TR_HOME=/scratch/.tr` is set and `/scratch/.tr/prompts/question-generation-policy.md` exists
- **THEN** `Load` returns the override from `/scratch/.tr/prompts/...`; the embedded default is not consulted

#### Scenario: Override directory missing but `$TR_HOME` set — falls back to embedded
- **WHEN** `TR_HOME=/scratch/.tr` is set but `/scratch/.tr/prompts/` does not exist
- **THEN** `Load` returns the embedded default with `Source == "embedded"`; no error is logged about the missing override directory (the absence is the common case)

---

### Requirement: Prompt-asset loading is idempotent and cached per process
The first `assets.Load(name)` call SHALL resolve and parse the asset; subsequent calls SHALL return the cached `PromptAsset` value without re-reading disk or re-parsing markdown. Caching MUST be implemented with `sync.Once` per asset name. The cache SHALL live for the lifetime of the process — there is no file-watching eviction; restarting the daemon is the cache invalidation mechanism.

#### Scenario: Repeated calls return the same value instance
- **WHEN** `Load("question-generation-policy")` is called 10 times in sequence
- **THEN** all 10 calls return the same `PromptAsset` value (or pointer-equal cached struct); the disk is read exactly once across the 10 calls

#### Scenario: Override taken at first call is preserved even if file is later removed
- **WHEN** the first `Load` call picks up an override at `$TR_HOME/prompts/`, and the override file is deleted before a subsequent `Load` call
- **THEN** the subsequent `Load` call still returns the cached override value; no re-stat is performed (restart to pick up disk changes)