---
title: "Star Configuration"
description: "Canonical configuration mechanism for star CLI including built-in and extension-defined sections"
status: active
created: 2025-02-07
updated: 2026-02-11
---

# Star Configuration

This document defines the canonical mechanism for configuration in the star CLI. All configuration sections — whether declared in extension.yaml or registered programmatically — follow this model.

## Overview

Configuration follows a hierarchical merge pattern with typed access:

```
Extension Defaults → User Config → Project Config → Environment Variables → CLI Flags
```

Higher-priority layers override lower ones. The final merged configuration is accessible via `config.get()` in Starlark or `ConfigAccessor` in Go.

## Core Types

| Type | File | Purpose |
|------|------|---------|
| `Config` | `unified.go` | Unified entry point — loads, registers, and provides access to all config |
| `ConfigSpec` | `root.go` | Schema definition for a config section: field names, types, and defaults |
| `ConfigElement` | `element.go` | Hierarchical composition node — forms the config tree |
| `ConfigAccessor` | `accessor.go` | Typed field access via reflection — reads values from generated structs |
| `ConfigValue` | `starlark.go` | Starlark adapter — wraps any Go struct for `cfg.lint.copyright.enabled` access |

## Declaring Configuration

### In extension.yaml (Declarative)

Extensions declare their config section in `extension.yaml` with a `config:` block. The `path` field determines where the section lives in `star/config.yaml`:

```yaml
# star/extensions/com.noblefactor.star.LintGo/extension.yaml

extension: com.noblefactor.star.LintGo
description: Run Go lint checks

commands:
  - name: lint.go
    help: Run Go lint checks
    implementation: commands/lint-go.star

config:
  path: lint.go
  fields:
    path: string
    skip_mod_tidy: bool
    config: "map[string]any"
  defaults:
    path: "./..."
    skip_mod_tidy: false
```

The runtime calls `cfg.RegisterExtension("lint.go", spec.ToConfigSpec())` automatically when loading extensions.

### In Go (Programmatic)

For config sections not owned by any extension, register directly:

```go
cfg.RegisterExtension("lint.go", config.ConfigSpec{
    Fields: map[string]string{
        "path":          "string",
        "skip_mod_tidy": "bool",
        "config":        "map[string]any",
    },
    Defaults: map[string]interface{}{
        "path":          "./...",
        "skip_mod_tidy": false,
    },
})
```

### ConfigSpec Field Types

| Type String | Go Type | Example |
|-------------|---------|---------|
| `"string"` | `string` | `"path": "string"` |
| `"bool"` | `bool` | `"enabled": "bool"` |
| `"int"` | `int` | `"indent": "int"` |
| `"float64"` | `float64` | `"threshold": "float64"` |
| `"[]string"` | `[]string` | `"exclude": "[]string"` |
| `"map[string]any"` | `map[string]interface{}` | `"config": "map[string]any"` |

Nested types use the `Nested` field in ConfigSpec:

```go
config.ConfigSpec{
    Fields: map[string]string{
        "required": "[]string",
        "optional": "[]string",
    },
    Defaults: map[string]interface{}{
        "required": []string{"title", "description"},
    },
}
```

## Loading Configuration

```go
cfg, err := config.Load()
```

`Load()` creates a `Config`, then the runtime registers extensions and merges YAML values:

1. Extensions register their `ConfigSpec` via `RegisterExtension(path, spec)` — this creates a typed struct at each path with default values
2. User config (`~/.config/star/config.yaml`) is merged via `mergeRaw()`
3. Project config (`${GIT_TOPLEVEL}/star/config.yaml`) is merged via `mergeRaw()`

## Accessing Values in Go

Navigate the `ConfigElement` hierarchy and use `ConfigAccessor` for typed reads:

```go
// Navigate to a section
elem := cfg.Navigate("lint.copyright")
acc := config.NewAccessor(elem)

// Typed access
enabled := acc.Bool("enabled")
license := acc.String("license")
exclude := acc.StringSlice("exclude")
indent := acc.Int("indent")

// With fallbacks
severity := acc.StringOr("severity", "warning")

// Check existence
if acc.Has("holder") {
    holder := acc.String("holder")
}

// Nested struct access
frontmatter := acc.Struct("frontmatter")
required := frontmatter.StringSlice("required")

// Raw map access
patterns := acc.Map("patterns")
```

## Starlark Access

Configuration is exposed to Starlark via the `config` receiver. `Config.ToStarlark()` returns a `ConfigValue` that uses `goToStarlarkReflect()` for reflection-based attribute access:

```python
cfg = config.get()

# Attribute-style access through ConfigValue
if cfg.lint.copyright.enabled:
    license = cfg.lint.copyright.license
    holder = cfg.lint.copyright.holder

    for lang, pattern in cfg.lint.copyright.patterns.items():
        match = pattern.match
        replace = pattern.replace
```

`ConfigValue` wraps any Go struct (including runtime-generated types from `reflect.StructOf`) and implements `starlark.HasAttrs`. No type-specific conversion code is needed — reflection handles everything.

## Extension Configuration Examples

### All Lint Sections

Each lint extension declares its own config section:

```yaml
# LintGo
config:
  path: lint.go
  fields:
    path: string
    skip_mod_tidy: bool
    config: "map[string]any"
  defaults:
    path: "./..."
    skip_mod_tidy: false

# LintShell
config:
  path: lint.shell
  fields:
    path: string
    severity: string
    indent: int
  defaults:
    path: "."
    severity: "warning"
    indent: 4

# LintMarkdown
config:
  path: lint.markdown
  fields:
    path: string
    exclude: "[]string"
    config: "map[string]any"
  defaults:
    path: "."
    exclude:
      - node_modules
      - vendor
      - .git

# LintCopyright
config:
  path: lint.copyright
  fields:
    enabled: bool
    license: string
    holder: string
    patterns: "map[string]any"
    exclude: "[]string"
  defaults:
    enabled: false
    license: "auto"
    exclude:
      - "**/testdata/**"
      - "**/vendor/**"
```

