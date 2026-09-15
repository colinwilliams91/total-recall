## Why

The drop-in prompt-asset feature shipped with observability (`tr asset list|reset|sync`, drift warning) but no user-facing *explanation*: the README section is a command reference that never says what a prompt asset is or why a user would care. Meanwhile the README's job is conversion — getting a visitor to install and run the tool — and every extra paragraph of feature detail on that page works against it. Two problems, one fix: a dedicated, concise `FEATURES.md` that explains what the tool does and where it is going, plus a clean README that teases it.

Secondarily, review surfaced two small hardening/documentation items that belong with this pass: `<name>` arguments on `tr asset reset|sync` are not validated (a path-traversal-shaped argument is contained only by accident), and `tr asset` has no `Long` help text of its own.

## What Changes

- **New: `FEATURES.md`** (repo root) — the concise, end-user features document. Shipped features (commit-time quizzes, terminal + MCP delivery, drop-in prompt customization, override observability), the community-sharing concept with a "policy files are prompts — review what you import" trust note, and adaptive difficulty under a "Coming soon" heading. Short enough to read in two minutes; the delight criterion is that reading it never feels like a prerequisite to using the tool.
- **Modified: `README.md`** — the current "Managing prompt-asset overrides" section slims down to a 3-4 sentence concept hook (quizzes are shaped by a markdown policy doc shipped in the binary; drop in your own; observability exists) plus a conversion teaser linking `FEATURES.md` for the full picture. README stays install-first; feature depth moves out.
- **Modified: `cmd/tr/asset.go`** — add `Long` help text to the `asset` command so `tr asset --help` / `tr help asset` self-document the override loop (terminal-first convention: the tool teaches itself).
- **Modified: `cmd/tr/asset.go` (hardening)** — `reset`/`sync` `<name>` arguments are validated against `^[a-z0-9-]+$` before path construction; invalid names exit 1 with a clear message instead of building traversal-shaped paths that only the embedded-name check accidentally contains (for `sync`) or silently operate outside the prompts dir (for `reset`).
- **Tests** — table tests for name validation (valid canonical name, invalid traversal `../x`, invalid characters, empty); help-text regression assertion in `main_test.go` style.

## Capabilities

### New Capabilities

- (none — no new runtime behavior; documentation is captured in tasks, not specs)

### Modified Capabilities

- `cli-asset-commands`: `reset`/`sync` SHALL reject `<name>` arguments that are not a single lowercase-hyphenated name token (no slashes, dots, whitespace, or non-ASCII), exiting 1 with an actionable message. The `tr asset` command SHALL expose self-documenting long help describing the override loop.

## Impact

- **Code:** `cmd/tr/asset.go` (Long help text; `validateAssetName` used by reset/sync).
- **Tests:** `cmd/tr/asset_test.go` (name-validation table tests); `cmd/tr/main_test.go` or `asset_test.go` (long-help assertion).
- **Docs:** `FEATURES.md` (new); `README.md` (section slim-down + teaser link); `DOCS/CONTRIBUTING.md` unaffected.
- **Specs:** `openspec/changes/features-doc-asset-hardening/specs/cli-asset-commands/spec.md` (MODIFIED requirements only).
- **Dependencies:** none new. **BREAKING:** none — name validation only narrows arguments that previously produced accidental behavior (traversal-shaped names), not any documented usage.

## Key Design Decisions

- **Docs IA: README converts, FEATURES.md explains.** The root README keeps install + a one-breath teaser; FEATURES.md owns the feature depth. A one-feature guide was previously judged premature, but the user conversion goal flips the calculus: FEATURES.md is the landing page for "what will this do for me", seeded now with the two features that exist and the one shipping next.
- **Community sharing is framed as concept, not feature.** The web-platform idea is not committed scope; FEATURES.md describes sharing as "bring a policy, review it before you import" so the trust note lands without promising a platform.
- **Name validation is a allowlist, not a denylist.** `^[a-z0-9-]+$` accepts exactly what the shipped asset naming uses; anything else is rejected up front rather than trying to enumerate traversal patterns.
- **Hardening rides along rather than waiting.** Both nits are one-function changes in a file this change already touches; splitting them into a separate change costs more than they weigh.

## Non-Goals

- A docs website or hosted platform for policy sharing — concept only; the "cheap web platform" remains an idea outside this change.
- Rewriting `DOCS/` (architecture/dev-facing docs stay as they are).
- Any change to asset loading, drift warning, or MCP behavior — `prompt-asset-observability` owns those.
- A `tr asset import <file>` command with diff preview — natural follow-up if community sharing materializes; not built now.
- Spec coverage for help text beyond the one self-documentation requirement (docs themselves are never spec'd).

## Developer Workflow Impact

Zero commit-time impact — no hook, daemon, or pipeline behavior changes. `tr asset reset|sync` reject malformed names with a clear message instead of surprising results; everything else is additive help text and documentation read at leisure, never mid-workflow.
