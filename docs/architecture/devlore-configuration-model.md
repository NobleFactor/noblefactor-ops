---
title: "Devlore Configuration Model"
description: "Canonical configuration mechanism for all devlore CLIs including built-in and extension-defined sections"
status: draft
created: 2025-02-07
updated: 2025-02-07
---

# Devlore Configuration Model

This document defines the canonical mechanism for configuration in all devlore CLIs. All configuration sections—whether built-in or extension-defined—follow this model.

## Overview

Configuration in devlore CLIs follows a hierarchical merge pattern with typed access:

```
Built-in Defaults → User Config → Project Config → Environment Variables → CLI Flags
```

Lower layers override higher layers. The final merged configuration is accessible via `config.Get()` in Go or `config.get()` in Starlark.

## Package API

### Creating a Config Registry

Each CLI creates a config registry with its name:

```go
import "github.com/NobleFactor/devlore/config"

// Create registry for "star" CLI
// This determines the config filename: star.yaml
cfg := config.New("star")
```

### Registering Configuration Sections

Sections are registered with a dotted path and a schema defining fields and defaults:

```go
cfg.Section("lint.go", config.Schema{
    "path":          config.String("./..."),
    "skip_mod_tidy": config.Bool(false),
    "config":        config.Map(nil),
})
```

### Schema Field Types

| Type | Function | Example |
|------|----------|---------|
| `config.String(default)` | String value | `config.String("./...")` |
| `config.Bool(default)` | Boolean value | `config.Bool(false)` |
| `config.Int(default)` | Integer value | `config.Int(4)` |
| `config.StringSlice(default)` | List of strings | `config.StringSlice([]string{"a", "b"})` |
| `config.Map(default)` | Arbitrary map | `config.Map(nil)` |
| `config.Struct(schema)` | Nested structure | `config.Struct(config.Schema{...})` |

### Loading Configuration

After registering all sections, load from the file hierarchy:

```go
if err := cfg.Load(); err != nil {
    return fmt.Errorf("config: %w", err)
}
```

This loads and merges:
1. Built-in defaults (from registered schemas)
2. User config: `~/.config/star/star.yaml`
3. Project config: `./star.yaml`

### Accessing Values

```go
// Get typed values by path
enabled := cfg.Get("lint.copyright.enabled").Bool()
license := cfg.Get("lint.copyright.license").String()
exclude := cfg.Get("lint.copyright.exclude").StringSlice()
indent := cfg.Get("lint.shell.indent").Int()

// Get with fallback if not set
severity := cfg.Get("lint.shell.severity").StringOr("warning")

// Check if value exists
if cfg.Has("lint.copyright.holder") {
    holder := cfg.Get("lint.copyright.holder").String()
}
```

### Unmarshaling to Structs

For complex sections, unmarshal to a typed struct:

```go
type CopyrightConfig struct {
    Enabled  bool              `yaml:"enabled"`
    License  string            `yaml:"license"`
    Holder   string            `yaml:"holder"`
    Patterns map[string]Pattern `yaml:"patterns"`
    Exclude  []string          `yaml:"exclude"`
}

var copyrightCfg CopyrightConfig
if err := cfg.Unmarshal("lint.copyright", &copyrightCfg); err != nil {
    return err
}
```

## Complete Example: Star CLI

This example shows how the `star` CLI registers all its configuration sections.

### Bootstrap Code

```go
// cmd/star/main.go

package main

import (
    "fmt"
    "os"

    "github.com/NobleFactor/devlore/config"
    "github.com/NobleFactor/devlore/starlark"
)

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func run() error {
    // Create config registry for star CLI
    cfg := config.New("star")

    // Register all configuration sections
    registerLintConfig(cfg)
    registerPrecommitConfig(cfg)

    // Load configuration from hierarchy
    if err := cfg.Load(); err != nil {
        return fmt.Errorf("loading config: %w", err)
    }

    // Create Starlark runtime
    rt := starlark.NewRuntime("ops")
    rt.SetConfig(cfg)

    // Register extension receivers
    starlark.RegisterReceiver("copyright", &CopyrightChecker{})

    // Load Starlark commands
    if err := rt.LoadAll(); err != nil {
        return fmt.Errorf("loading commands: %w", err)
    }

    // Run CLI
    return rt.Run(os.Args[1:])
}
```