### Precommit Section

```yaml
# ConfigSync (or HookPreCommit)
config:
  path: precommit
  fields:
    hooks: "[]any"
  defaults: {}
```

## Configuration File

Users configure star in `star/config.yaml`:

```yaml
# star/config.yaml

lint:
  go:
    path: "./..."
    skip_mod_tidy: false
    config:
      linters:
        enable: [gofmt, govet, errcheck, staticcheck]

  shell:
    path: "."
    severity: warning
    indent: 4

  markdown:
    path: "."
    exclude:
      - node_modules
      - vendor
    config:
      MD013: false
      MD033: false

  copyright:
    enabled: true
    license: MIT
    holder: Noble Factor
    exclude:
      - "**/testdata/**"
      - "**/vendor/**"

precommit:
  hooks:
    - id: star-lint-all
      name: Star quality gate
      entry: star lint all
      language: system
      pass_filenames: false
```

## Config Sync

The `config.sync` command generates tool-specific config files from `star/config.yaml`. This reads values through `ConfigAccessor`:

| Source Path | Generated File | Tool |
|---|---|---|
| `lint.go.config` | `.golangci.yaml` | golangci-lint |
| `lint.markdown.config` | `.markdownlint-cli2.yaml` | markdownlint-cli2 |
| `precommit.hooks` | `.pre-commit-config.yaml` | pre-commit |

Generated files include a header: `# GENERATED BY star lint sync - DO NOT EDIT`

## Flag Resolution Chain

Command flags resolve values in priority order:

1. **CLI argument** (highest) — `--severity error`
2. **Environment variable** — `STAR_LINT_SHELL_SEVERITY=error`
3. **Config file** — `lint.shell.severity: error`
4. **Default** (lowest) — From ConfigSpec registration

### Environment Variable Naming

Formula: `STAR_` + path (dots → underscores) + uppercase

| Config Path | Environment Variable |
|---|---|
| `lint.shell.severity` | `STAR_LINT_SHELL_SEVERITY` |
| `lint.copyright.enabled` | `STAR_LINT_COPYRIGHT_ENABLED` |
| `lint.go.skip_mod_tidy` | `STAR_LINT_GO_SKIP_MOD_TIDY` |

## Configuration File Locations

| Priority | Location | Purpose |
|---|---|---|
| 1 (lowest) | Extension defaults | From `ConfigSpec.Defaults` in extension.yaml |
| 2 | `~/.config/star/config.yaml` | User preferences |
| 3 (highest) | `${GIT_TOPLEVEL}/star/config.yaml` | Project settings |

The `XDG_CONFIG_HOME` environment variable is respected for user config location.

## How It Works Internally

### Registration

`Config.RegisterExtension(path, spec)` does:

1. Splits the path on `.` (e.g., `lint.copyright` → `["lint", "copyright"]`)
2. Creates intermediate `ConfigElement` nodes as needed (`lint` → `copyright`)
3. Calls `generateConfigType(spec)` which uses `reflect.StructOf` to create a Go struct type matching the ConfigSpec fields
4. Creates an instance with `newConfigInstance()` and populates defaults
5. Registers the instance as a child of the parent ConfigElement

### YAML Merge

`mergeRaw(raw)` walks the parsed YAML map and the `ConfigElement` tree in parallel. For each matching key, it calls `setFieldValue()` to update the generated struct's fields via reflection.

### Starlark Conversion

`Config.ToStarlark()` returns a `ConfigValue` wrapping the root `ConfigElement`. When Starlark accesses `cfg.lint.copyright.enabled`:

1. `ConfigValue.Attr("lint")` → finds `lint` ConfigElement child → wraps in new `ConfigValue`
2. `ConfigValue.Attr("copyright")` → finds generated struct → wraps in new `ConfigValue`
3. `ConfigValue.Attr("enabled")` → `reflect.Value.FieldByName("Enabled")` → `starlark.Bool`

All via `goToStarlarkReflect()` — no type-specific conversion code.

## Design Principles

1. **Single source of truth** — One config file per CLI (`star/config.yaml`)
2. **Hierarchical merge** — Defaults → User → Project (higher overrides lower)
3. **Typed access** — ConfigSpec-defined types, not stringly-typed
4. **Convention over configuration** — Sensible defaults, explicit overrides
5. **Extension-friendly** — Extensions declare config in extension.yaml
6. **Registration-based** — No hardcoded struct definitions; all types generated at runtime

## Files

| File | Purpose |
|---|---|
| `internal/config/unified.go` | `Config` — unified entry point, load, register, Starlark bridge |
| `internal/config/root.go` | `ConfigSpec`, `extensionsConfig` — schema definition, registration, YAML merge |
| `internal/config/element.go` | `ConfigElement` — hierarchical composition, navigation |
| `internal/config/accessor.go` | `ConfigAccessor` — typed field access via reflection |
| `internal/config/types.go` | Runtime type generation via `reflect.StructOf` |
| `internal/config/starlark.go` | `ConfigValue`, `goToStarlarkReflect()` — Starlark adapter |
| `internal/config/config.go` | Git workspace root detection, config file path resolution |
| `internal/config/sync.go` | Config sync — generates tool-specific config files |
