---
title: "Star Extension Model"
description: "Plan for implementing star's extension architecture and migrating all commands to extensions"
issue: https://github.com/NobleFactor/noblefactor-ops/issues/29
status: in-progress
created: 2025-02-07
updated: 2025-02-07
---

# Plan: Star Extension Model

## Summary

Replace the existing star configuration system with the extension model defined in `docs/architecture/devlore-extension-model.md`. All star commands will be implemented as extensions, using:

1. **Extension YAML specs** - Describe command, flags, and config schema
2. **Starlark implementations** - Command logic in `ops/*.star`
3. **Go binding functions** - Low-level primitives via receiver pattern
4. **Runtime type generation** - Config structs from YAML via `reflect.StructOf`

## Goals

1. Replace `internal/config/config.go` static structs with dynamic ConfigElement hierarchy
2. Migrate all existing commands to extension model
3. Enable third-party extensions without modifying core code
4. Maintain backward compatibility with existing `star.yaml` files

## Architecture Reference

See `docs/architecture/devlore-extension-model.md` for:
- Extension YAML specification format
- ConfigElement and Config hierarchy
- ConfigValue Starlark adapter
- Runtime type generation
- Registration from YAML

## Current State

| Component | Status | Location |
|-----------|--------|----------|
| Static config structs | To replace | `internal/config/config.go` |
| Starlark ToStarlark | To replace | `internal/config/starlark.go` |
| Receiver registration | Partial | `internal/starlark/receiver.go` |
| config.define() | Partial | `internal/starlark/builtin_config.go` |
| Flag resolution | Implemented | `internal/starlark/runtime.go` |

## Implementation Phases

### Phase 1: Core Configuration Infrastructure

Create the ConfigElement-based configuration system.

#### 1.1 ConfigElement Base Type

**Create** `internal/config/element.go`:

```go
// ConfigElement is the base type embedded by all config sections.
type ConfigElement struct {
    path     string
    children map[string]interface{}
}

func (e *ConfigElement) Path() string
func (e *ConfigElement) Register(name string, child interface{})
func (e *ConfigElement) Get(name string) interface{}
func (e *ConfigElement) Navigate(path string) interface{}
```

#### 1.2 Config Root

**Create** `internal/config/root.go`:

```go
// Config is the root of the configuration hierarchy.
type Config struct {
    ConfigElement  // path = ""
    source string
    dirty  bool
}

func Load(source string) (*Config, error)
func (c *Config) Save() error
func (c *Config) Source() string
func (c *Config) IsDirty() bool
func (c *Config) Accessor(path string) *ConfigAccessor
func (c *Config) ToStarlark() starlark.Value
```

#### 1.3 Runtime Type Generation

**Create** `internal/config/types.go`:

```go
func generateConfigType(spec ConfigSpec) reflect.Type
func goTypeFor(typeName string) reflect.Type
func getOrCreateType(path string, spec ConfigSpec) reflect.Type
func newConfigInstance(typ reflect.Type, defaults map[string]any) interface{}
```

#### 1.4 ConfigAccessor

**Create** `internal/config/accessor.go`:

```go
type ConfigAccessor struct {
    v reflect.Value
}

func (a *ConfigAccessor) Bool(name string) bool
func (a *ConfigAccessor) String(name string) string
func (a *ConfigAccessor) Int(name string) int
func (a *ConfigAccessor) Struct(name string) *ConfigAccessor
```

#### 1.5 Starlark Adapter

**Update** `internal/config/starlark.go`:

```go
// ConfigValue adapts Go config for Starlark access
type ConfigValue struct {
    elem interface{}
}

func (v *ConfigValue) Attr(name string) (starlark.Value, error)
func (v *ConfigValue) AttrNames() []string
func ToStarlark(cfg interface{}) starlark.Value
```

### Phase 2: Extension Registration

Build the extension loading and registration system.

#### 2.1 Extension Spec Parser

**Create** `internal/extension/spec.go`:

