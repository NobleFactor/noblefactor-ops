---
title: "Star Application Restructure"
issue: TBD
status: complete
created: 2026-03-19
updated: 2026-03-19
---

# Plan: Star Application Restructure

## Summary

The `internal/starlark/` package conflates two concerns: a `Runtime` type that
is really the star CLI application, and provider definitions that belong under
a `provider/` subtree. This plan restructures noblefactor-ops so that:

1. The application type (`Application`) lives in `cmd/star/` where it belongs.
2. Providers move from `internal/provider/` to `internal/starlark/provider/`.
3. Code generation gets a proper `make generate` target following devlore-cli's
   Makefile pattern.
4. Provider registration is auto-generated (not hand-written).
5. The application uses `op.Receivers()` discovery instead of enumerating receivers.

## Goals

1. **Right names in right places**: `cmd/star/` owns the `Application` type.
   `internal/starlark/` holds Starlark integration code (WASM receiver,
   provider registration). `internal/starlark/provider/` holds provider
   definitions.
2. **Generated registration**: `make generate-register` produces `register.go`
   with blank imports, same as devlore-cli. No hand-maintained import lists.
3. **Codegen in Makefile**: `make generate` runs the code generator for all
   providers. `make build` and `make test` depend on it.
4. **Provider discovery**: `NewApplication()` uses `op.Receivers()` to discover
   announced receivers instead of explicitly importing and passing each gen
   package's `Receiver` var.

## Current State

| Component             | Location                                 | Problem                                                                          |
| --------------------- | ---------------------------------------- | -------------------------------------------------------------------------------- |
| `Runtime` type        | `internal/starlark/runtime.go`           | Application code in a library package; name says "runtime" but it's the star app |
| `Command`, `Flag`     | `internal/starlark/command.go`           | Application types in a library package                                           |
| Provider definitions  | `internal/provider/**`                   | Disconnected from `internal/starlark/`                                           |
| Provider registration | `runtime.go` imports 15 gen packages     | Hand-maintained, defeats self-registration                                       |
| Code generation       | `scripts/check-codegen.sh` (verify only) | No `make generate` target to run codegen                                         |
| `wasm_receiver.go`    | `internal/starlark/`                     | Correct location                                                                 |

## Implementation Phases

### Phase 1: Add `make generate` target

Follow devlore-cli's Makefile pattern: per-provider grouped targets with `&:`,
`GEN_PROVIDERS` list, `generate-register` to auto-generate blank imports,
`generate` as the top-level target. `build` and `test` depend on `generate`.

The `STAR` variable points to the star binary (same repo, `bin/star`). The
`star` prerequisite target builds it.

`generate-register` produces `internal/starlark/provider/register.go` with
blank imports for the devlore-cli aggregate provider package
(`_ "github.com/NobleFactor/devlore-cli/pkg/op/provider"`) and each local
`gen/` directory under `internal/starlark/provider/`. The application imports
only `_ ".../internal/starlark/provider"` — one import triggers everything.

- [x] Add `STAR` variable and `star` order-only prerequisite target
- [x] Add `P := internal/starlark/provider` variable
- [x] Add per-provider grouped targets for all 6 local providers
- [x] Add `GEN_PROVIDERS` list
- [x] Add `generate-register` target
- [x] Add `generate` target depending on `generate-register`
- [x] Make `build` depend on `generate`
- [x] Make `test` depend on `generate`
- [x] Add `generate` and `generate-register` to `.PHONY`
- [x] Update `clean` to remove `register.go` and `gen/` directories
- [x] Update `help` target

**Files**:

| File       | Action |
| ---------- | ------ |
| `Makefile` | Modify |

### Phase 2: Move providers to `internal/starlark/provider/`

Move `internal/provider/**` to `internal/starlark/provider/**` using `git mv`.
Update import paths in all non-generated source files. Generated files will be
regenerated in Phase 3.

Import path changes:

- `github.com/NobleFactor/noblefactor-ops/internal/provider/<name>` becomes
  `github.com/NobleFactor/noblefactor-ops/internal/starlark/provider/<name>`

Files that import provider packages (non-generated):

- `internal/starlark/runtime.go` — imports `commandsprov`
- `cmd/star/main.go` — currently no direct provider imports (accesses via runtime)

The gen/ directories move with their parents but their contents are stale after
the move (wrong import paths in generated code). Phase 3 regenerates them.

- [x] `git mv internal/provider internal/starlark/provider`
- [x] Update import paths in `runtime.go` (`commandsprov`)
- [x] Verify non-generated source files compile with new paths

**Files**:

| File                           | Action                                  |
| ------------------------------ | --------------------------------------- |
| `internal/provider/**`         | Move to `internal/starlark/provider/**` |
| `internal/starlark/runtime.go` | Modify: update import path              |

### Phase 3: Regenerate providers

Run `make generate` to regenerate all `gen/` files with correct import paths.
This also produces `internal/starlark/provider/register.go`.

- [x] Run `make generate`
- [x] Verify `register.go` contains correct blank imports
- [x] Verify all gen files have updated import paths
- [x] `make test` passes

