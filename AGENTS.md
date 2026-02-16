# AGENTS.md

Guidance for coding agents working in this repository.
This file defines build/test commands, style conventions, and safety rules.

## Project Summary

- Name: `switcher`
- Language: Go
- Primary goal: install and switch Go toolchains
- Supported OS: macOS and Linux
- Scopes: `global` and `local` via `.switcher-version`
- Managed home: `~/.switcher`

## Repository Structure

- `cmd/switcher/main.go` executable entrypoint
- `internal/app` CLI wiring and orchestration
- `internal/switcher` paths, config, scope resolution, shims
- `internal/releases` official Go release metadata client
- `internal/install` archive download/checksum/extract logic
- `internal/tools` companion tool version mapping and install
- `internal/tui` Charm/Bubble Tea terminal UI
- `internal/versionutil` Go and dotted version comparison helpers
- `internal/progress` progress events and transfer formatting
- `scripts/install.sh` no-Go bootstrap installer

If the codebase changes, update this file to match reality.

## Required Tools

- Go version declared in `go.mod` (or newer compatible toolchain)
- `make`
- `golangci-lint` (for linting)
- Optional: `goimports`

## Build and Run

- Install from release binary (no local Go required): `./scripts/install.sh`
- Build binary: `make build`
- Install binary for terminal use: `make install`
- Bootstrap install from repo clone: `make bootstrap`
- Build output: `./bin/switcher`
- Run binary: `make run`
- Run from source: `go run ./cmd/switcher`
- Show command help: `./bin/switcher help`

Source build needs Go 1.23+ (Charm dependencies). `make` uses `GOTOOLCHAIN=auto`.

## Test Commands

- Full test suite: `make test`
- Full test suite (direct): `go test ./...`
- One package: `go test ./internal/switcher -v`
- One test: `go test ./internal/switcher -run '^TestResolveActiveVersion_LocalTakesPrecedence$' -v`
- One subtest: `go test ./internal/switcher -run '^TestResolveActiveVersion$/^local_takes_precedence$' -v`
- Make single-test helper: `make test-one PKG=./internal/switcher TEST='^TestResolveActiveVersion_LocalTakesPrecedence$'`
- Race detector: `go test ./... -race`
- Coverage profile: `go test ./... -coverprofile=coverage.out`

## Lint and Formatting

- Format all code: `make fmt`
- Direct formatting: `gofmt -w ./cmd ./internal`
- Imports (preferred when available): `goimports -w ./cmd ./internal`
- Lint: `make lint`
- Direct lint command: `golangci-lint run ./...`

## Day-to-Day Agent Workflow

1. Keep changes small and focused.
2. Run `make fmt` after edits.
3. Run targeted tests for touched packages.
4. Run `make test` before handoff for larger changes.
5. Run `make lint` when possible.

## Product Behavior That Must Stay True

- Local scope file is `.switcher-version`.
- Global scope config is `~/.switcher/config.json`.
- Resolution order is strict: local override, then global.
- Shims are expected in `~/.switcher/bin`.
- `go`, `gofmt`, and `golangci-lint` should resolve through switcher-managed binaries.

## Go Code Style

### Imports

- Use standard groups: stdlib, third-party, internal.
- Keep import blocks gofmt/goimports compatible.
- Avoid dot imports.
- Use aliases only for collision avoidance or clarity.

### Formatting

- Always keep files `gofmt` clean.
- Prefer early returns over deeply nested branches.
- Keep functions focused and reasonably small.

### Types and APIs

- Start with concrete types; introduce interfaces only when needed.
- Place interfaces near consumers.
- Keep exported API surface minimal.
- Use typed config structs, not loose `map[string]any` configs.

### Naming

- Package names are short, lower-case, no underscores.
- Exported names use `CamelCase`; unexported use `camelCase`.
- Sentinel errors use `Err...` naming.
- Avoid stutter (for example `switcher.Config`, not `switcher.SwitcherConfig`).

### Error Handling

- Return errors instead of panicking in normal control flow.
- Wrap context: `fmt.Errorf("...: %w", err)`.
- Match typed/sentinel errors with `errors.Is` / `errors.As`.
- Error strings should be lower-case and punctuation-free.

### Context, I/O, and Concurrency

- Pass `context.Context` to network and long-running operations.
- Respect cancellation for downloads and command execution.
- Use atomic writes for config and shim files.
- Validate paths before filesystem mutations.

### CLI and TUI Output

- Keep user output concise and actionable.
- Use stderr for diagnostics and failures.
- Avoid noisy logs in lower-level packages.

### Testing

- Prefer table-driven tests.
- Cover precedence logic, path resolution, and invalid input.
- Cover install edge cases: missing archives, checksum mismatch, unsupported platforms.
- Use temp directories; avoid external network in unit tests.

## Dependency Policy

- Keep dependencies minimal and justified.
- Charm stack is approved: `bubbletea`, `bubbles`, `lipgloss`.
- Avoid adding heavy CLI frameworks unless clearly necessary.

## Agent Safety Rules

- Never run destructive git commands unless explicitly asked.
- Never revert unrelated user changes.
- Never commit secrets or credentials.
- Prefer deterministic behavior and explicit errors.

## Cursor and Copilot Rules

Checked while writing this file:

- `.cursorrules` does not exist.
- `.cursor/rules/` does not exist.
- `.github/copilot-instructions.md` does not exist.

If any of these files appear later, treat them as higher-priority instructions and update this AGENTS.md.

## Maintenance Note

When you change commands, scope behavior, or package layout, update this file in the same PR.