### Registering Lint Configuration

```go
// cmd/star/config.go

package main

import "github.com/NobleFactor/devlore/config"

func registerLintConfig(cfg *config.Registry) {
    // lint.go section
    cfg.Section("lint.go", config.Schema{
        "path":          config.String("./..."),
        "skip_mod_tidy": config.Bool(false),
        "config":        config.Map(nil), // Inline golangci-lint config
    })

    // lint.shell section
    cfg.Section("lint.shell", config.Schema{
        "path":     config.String("."),
        "severity": config.String("warning"),
        "indent":   config.Int(4),
    })

    // lint.markdown section
    cfg.Section("lint.markdown", config.Schema{
        "path":    config.String("."),
        "exclude": config.StringSlice([]string{"node_modules", "vendor", ".git"}),
        "config":  config.Map(nil), // Inline markdownlint config
        "frontmatter": config.Struct(config.Schema{
            "required": config.StringSlice([]string{"title", "description"}),
            "optional": config.StringSlice(nil),
        }),
    })

    // lint.copyright section
    cfg.Section("lint.copyright", config.Schema{
        "enabled": config.Bool(false),
        "license": config.String("auto"),
        "holder":  config.String(""),
        "patterns": config.Map(map[string]interface{}{
            "go": map[string]string{
                "match":   `// SPDX-License-Identifier: \S+\s*\n// Copyright.*All rights reserved\.`,
                "replace": "// SPDX-License-Identifier: {license}\n// Copyright {holder}. All rights reserved.",
            },
            "star": map[string]string{
                "match":   `# SPDX-License-Identifier: \S+\s*\n# Copyright.*All rights reserved\.`,
                "replace": "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved.",
            },
            "shell": map[string]string{
                "match":   `# SPDX-License-Identifier: \S+\s*\n# Copyright.*All rights reserved\.`,
                "replace": "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved.",
            },
        }),
        "exclude": config.StringSlice([]string{"**/testdata/**", "**/vendor/**"}),
    })
}

func registerPrecommitConfig(cfg *config.Registry) {
    // precommit section
    cfg.Section("precommit", config.Schema{
        "hooks": config.Slice(config.Struct(config.Schema{
            "id":             config.String(""),
            "name":           config.String(""),
            "entry":          config.String(""),
            "language":       config.String("system"),
            "pass_filenames": config.Bool(false),
            "types":          config.StringSlice([]string{"file"}),
            "stages":         config.StringSlice([]string{"pre-commit"}),
        })),
    })
}
```

### Using Configuration in a Command

```go
// internal/lint/copyright.go

package lint

import "github.com/NobleFactor/devlore/config"

func RunCopyrightLint(cfg *config.Registry, fix bool) error {
    // Check if enabled
    if !cfg.Get("lint.copyright.enabled").Bool() {
        return nil // Disabled, nothing to do
    }

    // Get configuration values
    license := cfg.Get("lint.copyright.license").String()
    holder := cfg.Get("lint.copyright.holder").String()
    exclude := cfg.Get("lint.copyright.exclude").StringSlice()

    // Get patterns as map
    patterns := cfg.Get("lint.copyright.patterns").Map()

    // Or unmarshal to struct for complex access
    var copyrightCfg CopyrightConfig
    if err := cfg.Unmarshal("lint.copyright", &copyrightCfg); err != nil {
        return err
    }

    // Implementation...
    return nil
}
```

### Configuration File

Users configure star in `star.yaml`:

```yaml
# star.yaml

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
    frontmatter:
      required: [title, description]

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

## Starlark Access

Configuration is exposed to Starlark via the `config` module:

```python
cfg = config.get()

# Attribute-style access
if cfg.lint.copyright.enabled:
    license = cfg.lint.copyright.license
    holder = cfg.lint.copyright.holder

    for lang, pattern in cfg.lint.copyright.patterns.items():
        match = pattern.match
        replace = pattern.replace
```

