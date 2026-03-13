---
title: "Shared Provider Receivers"
status: in-progress
created: 2026-03-11
updated: 2026-03-12
---

# Plan: Shared Provider Receivers

## Summary

Replace hand-coded Starlark receivers in noblefactor-ops (JSON, YAML, Regexp, UI) with
framework-managed receivers from devlore-cli. Instead of manually calling `WrapReceiver`,
noblefactor-ops uses `op.StarlarkRuntime` — the same framework that devlore-cli uses to
set up its own scripting runtimes.

This also exports the framework's Starlark-Go conversion functions (`Marshal`, `UnmarshalToAny`)
so noblefactor-ops can delete its parallel `starlarkToGo`/`goToStarlark` implementations.

## Goals

1. **Use the framework** — noblefactor-ops sets up an immediate scripting runtime through
   `pkg/op`, not by hand-rolling receiver construction
2. **Eliminate receiver duplication** — 4 hand-coded receivers (~595 lines) replaced by
   framework-managed equivalents
3. **Eliminate conversion duplication** — `starlarkToGo`/`goToStarlark` replaced by
   framework's exported `Marshal`/`UnmarshalToAny`
4. **Unify the Starlark API surface** — both tools expose the same parameter names
5. **Clean consumer API** — consumers pass typed provider references, not strings

## Current State

### noblefactor-ops receivers (hand-coded)

| File                 | Lines | Notes                                                                   |
| -------------------- | ----- | ----------------------------------------------------------------------- |
| `receiver_json.go`   | 90    | Hand-coded Attr/AttrNames, calls `starlarkToGo`/`goToStarlark`          |
| `receiver_yaml.go`   | 159   | Hand-coded, DEFINES `starlarkToGo`/`goToStarlark` used by 5 other files |
| `receiver_regexp.go` | 252   | Hand-coded, own `sync.Map` cache (devlore provider has identical cache) |
| `receiver_ui.go`     | 94    | Hand-coded, wraps `*ui.Provider`                                        |
| `receivers.go`       | 32    | Singleton construction                                                  |

### devlore-cli framework

