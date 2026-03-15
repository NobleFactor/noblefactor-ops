---
title: "Shared Provider Receivers"
status: in-progress
created: 2026-03-11
updated: 2026-03-15
---

# Plan: Shared Provider Receivers

## Summary

Replace hand-coded Starlark receivers in noblefactor-ops (JSON, YAML, Regexp, UI) with framework-managed receivers
from devlore-cli. Instead of manually calling `WrapProviderInExecutingReceiver`, noblefactor-ops uses
`op.StarlarkRuntime` — the same framework that devlore-cli uses to set up its own scripting runtimes.

This also exports the framework's Starlark-Go conversion functions (`Marshal`, `UnmarshalToAny`) so noblefactor-ops
can delete its parallel `starlarkToGo`/`goToStarlark` implementations.

## Goals

1. **Use the framework** — noblefactor-ops sets up an immediate scripting runtime through `pkg/op`, not by
   hand-rolling receiver construction
2. **Eliminate receiver duplication** — 4 hand-coded receivers (~595 lines) replaced by framework-managed equivalents
3. **Eliminate conversion duplication** — `starlarkToGo`/`goToStarlark` replaced by framework's exported
   `Marshal`/`UnmarshalToAny`
4. **Unify the Starlark API surface** — both tools expose the same parameter names
5. **Clean consumer API** — consumers pass typed provider references, not strings

## Current State

### noblefactor-ops receivers (hand-coded, to be replaced in Phase 5)

| File                 | Lines | Notes                                                                   |
| -------------------- | ----- | ----------------------------------------------------------------------- |
| `receiver_json.go`   | 90    | Hand-coded Attr/AttrNames, calls `starlarkToGo`/`goToStarlark`          |
| `receiver_yaml.go`   | 159   | Hand-coded, DEFINES `starlarkToGo`/`goToStarlark` used by 5 other files |
| `receiver_regexp.go` | 252   | Hand-coded, own `sync.Map` cache (devlore provider has identical cache) |
| `receiver_ui.go`     | 94    | Hand-coded, wraps `*ui.Provider`                                        |
| `receivers.go`       | 32    | Singleton construction                                                  |

### devlore-cli framework (ready for consumption)

**`pkg/op/provider.go`** — interfaces:

- `ReceiverFactory` — `GetOrCreateProvider`, `ReceiverName`, `ProviderType`, `Register`
- `ExecutingReceiverFactory` — `NewExecuting`
- `PlanningReceiverFactory` — `NewPlanning`

**`pkg/op/`** — exported functions:

- `RegisterActions` (`action_reflect.go`) — 3-arg: registry, factory, params
- `WrapProviderInExecutingReceiver` (`receiver_reflect.go`)
- `WrapProviderInPlanningReceiver` (`planned_reflect.go`)
- `Marshal` / `UnmarshalToAny` (`starvalue_marshal.go`)
- `Announce` / `InitAll` / `Providers` (`announce.go`) — `reflect.Type` keys

**`pkg/op/`** — exported types:

- `BindingConfig` (`binding_config.go`) — builder pattern
- `StarlarkRuntime` (`starlark_runtime.go`)

**`internal/starlark/runtime.go`** — embeds `*op.StarlarkRuntime`, adds graph builder + module loader

**`pkg/op/provider/*/gen/receiver.gen.go`** — generated, tracked in git, each exports
`var Receiver op.ReceiverFactory`

### Naming taxonomy (established in PR #216)

| Old                                              | New                                                  |
| ------------------------------------------------ | ---------------------------------------------------- |
| `op.Provider` (interface)                        | `op.ReceiverFactory`                                 |
| `op.ImmediateProvider`                           | `op.ExecutingReceiverFactory`                        |
| `op.PlannedProvider`                             | `op.PlanningReceiverFactory`                         |
| `var Descriptor op.Provider`                     | `var Receiver op.ReceiverFactory`                    |
| `Name()`                                         | `ReceiverName()`                                     |
| `NewImmediate(cfg)`                              | `NewExecuting(ctx)`                                  |
| `NewPlanned(graph, project, reg)`                | `NewPlanning(graph, project, registry)`              |
| `WrapReceiver(name, provider)`                   | `WrapProviderInExecutingReceiver(factory, provider)` |
| `WrapPlanned(name, type, ...)`                   | `WrapProviderInPlanningReceiver(factory, ...)`       |
| `RegisterReflectedActions(reg, name, p, params)` | `RegisterActions(registry, factory, params)`         |
| `InitProvider(p, ctx)`                           | _(removed — factory owns lifecycle)_                 |

