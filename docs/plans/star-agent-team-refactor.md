---
title: "Star Agent Team Refactoring"
description: "Multi-agent implementation of the star extension model"
status: draft
created: 2025-02-08
updated: 2025-02-08
---

# Plan: Star Agent Team Refactoring

## Summary

Implement the full star extension model using a team of Claude Code agents working on separate packages. Each agent owns specific files and packages, with the Lead coordinating integration.

## Decisions

1. **Scope:** Full extension model (all 6 phases)
2. **Distill:** Rename of "knowledge" command - codebase knowledge extraction for LLMs
3. **Coordination:** Separate branches per package with strict file boundaries
4. **Extension package:** `internal/extension/`

## Model Allocation

| Agent | Model | Rationale |
|-------|-------|-----------|
| Lead | opus | Architectural decisions, integration coordination |
| Worker 1 (Config) | sonnet | Moderate complexity, well-defined patterns |
| Worker 2 (Starlark) | opus | Complex runtime integration, Starlark internals |
| Worker 3 (Extension) | sonnet | Spec parsing is straightforward |
| Worker 4 (Commands) | haiku | Pattern-based YAML/Starlark, low complexity |
| Worker 5 (Wasm) | opus | Novel Wasm integration, complex host callbacks |

**Estimated token usage:** 2-4M tokens across all phases (conservative: 1-2M, worst case: 5-8M)

---

## Extension Model

An extension can define **zero or more commands** and **zero or more binding functions**. All binding functions are defined using the receiver API.

### Extension Distribution

Extensions are distributed as **WebAssembly modules** for cross-platform compatibility. Wasm solves historical pain points of traditional plugin systems—security risks from third-party code and ABI fragility of native shared libraries.

**Why WebAssembly:**

| Benefit | Description |
|---------|-------------|
| Sandboxed Security | Extensions run isolated; no access to fs/network/env unless explicitly granted |
| Language Agnostic | Authors write in Rust, Go, C++, or TypeScript (AssemblyScript) |
| Single Artifact | One `.wasm` file works on all platforms (no per-OS binaries) |
| Near-Native Performance | ~1.1-1.5x native for compute-bound code |
| Instant Startups | Microsecond cold-start latency, critical for CLI tools |
| No CGO | Pure Go runtime simplifies builds and distribution |

**Real-World Examples:**