```go
type ExtensionSpec struct {
    Extension string
    Command   CommandSpec
    Flags     []FlagSpec
    Config    ConfigSpec
}

func ParseSpec(yamlPath string) (*ExtensionSpec, error)
```

#### 2.2 Extension Registry

**Create** `internal/extension/registry.go`:

```go
var registry = make(map[string]*ExtensionSpec)

func Register(spec *ExtensionSpec) error
func Get(name string) *ExtensionSpec
func All() map[string]*ExtensionSpec
func RegisterFromYAML(root *Config, yamlPath string) error
```

#### 2.3 Extension Discovery

**Create** `internal/extension/discovery.go`:

```go
func Discover(dir string) ([]*ExtensionSpec, error)
func LoadAll(root *Config, dir string) error
```

### Phase 3: Migrate Built-in Commands to Extensions

Convert each existing command to an extension with YAML spec and Starlark implementation.

#### 3.1 lint.go Extension

**Create** `extensions/lint-go.yaml`:

```yaml
extension: lint.go

command:
  help: Run Go linters via golangci-lint

flags:
  - name: path
    type: string
    default: "./..."
    help: Path to lint
  - name: fix
    type: bool
    default: false
    help: Apply fixes where possible

config:
  type: GoLintConfig
  fields:
    path: string
    skip_mod_tidy: bool
    config: map[string]any
  defaults:
    path: "./..."
    skip_mod_tidy: false
    config: {}
```

**Update** `ops/lint-go.star` to use extension pattern.

#### 3.2 lint.shell Extension

**Create** `extensions/lint-shell.yaml`:

```yaml
extension: lint.shell

command:
  help: Run shell linters via shellcheck

flags:
  - name: path
    type: string
    default: "."
    help: Path to lint
  - name: severity
    type: string
    default: warning
    help: Minimum severity level

config:
  type: ShellLintConfig
  fields:
    path: string
    severity: string
    indent: int
  defaults:
    path: "."
    severity: warning
    indent: 4
```

#### 3.3 lint.markdown Extension

**Create** `extensions/lint-markdown.yaml`

#### 3.4 lint.copyright Extension

Already documented in architecture. Update `ops/lint-copyright.star`.

#### 3.5 lint.all Extension

**Create** `extensions/lint-all.yaml`:

```yaml
extension: lint.all

command:
  help: Run all configured linters

flags:
  - name: fix
    type: bool
    default: false
    help: Apply fixes where possible

config:
  type: LintAllConfig
  fields:
    enabled: "[]string"
    parallel: bool
  defaults:
    enabled:
      - go
      - shell
      - markdown
      - copyright
    parallel: true
```

#### 3.6 setup.config Extension

**Create** `extensions/setup-config.yaml`

#### 3.7 hook.pre-commit Extension

**Create** `extensions/hook-pre-commit.yaml`

### Phase 4: Update Runtime

Integrate extension system into star runtime.

#### 4.1 Update Starlark Runtime

**Modify** `internal/starlark/runtime.go`:

- Load extensions before Starlark files
- Register config types from extension specs
- Build ConfigElement hierarchy
- Pass config to Starlark via `config.get()`

#### 4.2 Update Command Registration

**Modify** `internal/starlark/command.go`:

- Support flag resolution from extension spec
- Merge extension flags with Starlark-defined flags

#### 4.3 Update Main Entry Point

**Modify** `cmd/star/main.go`:

- Discover extensions from `extensions/` directory
- Load extension specs before config
- Register commands from extensions

### Phase 5: Remove Legacy Code

Once all commands are migrated and tested.

#### 5.1 Remove Static Config Structs

**Delete** from `internal/config/config.go`:
- `LintConfig`
- `GoLintConfig`
- `ShellLintConfig`
- `MarkdownLintConfig`
- `CopyrightLintConfig`
- `PrecommitConfig`
- `DefaultConfig()`

#### 5.2 Remove Legacy ToStarlark Methods

**Delete** from `internal/config/starlark.go`:
- All type-specific `ToStarlark()` methods
- Keep only `ConfigValue` and `ToStarlark(interface{})`

