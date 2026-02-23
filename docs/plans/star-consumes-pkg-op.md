---
title: "Star Consumes pkg/op"
status: draft
created: 2026-02-23
updated: 2026-02-23
---

# Plan: Star Consumes pkg/op

## Summary

noblefactor-ops (star) duplicates multiple components from devlore-cli: the UI
output surface, the Receiver base type, Starlark conversion helpers, the
`immediate_receiver` builtin template, and the file provider. This plan makes
star import `pkg/op` from devlore-cli, eliminating all copies. It also moves
gitignore filtering to a shared package, extends the code generator with
function parameter support, and renames star's shell analysis receiver to
`shellcheck` with an added format capability.

## Goals

1. **Single UI provider**: star uses `pkg/op/provider/ui.Provider` for all
   user-facing output — both Starlark scripts and Go-side callers.
2. **Single Receiver base**: star uses `op.Receiver` instead of its own
   `BaseReceiver`, `MakeAttr`, `NoSuchAttrError`, `BuiltinFunc`.
3. **Single conversion helpers**: generated code uses `op.ListToStringSlice`
   and `op.StarlarkDictToMap` instead of local lowercase copies.
4. **Single template surface**: the `ImmediateReceiverTemplate` builtin in
   `receiver_go_gen.go` generates code that imports `pkg/op` and
   `pkg/op/provider/<name>`, matching the devlore-cli local templates.
5. **Single file provider**: star uses `pkg/op/provider/file.Provider` for all
   filesystem operations — both Starlark scripts and Go-side callers.
6. **Shared gitignore filtering**: `ignore.Tracker` moves from
   `noblefactor-ops/internal/ignore` to `devlore-cli/pkg/op/ignore`, shared
   by both repos.
7. **Function parameter support in code generator**: `immediateArgExpr` handles
   `func(...)` parameter types by bridging `starlark.Callable` to Go closures.
8. **Shell analysis renamed**: star's `shell` receiver becomes `shellcheck`,
   with `format_check` replaced by `format(path, indent?, fix?)`.

## Current State

| Component | star (noblefactor-ops) | devlore-cli |
|-----------|----------------------|-------------|
| UI output | `internal/cli/output.go` — `Note`, `Warn`, `Error`, `Success`, `Failure` package-level functions with global `silent`/`programName` vars | `pkg/op/provider/ui/provider.go` — `Provider` struct with `Writer`, `ProgramName`, `Silent`, `Color` fields |
| Starlark UI builtins | `runtime.go` — 5 hand-written `NewBuiltin` registrations as bare globals calling `cli.*` | `receiver_ui_gen.go` — generated `UiReceiver` wrapping `ui.Provider` |
| Go-side UI callers | `receiver_setup.go` (8 calls), `receiver_lint.go` (1 call), `receiver_file.go` (4 calls) — all call `cli.Note`/`cli.Success` directly | All Go code calls `ui.Provider` methods |
| Receiver base | `receiver.go` — `BaseReceiver` struct, `Receiver` interface, `MakeAttr`, `NoSuchAttrError`, `BuiltinFunc` | `pkg/op/receiver.go` — `Receiver` struct, `MakeAttr`, `NoSuchAttrError`, `BuiltinFunc` |
| Conversion helpers | `immediateArgExpr` emits `listToStringSlice()` / `starlarkDictToMap()` into generated code | `pkg/op` exports `ListToStringSlice` / `StarlarkDictToMap` |
| Builtin template | `ImmediateReceiverTemplate` const — uses local `BaseReceiver` / `NewReceiver` / `MakeAttr` / `NoSuchAttrError` | Local `.go.template` files import `pkg/op` and `pkg/op/provider/<name>` |
| File provider | `receiver_file.go` — 14 hand-written methods (read, write, exists, is_file, is_directory, list, glob, walk_tree, join, name, parent, mkdir, remove, remove_all) with gitignore via `internal/ignore` and `DryRun` flag | `pkg/op/provider/file/provider.go` — 11 methods (Link, Copy, Backup, Unlink, Remove, Write, Move, Read (renamed from Source), Mkdir, Exists, IsDir) with compensation; no gitignore |
| Gitignore | `internal/ignore/` — `Tracker` type with `NewTracker`, `IsIgnored`, `Push`; depends on `go-git/v5` for pattern matching | None |
| Shell analysis | `receiver_shell.go` — `ShellReceiver` named `"shell"` with `lint`, `format_check`, `parse`, `complexity` | Not applicable — `pkg/op/provider/shell` is command execution |
| Code generator func params | Not supported | Same limitation |