### Phase 4: Move application to `cmd/star/`

Move the `Runtime` type to `cmd/star/` and rename to `Application`. Move
`Command` and `Flag` types alongside it — they are application types used only
by `main.go` and the application.

The application code currently accesses unexported methods on `Runtime`
(`loadExtensionCommands`, `loadExtensionCommand`, `loadWasmReceivers`,
`buildPredeclared`, `loadExtensionsFromPaths`). Moving to `cmd/star/` means
these become unexported methods on `Application` in `package main` — no
visibility change.

Tests that reference `Runtime`, `NewRuntime`, `Command` currently live in
`internal/starlark/` (same package). After the move, these tests need to either:

- Move to `cmd/star/` as `package main` tests (for unit tests of Application)
- Stay in `internal/starlark/` as integration tests that import `cmd/star/`
  (not possible — `cmd/` packages can't be imported)

The practical answer: tests that construct `NewRuntime()` / `NewApplication()`
move to `cmd/star/`. Tests that only exercise extension discovery or config
can stay if they don't reference the application type.

Files to move from `internal/starlark/` to `cmd/star/`:

- `runtime.go` → `application.go` (rename `Runtime` → `Application`,
  `NewRuntime` → `NewApplication`)
- `command.go` → `command.go`
- `runtime_test.go` → `application_test.go`
- `command_test.go` → `command_test.go`
- `lint_integration_test.go` → `lint_integration_test.go`
- `lint_copyright_test.go` → `lint_copyright_test.go`
- `config_integration_test.go` → `config_integration_test.go`

- [x] `git mv` source files from `internal/starlark/` to `cmd/star/`
- [x] Rename `Runtime` → `Application` in all moved files
- [x] Rename `NewRuntime` → `NewApplication`
- [x] Update `main.go`: remove import alias, use `Application` directly
- [x] Update test functions: `TestRuntime_*` → `TestApplication_*`
- [x] Update test helpers: `setupLintRuntime` → `setupLintApplication`
- [x] Verify `make test` passes

**Files**:

| File                                           | Action                                          |
| ---------------------------------------------- | ----------------------------------------------- |
| `internal/starlark/runtime.go`                 | Move to `cmd/star/application.go`               |
| `internal/starlark/command.go`                 | Move to `cmd/star/command.go`                   |
| `internal/starlark/runtime_test.go`            | Move to `cmd/star/application_test.go`          |
| `internal/starlark/command_test.go`            | Move to `cmd/star/command_test.go`              |
| `internal/starlark/lint_integration_test.go`   | Move to `cmd/star/`                             |
| `internal/starlark/lint_copyright_test.go`     | Move to `cmd/star/`                             |
| `internal/starlark/config_integration_test.go` | Move to `cmd/star/`                             |
| `cmd/star/main.go`                             | Modify: remove starlark import, use local types |

### Phase 5: Fix provider discovery in Application

Remove all explicit gen imports from `application.go`. The application adds
one blank import for provider registration:

- `_ "github.com/NobleFactor/noblefactor-ops/internal/starlark/provider"` —
  triggers generated `register.go`, which imports both devlore-cli providers
  (via `_ "github.com/NobleFactor/devlore-cli/pkg/op/provider"`) and all
  local providers

`NewApplication()` uses `op.Receivers()...` in `WithReceivers()` to discover
all announced providers.

- [x] Add `_ "github.com/NobleFactor/noblefactor-ops/internal/starlark/provider"` import
- [x] Remove all gen package imports from application.go
- [x] Replace `WithReceivers(filegen.Receiver, jsongen.Receiver, ...)` with
      `WithReceivers(op.Receivers()...)`
- [x] Verify `make test` passes

**Files**:

| File                      | Action |
| ------------------------- | ------ |
| `cmd/star/application.go` | Modify |

### Phase 6: Clean up

Verify `internal/starlark/` contains only:

- `provider/` — provider definitions + generated code + `register.go`
- `wasm_receiver.go` — WASM-to-Starlark bridge
- `wasm_receiver_test.go` — tests

Delete anything else that remains.

- [x] Verify `internal/starlark/` contents
- [x] Delete any remaining orphaned files
- [x] `make build` succeeds
- [x] `make test` passes
- [x] Grep for `internal/provider/` (old path) — zero matches in `.go` files

## Verification

1. `make generate` succeeds and produces correct gen files
2. `make build` succeeds
3. `make test` passes
4. `register.go` is generated (not hand-written)
5. No file in `cmd/star/` imports a `gen` package directly
6. `internal/starlark/` contains only `provider/`, `wasm_receiver.go`,
   `wasm_receiver_test.go`
7. Grep for `internal/provider/` (old import path) — zero matches
8. Grep for `NewRuntime` — zero matches (now `NewApplication`)
9. Grep for `type Runtime struct` — zero matches (now `Application`)

## Related Documents

- [star-consumes-pkg-op.md](./star-consumes-pkg-op.md) — parent migration plan
- devlore-cli `Makefile` — template for code generation targets
- devlore-cli `pkg/op/provider/register.go` — template for generated
  registration file