## Extension Configuration

Extensions can define their own configuration sections using `config.define()` in Starlark. See [devlore-extension-model.md](devlore-extension-model.md) for details.

```python
# ops/linters/yaml.star

config.define("lint.yaml", struct(
    enabled = True,
    schemas = [],
    cache_ttl = 3600,
))
```

Users then configure in `star.yaml`:

```yaml
lint:
  yaml:
    enabled: true
    cache_ttl: 7200
```

## Flag Resolution Chain

Command flags resolve values in priority order:

1. **CLI argument** (highest) — `--severity error`
2. **Environment variable** — `STAR_LINT_SHELL_SEVERITY=error`
3. **Config file** — `lint.shell.severity: error`
4. **Default** (lowest) — From schema registration

### Environment Variable Naming

Formula: `{CLI}_` + path (dots→underscores) + uppercase

| Config Path | Environment Variable |
|-------------|---------------------|
| `lint.shell.severity` | `STAR_LINT_SHELL_SEVERITY` |
| `lint.copyright.enabled` | `STAR_LINT_COPYRIGHT_ENABLED` |
| `lint.go.skip_mod_tidy` | `STAR_LINT_GO_SKIP_MOD_TIDY` |

### Resolving in Commands

```go
// Flag with automatic resolution
cmd.Flag("severity", config.FlagOptions{
    Help:    "Minimum severity level",
    Default: "warning",
    // Automatically resolves: CLI → ENV → Config → Default
})

// Access resolved value
severity := cmd.ResolvedFlag("severity")
```

## Configuration File Locations

| Priority | Location | Purpose |
|----------|----------|---------|
| 1 (lowest) | Built-in | Compiled defaults from `cfg.Section()` |
| 2 | `~/.config/{cli}/{cli}.yaml` | User preferences |
| 3 (highest) | `./{cli}.yaml` | Project settings |

The XDG_CONFIG_HOME environment variable is respected for user config location.

## Registry API Reference

```go
// New creates a config registry for a CLI.
func New(name string) *Registry

// Section registers a configuration section with schema.
func (r *Registry) Section(path string, schema Schema)

// Load loads configuration from file hierarchy.
func (r *Registry) Load() error

// LoadWithSources loads config and returns source information.
func (r *Registry) LoadWithSources() ([]Source, error)

// Get returns a value accessor for the given path.
func (r *Registry) Get(path string) *Value

// Has returns true if a value exists at the path.
func (r *Registry) Has(path string) bool

// Unmarshal decodes a section into a struct.
func (r *Registry) Unmarshal(path string, v interface{}) error

// ToStarlark returns the config as a Starlark struct.
func (r *Registry) ToStarlark() starlark.Value
```

## Value API Reference

```go
// Type accessors (panic if wrong type)
func (v *Value) String() string
func (v *Value) Bool() bool
func (v *Value) Int() int
func (v *Value) StringSlice() []string
func (v *Value) Map() map[string]interface{}

// Safe accessors with fallback
func (v *Value) StringOr(fallback string) string
func (v *Value) BoolOr(fallback bool) bool
func (v *Value) IntOr(fallback int) int

// Check if value is set
func (v *Value) IsSet() bool
```

## Design Principles

1. **Single source of truth** — One config file per CLI
2. **Hierarchical merge** — User → Project → CLI → ENV
3. **Typed access** — Schema-defined types, not stringly-typed
4. **Convention over configuration** — Sensible defaults, explicit overrides
5. **Extension-friendly** — Extensions declare schemas in Starlark
6. **Registration-based** — No hardcoded struct definitions in library

## Files

| File | Purpose |
|------|---------|
| `config/registry.go` | Registry and section registration |
| `config/schema.go` | Schema types (String, Bool, Int, etc.) |
| `config/value.go` | Value accessor with type methods |
| `config/loader.go` | File hierarchy loading and merge |
| `config/starlark.go` | Starlark conversion |