star does NOT currently depend on devlore-cli.

## Licensing

devlore-cli is SSPL-1.0. noblefactor-ops is MIT. `pkg/op/` is part of
devlore-cli and carries the SSPL-1.0 header. Importing `pkg/op` into
noblefactor-ops means noblefactor-ops depends on an SSPL-licensed library.

This is an internal monorepo concern — both repos are owned by Noble Factor.
The `pkg/` convention signals "importable by other modules" and the owner
accepts the licensing implication. No action needed beyond acknowledging it.

## Phase 1: Add devlore-cli dependency

Add `github.com/NobleFactor/devlore-cli` as a Go module dependency in
noblefactor-ops.

- [ ] `go get github.com/NobleFactor/devlore-cli@feature/binding-unification`
- [ ] Verify `go mod tidy` succeeds

**Files**:

| File | Action |
|------|--------|
| `go.mod` | Modify: add devlore-cli dependency |
| `go.sum` | Modify: auto-updated |

## Phase 2: Move gitignore to shared package and extend file provider

### 2a: Create `pkg/op/ignore`

Move `ignore.Tracker` from `noblefactor-ops/internal/ignore/` to
`devlore-cli/pkg/op/ignore/`. The API is unchanged: `NewTracker(root)`,
`Root()`, `IsIgnored(path, isDir)`, `Push(dir)`. The `go-git/v5` dependency
moves with it.

- [ ] Create `pkg/op/ignore/tracker.go` with `Tracker` type
- [ ] Create `pkg/op/ignore/walker.go` (tree-walking helper used by `Tracker`)
- [ ] Create `pkg/op/ignore/tracker_test.go`
- [ ] Add `go-git/v5` dependency to devlore-cli `go.mod`
- [ ] Update noblefactor-ops `receiver_file.go`: import
      `github.com/NobleFactor/devlore-cli/pkg/op/ignore` instead of
      `internal/ignore`
- [ ] Delete `noblefactor-ops/internal/ignore/` (contents moved)

### 2b: Add methods to `pkg/op/provider/file`

Add 7 methods. The file provider gains a `Root string` field used by `Glob`
and `WalkTree` to construct an `ignore.Tracker` when `gitignore=true`.

| Method | Signature | Access | Notes |
|--------|-----------|--------|-------|
| `IsFile` | `(path string) (bool, error)` | `both` | Complement to `IsDir` |
| `Glob` | `(pattern string, gitignore bool) ([]string, error)` | `immediate` | `ignore.Tracker` when gitignore=true |
| `WalkTree` | `(root string, fn func(string, bool) error, gitignore bool) error` | `immediate` | Callback traversal; first consumer of function parameter conversion |
| `Join` | `(parts ...string) string` | `immediate` | `filepath.Join` |
| `Name` | `(path string) string` | `immediate` | `filepath.Base` |
| `Parent` | `(path string) string` | `immediate` | `filepath.Dir` |
| `RemoveAll` | `(path string) error` | `immediate` | `os.RemoveAll`; no compensation |

Change access for methods star needs as immediate operations:

| Method | Current access | New access | Reason |
|--------|---------------|------------|--------|
| `Read` (renamed from `Source`) | `planned` | `both` | Star scripts read files immediately; name matches `file.read()` in Starlark |
| `Write` | `planned` | `both` | Star scripts write files immediately |
| `Mkdir` | `planned` | `both` | Star scripts create dirs immediately |
| `Remove` | `planned` | `both` | Star scripts delete files immediately |

The immediate receiver calls these methods directly and discards compensation
state. Methods like `Write` and `Remove` return 3 values
`(string, map[string]any, error)` — the immediate receiver ignores the first
two and returns only the error. The planned receiver wraps them in actions
with full undo via the compensation state.

