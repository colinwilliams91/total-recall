## 1. Config Schema & Loading

- [x] 1.1 Define `~/.tr/config.yaml` schema (privacy, ai, recall blocks) in Go structs
- [x] 1.2 Implement user-level config loader that reads from `~/.tr/config.yaml`
- [x] 1.3 Implement deep-merge logic: user-level defaults merged with per-repo `.tr.yaml` overrides
- [x] 1.4 Enforce privacy key isolation — log a warning and discard any `privacy` or `ai` block found in `.tr.yaml`
- [x] 1.5 Resolve `ai.api-key` values using `env:<VAR_NAME>` pattern; emit warning if raw key string detected

## 2. Init & Bootstrap

- [x] 2.1 Update `tr init` to create `~/.tr/config.yaml` with safe defaults if not already present
- [x] 2.2 Add plain-language conversation analysis opt-in prompt to `tr init` (default: N); write result to `privacy.conversation_analysis`
- [x] 2.3 Ensure `tr init` does not overwrite an existing user config; notify user if already exists
- [x] 2.4 Implement auto-creation fallback: if Core Engine starts (daemon or hook) and `~/.tr/config.yaml` is absent, create it with safe defaults and emit a single advisory message
- [x] 2.5 Ensure auto-created config is never overwritten on subsequent runs
- [x] 2.6 Add `--quiet` global persistent flag to Cobra root; suppress advisory messages (non-errors) when set

## 3. Config Inspection CLI

- [x] 3.1 Implement `tr config --show` command that prints the fully resolved (merged) config
- [x] 3.2 Annotate each key in the output with its source (`user` or `repo`)

## 4. MCP Conversation Analysis Gate

- [x] 4.1 Add `conversation_analysis` check to the MCP Event Monitor before forwarding conversation signals to the Incremental Analysis Pipeline
- [x] 4.2 Ensure Signal 3 (code context correlation) bypasses the flag and always runs
- [x] 4.3 Implement extract-and-discard: pipeline writes only concept fingerprints to the cache, never raw conversation text
- [x] 4.4 Confirm no one-time acknowledgment logic is introduced; opt-in is communicated entirely at `tr init`

## 5. Documentation & Terminal Copy

- [x] 5.1 Write the exact terminal copy for the `tr init` conversation analysis prompt — plain language, earnest tone, no legal jargon
- [x] 5.2 Write the exact advisory message shown on auto-creation bypass
- [x] 5.3 Document both config files (schema, hierarchy, examples) in `DOCS/ARCHITECTURE/` — make the two-tier model and override chain explicit
- [x] 5.4 Update `README.md` config examples to show both `~/.tr/config.yaml` and `.tr.yaml` with clear annotations
- [x] 5.5 Update `DOCS/ARCHITECTURE/INDEX.md` to document the config loading and merge lifecycle
- [x] 5.6 Remove `~/.tr/config.yaml` from stretch goal status in `ROADMAP.md`; mark as Phase 1
