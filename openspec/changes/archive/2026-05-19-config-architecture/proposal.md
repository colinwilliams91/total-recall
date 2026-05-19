## Why

The current config model puts all settings in a single per-repo `.tr.yaml`, which conflates personal preferences with project-level settings. This creates two concrete problems: API keys (BYOK) are duplicated across every repo and risk accidental commits, and privacy decisions — specifically whether to allow MCP conversation content to feed the concept cache — are personal stances that shouldn't need to be set per project. Promoting a user-level config to Phase 1 resolves both.

## What Changes

- Introduce `~/.tr/config.yaml` as a first-class Phase 1 artifact (removed from stretch goal status)
- Move `ai` config block (provider, model, api-key) to user-level — API credentials are personal, not project-specific
- Move `recall` defaults (difficulty, max_questions) to user-level — personal learning preferences
- Add new `privacy` block at user-level with `conversation_analysis: false` (opt-in gate for MCP conversation signal capture)
- Define a two-tier override chain: user-level sets defaults, per-repo `.tr.yaml` overrides where needed
- `.tr.yaml` retains project-specific settings: `hooks`, `mode`, `presentation`

## Capabilities

### New Capabilities

- `user-level-config`: The `~/.tr/config.yaml` schema, location, defaults, and Core Engine loading lifecycle
- `config-override-chain`: Precedence model defining how user-level defaults are merged with per-repo overrides
- `mcp-conversation-analysis`: Privacy opt-in gate controlling whether MCP conversation content (user questions + agent explanations) is processed as input signals by the Incremental Analysis Pipeline

### Modified Capabilities

<!-- No existing specs to delta against — specs/ is currently empty -->

## Impact

- `README.md` config examples need updating to reflect the two-tier split
- Core Go Engine must load and deep-merge both config files at startup, with per-repo values winning on conflict
- MCP Event Monitor must check `privacy.conversation_analysis` before processing conversation content as a pipeline signal — code context correlation (Signal 3) remains unaffected by this flag since it is already captured by the Filesystem and Git Index watchers
- `.tr.yaml` schema narrows: `ai` and `recall` blocks become optional overrides rather than required configuration