Adding `Root string` to `file.Provider` changes it from a zero-field struct.
All devlore-cli callers that construct `file.Provider{}` must add `Root`.
Grep for `file.Provider{` in devlore-cli to find all sites.

- [ ] Add `Root string` field to `file.Provider`
- [ ] Add `//+devlore:access=both` to `IsFile`; implement
- [ ] Add `//+devlore:access=immediate` to `Glob`, `WalkTree`, `Join`,
      `Name`, `Parent`, `RemoveAll`; implement
- [ ] Rename `Source` to `Read`; change from `planned` to `both`
- [ ] Change `Write`, `Mkdir`, `Remove` from `planned` to `both`
- [ ] Update all `&file.Provider{}` constructor sites in devlore-cli to add
      `Root` field (20+ sites in test files: `execution_test.go`,
      `compensation_test.go`, etc.)
- [ ] Update `plan_root.go:174`: `"file.source"` → `"file.read"`
- [ ] Update docs referencing `file.source`: `devlore-operation-namespaces.md`,
      `projected-provider-api.md`, `compensation.md`, `resource-provider.md`,
      `resource-provider/phase-2c.md`
- [ ] Add tests for all 7 new methods
- [ ] Regenerate devlore-cli receivers: `star devlore actions generate`

**Files** (devlore-cli):

| File | Action |
|------|--------|
| `pkg/op/ignore/tracker.go` | Create |
| `pkg/op/ignore/tracker_test.go` | Create |
| `pkg/op/provider/file/provider.go` | Modify: add `Root` field, 7 methods, change 4 access directives |
| `pkg/op/provider/file/provider_test.go` | Modify: add tests |
| Generated receivers | Regenerate |

**Files** (noblefactor-ops):

| File | Action |
|------|--------|
| `internal/ignore/` | Delete (moved to devlore-cli) |
| `internal/starlark/receiver_file.go` | Modify: import path (temporary — deleted in Phase 6) |

## Phase 3: Replace Receiver base type, fix code generator, delete dead weight

Replace `BaseReceiver` with `op.Receiver` from `pkg/op`. Delete the
`Receiver` interface — nothing type-asserts against it, and `op.Receiver`
already satisfies `starlark.Value`. Each concrete receiver directly embeds
`op.Receiver` and implements `starlark.HasAttrs`.

Simultaneously update the `ImmediateReceiverTemplate` const and conversion
helpers so that generated code also uses `pkg/op`. These must change in the
same phase — deleting `receiver.go` while the template still references its
symbols would break all generated receivers.

Note: `immediateArgExpr` currently emits `listToStringSlice()` and
`starlarkDictToMap()` (lowercase, unexported) — these are undefined today.
This is a pre-existing defect. Fixing them to `op.ListToStringSlice()` and
`op.StarlarkDictToMap()` is part of this phase.

| star (`internal/starlark`) | `pkg/op` |
|---------------------------|----------|
| `BaseReceiver` | `op.Receiver` |
| `NewBaseReceiver(name)` | `op.NewReceiver(name)` |
| `MakeAttr(name, fn)` | `op.MakeAttr(name, fn)` |
| `NoSuchAttrError(recv, attr)` | `op.NoSuchAttrError(recv, attr)` |
| `BuiltinFunc` | `op.BuiltinFunc` |
| `Receiver` interface | deleted |
| `ImmediateReceiverTemplate` | updated: imports `pkg/op` |
| `immediateArgExpr` → `listToStringSlice` | `op.ListToStringSlice` |
| `immediateArgExpr` → `starlarkDictToMap` | `op.StarlarkDictToMap` |

### 3a: Hand-written receivers

- [ ] Add `import "github.com/NobleFactor/devlore-cli/pkg/op"` to all
      receiver files that embed `BaseReceiver`
- [ ] Replace `BaseReceiver` with `op.Receiver` in struct definitions
- [ ] Replace `NewBaseReceiver` with `op.NewReceiver` in constructors
- [ ] Replace `MakeAttr` calls with `op.MakeAttr`
- [ ] Replace `NoSuchAttrError` calls with `op.NoSuchAttrError`
- [ ] Replace `BuiltinFunc` type references with `op.BuiltinFunc`
- [ ] Delete `receiver.go` entirely (all contents moved to `pkg/op`)