## Architecture

### Two-layer runtime design

**`op.StarlarkRuntime`** (in `pkg/op/`) — the base runtime. An open set of receivers that external consumers load
into. Manages receiver construction, action registration, and context lifecycle. This is what noblefactor-ops uses —
it only needs immediate receivers.

**`internal/starlark.Runtime`** — embeds `*op.StarlarkRuntime`. Adds devlore-specific capabilities that depend on
internal types: the graph builder (`plan.*` namespace via `PlanRoot`) and the `@devlore//` module loader (with
caching). Used by lore (graph building), the e2e test runner, and any devlore CLI tool that executes `.star` scripts.

```
op.StarlarkRuntime             ← load receivers, manage lifecycle (exported)
  └─ internal/starlark.Runtime ← + graph builder, module loader (internal)
```

The split exists because `PlanRoot` and `collectPlannedProviders` depend on internal types that cannot move to
`pkg/op/`. The `@devlore//` module prefix is product-specific.

A third usage pattern — action registration without script execution (writ, some lore paths) — calls `op.InitAll`
directly and needs no runtime at all.

### Consumer API

Consumers construct a runtime via the `BindingConfig` builder:

```go
rt := loreStar.NewRuntime(
    op.NewBindingConfig("star").
        WithGraphBuilder().
        WithProviders(jsongen.Receiver, yamlgen.Receiver, uigen.Receiver).
        WithColor(),
)
```

- `NewBindingConfig(programName)` — required, creates config with `Writer` defaulting to `os.Stderr`
- `WithGraphBuilder()` — enables the `plan.*` graph namespace
- `WithProviders(...)` — typed `ReceiverFactory` references (compile-time checked)
- `WithWriter(w)` — override output destination
- `WithColor()` — enable ANSI color codes

`Platform` is built internally via `op.NewPlatform()` — not passed in config.

### Provider registration

Generated code follows Go ecosystem norms: generated files are committed to git. `go get` fetches source; consumers
import gen packages directly.

Each gen package exports `var Receiver op.ReceiverFactory` and calls `op.Announce()` in `init()`. The announcement
registry uses `reflect.Type` as keys for type-safe lookup.

```
gen/receiver.gen.go → var Receiver op.ReceiverFactory = &Factory{}
                    → init() → op.Announce(Receiver)
                                    ↓
                    op.InitAll() → Register() on each announced factory
                                    ↓
                    StarlarkRuntime.BuildReceivers() → ExecutingReceiverFactory.NewExecuting(ctx)
```

### Provider lifecycle

The `Factory` caches the provider per `Root` — singleton within a graph/runtime scope, invalidated on Root change.
This ensures providers are shared across action calls within an execution context but not across independent
executions.

```go
func (f *Factory) GetOrCreateProvider(ctx op.Context) op.ContextProvider {
    if f.provider == nil || f.root != ctx.Root {
        f.provider = provider.NewProvider(ctx)
        f.root = ctx.Root
    }
    return f.provider
}
```

### Control flow receivers

The `plan.*` namespace contains graph construction primitives (`plan.choose`, `plan.source`, `plan.gather`) that are
inherently hand-coded in `PlanRoot`. They manipulate `*op.Graph`, `*op.Node`, `*op.Output` directly — they don't map
to a Go struct with methods. These are the only receivers that do not go through codegen.

### Internal receiver plumbing

`newReceiver` and `builtinFunc` in `pkg/op/receiver.go` are internal (unexported) plumbing used by
`WrapProviderInExecutingReceiver` and `buildMethodBridge`. They are not part of the public API. `MakeAttr` has been
deleted — callers use `starlark.NewBuiltin` directly.

## Implementation Phases

### Phase 1: Rename regexp param in devlore-cli (complete)

- [x] Rename `s` → `text` in `pkg/op/provider/regexp/provider.go` (all 8 methods)
- [x] Update `test_regexp.star` and `test_imm_regexp.star` keyword args
- [x] `make build` and `make test` pass

### Phase 2: Extract `ContextBase` from `Context` (complete)

