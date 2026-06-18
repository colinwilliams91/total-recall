# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-06-17

Initial release of Total Recall — a CLI tool that preserves engineering cognition in the age of AI-assisted coding.

### Features

- **CLI**: Cobra-based command structure with `serve`, `init`, `config`, `status`, and `ask` subcommands
- **Configuration**: Two-tier config system — user-level (`~/.tr/config.yaml`) deep-merged with per-repo (`.tr.yaml`); `config --show` with source annotations; `--quiet` flag for CI/CD
- **AI Providers**: Multi-provider support (Anthropic, OpenAI, Groq, Ollama, LM Studio, Custom) with BYOK-first design using `env:VAR_NAME` pattern for secure key management
- **Daemon**: HTTP server on `localhost:7331` with health endpoint; hook routes for `pre-commit`, `commit-msg`, and `pre-push`
- **Hooks**: Git hook installation via `total-recall init` with sentinel chaining, idempotent re-runs, and graceful degradation when daemon is not running; `.sh` and `.bat` variants
- **Concept Extraction**: Pipeline that analyzes staged diffs (8 KB guard) and extracts concepts via AI provider
- **SQLite Cache**: Pure-Go SQLite concept store (`~/.tr/concepts.db`) via `modernc.org/sqlite` — no CGo required
- **Recall Engine**: Question synthesis from cached concepts with Fisher-Yates answer shuffling
- **MCP Server**: Mounted at `/mcp/` — AI coding agents receive questions via `recall_next` tool, subscribe to `recall://queue` resource, and follow `recall_workflow` prompt
- **REST API**: `GET /recall/next` (atomic dequeue) and `POST /recall/answer` (answer/skip recording)
- **TUI**: Bubbletea-based multiple-choice question handler with 30-second timeout, TTY-aware (silent in CI/CD)
- **Post-commit Hook**: Surfaces recall questions after each successful commit via `total-recall ask`
- **Memory Store**: Unified SQLite backing store (`~/.tr/memory.db`) with `questions` table and exactly-once atomic dequeue

### Bug Fixes

- TUI exit hang when BubbleTea process doesn't return to CWD
- Cross-platform argument passing for hook scripts (multiline string fix)
- Post-commit hook now uses `os.Executable()` for PATH-agnostic binary resolution
- Unresolvable environment variable warnings on server start
- Answer choice randomization (was always correct at index 0)
- Hook sentinel comment consistency across all hook writers

### Testing

- End-to-end testing across all phases (config, daemon, hooks, intelligence, delivery)
- Daemon adapter and `ask` subcommand tests
- Question queue dequeue behavior tests

### Documentation

- README with elevator pitch, research examples, philosophy, and roadmap
- Architecture and delivery documentation
- OpenSpec-driven design documents for all phases
- Contributing guide with build, run, and test instructions