### 3b: Code generator template and conversion helpers

- [ ] Update `ImmediateReceiverTemplate` const: import `pkg/op`, use
      `op.Receiver` / `op.NewReceiver` / `op.MakeAttr` / `op.NoSuchAttrError`
- [ ] Update `immediateArgExpr`: emit `op.ListToStringSlice(...)` instead
      of `listToStringSlice(...)`
- [ ] Update `immediateArgExpr`: emit `op.StarlarkDictToMap(...)` instead
      of `starlarkDictToMap(...)`

**Files**:

| File | Action |
|------|--------|
| `internal/starlark/receiver.go` | Delete |
| `internal/starlark/receiver_go_gen.go` | Modify: template, `immediateArgExpr` conversion helpers |
| `internal/starlark/receiver_go_gen_test.go` | Modify: update assertions |
| `internal/starlark/receiver_file.go` | Modify (temporary — deleted in Phase 6) |
| `internal/starlark/receiver_shell.go` | Modify (renamed in Phase 7) |
| `internal/starlark/receiver_json.go` | Modify |
| `internal/starlark/receiver_yaml.go` | Modify |
| `internal/starlark/receiver_schema.go` | Modify |
| `internal/starlark/receiver_regexp.go` | Modify |
| `internal/starlark/receiver_lint.go` | Modify |
| `internal/starlark/receiver_setup.go` | Modify |
| `internal/starlark/receiver_config.go` | Modify |
| `internal/starlark/receiver_starlark.go` | Modify |
| `internal/starlark/receiver_commands.go` | Modify |
| `internal/starlark/receiver_go.go` | Modify |
| `internal/starlark/wasm_receiver.go` | Modify |

## Phase 4: Extend code generator — new conversion types

### 4a: Function parameter conversion

Add `func(...)` as a recognized parameter type. When a provider method has a
function parameter, the generated receiver:

1. Unpacks a `starlark.Callable` from kwargs
2. Emits a bridging closure that calls the callable via `starlark.Call`,
   converting Go args to Starlark values and back
3. Passes the closure to the provider method

The generated method signature must pass `*starlark.Thread` (currently `_`)
to implementations that have function parameters, so the bridging closure can
call back into Starlark.

First consumer: `file.WalkTree(root, func(string, bool) error, gitignore)`.

- [ ] Update `generate.star`: detect `func(...)` parameter types in method
      signatures
- [ ] Update `immediateArgExpr`: emit `starlark.Callable` unpacking and
      closure bridge code
- [ ] Update `ImmediateReceiverTemplate`: pass thread to methods with
      function parameters
- [ ] Update tests for all code generator changes

### 4b: Additional conversion types

The file provider methods introduce 4 conversion patterns not yet handled by
the code generator:

| Pattern | Example | Generator change |
|---------|---------|-----------------|
| Variadic params | `Join(parts ...string)` | Unpack `*args` as `[]string`, pass with `...` |
| Non-error returns | `Join(parts ...string) string` | Wrap return as `starlark.String`; no error check |
| `[]byte` return | `Read(path string) ([]byte, error)` | Convert to `starlark.String` |
| 3-return compensation | `Write(path string, content []byte, mode os.FileMode) (string, map[string]any, error)` | Immediate receiver discards middle 2 values; return only error |

- [ ] Update `immediateArgExpr`: handle variadic parameter expansion
- [ ] Update return value emitter: handle non-error single returns
- [ ] Update return value emitter: convert `[]byte` to `starlark.String`
- [ ] Update return value emitter: discard compensation state in immediate
      receivers (keep only error)

**Files**:

| File | Action |
|------|--------|
| `internal/starlark/receiver_go_gen.go` | Modify: `immediateArgExpr` function param support, 4b conversion types |
| `internal/starlark/receiver_go_gen_test.go` | Modify: update assertions |
| `star/extensions/com.noblefactor.devlore.Actions/commands/generate.star` | Modify: detect function parameter types, variadic params, return patterns |

## Phase 5: Replace UI output in noblefactor-ops