#### 5.3 Clean Up Unused Files

Review and remove:
- `internal/config/schema.go`
- `internal/config/value.go`
- `internal/config/registry.go`
- `internal/config/loader.go`
- `internal/config/extensions.go`

### Phase 6: Documentation and Testing

#### 6.1 Extension Author Guide

**Create** `docs/guides/writing-extensions.md`:
- How to create an extension YAML spec
- How to write Starlark implementation
- How to add Go binding functions
- Testing extensions

#### 6.2 Migration Guide

**Create** `docs/guides/config-migration.md`:
- Breaking changes in config access
- Updating existing star.yaml files
- Updating existing Starlark scripts

#### 6.3 Tests

- Unit tests for ConfigElement, Config, ConfigAccessor
- Unit tests for runtime type generation
- Integration tests for extension loading
- End-to-end tests for each migrated command

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/config/element.go` | ConfigElement base type |
| `internal/config/root.go` | Config root with Load/Save |
| `internal/config/types.go` | Runtime type generation |
| `internal/config/accessor.go` | ConfigAccessor typed access |
| `internal/extension/spec.go` | Extension spec parsing |
| `internal/extension/registry.go` | Extension registration |
| `internal/extension/discovery.go` | Extension discovery |
| `extensions/lint-go.yaml` | Go linter extension spec |
| `extensions/lint-shell.yaml` | Shell linter extension spec |
| `extensions/lint-markdown.yaml` | Markdown linter extension spec |
| `extensions/lint-copyright.yaml` | Copyright linter extension spec |
| `extensions/lint-all.yaml` | All linters extension spec |
| `extensions/setup-config.yaml` | Config setup extension spec |
| `extensions/hook-pre-commit.yaml` | Pre-commit hook extension spec |

### Modified Files

| File | Changes |
|------|---------|
| `internal/config/starlark.go` | Replace with ConfigValue adapter |
| `internal/starlark/runtime.go` | Extension loading, config integration |
| `internal/starlark/builtin_config.go` | config.get() returns new Config |
| `cmd/star/main.go` | Extension discovery and loading |
| `ops/*.star` | Update to use extension pattern |

### Deleted Files

| File | Reason |
|------|--------|
| `internal/config/config.go` | Replaced by element.go + types.go |
| `internal/config/schema.go` | Unused |
| `internal/config/value.go` | Unused |
| `internal/config/registry.go` | Replaced by extension/registry.go |
| `internal/config/loader.go` | Replaced by root.go |
| `internal/config/extensions.go` | Replaced by extension package |

## Backward Compatibility

### star.yaml Format

The `star.yaml` format remains unchanged. Users configure extensions the same way:

```yaml
lint:
  go:
    path: "./..."
  copyright:
    enabled: true
    license: MIT
    holder: Noble Factor
```

### Starlark API

The `config.get()` API remains unchanged:

```python
cfg = config.get().lint.copyright
if cfg.enabled:
    license = cfg.license
```

### Breaking Changes

1. **Go code accessing config**: Must use `ConfigAccessor` or `Navigate()` instead of direct struct field access
2. **Custom extensions**: Must provide extension YAML spec

## Testing Strategy

1. **Unit tests**: Each new file gets comprehensive unit tests
2. **Integration tests**: Extension loading and registration
3. **Regression tests**: Existing `ops/*.star` scripts continue to work
4. **End-to-end tests**: `star lint all`, `star setup config`, `star hook pre-commit`

## Rollout Plan

1. **Phase 1-2**: Build infrastructure without breaking existing code
2. **Phase 3**: Migrate commands one at a time, validate each
3. **Phase 4**: Switch runtime to use new system
4. **Phase 5**: Remove legacy code after validation
5. **Phase 6**: Documentation and final testing

## Open Questions (Resolved)

1. ~~Config validation at load time or access time?~~ → Load time
2. ~~Config schema conflicts between extensions?~~ → Last registration wins, warning emitted
3. ~~Receivers declaring config in Go?~~ → No, config is Starlark/YAML only
