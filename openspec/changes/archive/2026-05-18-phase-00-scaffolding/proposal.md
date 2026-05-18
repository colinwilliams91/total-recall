# Phase 00: Go Project Scaffolding

## Problem

The Total Recall repository is entirely documentation and OpenSpec artifacts. Nothing compiles. Before any implementation work can begin, the project needs a buildable Go foundation: a module, a package structure that mirrors the documented architecture, a CLI entry point, and build infrastructure.

## What This Change Delivers

A clean, compilable Go project that:

- Has the correct module path and binary name
- Mirrors the documented subsystem architecture as internal packages
- Has a working CLI entry point with stubbed subcommands
- Has hook template files ready to be installed by `tr init`
- Has a `Makefile` for common dev tasks
- Builds cleanly on both Windows and Unix (`go build ./...` with zero CGO)
- Has the right initial dependencies declared in `go.mod`

Phase 00 delivers **structure, not behavior**. No subsystem logic is implemented here. Every internal package contains only a `doc.go` with a package declaration and a description comment. The CLI entry point stubs every subcommand with a placeholder. The goal is a foundation that compiles, runs `go test ./...` without errors, and is immediately ready for Phase 01 implementation work.

## What This Change Does Not Deliver

- Any runtime behavior (daemon, hooks, MCP server, recall engine)
- Any config loading logic
- Any SQLite schema or queries
- Any AI provider integration
- Tests (there is nothing to test yet)

## Why Now

All architectural decisions needed to lay out the package structure have been made:

- Module path, binary name, and IPC transport are resolved
- The two-tier config architecture is fully specified (`config-architecture` change)
- The subsystem boundaries are documented in `DOCS/ARCHITECTURE/`
- The daemon's HTTP server model is documented in `DOCS/ARCHITECTURE/DAEMON/INDEX.md`

Proceeding without scaffolding forces all implementation work to also carry the overhead of deciding where things live. Phase 00 removes that overhead for every subsequent phase.