Replace the 5 hand-written Starlark builtins and all Go-side `cli.*` output
calls with `ui.Provider`.

**Starlark surface**: Inject `ui.Provider` as a `"ui"` receiver in
`buildPredeclared`, matching devlore-cli. The bare globals (`note`, `warn`,
`error`, `success`, `fail`) are deleted.

**Go-side callers**: `receiver_setup.go` (8 calls), `receiver_lint.go`
(1 call), and `receiver_file.go` (4 calls) currently call `cli.Note` /
`cli.Success`. These receivers gain a `*ui.Provider` field and call its
methods directly.

**UiReceiver adapter**: Write a local `receiver_ui.go` that embeds
`op.Receiver` and wraps `*ui.Provider` as a Starlark `HasAttrs` value.
devlore-cli's generated `UiReceiver` lives in `internal/starlark/` (not
importable). The adapter is trivial — 5 methods, each unpacks a `msg`
string arg and delegates to the provider.

- [ ] Create `internal/starlark/receiver_ui.go` — `UiReceiver` adapter
- [ ] Construct `ui.Provider{Writer: os.Stderr, ProgramName: "star", Color: true}`
      in `NewRuntime`; store as field on `Runtime`
- [ ] Wire `--silent` flag to `ui.Provider.Silent`
- [ ] Add `"ui": uiReceiver` to `buildPredeclared`
- [ ] Delete the 5 bare globals (`note`, `warn`, `error`, `success`, `fail`)
      from `buildPredeclared`
- [ ] Delete `noteBuiltin`, `warnBuiltin`, `errorBuiltin`, `successBuiltin`,
      `failBuiltin`, `extractMessage` from `runtime.go`
- [ ] Add `*ui.Provider` field to `SetupReceiver`, `LintReceiver`;
      replace all `cli.Note`/`cli.Success` calls with provider method calls
- [ ] Delete status output section from `internal/cli/output.go`: `Note`,
      `Warn`, `Error`, `Success`, `Failure`, color/symbol constants,
      `programName`/`silent` vars, `SetProgramName`, `SetSilent`,
      `AddSilentFlag`
- [ ] Update `cmd/star/` root command: wire `--silent` to
      `runtime.UIProvider.Silent` instead of `cli.SetSilent`
- [ ] Grep for `cli.Note`, `cli.Warn`, `cli.Error`, `cli.Success`,
      `cli.Failure`, `cli.SetProgramName`, `cli.SetSilent` — zero matches

**Files**:

| File | Action |
|------|--------|
| `internal/starlark/receiver_ui.go` | Create |
| `internal/starlark/runtime.go` | Modify: add `ui.Provider` to `Runtime`; add `"ui"` to predeclared; delete 5 bare globals and helpers |
| `internal/starlark/receiver_setup.go` | Modify: `*ui.Provider` field; replace `cli.*` calls |
| `internal/starlark/receiver_lint.go` | Modify: same |
| `internal/starlark/receiver_file.go` | Modify: `*ui.Provider` field; replace 4 `cli.Note` calls (temporary — deleted in Phase 6) |
| `internal/cli/output.go` | Modify: delete status output section |
| `cmd/star/main.go` or root command | Modify: wire `--silent` to `ui.Provider.Silent` |

## Phase 6: Replace FileReceiver — consume `pkg/op/provider/file`

Delete star's hand-written `FileReceiver` and replace it with a generated
receiver wrapping `*file.Provider`.

The generated receiver is produced by the code generator (Phase 4) using the
`ImmediateReceiverTemplate`. It imports `pkg/op/provider/file`, unpacks
Starlark args, and delegates to provider methods. `WalkTree` uses the
function parameter bridge (Phase 4b) to convert `starlark.Callable` to
`func(string, bool) error`.

Star's current `DryRun` flag for file operations is dropped. Dry-run for
writes uses planned actions in the operation graph. Immediate file operations
execute directly.

The generated immediate receiver exposes methods with `access=immediate` or
`access=both`: `Exists`, `IsDir`, `IsFile`, `Read`, `Write`, `Mkdir`,
`Remove`, `Glob`, `WalkTree`, `Join`, `Name`, `Parent`, `RemoveAll`.