- [x] Create `ContextBase` with fields needed by every immediate environment
- [x] Refactor `Context` to embed `ContextBase`, keeping graph-specific fields
- [x] Update all code that accesses base fields
- [x] Update codegen template and regenerate all gen test files
- [x] `make build` and `make test` pass

### Phase 3: Extract `StarlarkRuntime` and rename `BindingSet` (complete)

- [x] Create `pkg/op/starlark_runtime.go` with `StarlarkRuntime`
- [x] Rename `BindingSet` → `Runtime` in `internal/starlark/`
- [x] Fix UI `NewExecuting` to wire `BindingConfig` fields via `+devlore:bind`
- [x] Update all call sites
- [x] `make build` and `make test` pass

### Phase 4: Export Starlark-Go conversion functions (complete)

- [x] Export `unmarshalToAny` as `UnmarshalToAny`
- [x] Export `marshal` as `Marshal`
- [x] Update all internal call sites
- [x] `make build` and `make test` pass

### Phase 4a: BindingConfig builder pattern + ReceiverFactory taxonomy (complete)

Redesign `BindingConfig` with builder pattern, refactor the entire provider/receiver API to the `ReceiverFactory`
taxonomy, update codegen templates, regenerate all providers.

- [x] Builder pattern: `NewBindingConfig(programName)` with `WithGraphBuilder()`, `WithProviders(...)`,
      `WithWriter(w)`, `WithColor()`
- [x] Replace `"plan"` string with `WithGraphBuilder()` boolean
- [x] Remove `Platform` from config (built internally)
- [x] Default `Writer` to `os.Stderr`
- [x] `Provider` → `ReceiverFactory` with `GetOrCreateProvider`, `ReceiverName`, `ProviderType`
- [x] `ImmediateProvider` → `ExecutingReceiverFactory` with `NewExecuting`
- [x] `PlannedProvider` → `PlanningReceiverFactory` with `NewPlanning`
- [x] `actionBase`: `provider reflect.Value` → `factory ReceiverFactory` + `getProvider(ctx)`
- [x] `RegisterReflectedActions(reg, name, p, params)` → `RegisterActions(registry, factory, params)`
- [x] `WrapReceiver(name, p)` → `WrapProviderInExecutingReceiver(factory, p)`
- [x] `WrapPlanned(name, type, ...)` → `WrapProviderInPlanningReceiver(factory, ...)`
- [x] Removed `InitProvider`, `InitActionProvider`
- [x] `BindingConfig.Providers` from `[]string` to `[]ReceiverFactory`
- [x] `WithReceivers(...)` → `WithProviders(...)`
- [x] `reflect.Type` as announcement registry keys
- [x] Deleted `MakeAttr`
- [x] Platform provider moved from `pkg/op/provider/platform/` to `pkg/op/`
- [x] Pointer-receiver method lookup: `reflect.PointerTo(factory.ProviderType())`
- [x] Provider singleton scoping: Factory caches per `Root`
- [x] Codegen templates renamed to match output filenames
- [x] `planned_receiver.go.template` and `immediate_receiver.go.template` absorbed into `receiver.gen.go.template`
- [x] Gen template exports `var Receiver op.ReceiverFactory = &Factory{}`
- [x] Removed `gen/` from `.gitignore`, committed all gen files
- [x] All 19 providers regenerated from new templates
- [x] `make build` and `make test` pass

**PRs** (devlore-cli): #210, #211, #212, #213, #216

### Phase 4.5: Cleanup after Claude edits (complete)

Bring every file touched in PR #216 into full compliance with the Go style guidelines. Fix the codegen templates
first (one template fix = all 19 providers fixed on regeneration), then audit and fix every hand-edited file.

**Generated files** (via template fixes + regeneration):