| Project | Use Case |
|---------|----------|
| [sqlc](https://sqlc.dev/) | Go-based SQL compiler with Wasm code generation plugins |
| [moonrepo](https://moonrepo.dev/) | Build tool with Wasm plugin system |
| [wasmCloud](https://wasmcloud.com/) | CLI built around Wasm component plugins |

**Runtime Options:**

| Runtime | Pros | Cons |
|---------|------|------|
| [wazero](https://wazero.io/) | Pure Go, no CGO, simple embedding | Newer ecosystem |
| [Wasmtime](https://wasmtime.dev/) | Mature, fast, broad adoption | Requires CGO |
| [Extism](https://extism.org/) | Universal plugin SDK with PDKs | Additional abstraction |

**Current choice:** wazero for zero-dependency Go builds. Extism may be considered later for multi-language authoring SDKs.

### Extension Types

| Type | Commands | Bindings | Example |
|------|----------|----------|---------|
| Command-only | 1+ | 0 | `lint.all` - orchestrates other linters |
| Binding-only | 0 | 1+ | `copyright` - provides primitives for other extensions |
| Full | 1+ | 1+ | `lint.copyright` - has command AND binding functions |
| Built-in | varies | varies | Core extensions compiled into star binary |

### Project Structure

```
project/
├── cmd/star/                    # CLI entry point
│   └── main.go
│
├── internal/
│   ├── config/                  # Configuration system
│   │   ├── config.go           # Config loading (star/config.yaml)
│   │   └── starlark.go         # Starlark adapter
│   │
│   ├── extension/               # Extension loading
│   │   ├── spec.go             # ExtensionSpec types
│   │   ├── registry.go         # Extension registry
│   │   └── discovery.go        # Directory scanning (star/extensions/)
│   │
│   ├── wasm/                    # Wasm host runtime
│   │   ├── host.go             # wazero host setup
│   │   └── receiver.go         # WasmReceiver implementation
│   │
│   ├── starlark/                # Starlark runtime
│   │   ├── runtime.go          # Script execution, extension loading
│   │   ├── receiver.go         # Receiver API
│   │   ├── command.go          # Command types
│   │   └── receiver_*.go       # Built-in receivers
│   │
│   └── cli/                     # Output formatting
│
└── star/                         # Star project directory
    ├── config.yaml              # Project configuration
    └── extensions/              # Extension packages
        ├── com.noblefactor.star.LintCopyright/
        │   ├── extension.yaml   # Single source of truth
        │   └── commands/
        │       └── lint-copyright.star  # Just defines run(ctx)
        │
        ├── com.noblefactor.star.LintGo/
        │   ├── extension.yaml
        │   └── commands/
        │       └── lint-go.star
        │
        └── ...
```

**Key design principles:**
- `extension.yaml` is the single source of truth for command metadata
- `.star` files only define a `run(ctx)` function (no `command()` registration)
- Config lives in `star/config.yaml` (not `star.yaml`)
- Extensions live in `star/extensions/` (not `extensions/`)

### Extension Specification Format

```yaml
# extensions/lint-copyright/extension.yaml
extension: lint.copyright
description: "Check or fix copyright headers in source files"

# Receivers this extension provides (binding functions)
# Can be built-in (compiled into star) or wasm (loaded at runtime)
receivers:
  - name: copyright
    wasm: lint-copyright.wasm    # Wasm module providing bindings
    functions:
      - check: "Verify files have correct headers"
      - fix: "Add or update headers"
      - detect_license: "Detect SPDX from LICENSE file"

# Capabilities required by this extension (sandboxing)
capabilities:
  fs:
    read: ["/workspace"]         # Read project files
    write: ["/workspace"]        # Write fixes
  host_calls: []                 # No external tool calls needed

# Command this extension provides
command:
  help: |
    Check or fix copyright headers.
  implementation: lint-copyright.star

flags:
  - name: fix
    type: bool
    default: false
    help: Add missing headers

# Configuration schema
config:
  type: CopyrightConfig
  fields:
    enabled: bool
    license: string
    holder: string
  defaults:
    enabled: false
    license: "auto"
```

### Built-in Extension Example

Core extensions compiled into the star binary (no Wasm):

```yaml
# extensions/lint-go/extension.yaml
extension: lint.go
description: "Run Go linters via golangci-lint"

receivers:
  - name: go
    builtin: true                # Compiled into star binary
    type: GoRunner               # Go type in internal/starlark/

capabilities:
  fs:
    read: ["/workspace"]
  host_calls:
    - shell.run                  # Needs to run golangci-lint

command:
  help: "Run Go linters"
  implementation: lint-go.star
```

### Third-Party Extension Example (Wasm)

```yaml
# extensions/custom-linter/extension.yaml
extension: custom.linter
description: "Custom code analysis"

receivers:
  - name: customlint
    wasm: customlint.wasm
    functions:
      - analyze
      - report

capabilities:
  fs:
    read: ["/workspace"]
    write: []                    # Read-only
  host_calls: []                 # Pure analysis, no external tools

command:
  help: "Run custom analysis"
  implementation: custom-linter.star
```

### Sandboxing Model

Extensions run in a WASI sandbox with declared capabilities:

| Capability | Description | Granted By |
|------------|-------------|------------|
| `fs.read` | Read files in listed directories | Host validates paths |
| `fs.write` | Write files in listed directories | Host validates paths |
| `host_calls.shell.run` | Execute shell commands | Host executes, returns result |
| `host_calls.http.get` | Make HTTP requests | Host executes, returns result |

**Host Callback Protocol:**
```
Extension (Wasm) → Host Request → Host Validates → Host Executes → Result
```

Extensions cannot:
- Access files outside granted directories
- Spawn processes directly
- Make network requests directly
- Access environment variables not explicitly passed

---

## Package Structure

| Package | Location | Purpose | Owner |
|---------|----------|---------|-------|
| `cmd/star` | `cmd/star/` | CLI entry point, command discovery | Lead |
| `config` | `internal/config/` | Configuration hierarchy, type generation | Worker 1 |
| `starlark` | `internal/starlark/` | Starlark runtime, receiver API, builtins | Worker 2 |
| `extension` | `internal/extension/` | Extension spec parsing, registry, discovery | Worker 3 |
| `wasm` | `internal/wasm/` | Wasm host runtime, host callbacks, capabilities | Worker 5 |
| `cli` | `internal/cli/` | Output formatting | (unchanged) |

### Command Areas

- **config** - Configuration management commands
- **distill** - Codebase knowledge extraction for LLMs (rename from "knowledge")
- **lint** - Quality gate commands (go, shell, markdown, copyright, starlark, all)
- **self** - Self-install, upgrade, version commands

---

## Agent Team

### Lead Agent
**Branch:** `feat/ext-lead-integration`
**Owned Files:**
- `cmd/star/main.go` - Entry point modifications
- `cmd/star/config.go` - Configuration initialization
- Integration tests in `tests/integration/`

**Responsibilities:**
- Coordinate interface contracts between agents
- Review PRs from all workers before merging
- Manage merge order and resolve conflicts
- Run integration tests after each phase

### Worker 1: Config Package
**Branch:** `feat/ext-config-phase-{N}`
**Owned Files (exclusive):**
- `internal/config/element.go` (NEW) - ConfigElement base type
- `internal/config/root.go` (NEW) - Config root with Load/Save
- `internal/config/types.go` (NEW) - Runtime type generation
- `internal/config/accessor.go` (NEW) - Typed field access
- `internal/config/starlark.go` (MODIFY) - ConfigValue adapter
- Legacy files (DELETE in Phase 5): `schema.go`, `value.go`, `registry.go`, `loader.go`, `extensions.go`

### Worker 2: Starlark Package
**Branch:** `feat/ext-starlark-phase-{N}`
**Owned Files (exclusive):**
- `internal/starlark/runtime.go` (MODIFY)
- `internal/starlark/builtin_config.go` (MODIFY)
- `internal/starlark/command.go` (NEW) - Extracted from runtime.go
- `internal/starlark/receiver.go` (MODIFY)
- `internal/starlark/builtin_*.go` (MODIFY for receiver migration)

### Worker 3: Extension Package
**Branch:** `feat/ext-extension-phase-{N}`
**Owned Files (exclusive):**
- `internal/extension/spec.go` (NEW) - ExtensionSpec struct
- `internal/extension/registry.go` (NEW) - Extension registry
- `internal/extension/discovery.go` (NEW) - Directory scanning

### Worker 4: Extensions and Commands
**Branch:** `feat/ext-commands-phase-{N}`
**Owned Files (exclusive):**
- `extensions/*/extension.yaml` (NEW) - All extension specs
- `extensions/*/*.star` (NEW) - Starlark implementations
- `ops/*.star` (MODIFY) - Update to extension pattern
- `docs/guides/writing-extensions.md` (NEW)

### Worker 5: Wasm Runtime
**Branch:** `feat/ext-wasm-phase-{N}`
**Owned Files (exclusive):**
- `internal/wasm/host.go` (NEW) - wazero host setup
- `internal/wasm/callbacks.go` (NEW) - Host callback implementations (shell.run, http.get)
- `internal/wasm/capabilities.go` (NEW) - Capability validation and enforcement
- `internal/wasm/protocol.go` (NEW) - Extension ↔ Host communication protocol

---

## Interface Contracts

### Contract 1: Config Registration (Worker 3 → Worker 1)
```go
// Worker 1 implements, Worker 3 consumes
type ConfigSpec struct {
    Type     string
    Fields   map[string]string
    Defaults map[string]any
}

func (c *Config) RegisterExtension(path string, spec ConfigSpec) error
func (c *Config) Navigate(path string) interface{}
```

### Contract 2: Starlark Config (Worker 1 → Worker 2)
```go
// Worker 1 implements, Worker 2 consumes
type ConfigValue struct { elem interface{} }
func (v *ConfigValue) Attr(name string) (starlark.Value, error)
func ToStarlark(elem interface{}) starlark.Value
```

### Contract 3: Extension Spec (Worker 3 → Worker 4)
```go
// Worker 3 implements, Worker 4 uses for YAML validation
type ExtensionSpec struct {
    Extension string      `yaml:"extension"`
    Command   CommandSpec `yaml:"command"`
    Flags     []FlagSpec  `yaml:"flags"`
    Config    ConfigSpec  `yaml:"config"`
}
func ParseSpec(yamlPath string) (*ExtensionSpec, error)
```

### Contract 4: Runtime Extension Loading (Worker 3 → Worker 2)
```go
// Worker 3 implements, Worker 2 consumes
func All() map[string]*ExtensionSpec
func Get(name string) *ExtensionSpec
```

### Contract 5: Wasm Host Runtime (Worker 5 → Worker 3)
```go
// Worker 5 implements, Worker 3 consumes
// Uses extension.Capabilities from internal/extension/spec.go
type WasmHost struct { ... }
type WasmModule struct { ... }

func NewHost(ctx context.Context, caps extension.Capabilities) (*WasmHost, error)
func (h *WasmHost) LoadModule(wasmPath string) (*WasmModule, error)
func (h *WasmHost) Close() error

// Call is on WasmModule for idiomatic usage
func (m *WasmModule) Call(ctx context.Context, function string, args []byte) ([]byte, error)
func (m *WasmModule) Path() string
func (m *WasmModule) ExportedFunctions() []string
```

### Contract 6: Host Callbacks (Worker 5 implements)
```go
// Host callback functions available to Wasm extensions
type HostCallbacks interface {
    ShellRun(cmd string, args []string, dir string) (stdout, stderr string, exitCode int, err error)
    HTTPGet(url string, headers map[string]string) (body []byte, statusCode int, err error)
    FSRead(path string) ([]byte, error)   // Within granted paths
    FSWrite(path string, data []byte) error // Within granted paths
}
```

### Contract 7: Capabilities (Worker 3 defines, Worker 5 validates)
```go
// Capabilities and FSCapabilities are defined in internal/extension/spec.go
// Worker 5 provides validation via CapabilityChecker wrapper

// wasm.ValidateCapabilities validates capability declarations
func ValidateCapabilities(caps extension.Capabilities) error

// wasm.CapabilityChecker wraps capabilities for access checking
type CapabilityChecker struct { ... }

func NewCapabilityChecker(caps extension.Capabilities) *CapabilityChecker
func (c *CapabilityChecker) AllowsRead(path string) bool
func (c *CapabilityChecker) AllowsWrite(path string) bool
func (c *CapabilityChecker) AllowsHostCall(name string) bool
func (c *CapabilityChecker) CheckRead(path string) error
func (c *CapabilityChecker) CheckWrite(path string) error
func (c *CapabilityChecker) CheckHostCall(name string) error
```

---

## Phase Execution

```
Phase 1: Core Configuration Infrastructure
         └── Worker 1 only (no dependencies)

Phase 2: Extension Registration + Wasm Runtime
         ├── Worker 3 (depends on Worker 1's ConfigSpec)
         └── Worker 5 (Wasm host, can work in parallel)

Phase 3: Migrate Commands to Extensions
         ├── Worker 4 (depends on Worker 3's spec format)
         └── Worker 2 starts command.go extraction

Phase 4: Update Runtime + Wasm Integration
         ├── Worker 2 (depends on Workers 1, 3)
         ├── Worker 5 (host callbacks integration)
         └── Lead: Integration testing begins

Phase 5: Remove Legacy Code
         ├── Worker 1 (cleanup)
         └── Lead: Verify no regressions

Phase 6: Documentation and Testing
         ├── Worker 4 (documentation)
         └── All: Final testing
```

### Merge Order per Phase

| Phase | Order | Branch | Agent |
|-------|-------|--------|-------|
| 1 | 1 | `feat/ext-config-phase-1` | Worker 1 |
| 2 | 1 | `feat/ext-extension-phase-2` | Worker 3 |
| 2 | 2 | `feat/ext-wasm-phase-2` | Worker 5 |
| 3 | 1 | `feat/ext-commands-phase-3` | Worker 4 |
| 3 | 2 | `feat/ext-starlark-phase-3` | Worker 2 |
| 4 | 1 | `feat/ext-wasm-phase-4` | Worker 5 |
| 4 | 2 | `feat/ext-starlark-phase-4` | Worker 2 |
| 4 | 3 | `feat/ext-lead-integration` | Lead |
| 5 | 1 | `feat/ext-config-phase-5` | Worker 1 |
| 6 | 1 | `feat/ext-commands-phase-6` | Worker 4 |

---

## Phase Details

### Phase 1: Core Configuration Infrastructure (Worker 1)

**Create:**
- `internal/config/element.go` - ConfigElement with path, children, Navigate()
- `internal/config/root.go` - Config root with Load(), Save(), RegisterExtension()
- `internal/config/types.go` - generateConfigType() via reflect.StructOf
- `internal/config/accessor.go` - ConfigAccessor with Bool(), String(), Int()

**Modify:**
- `internal/config/starlark.go` - Add ConfigValue implementing starlark.HasAttrs

### Phase 2: Extension Registration (Worker 3) + Wasm Runtime (Worker 5)

**Worker 3 Creates:**
- `internal/extension/spec.go` - ExtensionSpec, ReceiverSpec, CommandSpec, FlagSpec, ConfigSpec
- `internal/extension/registry.go` - Register(), Get(), All()
- `internal/extension/discovery.go` - Discover(), LoadAll()
- `internal/extension/wasm.go` - Integration with wasm.Host

**Worker 5 Creates:**
- `internal/wasm/host.go` - wazero host setup, module loading
- `internal/wasm/capabilities.go` - Capability validation, path checking
- `internal/wasm/protocol.go` - Extension ↔ Host message format

### Phase 3: Migrate Commands (Worker 4 + Worker 2)

**Worker 4 Creates:**
- `extensions/lint-go.yaml`
- `extensions/lint-shell.yaml`
- `extensions/lint-copyright.yaml`
- `extensions/lint-all.yaml`
- `extensions/distill.yaml`
- `extensions/self.yaml`

**Worker 2 Creates:**
- `internal/starlark/command.go` - Extract Command, Flag types

### Phase 4: Update Runtime (Worker 2 + Worker 5 + Lead)

**Worker 2 Modifies:**
- `runtime.go` - Load extensions before Starlark files
- `builtin_config.go` - Use new Config system

**Worker 5 Creates:**
- `internal/wasm/callbacks.go` - Host callback implementations (shell.run, http.get, fs.*)

**Lead Modifies:**
- `cmd/star/main.go` - Extension discovery and loading

### Phase 5: Directory Structure Consolidation (Lead)

**COMPLETED in Phase 4:**

The directory structure has been consolidated. All paths now use the `star/` prefix:

| Before | After |
|--------|-------|
| `star.yaml` | `star/config.yaml` |
| `extensions/` | `star/extensions/` |
| `ops/` | Deleted (stale, covered by extensions) |

**Code changes completed:**
- `internal/config/config.go` - Updated to use `star/config.yaml`
- `internal/extension/discovery.go` - Updated to use `star/extensions/`
- `cmd/star/main.go` - Removed `ops/` loading
- `internal/starlark/runtime.go` - Removed `Load()` method, `command()` builtin, and `commandCollector`
- `internal/starlark/command.go` - Removed `commandCollector` and `parseFlag`

**Single source of truth:** `extension.yaml` is now the only way to declare extensions. The `.star` files just define a `run` function.

**Legacy code removed:**
- `commandCollector` type and `commandBuiltin` function
- `parseFlag` function
- `applyFlagDefaults` method
- `Load()` method (loaded individual .star files with command() calls)
- `SetExtensionsDir()` method
- All `command()` calls from extension .star files

### Phase 6: Documentation and Testing (Worker 4)

**Update:**
- `docs/architecture/star-extensions.md` - Updated for `star/` paths and run function convention
- `docs/plans/star-extension-model.md` - Update for new structure

**Create:**
- `docs/guides/writing-extensions.md` - How to create extensions with extension.yaml + run function
- `docs/guides/config-migration.md` - Migration from old paths to `star/` prefix

---

## Verification

### Worker Self-Verification
```bash
# All workers run before PR
go test ./internal/{package}/...
go build ./...
```

### Integration Verification (Lead)
```bash
go test ./...
./star lint all
./star lint copyright --fix
./star setup config
```

---

## Critical Files

1. `internal/config/element.go` - Foundation for config hierarchy
2. `internal/extension/spec.go` - Extension YAML structure with Wasm support
3. `internal/wasm/host.go` - wazero runtime, sandboxed execution
4. `internal/wasm/capabilities.go` - Security boundary enforcement
5. `internal/starlark/runtime.go` - Integration point
6. `cmd/star/main.go` - Entry point changes

---

## Agent Prompts

These prompts provide each agent with the context needed to start work independently. All agents should read the architecture documents before beginning.

### Lead Agent Prompt

```
You are the Lead Agent for the star extension model refactoring.

## Model
opus (architectural decisions, integration coordination)

## Your Role
- Coordinate interface contracts between workers
- Review and merge PRs from all workers
- Run integration tests after each phase
- Own cmd/star/main.go and integration tests

## Key Documents
Read these before starting:
- docs/architecture/star-extensions.md (canonical architecture)
- docs/plans/star-extension-model.md (implementation phases)
- docs/plans/star-agent-team-refactor.md (this plan)

## Your Files (exclusive ownership)
- cmd/star/main.go
- cmd/star/config.go
- tests/integration/

## Branch
feat/ext-lead-integration

## Phase Responsibilities
- Phase 4: Modify cmd/star/main.go for extension discovery
- All phases: Coordinate merges, run integration tests

## Success Criteria
All existing commands (lint go, lint shell, lint copyright, etc.) work
after refactoring. No regressions.

## Interface Oversight
You approve all interface contract changes. Workers propose, you decide.
```

### Worker 1: Config Package Prompt

```
You are Worker 1, responsible for the config package.

## Model
sonnet (moderate complexity, well-defined patterns)

## Your Role
Build the configuration infrastructure that supports runtime type generation
from YAML extension specs.

## Key Documents
Read these before starting:
- docs/architecture/star-extensions.md
  - Section: "Configuration Architecture" (ConfigElement, Config root)
  - Section: "Runtime Type Generation" (reflect.StructOf)
  - Section: "ConfigValue (Starlark Adapter)"
- docs/plans/star-agent-team-refactor.md (this plan)

## Your Files (exclusive ownership)
Create:
- internal/config/element.go - ConfigElement base type with path, children, Navigate()
- internal/config/root.go - Config root with Load(), Save(), RegisterExtension()
- internal/config/types.go - generateConfigType() via reflect.StructOf
- internal/config/accessor.go - ConfigAccessor with Bool(), String(), Int()

Modify:
- internal/config/starlark.go - Add ConfigValue implementing starlark.HasAttrs

Delete (Phase 5 only):
- internal/config/schema.go
- internal/config/value.go
- internal/config/registry.go
- internal/config/loader.go
- internal/config/extensions.go

## Branch
feat/ext-config-phase-{N}

## Interface Contract (you implement, Worker 3 consumes)
type ConfigSpec struct {
    Type     string
    Fields   map[string]string
    Defaults map[string]any
}

func (c *Config) RegisterExtension(path string, spec ConfigSpec) error
func (c *Config) Navigate(path string) interface{}

## Interface Contract (you implement, Worker 2 consumes)
type ConfigValue struct { elem interface{} }
func (v *ConfigValue) Attr(name string) (starlark.Value, error)
func ToStarlark(elem interface{}) starlark.Value

## Phase 1 Deliverables
- ConfigElement with hierarchical navigation
- Config root that can register extensions at dotted paths
- Runtime type generation from ConfigSpec
- ConfigValue Starlark adapter

## Verification
go test ./internal/config/...
go build ./...
```

### Worker 2: Starlark Package Prompt

```
You are Worker 2, responsible for the starlark package.

## Model
opus (complex runtime integration, Starlark internals)

## Your Role
Integrate the extension system with the Starlark runtime. Update runtime
to load extensions and use the new config system.

## Key Documents
Read these before starting:
- docs/architecture/star-extensions.md
  - Section: "Binding Functions (Go)" (receiver pattern)
  - Section: "Receiver Registration"
  - Section: "Command (Starlark)"
- docs/plans/star-agent-team-refactor.md (this plan)

## Your Files (exclusive ownership)
Create:
- internal/starlark/command.go - Extract Command, Flag types from runtime.go

Modify:
- internal/starlark/runtime.go - Load extensions before Starlark files
- internal/starlark/builtin_config.go - Use new Config system
- internal/starlark/receiver.go - Ensure receiver API supports extension specs
- internal/starlark/builtin_*.go - Migrate to receiver pattern as needed

## Branch
feat/ext-starlark-phase-{N}

## Interface Contract (Worker 1 implements, you consume)
type ConfigValue struct { elem interface{} }
func (v *ConfigValue) Attr(name string) (starlark.Value, error)
func ToStarlark(elem interface{}) starlark.Value

## Interface Contract (Worker 3 implements, you consume)
func extension.All() map[string]*ExtensionSpec
func extension.Get(name string) *ExtensionSpec

## Key Concept: Receiver API
All binding functions are defined using the receiver API. A receiver is a
Go struct whose exported methods become Starlark module functions.

type CopyrightChecker struct{}

func (c *CopyrightChecker) Check(...) (starlark.Value, error) { ... }

func init() {
    RegisterReceiver("copyright", &CopyrightChecker{})
}

Methods are exposed as copyright.check() in Starlark.

## Phase 3 Deliverables
- Extract Command, Flag types to command.go

## Phase 4 Deliverables
- Runtime loads extensions before Starlark files
- Flag resolution uses extension spec defaults
- builtin_config.go uses new Config system

## Verification
go test ./internal/starlark/...
go build ./...
# Run existing commands to verify no regression
```

### Worker 3: Extension Package Prompt

```
You are Worker 3, responsible for the extension package.

## Model
sonnet (spec parsing is straightforward, well-defined contracts)

## Your Role
Build the extension loading system that parses YAML specs and registers
them with the config and runtime systems.

## Key Documents
Read these before starting:
- docs/architecture/star-extensions.md
  - Section: "Extension Specification" (YAML format)
  - Section: "Registration from YAML"
- docs/plans/star-agent-team-refactor.md
  - Section: "Extension Model" (extension types)
  - Section: "Extension Specification Format"

## Your Files (exclusive ownership)
Create:
- internal/extension/spec.go - ExtensionSpec, ReceiverSpec, CommandSpec, FlagSpec, ConfigSpec
- internal/extension/registry.go - Register(), Get(), All()
- internal/extension/discovery.go - Discover(), LoadAll()

## Branch
feat/ext-extension-phase-{N}

## Interface Contract (you implement, Worker 2 consumes)
func All() map[string]*ExtensionSpec
func Get(name string) *ExtensionSpec

## Interface Contract (Worker 1 implements, you consume)
func (c *Config) RegisterExtension(path string, spec ConfigSpec) error

## Key Types
type ExtensionSpec struct {
    Extension   string         `yaml:"extension"`
    Description string         `yaml:"description"`
    Receivers   []ReceiverSpec `yaml:"receivers"`  // 0 or more
    Command     *CommandSpec   `yaml:"command"`    // optional
    Flags       []FlagSpec     `yaml:"flags"`
    Config      *ConfigSpec    `yaml:"config"`     // optional
}

type ReceiverSpec struct {
    Name        string   `yaml:"name"`
    Type        string   `yaml:"type"`        // Go type name
    Description string   `yaml:"description"`
    Functions   []string `yaml:"functions"`   // exposed methods
}

type CommandSpec struct {
    Help string `yaml:"help"`
}

type FlagSpec struct {
    Name    string `yaml:"name"`
    Type    string `yaml:"type"`
    Default string `yaml:"default"`
    Help    string `yaml:"help"`
}

## Extension Types
An extension MUST have at least one of:
- receivers (binding functions via receiver API)
- command (CLI subcommand)

Three patterns:
1. Binding-only: receivers but no command
2. Command-only: command but no receivers
3. Full: both receivers and command

## Phase 2 Deliverables
- Parse extension YAML specs
- Registry with Register/Get/All
- Discovery from extensions/ directory
- Integration with config.RegisterExtension()

## Verification
go test ./internal/extension/...
go build ./...
```

### Worker 4: Extensions and Commands Prompt

```
You are Worker 4, responsible for extension specs and Starlark commands.

## Model
haiku (pattern-based YAML/Starlark, low complexity)

## Your Role
Create extension YAML specs for existing commands. Update Starlark
implementations to work with the extension system.

## Key Documents
Read these before starting:
- docs/architecture/star-extensions.md
  - Section: "Extension Specification" (canonical YAML format)
  - Section: "Command (Starlark)" (command implementation)
- docs/plans/star-agent-team-refactor.md
  - Section: "Extension Specification Format"
  - Section: "Extension Types"

## Your Files (exclusive ownership)
Create:
- extensions/lint-go.yaml
- extensions/lint-shell.yaml
- extensions/lint-markdown.yaml
- extensions/lint-copyright.yaml
- extensions/lint-all.yaml
- extensions/distill.yaml (rename from knowledge)
- extensions/self.yaml
- extensions/config.yaml
- docs/guides/writing-extensions.md

Modify:
- ops/*.star - Update to work with extension system

## Branch
feat/ext-commands-phase-{N}

## Extension Spec Template
extension: lint.copyright
description: "Check or fix copyright headers in source files"

# OPTIONAL: Receivers (binding functions via receiver API)
receivers:
  - name: copyright
    type: CopyrightChecker
    description: "Copyright header primitives"
    functions:
      - check
      - fix
      - detect_license

# OPTIONAL: Command (CLI subcommand)
command:
  help: |
    Check or fix copyright headers.

    Examples:
      star lint copyright
      star lint copyright --fix

# Flags for the command
flags:
  - name: fix
    type: bool
    default: "false"
    help: Add missing headers

# OPTIONAL: Configuration schema
config:
  type: CopyrightConfig
  fields:
    enabled: bool
    license: string
    holder: string
  defaults:
    enabled: false
    license: "auto"

## Existing Implementations to Package
Review these files to understand what needs extension specs:
- internal/starlark/builtin_copyright.go → extensions/lint-copyright.yaml
- internal/starlark/builtin_lint.go → extensions/lint-*.yaml
- internal/starlark/builtin_go.go → extensions/lint-go.yaml (uses go receiver)
- ops/devlore-registry/build-knowledge.star → extensions/distill.yaml

## Phase 3 Deliverables
- Extension YAML specs for all existing commands
- Updated ops/*.star files

## Phase 6 Deliverables
- docs/guides/writing-extensions.md
- docs/guides/config-migration.md

## Verification
yamllint extensions/*.yaml
go build ./...
# Run each command to verify
```

### Worker 5: Wasm Runtime Prompt

```
You are Worker 5, responsible for the Wasm runtime package.

## Model
opus (novel Wasm integration, complex host callbacks)

## Your Role
Build the WebAssembly host runtime that loads and executes extension
Wasm modules with sandboxed capabilities.

## Key Documents
Read these before starting:
- docs/architecture/star-extensions.md
  - Section: "Extension Distribution" (Wasm rationale, real-world examples)
- docs/plans/star-agent-team-refactor.md
  - Section: "Extension Distribution" (Wasm model, runtime options)
  - Section: "Sandboxing Model" (capabilities)
- https://wazero.io/docs/ (wazero documentation)
- https://extism.org/docs/ (Extism - potential future alternative with PDKs)
- Reference implementations: sqlc, moonrepo, wasmCloud (see Extension Distribution)

## Your Files (exclusive ownership)
Create:
- internal/wasm/host.go - wazero runtime setup, module compilation/caching
- internal/wasm/capabilities.go - Capability struct, path validation
- internal/wasm/protocol.go - Message format for extension ↔ host communication
- internal/wasm/callbacks.go - Host callback implementations

## Branch
feat/ext-wasm-phase-{N}

## Interface Contract (you implement, Worker 3 consumes)
// Uses extension.Capabilities from internal/extension/spec.go
type WasmHost struct { ... }
type WasmModule struct { ... }

func NewHost(ctx context.Context, caps extension.Capabilities) (*WasmHost, error)
func (h *WasmHost) LoadModule(wasmPath string) (*WasmModule, error)
func (h *WasmHost) Close() error

// Call is on WasmModule for idiomatic usage
func (m *WasmModule) Call(ctx context.Context, function string, args []byte) ([]byte, error)

## Capabilities System
// Capabilities defined in internal/extension/spec.go (Worker 3)
// Worker 5 provides validation via CapabilityChecker

func ValidateCapabilities(caps extension.Capabilities) error

type CapabilityChecker struct { ... }
func NewCapabilityChecker(caps extension.Capabilities) *CapabilityChecker
func (c *CapabilityChecker) AllowsRead(path string) bool
func (c *CapabilityChecker) AllowsWrite(path string) bool
func (c *CapabilityChecker) AllowsHostCall(name string) bool

## Host Callbacks
Extensions call back to host for privileged operations:
type HostCallbacks interface {
    // shell.run - Execute shell command
    ShellRun(cmd string, args []string, dir string) (stdout, stderr string, exitCode int, err error)

    // http.get - Make HTTP request
    HTTPGet(url string, headers map[string]string) (body []byte, statusCode int, err error)

    // fs.read - Read file (within granted paths)
    FSRead(path string) ([]byte, error)

    // fs.write - Write file (within granted paths)
    FSWrite(path string, data []byte) error
}

## wazero Integration
Use wazero (pure Go, no CGO) with compilation caching:

func NewHost(ctx context.Context, caps extension.Capabilities) (*WasmHost, error) {
    if err := ValidateCapabilities(caps); err != nil {
        return nil, err
    }

    // Compilation cache for 10x faster repeated loads
    cacheDir := filepath.Join(os.UserCacheDir(), "star", "wasm", "wazero")
    cache, _ := wazero.NewCompilationCacheWithDir(cacheDir)

    config := wazero.NewRuntimeConfig().
        WithCompilationCache(cache).
        WithCloseOnContextDone(true)

    rt := wazero.NewRuntimeWithConfig(ctx, config)
    wasi_snapshot_preview1.Instantiate(ctx, rt)

    return &WasmHost{runtime: rt, cache: cache, caps: caps}, nil
}

// Call uses stdin/stdout JSON protocol
func (m *WasmModule) Call(ctx context.Context, function string, args []byte) ([]byte, error) {
    // Request/Response JSON envelopes via stdin/stdout
    // See protocol.go for message format
}

## Phase 2 Deliverables
- wazero host setup
- Capabilities validation
- Module loading and caching
- Basic protocol for function calls

## Phase 4 Deliverables
- Host callback implementations (shell.run, http.get, fs.*)
- Integration tests with sample Wasm modules

## Verification
go test ./internal/wasm/...
go build ./...
# Test with a simple Wasm module
```

---

## Acceptance Criteria

All existing commands must be functional after refactoring:
- `star lint go`
- `star lint shell`
- `star lint markdown`
- `star lint copyright`
- `star lint copyright --fix`
- `star lint all`
- `star setup config`
- `star hook pre-commit`

Each command should work identically to its current behavior. The refactoring packages existing implementations as extensions without changing functionality.

---

## Related Documents

- [Star Extension Model](./star-extension-model.md) - Implementation plan
- [Star Extensions](../architecture/star-extensions.md) - Architecture
- Issue #29 - Star extension model tracking