- [ ] Generate `internal/starlark/receiver_file_gen.go` from
      `pkg/op/provider/file.Provider`
- [ ] Wire `file.Provider{Root: workDir}` in `NewRuntime`; add `"file"`
      to `buildPredeclared`
- [ ] Delete `internal/starlark/receiver_file.go`
- [ ] Remove `DryRun` checks from `receiver_file.go` (deleted with the file)
- [ ] Verify all methods work in Starlark

Note: `DryRun` cannot be fully deleted in this phase. `receiver_setup.go`
has 5 `DryRun` checks and `command.go` exposes it as the `dry_run` Starlark
attribute. These consumers remain. `DryRun` deletion is deferred until
`receiver_setup.go` is refactored (out of scope for this plan).

**Files**:

| File | Action |
|------|--------|
| `internal/starlark/receiver_file_gen.go` | Create (generated) |
| `internal/starlark/receiver_file.go` | Delete |
| `internal/starlark/runtime.go` | Modify: wire `file.Provider`, update `buildPredeclared` |

## Phase 7: Rename shell to shellcheck and add format

Rename `ShellReceiver` to `ShellcheckReceiver`, change the predeclared key
from `"shell"` to `"shellcheck"`, and replace `format_check` with `format`.

### Rename

- [ ] Rename `receiver_shell.go` to `receiver_shellcheck.go`
- [ ] Rename `ShellReceiver` to `ShellcheckReceiver`
- [ ] Rename `NewShellReceiver` to `NewShellcheckReceiver`
- [ ] Change `NewBaseReceiver("shell")` to `op.NewReceiver("shellcheck")`
- [ ] Change predeclared key from `"shell"` to `"shellcheck"` in `runtime.go`
- [ ] Update `Attr` case strings and `MakeAttr` prefixes
- [ ] Update `AttrNames` return value

### Replace format_check with format(fix?)

| Method | Signature | Behavior |
|--------|-----------|----------|
| `format` | `(path, indent?, fix?)` | `fix=false` (default): `shfmt -d` — check only, return diffs. `fix=true`: `shfmt -w` — rewrite files. |

- [ ] Delete `formatCheck` method
- [ ] Add `format` method with `fix` bool parameter (default false)
- [ ] `fix=false`: run `shfmt -d`, return `{passed, files_checked, files_failed}`
- [ ] `fix=true`: run `shfmt -w`, return `{files_checked, files_formatted}`
- [ ] Update `Attr` switch: remove `format_check`, add `format`
- [ ] Update `AttrNames`: `{"complexity", "format", "lint", "parse"}`

**Files**:

| File | Action |
|------|--------|
| `internal/starlark/receiver_shell.go` | Delete (renamed) |
| `internal/starlark/receiver_shellcheck.go` | Create (renamed + modified) |
| `internal/starlark/receiver_shellcheck_test.go` | Create: test `format(fix=false)` and `format(fix=true)` |
| `internal/starlark/runtime.go` | Modify: `"shellcheck"` key |

## Phase 8: Update extension scripts (noblefactor-ops)

Update all noblefactor-ops extension scripts.

**Transformations**:

| Old | New |
|-----|-----|
| `note("msg")` | `ui.note("msg")` |
| `warn("msg")` | `ui.warn("msg")` |
| `error("msg")` | `ui.error("msg")` |
| `success("msg")` | `ui.success("msg")` |
| `fail("msg")` | `ui.fail("msg")` |
| `shell.lint(...)` | `shellcheck.lint(...)` |
| `shell.format_check(...)` | `shellcheck.format(...)` |
| `shell.parse(...)` | `shellcheck.parse(...)` |
| `shell.complexity(...)` | `shellcheck.complexity(...)` |
| `file.list(path)` | `file.glob(path + "/*")` |
| `file.read(path)` | `file.read(path)` (unchanged — provider renamed from `Source` to `Read` in Phase 2b) |
| `file.is_directory(path)` | `file.is_dir(path)` |
| `file.is_file(path)` | `file.is_file(path)` (unchanged) |