- `pkg/op/provider/appnet/gen/actions_gen_test.go`
- `pkg/op/provider/appnet/gen/params.gen.go`
- `pkg/op/provider/appnet/gen/receiver.gen.go`
- `pkg/op/provider/appnet/gen/resource.gen.go`
- `pkg/op/provider/archive/gen/actions_gen_test.go`
- `pkg/op/provider/archive/gen/params.gen.go`
- `pkg/op/provider/archive/gen/receiver.gen.go`
- `pkg/op/provider/encryption/gen/actions_gen_test.go`
- `pkg/op/provider/encryption/gen/params.gen.go`
- `pkg/op/provider/encryption/gen/receiver.gen.go`
- `pkg/op/provider/file/gen/actions_gen_test.go`
- `pkg/op/provider/file/gen/params.gen.go`
- `pkg/op/provider/file/gen/receiver.gen.go`
- `pkg/op/provider/file/gen/resource.gen.go`
- `pkg/op/provider/git/gen/actions_gen_test.go`
- `pkg/op/provider/git/gen/params.gen.go`
- `pkg/op/provider/git/gen/receiver.gen.go`
- `pkg/op/provider/git/gen/resource.gen.go`
- `pkg/op/provider/json/gen/actions_gen_test.go`
- `pkg/op/provider/json/gen/params.gen.go`
- `pkg/op/provider/json/gen/receiver.gen.go`
- `pkg/op/provider/json/gen/receiver_gen_test.go`
- `pkg/op/provider/mem/gen/resource.gen.go`
- `pkg/op/provider/pkg/gen/actions_gen_test.go`
- `pkg/op/provider/pkg/gen/params.gen.go`
- `pkg/op/provider/pkg/gen/receiver.gen.go`
- `pkg/op/provider/pkg/gen/resource.gen.go`
- `pkg/op/provider/regexp/gen/actions_gen_test.go`
- `pkg/op/provider/regexp/gen/params.gen.go`
- `pkg/op/provider/regexp/gen/receiver.gen.go`
- `pkg/op/provider/regexp/gen/receiver_gen_test.go`
- `pkg/op/provider/service/gen/actions_gen_test.go`
- `pkg/op/provider/service/gen/params.gen.go`
- `pkg/op/provider/service/gen/receiver.gen.go`
- `pkg/op/provider/service/gen/resource.gen.go`
- `pkg/op/provider/shell/gen/actions_gen_test.go`
- `pkg/op/provider/shell/gen/params.gen.go`
- `pkg/op/provider/shell/gen/receiver.gen.go`
- `pkg/op/provider/staranalysis/gen/params.gen.go`
- `pkg/op/provider/staranalysis/gen/receiver.gen.go`
- `pkg/op/provider/staranalysis/gen/receiver_gen_test.go`
- `pkg/op/provider/starcode/gen/params.gen.go`
- `pkg/op/provider/starcode/gen/receiver.gen.go`
- `pkg/op/provider/starcode/gen/receiver_gen_test.go`
- `pkg/op/provider/starcomplexity/gen/params.gen.go`
- `pkg/op/provider/starcomplexity/gen/receiver.gen.go`
- `pkg/op/provider/starcomplexity/gen/receiver_gen_test.go`
- `pkg/op/provider/starindex/gen/params.gen.go`
- `pkg/op/provider/starindex/gen/receiver.gen.go`
- `pkg/op/provider/starindex/gen/receiver_gen_test.go`
- `pkg/op/provider/starstats/gen/params.gen.go`
- `pkg/op/provider/starstats/gen/receiver.gen.go`
- `pkg/op/provider/starstats/gen/receiver_gen_test.go`
- `pkg/op/provider/template/gen/actions_gen_test.go`
- `pkg/op/provider/template/gen/params.gen.go`
- `pkg/op/provider/template/gen/receiver.gen.go`
- `pkg/op/provider/template/gen/receiver_gen_test.go`
- `pkg/op/provider/ui/gen/params.gen.go`
- `pkg/op/provider/ui/gen/receiver.gen.go`
- `pkg/op/provider/ui/gen/receiver_gen_test.go`
- `pkg/op/provider/yaml/gen/actions_gen_test.go`
- `pkg/op/provider/yaml/gen/params.gen.go`
- `pkg/op/provider/yaml/gen/receiver.gen.go`
- `pkg/op/provider/yaml/gen/receiver_gen_test.go`

**Codegen templates** (source of generated file fixes):

- `star/extensions/com.noblefactor.devlore.Actions/templates/receiver.gen.go.template`
- `star/extensions/com.noblefactor.devlore.Actions/templates/params.gen.go.template`
- `star/extensions/com.noblefactor.devlore.Actions/templates/actions_gen_test.go.template`
- `star/extensions/com.noblefactor.devlore.Actions/templates/receiver_gen_test.go.template`
- `star/extensions/com.noblefactor.devlore.Actions/templates/resource.gen.go.template`
- `star/extensions/com.noblefactor.devlore.Actions/commands/generate.star`

**Hand-edited pkg/op files** (audit + fix each):

