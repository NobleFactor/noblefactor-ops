# Phase 5 Design Analysis: Blockers and Required Changes

**Date**: 2026-03-12
**Context**: Phase 5 of `shared-provider-receivers` plan attempted and backed out.
All code changes reverted. Both repos pass all tests.

## What Phase 5 Tried to Do

Replace 4 hand-coded Starlark receivers in noblefactor-ops (JSON, YAML, Regexp, UI —
~595 lines) with framework-managed receivers from devlore-cli's `op.StarlarkRuntime`,
and replace `starlarkToGo`/`goToStarlark` with exported `op.UnmarshalToAny`/`op.Marshal`.

## Blocker 1: Gen Packages Not Available to External Consumers

### The problem

The plan assumes noblefactor-ops can blank-import devlore-cli gen packages to trigger
`op.Announce()`:

```go
_ "github.com/NobleFactor/devlore-cli/pkg/op/provider/json/gen"
_ "github.com/NobleFactor/devlore-cli/pkg/op/provider/yaml/gen"
_ "github.com/NobleFactor/devlore-cli/pkg/op/provider/regexp/gen"
_ "github.com/NobleFactor/devlore-cli/pkg/op/provider/ui/gen"
```

These `gen/` directories are `.gitignore`d. They exist only locally after running
code generation. The published Go module does not contain them. `go mod tidy` fails
with "does not contain package" for any gen import.

### Why this matters

The entire `StarlarkRuntime.BuildReceivers()` flow depends on providers being announced
via `init()` functions in gen packages:

```
gen/provider.gen.go → init() → op.Announce(provider)
                                    ↓
op.InitAll() → Register() on each announced provider
                                    ↓
StarlarkRuntime.BuildReceivers() → ImmediateProvider.NewImmediate(cfg)
                                    ↓
gen/immediate.gen.go → NewJsonReceiver(p) → op.WrapReceiver("json", p)
```

Without the gen packages, there are no announced providers, and `BuildReceivers()`
produces an empty dict.

### Possible solutions (require design decision)

1. **Track gen files in git.** Remove `gen/` from `.gitignore` and commit the generated
   files. External consumers can then import them. Downside: generated files in source
   control, risk of stale generated code, merge conflicts on regeneration.

2. **Create a non-generated registration package.** A hand-written package (e.g.,
   `pkg/op/provider/register/register.go`) that blank-imports all gen packages. This
   package itself would need to be tracked in git even though its imports point to
   gitignored packages — so this doesn't solve the root problem.

3. **Provide a non-gen API for provider construction.** Export factory functions or
   constructors (e.g., `json.NewProvider()`, `json.NewReceiver(cfg)`) directly from the
   non-gen provider packages (`pkg/op/provider/json/`, etc.). These packages ARE tracked
   in git. noblefactor-ops would call these directly instead of going through the
   `Announce`/`InitAll`/`BuildReceivers` pipeline. This bypasses the gen layer entirely
   for external consumers.

4. **Export `WrapReceiver` + `MethodParams` without requiring gen imports.** Provide
   enough exported surface that noblefactor-ops can construct receivers by calling
   `op.WrapReceiver("json", jsonProvider)` with a locally-instantiated provider and
   locally-defined params. The params could be published as constants in the non-gen
   provider packages.

## Blocker 2: Unexported External API (`NewReceiver`, `BuiltinFunc`)

### The problem

In `pkg/op/receiver.go` (devlore-cli), two symbols were unexported during Phase 3/4
extraction work:

- `NewReceiver` → `newReceiver` (line 20)
- `BuiltinFunc` → `builtinFunc` (line 53)

### Who consumes them in noblefactor-ops

**`op.NewReceiver` — 13 call sites** (constructors for hand-coded receivers):

| File                     | Line | Usage                                  |
| ------------------------ | ---- | -------------------------------------- |
| `receiver_config.go`     | 23   | `op.NewReceiver("config")`             |
| `receiver_shellcheck.go` | 29   | `op.NewReceiver("shellcheck")`         |
| `receiver_regexp.go`     | 24   | `op.NewReceiver("regexp")`             |
| `receiver_go.go`         | 38   | `op.NewReceiver("go")`                 |
| `receiver_commands.go`   | 28   | `op.NewReceiver("commands")`           |
| `receiver_starlark.go`   | 28   | `op.NewReceiver("starlark_parse")`     |
| `receiver_setup.go`      | 31   | `op.NewReceiver("setup")`              |
| `receiver_json.go`       | 23   | `op.NewReceiver("json")`               |
| `wasm_receiver.go`       | 34   | `op.NewReceiver(name)` (dynamic)       |
| `receiver_yaml.go`       | 24   | `op.NewReceiver("yaml")`               |
| `receiver_ui.go`         | 23   | `op.NewReceiver("ui")`                 |
| `receiver_file.go`       | 30   | `op.NewReceiver("file")`               |
| `receiver_schema.go`     | 26   | `op.NewReceiver("schema")`             |
| `receiver_lint.go`       | 34   | `op.NewReceiver("lint")`               |

**`op.BuiltinFunc` — 1 type reference:**

| File                | Line | Usage                                              |
| ------------------- | ---- | -------------------------------------------------- |
| `wasm_receiver.go`  | 58   | `func (r *WasmReceiver) makeCall(method string) op.BuiltinFunc` |

**`op.Receiver` — 14 struct embeddings** (same files as `op.NewReceiver` plus `receiver_lint.go`)