- [ ] `com.noblefactor.star.Setup/commands/setup.star`
- [ ] `com.noblefactor.star.SetupTools/commands/setup-tools.star`
- [ ] `com.noblefactor.star.SetupConfig/commands/setup-config.star`
- [ ] `com.noblefactor.star.SetupHooks/commands/setup-hooks.star`
- [ ] `com.noblefactor.star.SetupCheck/commands/setup-check.star`
- [ ] `com.noblefactor.star.LintAll/commands/lint-all.star`
- [ ] `com.noblefactor.star.LintGo/commands/lint-go.star`
- [ ] `com.noblefactor.star.LintShell/commands/lint-shell.star`
- [ ] `com.noblefactor.star.LintMarkdown/commands/lint-markdown.star`
- [ ] `com.noblefactor.star.LintCopyright/commands/lint-copyright.star`
- [ ] `com.noblefactor.star.LintTools/commands/lint-tools.star`
- [ ] `com.noblefactor.star.HookPreCommit/commands/hook-pre-commit.star`
- [ ] `com.noblefactor.star.HookPrePush/commands/hook-pre-push.star`
- [ ] `com.noblefactor.star.ConfigSync/commands/config-sync.star`
- [ ] `com.noblefactor.star.ConfigShow/commands/config-show.star`

## Phase 9: Update extension scripts (devlore-cli)

Update all devlore-cli extension scripts. Same UI and file transformations
as Phase 8 where applicable.

Note: `starlark/lore_builtins.star` and `docs/guides/lore/plan-bindings.md`
were deleted as dead weight (committed separately). All references cleaned up.

- [ ] `com.noblefactor.devlore.Actions/commands/generate.star` (Phase 4 handles code generator changes; this covers only UI/file transforms)
- [ ] `com.noblefactor.devlore.Actions/commands/validate.star`
- [ ] `com.noblefactor.devlore.Package/commands/index.star`
- [ ] `com.noblefactor.devlore.Package/commands/sign.star`
- [ ] `com.noblefactor.devlore.Package/commands/validate.star`
- [ ] `com.noblefactor.devlore.Knowledge/commands/extract.star`
- [ ] `com.noblefactor.devlore.Knowledge/commands/index.star`
- [ ] `com.noblefactor.devlore.Knowledge/commands/sign.star`
- [ ] `com.noblefactor.devlore.Knowledge/commands/validate.star`
- [ ] `com.noblefactor.devlore.Model/commands/build.star`

## Verification

1. `make test` passes (both repos)
2. `make build` succeeds (both repos)
3. Grep for `BaseReceiver` — zero matches
4. Grep for `NewBaseReceiver` — zero matches
5. Grep for `cli.Note`, `cli.Warn`, `cli.Error`, `cli.Success`,
   `cli.Failure` — zero matches
6. Grep for `listToStringSlice` (lowercase) in `*.go` — zero matches
   (only `op.ListToStringSlice`)
7. Grep for `\bnote(` / `\bwarn(` / `\bsuccess(` / `\bfail(` not preceded
   by `ui.` in `*.star` — zero matches
8. Grep for `"shell"` as predeclared key — zero matches (now `"shellcheck"`)
9. Grep for `shell\.lint` / `shell\.format` / `shell\.parse` /
   `shell\.complexity` in `*.star` — zero matches (now `shellcheck.*`)
10. Grep for `format_check` — zero matches (now `format`)
11. Grep for `file\.list(` in `*.star` — zero matches (now `file.glob`)
12. Grep for `file\.is_directory(` in `*.star` — zero matches (now
    `file.is_dir`)
13. Grep for `internal/ignore` — zero matches (now `pkg/op/ignore`)
14. Grep for `file\.source` in `*.go` — zero matches (now `file.read`)
15. Grep for `file\.Source` (struct) in `*.go` — zero matches (now `file.Read`)
16. `--silent` flag still suppresses output
17. `receiver.go` no longer exists
18. `receiver_file.go` no longer exists (replaced by `receiver_file_gen.go`)
19. `receiver_shell.go` no longer exists (replaced by `receiver_shellcheck.go`)
20. All `&file.Provider{}` sites in devlore-cli updated with `Root` field
21. Generated `receiver_file_gen.go` compiles and exposes all 13 immediate methods
22. `WalkTree` callback from Starlark works (function parameter bridge)