- `pkg/op/action_reflect.go`
- `pkg/op/action_reflect_test.go`
- `pkg/op/announce.go`
- `pkg/op/announce_test.go`
- `pkg/op/binding_config.go`
- `pkg/op/context.go`
- `pkg/op/convert_test.go`
- `pkg/op/graph_test.go`
- `pkg/op/lifetime.go`
- `pkg/op/output_test.go`
- `pkg/op/planned_reflect.go`
- `pkg/op/planned_reflect_test.go`
- `pkg/op/platform.go`
- `pkg/op/platform_darwin.go`
- `pkg/op/platform_darwin_panic.go`
- `pkg/op/platform_helpers.go`
- `pkg/op/platform_linux.go`
- `pkg/op/platform_linux_panic.go`
- `pkg/op/platform_new.go`
- `pkg/op/platform_test.go`
- `pkg/op/platform_windows.go`
- `pkg/op/platform_windows_panic.go`
- `pkg/op/provider.go`
- `pkg/op/receiver.go`
- `pkg/op/receiver_reflect.go`
- `pkg/op/receiver_reflect_test.go`
- `pkg/op/receiver_test.go`
- `pkg/op/registry_test.go`
- `pkg/op/resource.go`
- `pkg/op/resource_catalog.go`
- `pkg/op/root_test.go`
- `pkg/op/starlark_runtime.go`
- `pkg/op/starvalue_marshal.go`
- `pkg/op/starvalue_marshal_test.go`

**Hand-edited provider files** (audit + fix each):

- `pkg/op/provider/appnet/provider.go`
- `pkg/op/provider/archive/provider.go`
- `pkg/op/provider/encryption/provider.go`
- `pkg/op/provider/file/provider.go`
- `pkg/op/provider/file/provider_test.go`
- `pkg/op/provider/git/provider.go`
- `pkg/op/provider/json/provider.go`
- `pkg/op/provider/mem/callable.go`
- `pkg/op/provider/mem/callable_test.go`
- `pkg/op/provider/mem/extract.go`
- `pkg/op/provider/mem/extract_test.go`
- `pkg/op/provider/pkg/provider.go`
- `pkg/op/provider/pkg/provider_test.go`
- `pkg/op/provider/pkg/resource.go`
- `pkg/op/provider/pkg/resource_test.go`
- `pkg/op/provider/platform/provider.go`
- `pkg/op/provider/platform/helpers.go`
- `pkg/op/provider/platform/provider_darwin.go`
- `pkg/op/provider/platform/provider_darwin_panic.go`
- `pkg/op/provider/platform/provider_linux.go`
- `pkg/op/provider/platform/provider_linux_panic.go`
- `pkg/op/provider/platform/provider_test.go`
- `pkg/op/provider/platform/provider_windows.go`
- `pkg/op/provider/platform/provider_windows_panic.go`
- `pkg/op/provider/regexp/provider.go`
- `pkg/op/provider/service/provider.go`
- `pkg/op/provider/service/provider_test.go`
- `pkg/op/provider/shell/provider.go`
- `pkg/op/provider/staranalysis/provider.go`
- `pkg/op/provider/staranalysis/provider_test.go`
- `pkg/op/provider/starcode/integration_test.go`
- `pkg/op/provider/starcode/provider.go`
- `pkg/op/provider/starcode/receiver_test.go`
- `pkg/op/provider/starcomplexity/provider.go`
- `pkg/op/provider/starindex/provider.go`
- `pkg/op/provider/starstats/provider.go`
- `pkg/op/provider/template/provider.go`
- `pkg/op/provider/ui/provider.go`
- `pkg/op/provider/yaml/provider.go`

**Hand-edited internal/cmd/prototype files** (audit + fix each):