| Component                           | Location                                | Status                                       |
| ----------------------------------- | --------------------------------------- | -------------------------------------------- |
| `Runtime`                           | `internal/starlark/runtime.go`          | Internal — embeds `*op.StarlarkRuntime`, adds graph builder + module loader |
| `BindingConfig`                     | `pkg/op/binding_config.go`              | Exported, builder pattern                    |
| `StarlarkRuntime`                   | `pkg/op/starlark_runtime.go`            | Exported                                     |
| `Announce`/`InitAll`/`Providers`    | `pkg/op/announce.go`                    | Exported                                     |
| `ImmediateProvider`                 | `pkg/op/provider.go`                    | Exported                                     |
| `WrapReceiver`                      | `pkg/op/receiver_reflect.go`            | Exported                                     |
| `Marshal`/`UnmarshalToAny`          | `pkg/op/starvalue_marshal.go`           | Exported (Phase 4, PR #212)                  |
| Gen providers (`init` → `Announce`) | `pkg/op/provider/*/gen/provider.gen.go` | Generated, to be tracked in git (Phase 5a)   |

### Known gaps (at plan creation)

- **UI `NewImmediate` ignores `BindingConfig`**: The generated `NewImmediate` creates a bare
  `&ui.Provider{}` without wiring `Writer`, `ProgramName`, `Color` from `BindingConfig`.
  noblefactor-ops currently configures these explicitly. Fixed by adding `+devlore:bind`
  directives to the UI provider.
- **Regexp param name**: devlore-cli uses `s`; noblefactor-ops uses `text`. Must rename
  in devlore-cli before unification.

## Architecture

### Two-layer runtime design

**`op.StarlarkRuntime`** (in `pkg/op/`) — the base runtime. An open set of receivers that
external consumers load into. Manages receiver construction, action registration, and context
lifecycle. This is what noblefactor-ops uses — it only needs immediate receivers.

**`internal/starlark.Runtime`** — embeds `*op.StarlarkRuntime`. Adds devlore-specific
capabilities that depend on internal types: the graph builder (`plan.*` namespace via
`PlanRoot`) and the `@devlore//` module loader (with caching). Used by lore (graph building),
the e2e test runner, and any devlore CLI tool that executes `.star` scripts.

```
op.StarlarkRuntime             ← load receivers, manage lifecycle (exported)
  └─ internal/starlark.Runtime ← + graph builder, module loader (internal)
```

The split exists because `PlanRoot` and `collectPlannedProviders` depend on internal types
that cannot move to `pkg/op/`. The `@devlore//` module prefix is product-specific.

A third usage pattern — action registration without script execution (writ, some lore paths)
— calls `op.InitAll` directly and needs no runtime at all.

### Consumer API

Consumers construct a runtime via the `BindingConfig` builder:

```go
rt := loreStar.NewRuntime(
    op.NewBindingConfig("star").
        WithGraphBuilder().
        WithReceivers(json.Receiver, yaml.Receiver, ui.Receiver).
        WithColor(),
)
```

- `NewBindingConfig(programName)` — required, creates config with `Writer` defaulting to
  `os.Stderr`
- `WithGraphBuilder()` — enables the `plan.*` graph namespace
- `WithReceivers(...)` — typed provider references (compile-time checked)
- `WithWriter(w)` — override output destination
- `WithColor()` — enable ANSI color codes

`Platform` is built internally via `platform.New()` — not passed in config.

### Provider registration

Generated code follows Go ecosystem norms: generated files are committed to git.
`go get` fetches source; consumers import gen packages directly.

Each gen package exports `var Receiver op.Provider` and calls `op.Announce()` in `init()`.
The announcement registry uses `reflect.Type` as keys for type-safe lookup.

```
gen/provider.gen.go → var Receiver op.Provider = &jsonProvider{}
                    → init() → op.Announce(Receiver)
                                    ↓
                    op.InitAll() → Register() on each announced provider
                                    ↓
                    StarlarkRuntime.BuildReceivers() → ImmediateProvider.NewImmediate(cfg)
```

### Control flow receivers

The `plan.*` namespace contains graph construction primitives (`plan.choose`,
`plan.source`, `plan.gather`) that are inherently hand-coded in `PlanRoot`. They
manipulate `*op.Graph`, `*op.Node`, `*op.Output` directly — they don't map to a
Go struct with methods. These are the only receivers that do not go through codegen.

### Vestigial hand-coding API

`NewReceiver`, `BuiltinFunc`, and `MakeAttr` in `pkg/op/receiver.go` are plumbing
from before the codegen system. Codegen replaces all three. They are unexported (correct)
and will be deleted once all consumers migrate to codegen providers.

## Implementation Phases

### Phase 1: Rename regexp param in devlore-cli (complete)

- [x] Rename `s` → `text` in `pkg/op/provider/regexp/provider.go` (all 8 methods)
- [x] Update `test_regexp.star` and `test_imm_regexp.star` keyword args
- [x] `make build` and `make test` pass

**Files** (devlore-cli):

- `pkg/op/provider/regexp/provider.go` — Modify
- `internal/e2e/testrunner/data/test_regexp.star` — Modify
- `internal/e2e/testrunner/data/test_imm_regexp.star` — Modify

### Phase 2: Extract `ContextBase` from `Context` (complete)

Split `Context` into a base type for immediate environments and a full type for
graph execution. `StarlarkRuntime.Initialize` takes `ContextBase`; the graph executor
embeds it and adds graph-specific fields.

- [x] Create `ContextBase` in `pkg/op/context.go` with fields needed by every immediate
      environment: `context.Context`, `Root`, `DryRun`, `Platform`, `Writer`, `Data`
- [x] Refactor `Context` to embed `ContextBase`, keeping graph-specific fields:
      `RecoverySite`, `Catalog`, `Graph`, `NodeID`, `Thread`
- [x] Update all code that accesses base fields — embedding makes this transparent
- [x] Update codegen template (`actions_test.go.template`) and regenerate all gen test files
- [x] `make build` and `make test` pass

**Files** (devlore-cli):

- `pkg/op/context.go` — Modify (split into `ContextBase` + `Context`)
- `star/extensions/.../templates/actions_test.go.template` — Modify
- `internal/execution/executor.go` — Modify (struct literal)
- `internal/execution/flow/gather.go` — Modify (struct literal)
- `pkg/op/provider/starcode/provider.go` — Modify (struct literal)
- `internal/writ/migrate/execute.go` — Modify (struct literal)
- `internal/e2e/testrunner/runner.go` — Modify (struct literal)
- ~10 test files — Modify (struct literals)

### Phase 3: Extract `StarlarkRuntime` and rename `BindingSet` (complete)

Extract the immediate-mode runtime from `BindingSet` into `pkg/op/StarlarkRuntime`.
Rename `BindingSet` to `Runtime`. Fix UI `NewImmediate` to wire `BindingConfig` fields.

- [x] Add `+devlore:bind Writer=Writer, ProgramName=ProgramName, Color=Color` to UI
      provider doc comment, regenerate
- [x] Create `pkg/op/starlark_runtime.go` with `StarlarkRuntime` struct
  - `NewStarlarkRuntime(cfg *BindingConfig)` — constructor
  - `Initialize(reg *ActionRegistry, ctx ContextBase)` — calls `InitAll`, stores context
  - `BuildReceivers() starlark.StringDict` — builds receivers for all included providers
  - `BuildReceiver(name string) (starlark.Value, bool)` — builds one receiver by name
  - `HasGraphBuilder() bool` — reports whether graph builder is enabled
- [x] Rename `BindingSet` → `Runtime` in `internal/starlark/binding_set.go`
  - Rename file to `runtime.go`
  - Embed `*op.StarlarkRuntime`
  - `NewRuntime(cfg *op.BindingConfig)` replaces `NewBindingSet`
  - `RegisterActions` delegates to `env.Initialize`
  - `BuildGlobals` delegates immediate receivers to `env.BuildReceivers()`, uses
    `HasGraphBuilder()` for plan namespace
  - `resolveProvider` delegates to `env.BuildReceiver(name)` for immediate providers
  - `NewPopulatedRegistry`, `ConfigureThread`, `makeLoader`, `buildPlanModule`,
    `collectPlannedProviders` stay on `Runtime`
- [x] Update all call sites (`loreStar.NewBindingSet` → `loreStar.NewRuntime`)
- [x] Rename test file `binding_set_test.go` → `runtime_test.go`, update test names
- [x] `make build` and `make test` pass

**Files** (devlore-cli):

- `pkg/op/provider/ui/provider.go` — Modify (add `+devlore:bind` directives)
- `pkg/op/starlark_runtime.go` — Create
- `internal/starlark/binding_set.go` → `internal/starlark/runtime.go` — Rename + modify
- `internal/starlark/binding_set_test.go` → `internal/starlark/runtime_test.go` — Rename + modify
- `internal/starlark/integration_test.go` — Modify (call site update)
- `internal/lore/builder.go` — Modify (call site update)
- `internal/e2e/testrunner/runner.go` — Modify (call site update)

### Phase 4: Export Starlark-Go conversion functions (complete)

Export the framework's canonical conversion functions so noblefactor-ops can replace
its parallel `starlarkToGo`/`goToStarlark`.

- [x] Export `unmarshalToAny` as `UnmarshalToAny` in `pkg/op/starvalue_marshal.go`
- [x] Export `marshal` as `Marshal` in `pkg/op/starvalue_marshal.go`
- [x] Update all internal call sites (`marshal(` → `Marshal(`, `unmarshalToAny(` → `UnmarshalToAny(`)
- [x] `make build` and `make test` pass

**Files** (devlore-cli):

- `pkg/op/starvalue_marshal.go` — Modify (export two functions + update internal calls)
- `pkg/op/output.go` — Modify (call site)
- `pkg/op/receiver_reflect.go` — Modify (doc comments)
- `pkg/op/output_test.go` — Modify (call sites)
- `pkg/op/receiver_reflect_test.go` — Modify (doc comment)
- `pkg/op/starvalue_marshal_test.go` — Modify (call sites)

### Phase 4a: BindingConfig builder pattern (in progress)

Redesign `BindingConfig` with builder pattern and cleaner consumer API.

- [x] Builder pattern: `NewBindingConfig(programName)` with `WithGraphBuilder()`,
      `WithReceivers(...)`, `WithWriter(w)`, `WithColor()`
- [x] Replace `"plan"` string in receivers with `WithGraphBuilder()` boolean
      (field: `GraphBuilder`, method: `WithGraphBuilder()`, query: `HasGraphBuilder()`)
- [x] Remove `Platform` from config (built internally via `platform.New()`)
- [x] Default `Writer` to `os.Stderr`
- [x] Update all call sites and tests to builder pattern
- [x] `make build` and `make test` pass
- [ ] Change `Receivers` from `[]string` to `[]Provider` (requires Phase 5a)
- [ ] Update gen template to export `var Receiver op.Provider` per gen package
- [ ] Use `reflect.Type` as keys in announcement registry

**Files** (devlore-cli):

- `pkg/op/binding_config.go` — Rewrite (builder pattern)
- `pkg/op/starlark_runtime.go` — Modify (takes `*BindingConfig`, add `HasGraphBuilder`)
- `internal/starlark/runtime.go` — Modify (takes `*BindingConfig`, use `HasGraphBuilder`)
- `internal/e2e/testrunner/runner.go` — Modify (`withGraphBuilder` field, builder call site)
- `internal/lore/builder.go` — Modify (builder call site)
- `internal/starlark/runtime_test.go` — Modify (all tests to builder pattern)
- `internal/starlark/integration_test.go` — Modify (all tests to builder pattern)

### Phase 5a: Commit generated code in devlore-cli (pending)

Track gen packages in version control so external consumers can import them.
Each gen package exports `var Receiver op.Provider` for typed consumer references.

- [ ] Remove `gen/` from `.gitignore`
- [ ] Commit all existing generated files under `pkg/op/provider/*/gen/`
- [ ] Update gen template to export `var Receiver op.Provider` per package
- [ ] Change `BindingConfig.Receivers` from `[]string` to `[]Provider`
- [ ] Use `reflect.Type` as keys in announcement registry
- [ ] Delete `newReceiver`, `builtinFunc`, `MakeAttr` from `pkg/op/receiver.go`
- [ ] Add CI validation: run codegen, `git diff --exit-code` to catch stale output
- [ ] `make build` and `make test` pass

**Files** (devlore-cli):

- `.gitignore` — Modify (remove `gen/` exclusion)
- `pkg/op/provider/*/gen/*.gen.go` — Add to git
- `pkg/op/binding_config.go` — Modify (`Receivers []Provider`)
- `pkg/op/announce.go` — Modify (`reflect.Type` registry keys)
- `pkg/op/receiver.go` — Modify (delete vestigial hand-coding API)
- Gen template — Modify (export `Receiver` var)
- All call sites — Modify (pass provider references instead of strings)
- CI config — Modify (add codegen freshness check)

### Phase 5: Use framework in noblefactor-ops (pending, blocked on 5a)

Replace hand-coded receivers with framework-managed receivers. Replace
`starlarkToGo`/`goToStarlark` with exported framework equivalents.

- [ ] Import devlore-cli gen packages (now tracked in git) to get provider references
- [ ] Create `StarlarkRuntime` via builder:
      `op.NewBindingConfig("star").WithReceivers(json.Receiver, yaml.Receiver, ...).WithColor()`
- [ ] Call `BuildReceivers()` in `buildPredeclared()`, merge with custom receivers
- [ ] Replace `starlarkToGo` calls with `op.UnmarshalToAny` (in `receiver_schema.go`,
      `render.go`, `wasm_receiver.go`)
- [ ] Replace `goToStarlark` calls with `op.Marshal` (in `render.go`, `wasm_receiver.go`)
- [ ] Delete `receiver_json.go`, `receiver_yaml.go`, `receiver_regexp.go`, `receiver_ui.go`
- [ ] Delete `starlarkToGo`/`goToStarlark` (no remaining callers)
- [ ] Update `receivers.go` — remove JSON, YAML, Regexp singletons (framework manages them)
- [ ] Update `go.mod` — `go get` updated devlore-cli, `go mod tidy`
- [ ] `make build` and `make test` pass

**Files** (noblefactor-ops):

- `internal/starlark/runtime.go` — Modify (add `StarlarkRuntime`, provider imports, update `buildPredeclared`)
- `internal/starlark/receivers.go` — Modify (remove replaced singletons)
- `internal/starlark/receiver_schema.go` — Modify (replace `starlarkToGo` → `op.UnmarshalToAny`)
- `internal/starlark/render.go` — Modify (replace both conversion functions)
- `internal/starlark/render_test.go` — Modify (update tests)
- `internal/starlark/wasm_receiver.go` — Modify (replace `starlarkToGo`)
- `internal/starlark/receiver_json.go` — Delete
- `internal/starlark/receiver_yaml.go` — Delete
- `internal/starlark/receiver_regexp.go` — Delete
- `internal/starlark/receiver_ui.go` — Delete

### Phase 6: All provider and plan operation tests pass (pending)

Fix all skipped provider and plan operation tests. No test skips allowed
except environment-conditional guards (PowerShell not installed, E2E_TEST
not set, etc.).

- [ ] Fix `TestChooseNotExists` — choose executor runs then-branch even when predicate is false
- [ ] Fix `TestIsFile` — choose executor passes empty path for Output captured in lambda closure
- [ ] Fix `TestFileJoin` — reflection bug: cannot use []string as variadic string in generated receiver
- [ ] Fix `TestIntegration` — issue #172
- [ ] Fix `TestStarcodeIntegration` — issue #173
- [ ] Remove all `t.Skip` calls for the above tests
- [ ] `make build` and `make test` pass with zero skipped provider/plan tests

**Files** (devlore-cli):

- `internal/execution/` — Modify (fix choose executor predicate evaluation)
- `pkg/op/receiver_reflect.go` — Modify (fix variadic []string reflection)
- `internal/e2e/testrunner/runner_test.go` — Modify (remove t.Skip calls)
- `internal/starlark/integration_test.go` — Modify (remove t.Skip)
- `pkg/op/provider/starcode/integration_test.go` — Modify (remove t.Skip)
- Additional files TBD based on root cause analysis

## Design Decisions

1. **Two-layer runtime: `op.StarlarkRuntime` + `internal/starlark.Runtime`.**
   The base runtime in `pkg/op/` handles receiver management, action registration, and
   context lifecycle — everything an external consumer needs. The internal `Runtime` (formerly
   `BindingSet`) embeds it and adds graph builder and module loading, which depend on internal
   types (`PlanRoot`, `collectPlannedProviders`) and devlore-specific conventions (`@devlore//`).

2. **`StarlarkRuntime.Initialize()` takes `ContextBase`, not full `Context`.**
   `ContextBase` includes only what every immediate environment needs (Root, DryRun, Platform,
   Writer, Data). `Context` embeds `ContextBase` and adds graph-specific fields. (Phase 2)

3. **No string fallback for unknown types.** `UnmarshalToAny` returns an error for
   unsupported Starlark types. noblefactor-ops callers must handle the error. The old
   `starlarkToGo` fallback (`v.String()`) silently loses type information.

4. **`marshal` handles `map[interface{}]interface{}` via generic `reflect.Map`.** Keys are
   marshaled to their natural Starlark type (not coerced to string). Behaviorally equivalent
   for YAML round-tripping since YAML keys are typically strings.

5. **UI `NewImmediate` wired via `+devlore:bind` directives.** Adding
   `+devlore:bind Writer=Writer, ProgramName=ProgramName, Color=Color` to the UI provider's
   doc comment causes the codegen to generate a `NewImmediate` that wires `BindingConfig`
   fields into the provider struct. No template change needed.

6. **Builder pattern for `BindingConfig`.** Consumers construct config via method chaining:
   `NewBindingConfig(programName).WithGraphBuilder().WithReceivers(...)`. ProgramName is the
   only required parameter. Writer defaults to os.Stderr. Go's line-continuation rules
   require trailing `.` on each chained line.

7. **Graph builder is a capability flag, not a receiver.** The `plan.*` namespace is not a
   provider — it's a graph construction capability containing control flow primitives
   (`plan.choose`, `plan.gather`, `plan.source`). It's enabled via `WithGraphBuilder()` on
   the config and `HasGraphBuilder()` on the runtime, not by including `"plan"` in the
   receiver list.

8. **Platform built internally.** `Platform` is removed from `BindingConfig`. The runtime
   calls `platform.New()` when needed. Consumers don't construct or pass platform objects.

9. **Generated code committed to git.** Gen packages are source — Go modules are source-only.
   `.gitignore`-ing gen packages makes them invisible to external consumers. CI validates
   freshness: run generator, `git diff --exit-code`.

10. **Typed provider references replace string names.** `Receivers` changes from `[]string`
    to `[]Provider`. Each gen package exports `var Receiver op.Provider`. A typo in a string
    is a silent runtime bug; a typo in an import path is a compile error.

11. **`reflect.Type` as announcement registry keys.** The provider announcement registry
    uses `reflect.Type` as map keys for type-safe, collision-free lookup.

12. **Vestigial hand-coding API deleted.** `NewReceiver`, `BuiltinFunc`, `MakeAttr` predate
    the codegen system. Codegen replaces all three. The only hand-coded receivers are control
    flow primitives in `PlanRoot`, which use `starlark.NewBuiltin` directly.

## Related Documents

- devlore-cli `pkg/op/binding_config.go` — `BindingConfig` builder pattern
- devlore-cli `pkg/op/starlark_runtime.go` — `StarlarkRuntime`
- devlore-cli `pkg/op/announce.go` — `Announce`/`InitAll`/`Providers`
- devlore-cli `pkg/op/receiver_reflect.go` — `WrapReceiver`, `SetContext`
- devlore-cli `pkg/op/starvalue_marshal.go` — `Marshal`/`UnmarshalToAny`
- devlore-cli `pkg/op/provider.go` — `ImmediateProvider` interface
- devlore-cli `internal/starlark/plan_root.go` — Control flow receivers
- noblefactor-ops `docs/plans/shared-provider-receivers-phase5-analysis.md` — Blocker analysis
