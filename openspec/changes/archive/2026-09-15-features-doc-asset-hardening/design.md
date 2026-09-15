## Context

The drop-in prompt-asset feature (override slot at `<data-dir>/prompts/<name>.md`) shipped with command-level observability but zero user-facing narrative: the README section is a command reference, the review-thread security analysis never left the conversation, and `tr asset` renders only Cobra's default one-line help. At the same time the README carries the conversion burden alone, and feature depth works against that job.

## Goals / Non-Goals

**Goals:**
- Give the end user a two-minute, delightful `FEATURES.md`: what the tool does, how to bend it to their domain, where it is going.
- Keep the README install-first with a one-breath teaser that earns the click to FEATURES.md.
- Make `tr asset` teach itself in the terminal (Long help), per the terminal-first convention.
- Close the review-thread hardening nit: validate `<name>` args before path construction.

**Non-Goals:**
- A hosted policy-sharing platform (concept only).
- Rewriting `DOCS/` architecture material.
- Any runtime behavior change beyond name validation and help text (asset loading, drift warning, MCP are `prompt-asset-observability`'s).

## Decisions

### Decision: Docs IA — README converts, FEATURES.md explains

The README keeps Setup + Configuration + Philosophy/Problem Statement and slims the prompt-asset section to a 3-4 sentence concept hook plus a teaser link. `FEATURES.md` (repo root, like CONFIG.md/DATA.md reference style) is structured as:

```
FEATURES.md
+----------------------------------------------+
| # Features                                   |
| ## What you get today                        |
|   - Commit-time recall quizzes (the core)    |
|   - Delivery surfaces: terminal TUI + MCP    |
|   - Drop-in prompt customization  <-- detail |
|       (override loop, sync/reset, drift      |
|        warning, restart caveat)              |
|   - Override observability (config --show)   |
| ## Coming soon                               |
|   - Adaptive difficulty (in development)     |
| ## Community                                 |
|   - Share your policy doc: concept + the     |
|     "policy files are prompts — review what  |
|     you import" trust note                   |
+----------------------------------------------+
```

**Alternatives considered:**
- *Keep all feature detail in the README.* Rejected: directly conflicts with the conversion goal; the README already carries install + config + philosophy + research.
- *A `DOCS/GUIDES/` tree.* Rejected: one feature does not justify a tree; root-level single file matches the existing CONFIG.md/DATA.md pattern and is discoverable in the repo listing.

### Decision: README teaser placement and shape

The teaser lives where the prompt-asset section currently sits (post-setup, pre-Configuration) — a reader who just finished setup is exactly the audience asking "what else can this do?". Shape: concept hook (quizzes are shaped by a markdown policy doc shipped in the binary; drop in your own — no recompile), one line that `tr asset list|sync|reset` manage it, then the FEATURES.md link framed as "the full tour". No duplication of drift-warning detail in the README — that moves to FEATURES.md.

**Alternatives considered:**
- *Teaser at the very top of the README.* Rejected: competes with the tagline and pushes install instructions below the fold; conversion wants install reachable in one scroll.
- *Keep the full current section AND link FEATURES.md.* Rejected: duplication goes stale and keeps the README heavy — the exact failure this change fixes.

### Decision: Name validation is an allowlist regex at the CLI boundary

`validateAssetName(name string) error` accepts `^[a-z0-9-]+$` and is called by `reset` and `sync` before any path construction. Rationale: shipped asset names are exactly lowercase-hyphenated; an allowlist can't be outsmarted by traversal encodings, and the error message can teach the expected form. The batch (no-name) reset path is untouched — it enumerates the directory. Failure exits 1 via the existing RunE error path (Cobra prints it; consistent with the other refusal messages).

**Alternatives considered:**
- *Reject only `path/filepath` separators and `..`.* Rejected: denylists age badly; a dotfile name (`..policy`) or reserved-OS name would still slip through to weird places.
- *Validate in the `assets` package instead.* Rejected: assets is a library consumed by the daemon and `cmd/promptgen`, where names come from the embedded FS, not users; the boundary that needs the check is the CLI argument.

### Decision: Long help carries the loop, not the marketing

`tr asset` `Long` help covers: what an override is, when `list` vs `reset` vs `sync` applies, the restart caveat, and where the override lives. No mission-statement prose — the terminal is for doing.

## Risks / Trade-offs

- **[Trade-off] FEATURES.md can drift from shipped behavior.** Mitigated: it documents only shipped surface + one explicitly-marked "Coming soon" item; the doc-sync task pattern from prior changes applies (update FEATURES.md in the same change that ships a feature).
- **[Risk] Two unarchived changes now modify `cli-asset-commands`.** This change's ADDED requirements are order-robust against `prompt-asset-observability`'s archive (ADDED merges cleanly whether the canonical spec exists yet or not), but the *headers* are new — no merge-by-intent reconciliation needed at either archive order.

## Migration Plan

1. Write `FEATURES.md`; slim the README section + add the teaser link.
2. Add `validateAssetName` + wire into reset/sync; add `Long` help to `assetCmd`.
3. Table tests for validation; help-text assertion; full build/vet/test.
4. Docs and code land in the same change so the teaser never points at a doc that doesn't exist.

## Open Questions

- Should the README "Managing prompt-asset overrides" heading survive (as the slim section's title) or be folded under a broader "Customizing" heading when FEATURES.md exists? Leaning toward keeping the heading — anchors the `tr asset` commands next to the teaser. Decide at implementation.