**`op.MakeAttr` — 63 call sites** (all `Attr()` method implementations)

**`op.NoSuchAttrError` — used in `Attr()` default cases**

### Current state of the exported API surface

| Symbol             | Status in devlore-cli            | Used by noblefactor-ops |
| ------------------ | -------------------------------- | ----------------------- |
| `Receiver` (type)  | **Exported**                     | Yes (14 embeddings)     |
| `newReceiver`      | **Unexported** (was `NewReceiver`) | Yes (13 calls) — BROKEN |
| `builtinFunc`      | **Unexported** (was `BuiltinFunc`) | Yes (1 reference) — BROKEN |
| `MakeAttr`         | **Exported**                     | Yes (63 calls)          |
| `NoSuchAttrError`  | **Exported**                     | Yes                     |

### Why this matters

noblefactor-ops cannot compile against the current devlore-cli. Even if we solve
Blocker 1, any `go mod tidy` or `go build` will fail with "undefined: op.NewReceiver"
(13 errors) and "undefined: op.BuiltinFunc" (1 error).

Phase 5 planned to delete only 4 of the 13 receivers that use `op.NewReceiver` (JSON,
YAML, Regexp, UI). The remaining 9 custom receivers (Config, Shellcheck, Go, Commands,
StarlarkParse, Setup, File, Schema, Lint) and WasmReceiver still need `op.NewReceiver`.

### Solution

Re-export `NewReceiver` and `BuiltinFunc` in devlore-cli. These were unexported during
internal refactoring but they are part of the external API contract — noblefactor-ops
(and any future external Starlark consumer) needs them to build custom receivers.

## Blocker 2a: `UnmarshalToAny` Signature Mismatch

### The problem

`op.UnmarshalToAny(starlark.Value) (any, error)` returns an error for unsupported types.
The noblefactor-ops `starlarkToGo(starlark.Value) interface{}` never returns an error —
it falls back to `v.String()` for unknown types.

Every call site (8 in `wasm_receiver.go`, 1 in `receiver_schema.go`, 1 in `render.go`,
2 in `receiver_json.go`, 1 in `receiver_yaml.go`) needs error handling added.

### Why this matters

This is not a blocker per se — it's mechanical work. But it changes the contract:
callers must decide what to do when conversion fails. The old code silently stringified;
the new code forces explicit handling. This is the correct behavior (per plan Design
Decision #3), but it's worth noting because `wasm_receiver.go` in particular has many
call sites where adding error handling cascades through the function.

## `starlarkToGo` / `goToStarlark` Call Sites

### Files to update (replace with `op.UnmarshalToAny` / `op.Marshal`)

| File                  | Function          | Calls              |
| --------------------- | ----------------- | ------------------ |
| `render.go`           | `goRender`        | `starlarkToGo` ×1  |
| `render_test.go`      | 5 test functions  | `starlarkToGo` ×5  |
| `receiver_schema.go`  | `validate`        | `starlarkToGo` ×1  |
| `wasm_receiver.go`    | `wasmArgsToJSON`  | `starlarkToGo` ×4  |
| `receiver_json.go`    | `encode`, `encodeIndent` | `starlarkToGo` ×2  |
| `receiver_json.go`    | `decode`          | `goToStarlark` ×1  |
| `receiver_yaml.go`    | `encode`          | `starlarkToGo` ×1  |
| `receiver_yaml.go`    | `decode`          | `goToStarlark` ×1  |

### Files that define them (to be deleted)

`receiver_yaml.go` lines 73–159 define both `starlarkToGo` and `goToStarlark`.

## What Needs to Happen Before Phase 5 Can Proceed

### In devlore-cli (prerequisite changes)

1. **Re-export `NewReceiver`** — rename `newReceiver` back to `NewReceiver` in
   `pkg/op/receiver.go`. Update all internal call sites.

2. **Re-export `BuiltinFunc`** — rename `builtinFunc` back to `BuiltinFunc` in
   `pkg/op/receiver.go`. Update all internal call sites (including `MakeAttr` signature).

3. **Solve gen package availability** — choose one of the solutions from Blocker 1.
   This is the key design decision that determines the shape of Phase 5.

### In noblefactor-ops (Phase 5 proper, depends on above)

Shape depends entirely on which Blocker 1 solution is chosen.

## Inventory of noblefactor-ops Custom Receivers

For completeness, these are the receivers that Phase 5 does NOT replace. They remain
hand-coded, embedding `op.Receiver`, and using `op.NewReceiver` + `op.MakeAttr`:

| Receiver                | File                     | Methods |
| ----------------------- | ------------------------ | ------- |
| `ConfigReceiver`        | `receiver_config.go`     | 3       |
| `ShellcheckReceiver`    | `receiver_shellcheck.go` | 4       |
| `GoReceiver`            | `receiver_go.go`         | 16      |
| `CommandsReceiver`      | `receiver_commands.go`   | 7       |
| `StarlarkParseReceiver` | `receiver_starlark.go`   | 3       |
| `SetupReceiver`         | `receiver_setup.go`      | 7       |
| `FileReceiver`          | `receiver_file.go`       | 15      |
| `SchemaReceiver`        | `receiver_schema.go`     | 1       |
| `LintReceiver`          | `receiver_lint.go`       | 4       |
| `WasmReceiver`          | `wasm_receiver.go`       | dynamic |
| `UiReceiver`            | `receiver_ui.go`         | 5       |
