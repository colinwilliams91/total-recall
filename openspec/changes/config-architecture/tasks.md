## 1. Config Schema & Loading

- [ ] 1.1 Define `~/.tr/config.yaml` schema (privacy, ai, recall blocks) in Go structs
- [ ] 1.2 Implement user-level config loader that reads from `~/.tr/config.yaml`
- [ ] 1.3 Implement deep-merge logic: user-level defaults merged with per-repo `.tr.yaml` overrides
- [ ] 1.4 Enforce privacy key isolation — log a warning and discard any `privacy` or `ai` block found in `.tr.yaml`
- [ ] 1.5 Resolve `ai.api-key` values using `env:<VAR_NAME>` pattern; emit warning if raw key string detected

## 2. Init & Bootstrap

- [ ] 2.1 Update `tr init` to create `~/.tr/config.yaml` with safe defaults if not already present
- [ ] 2.2 Add plain-language conversation analysis opt-in prompt to `tr init` (default: N); write result to `privacy.conversation_analysis`
- [ ] 2.3 Ensure `tr init` does not overwrite an existing user config; notify user if already exists
- [ ] 2.4 Implement auto-creation fallback: if Core Engine starts (daemon or hook) and `~/.tr/config.yaml` is absent, create it with safe defaults and emit a single advisory message
- [ ] 2.5 Ensure auto-created config is never overwritten on subsequent runs

## 3. Config Inspection CLI

- [ ] 3.1 Implement `tr config --show` command that prints the fully resolved (merged) config
- [ ] 3.2 Annotate each key in the output with its source (`user` or `repo`)

## 4. MCP Conversation Analysis Gate

- [ ] 4.1 Add `conversation_analysis` check to the MCP Event Monitor before forwarding conversation signals to the Incremental Analysis Pipeline
- [ ] 4.2 Ensure Signal 3 (code context correlation) bypasses the flag and always runs
- [ ] 4.3 Implement extract-and-discard: pipeline writes only concept fingerprints to the cache, never raw conversation text
- [ ] 4.4 Remove one-time acknowledgment on opt-in (handled at init instead)

## 5. Documentation & Terminal Copy

- [ ] 5.1 Write the exact terminal copy for the `tr init` conversation analysis prompt — plain language, earnest tone, no legal jargon
- [ ] 5.2 Write the exact advisory message shown on auto-creation bypass
- [ ] 5.3 Document both config files (schema, hierarchy, examples) in `DOCS/ARCHITECTURE/` — make the two-tier model and override chain explicit
- [ ] 5.4 Update `README.md` config examples to show both `~/.tr/config.yaml` and `.tr.yaml` with clear annotations
- [ ] 5.5 Update `DOCS/ARCHITECTURE/INDEX.md` to document the config loading and merge lifecycle
- [ ] 5.6 Remove `~/.tr/config.yaml` from stretch goal status in `ROADMAP.md`; mark as Phase 1