- `cmd/indexgen/main.go`
- `cmd/indexgen/main_test.go`
- `internal/cli/output.go`
- `internal/cli/viper.go`
- `internal/config/config_test.go`
- `internal/console/model.go`
- `internal/credentials/helper.go`
- `internal/devloretest/commands.go`
- `internal/devloretest/commands_test.go`
- `internal/e2e/migrate_test.go`
- `internal/e2e/testrunner/runner.go`
- `internal/e2e/testrunner/runner_test.go`
- `internal/execution/compensation_test.go`
- `internal/execution/executor.go`
- `internal/execution/flow/flow_test.go`
- `internal/execution/flow/planned.go`
- `internal/execution/flow/provider.go`
- `internal/execution/flow_test.go`
- `internal/lore/builder.go`
- `internal/lore/commands.go`
- `internal/lorepackage/action_test.go`
- `internal/lorepackage/package.go`
- `internal/lorepackage/package_test.go`
- `internal/lorepackage/registry_test.go`
- `internal/lorepackage/search.go`
- `internal/lorepackage/search_test.go`
- `internal/model/provider.go`
- `internal/pwsh/pwsh.go`
- `internal/signing/gcp_kms_test.go`
- `internal/starlark/integration_test.go`
- `internal/starlark/interfaces_test.go`
- `internal/starlark/package.go`
- `internal/starlark/plan_root.go`
- `internal/starlark/receiver_test.go`
- `internal/starlark/runtime.go`
- `internal/starlark/runtime_test.go`
- `internal/tools/docgen/generator_test.go`
- `internal/tools/docgen/template.go`
- `internal/writ/commands.go`
- `internal/writ/graph_builder.go`
- `internal/writ/migrate/execute.go`
- `internal/writ/migrate/migrate_test.go`
- `internal/writ/migrate/plan.go`
- `internal/writ/migrate/session.go`
- `internal/writ/migrate/session_test.go`
- `internal/writ/segment/segment_test.go`
- `prototype/bindgen/internal/codegen.go`
- `prototype/bindgen/internal/schema_test.go`
- `prototype/bindgen/internal/stubgen.go`

### Phase 5: Use framework in noblefactor-ops (complete)

Replace hand-coded receivers with framework-managed receivers. Replace `starlarkToGo`/`goToStarlark` with exported
framework equivalents.

- [ ] Import devlore-cli gen packages to get `ReceiverFactory` references
- [ ] Create `StarlarkRuntime` via builder:
      `op.NewBindingConfig("star").WithProviders(jsongen.Receiver, yamlgen.Receiver, ...).WithColor()`
- [ ] Call `BuildReceivers()` in `buildPredeclared()`, merge with custom receivers
- [ ] Replace `starlarkToGo` calls with `op.UnmarshalToAny` (in `receiver_schema.go`, `render.go`,
      `wasm_receiver.go`)
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

### Phase 6: Variadic reflection, choose executor branch isolation (complete)

Fix variadic parameter support and choose executor branch isolation.

- [x] Fix `TestChooseNotExists` — branch phases marked `Branch=true`, skipped by top-level executor
- [x] Fix `TestIsFile` — choose seeds branch results from `ctx.Results` for cross-phase promise resolution
- [x] Fix `TestFileJoin` — strip `*` prefix in action/planned paths, use `CallSlice` for variadic methods
- [x] `make build` and `make test` pass

**PR #219** (devlore-cli): variadic reflection, choose executor branch isolation

### Phase 7: Dependent type codegen and t.Skip elimination (pending)

Implement codegen for dependent types (structs returned by provider methods that have their own exported
methods). Today the only dependent type is `starcode.Sources`. Then eliminate all remaining `t.Skip` calls.

- [ ] Implement dependent type codegen in `generate.star` (currently skipped with TODO)
- [ ] Create dependent type HasAttrs wrapper template
- [ ] Regenerate starcode — produces `Sources` wrapper in `gen/`
- [ ] Fix `TestIntegration` — issue #172
- [ ] Fix `TestStarcodeIntegration` — issue #173
- [ ] Fix 3 starcode `receiver_test.go` tests
- [ ] Remove all remaining `t.Skip` calls (replace with `t.Fatalf` where env-conditional)
- [ ] `make build` and `make test` pass with zero `t.Skip` calls

**Files** (devlore-cli):

- `star/extensions/com.noblefactor.devlore.Actions/commands/generate.star` — implement dependent type codegen
- New template for dependent type HasAttrs wrappers
- `pkg/op/provider/starcode/gen/` — regenerated with Sources wrapper
- `internal/starlark/integration_test.go` — remove t.Skip
- `pkg/op/provider/starcode/integration_test.go` — remove t.Skip
- `pkg/op/provider/starcode/receiver_test.go` — remove 3 t.Skip calls
- All files with remaining `t.Skip` calls

## Design Decisions

