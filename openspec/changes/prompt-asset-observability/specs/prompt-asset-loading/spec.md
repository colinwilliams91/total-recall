## MODIFIED Requirements

### Requirement: `assets.Load` emits a structured startup log line for every resolved override
When `assets.Load` resolves to an override file in the data dir's `prompts/` directory (the Total Recall data dir is `$TR_HOME` when set, else `~/.tr`), the package SHALL emit a log line including the asset name, the resolved override path, the override file's mtime (RFC3339), and the embedded default's compile-time reference. The log line format SHALL be: `[assets] prompt asset %q loaded from override at %s (mtime=%s, embedded ref=%s)`. When no override file exists for the asset (the embedded path is used), no enriched log line is emitted — the embedded path is silent (the absence of an override is the common case and should not be noisy).

#### Scenario: Override log line contains mtime and embedded reference
- **WHEN** `Load("question-generation-policy")` resolves to an override file for that asset in the data dir's `prompts/` directory
- **THEN** the log line contains the absolute override path, the override's mtime in RFC3339 format, and the embedded-reference marker

#### Scenario: Embedded path is silent
- **WHEN** `Load("question-generation-policy")` resolves to the embedded default (no override file exists)
- **THEN** no override log line is emitted

---

### Requirement: `assets.Load` resolves overrides from the data dir without requiring `TR_HOME` to be set
The override slot SHALL live at `<data-dir>/prompts/<name>.md`, where the data dir is `$TR_HOME` when set to a non-empty value and `~/.tr` otherwise — the same resolution rule as `config.UserConfigDir` and `cache.trDir`. When the data dir cannot be resolved at all (no home directory available), `Load` SHALL fall through to the embedded default rather than failing. The override mechanism SHALL NOT be gated on `TR_HOME` being set: a default install that never defines `TR_HOME` still reads `~/.tr/prompts/`.

#### Scenario: Override in the default data dir
- **WHEN** `TR_HOME` is unset and `~/.tr/prompts/question-generation-policy.md` exists
- **THEN** `Load("question-generation-policy")` returns `Source == "$TR_HOME"` with the absolute `~/.tr/prompts/...` path

#### Scenario: Unresolvable data dir falls back to embedded
- **WHEN** neither `$TR_HOME` nor a home directory can be resolved
- **THEN** `Load` returns the embedded default (or the `fallback` sentinel when the asset is unknown), not an error

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

### Requirement: `tr config show` includes a "prompt assets" section listing resolved sources and ages
`internal/config` `Show()` SHALL, after rendering the existing config sections, query `assets.Load` for every known embedded prompt name (currently only `question-generation-policy`; future assets follow the same pattern) and print one line per resolved asset under a `prompt assets:` section header. Each line SHALL be: `  <name>: <resolved-path>  # [<source-tag>], <age>`. The `<source-tag>` is one of `embedded`, `override`, or `fallback`; the `<age>` is a human-readable mtime-relative string (e.g., `embedded` for the no-override case, `6mo old` for an override). The section lives at the end of the `Show()` output.

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