1. **Two-layer runtime: `op.StarlarkRuntime` + `internal/starlark.Runtime`.** The base runtime in `pkg/op/` handles
   receiver management, action registration, and context lifecycle — everything an external consumer needs. The
   internal `Runtime` (formerly `BindingSet`) embeds it and adds graph builder and module loading, which depend on
   internal types (`PlanRoot`, `collectPlannedProviders`) and devlore-specific conventions (`@devlore//`).

2. **`StarlarkRuntime.Initialize()` takes `ContextBase`, not full `Context`.** `ContextBase` includes only what
   every immediate environment needs (Root, DryRun, Platform, Writer, Data). `Context` embeds `ContextBase` and adds
   graph-specific fields. (Phase 2)

3. **No string fallback for unknown types.** `UnmarshalToAny` returns an error for unsupported Starlark types.
   noblefactor-ops callers must handle the error. The old `starlarkToGo` fallback (`v.String()`) silently loses type
   information.

4. **`Marshal` handles `map[interface{}]interface{}` via generic `reflect.Map`.** Keys are marshaled to their natural
   Starlark type (not coerced to string). Behaviorally equivalent for YAML round-tripping since YAML keys are
   typically strings.

5. **UI `NewExecuting` wired via `+devlore:bind` directives.** Adding `+devlore:bind Writer=Writer,
ProgramName=ProgramName, Color=Color` to the UI provider's doc comment causes the codegen to generate a
   `NewExecuting` that wires `BindingConfig` fields into the provider struct. No template change needed.

6. **Builder pattern for `BindingConfig`.** Consumers construct config via method chaining:
   `NewBindingConfig(programName).WithGraphBuilder().WithProviders(...)`. ProgramName is the only required parameter.
   Writer defaults to os.Stderr.

7. **Graph builder is a capability flag, not a receiver.** The `plan.*` namespace is not a provider — it's a graph
   construction capability containing control flow primitives (`plan.choose`, `plan.gather`, `plan.source`). It's
   enabled via `WithGraphBuilder()` on the config and `HasGraphBuilder()` on the runtime, not by including `"plan"`
   in the receiver list.

8. **Platform built internally.** `Platform` is removed from `BindingConfig`. The runtime calls `op.NewPlatform()`
   when needed. Consumers don't construct or pass platform objects.

9. **Generated code committed to git.** Gen packages are source — Go modules are source-only. `.gitignore`-ing gen
   packages makes them invisible to external consumers. CI validates freshness: run generator, `git diff
--exit-code`.

10. **Typed provider references replace string names.** `WithProviders` accepts `...ReceiverFactory`. Each gen
    package exports `var Receiver op.ReceiverFactory`. A typo in a string is a silent runtime bug; a typo in an
    import path is a compile error.

11. **`reflect.Type` as announcement registry keys.** The provider announcement registry uses `reflect.Type` as map
    keys for type-safe, collision-free lookup.

12. **Root-scoped provider singleton.** The `Factory` caches the provider keyed by `Root`. Same Root = same provider
    (shared within a graph/runtime). Different Root = fresh provider (no stale state across executions).

13. **ReceiverFactory taxonomy.** `Provider` was overloaded (the Go struct, the interface, the gen export). The new
    naming: `ReceiverFactory` (interface), `Factory` (gen struct), `Receiver` (gen export), `Provider` (domain struct
    with methods).

## Related Documents

- devlore-cli `pkg/op/binding_config.go` — `BindingConfig` builder pattern
- devlore-cli `pkg/op/starlark_runtime.go` — `StarlarkRuntime`
- devlore-cli `pkg/op/announce.go` — `Announce`/`InitAll`/`Providers`
- devlore-cli `pkg/op/receiver_reflect.go` — `WrapProviderInExecutingReceiver`, `SetContext`
- devlore-cli `pkg/op/planned_reflect.go` — `WrapProviderInPlanningReceiver`
- devlore-cli `pkg/op/action_reflect.go` — `RegisterActions`
- devlore-cli `pkg/op/starvalue_marshal.go` — `Marshal`/`UnmarshalToAny`
- devlore-cli `pkg/op/provider.go` — `ReceiverFactory`, `ExecutingReceiverFactory`, `PlanningReceiverFactory`
- devlore-cli `internal/starlark/plan_root.go` — Control flow receivers
- devlore-cli `docs/plans/shared-provider-receivers.md` — devlore-cli side implementation details
- noblefactor-ops `docs/plans/shared-provider-receivers-phase5-analysis.md` — Blocker analysis
