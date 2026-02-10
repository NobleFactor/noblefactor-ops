---
title: "Devlore Extension Model"
description: "Architecture for extending devlore CLIs with new capabilities via YAML specs, Starlark commands, and Go bindings"
status: draft
created: 2025-02-07
updated: 2025-02-09
---

# Devlore Extension Model

This document defines how to extend devlore CLIs with new capabilities.

## Extension Source Structure

Extensions use a reverse domain naming convention and organized directory structure:

```
extensions/
└── com.noblefactor.star.LintCopyright/
    ├── extension.yaml           # Manifest
    ├── commands/
    │   └── lint-copyright.star  # One file per command
    └── receivers/
        └── gitignore.wasm       # WASM receiver modules (optional)
```

### Directory Naming

- Directory name = extension identifier (reverse domain format)
- Example: `com.noblefactor.star.LintCopyright`
- Commands go in `commands/` subdirectory
- WASM receivers go in `receivers/` subdirectory

### Extension Search Path

Extensions are discovered by walking up from project to user global:

```
1. ${PROJECT}/star/extensions/          # Project-local
2. ${PROJECT}/../star/extensions/       # Walk up...
3. ...
4. $XDG_DATA_HOME/star/extensions/      # User global (~/.local/share/star/extensions/)
```

Project extensions shadow global ones (first match wins).

## Extension Specification

An extension is described by a YAML specification. The `com.noblefactor.star.LintCopyright` extension serves as the canonical example.

```yaml
extension: com.noblefactor.star.LintCopyright
description: Check or fix copyright headers in source files

receivers:
  - name: file
    builtin: true
    type: FileReceiver
    description: Filesystem operations
    functions:
      read: Read file contents
      write: Write file contents
      glob: Find files matching pattern
      exists: Check if file exists
  - name: regexp
    builtin: true
    type: RegexpReceiver
    description: Regular expression operations
    functions:
      match: Test if pattern matches text
      find_submatch: Find first match with groups
  - name: config
    builtin: true
    type: ConfigReceiver
    description: Configuration access
    functions:
      get: Load merged config

commands:
  - name: lint.copyright
    help: Check or fix copyright headers in source files
    implementation: commands/lint-copyright.star
    flags:
      - name: fix
        type: bool
        default: "false"
        help: Add missing headers and update old format
      - name: path
        type: string
        default: "."
        help: Path to check

config:
  type: CopyrightConfig
  fields:
    enabled: bool
    license: string
    holder: string
    exclude: "[]string"
  defaults:
    enabled: false
    license: "auto"
```

### Naming Convention

The extension uses reverse domain naming. The command name determines CLI paths:

| Element | Value | Derivation |
|---------|-------|------------|
| Extension | `com.noblefactor.star.LintCopyright` | Reverse domain identifier |
| Directory | `com.noblefactor.star.LintCopyright/` | Same as extension name |
| Command | `star lint copyright` | Command name with dots as subcommands |
| Config path | `lint.copyright` | Command name |
| Env var prefix | `STAR_LINT_COPYRIGHT_` | `STAR_` + command name (dots→underscores, uppercase) |

### Flag Resolution

Each flag resolves in priority order:

1. CLI argument: `--fix`
2. Environment variable: `STAR_LINT_COPYRIGHT_FIX`
3. Config file: `lint.copyright.fix`
4. Default: `"false"`

## Extension Types

Extensions fall into three categories based on what they provide:

### Receiver-Only Extension

Provides primitives for other extensions to use. No CLI command.

```yaml
extension: com.noblefactor.star.Gitignore
description: Gitignore pattern matching using BurntSushi's ignore crate

receivers:
  - name: gitignore
    wasm: receivers/gitignore.wasm
    description: Match paths against .gitignore patterns
    functions:
      matches: Check if a path matches gitignore patterns
      filter: Filter a list of paths, returning non-ignored ones
    capabilities:
      fs:
        read: ["."]
        write: []
      host_calls: []
```

Usage in other extensions:

```python
# Another extension can use these bindings
if gitignore.matches(path):
    # Skip ignored file
    continue
```

### Command-Only Extension

Orchestrates existing bindings. No new Go code.

```yaml
extension: com.noblefactor.star.LintAll
description: Run all configured linters

receivers:
  - name: commands
    builtin: true
    type: CommandsReceiver

commands:
  - name: lint.all
    help: Run all linters
    implementation: commands/lint-all.star
```

### Full Extension

Provides both receivers and commands.

```yaml
extension: com.noblefactor.star.LintCopyright
description: Check or fix copyright headers

receivers:
  - name: file
    builtin: true
    type: FileReceiver
  - name: regexp
    builtin: true
    type: RegexpReceiver
  - name: config
    builtin: true
    type: ConfigReceiver

commands:
  - name: lint.copyright
    help: Check or fix copyright headers
    implementation: commands/lint-copyright.star
    flags:
      - name: fix
        type: bool
        default: "false"
        help: Auto-fix issues
```

## Extension Distribution

Extensions support two receiver types: built-in (Go) and WASM.

### Built-in vs WASM Receivers

| Type | Declaration | Implementation | Performance |
|------|-------------|----------------|-------------|
| Built-in | `builtin: true` | Go code in `internal/starlark/receiver_*.go` | Native |
| WASM | `wasm: receivers/foo.wasm` | Compiled Wasm module | Near-native |

### Why WebAssembly?

Wasm solves historical pain points of traditional plugin systems—security risks from third-party code and ABI fragility of native shared libraries.

- **Sandboxed Security**: Extensions run in a secure, isolated environment by default
- **Language Agnostic**: Authors can write in Rust, Go, C++, or TypeScript
- **Single Artifact**: One `.wasm` file works on all platforms
- **Near-Native Performance**: Unlike interpreted languages, Wasm achieves near-native speed
- **No CGO**: Uses wazero, a pure Go Wasm runtime

### Sandboxing Model

WASM receivers declare capabilities per-receiver:

```yaml
receivers:
  - name: gitignore
    wasm: receivers/gitignore.wasm
    capabilities:
      fs:
        read: ["."]      # Directories extension can read
        write: []        # Directories extension can write
      host_calls:
        - shell.run      # Can request host to run commands
```

Host validates all capability requests. Extensions cannot:
- Access files outside granted directories
- Spawn processes directly
- Make network requests directly
- Access environment variables not explicitly passed

## Extension Components

An extension provides **zero or more** of these components:

| Component | Required | Language | Purpose |
|-----------|----------|----------|---------|
| **Receivers** | Optional | Go or WASM | Primitives via receiver API |
| **Config Schema** | Optional | YAML | Typed configuration with defaults |
| **Commands** | Optional | Starlark | CLI subcommand implementations |

**Minimum requirement:** An extension must provide at least one receiver OR one command.

## Receiver Functions (Go)

All binding functions are defined using the receiver API. A receiver is a Go struct whose exported methods become Starlark module functions.

### Receiver Pattern

A receiver is a Go struct implementing `starlark.HasAttrs`:

```go
// internal/starlark/receiver_file.go

// FileReceiver provides filesystem operations.
type FileReceiver struct {
    BaseReceiver
}

var File = &FileReceiver{BaseReceiver{name: "file"}}

func (r *FileReceiver) Attr(name string) (starlark.Value, error) {
    switch name {
    case "read":
        return starlark.NewBuiltin("file.read", r.read), nil
    case "write":
        return starlark.NewBuiltin("file.write", r.write), nil
    case "glob":
        return starlark.NewBuiltin("file.glob", r.glob), nil
    case "exists":
        return starlark.NewBuiltin("file.exists", r.exists), nil
    default:
        return nil, NoSuchAttrError("file", name, r.AttrNames())
    }
}

func (r *FileReceiver) read(
    _ *starlark.Thread,
    _ *starlark.Builtin,
    args starlark.Tuple,
    kwargs []starlark.Tuple,
) (starlark.Value, error) {
    var path string
    if err := starlark.UnpackArgs("file.read", args, kwargs, "path", &path); err != nil {
        return nil, err
    }
    content, err := os.ReadFile(path)
    if err != nil {
        return starlark.None, nil
    }
    return starlark.String(content), nil
}
```

### Method Signature

Methods must match this exact signature:

```go
func (r *Receiver) methodName(
    thread *starlark.Thread,
    b *starlark.Builtin,
    args starlark.Tuple,
    kwargs []starlark.Tuple,
) (starlark.Value, error)
```

### Name Conversion

Method names use snake_case in Starlark:

| Go Method | Starlark Function |
|-----------|-------------------|
| `read` | `file.read` |
| `findSubmatch` | `regexp.find_submatch` |
| `detectLicense` | `copyright.detect_license` |

## Config Schema

Extensions declare their configuration in YAML. The config section is optional.

```yaml
config:
  type: CopyrightConfig
  fields:
    enabled: bool
    license: string
    holder: string
    exclude: "[]string"
  defaults:
    enabled: false
    license: "auto"
```

### User Configuration

Users override defaults in `star.yaml`:

```yaml
lint:
  copyright:
    enabled: true
    license: MIT
    holder: Noble Factor
    exclude:
      - "**/vendor/**"
      - "**/testdata/**"
```

### Accessing Config

```python
cfg = config.get().lint.copyright

if cfg.enabled:
    license = cfg.license
    holder = cfg.holder
```

## Command Implementation

Commands are registered using the `command()` builtin. Each command has its own `.star` file in the `commands/` subdirectory.

```python
# extensions/com.noblefactor.star.LintCopyright/commands/lint-copyright.star

def run(ctx):
    fix_mode = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")

    cfg = config.get().lint.copyright

    if not cfg.enabled:
        warn("Copyright checking is disabled")
        return

    # Detect license if auto
    license = cfg.license
    if license == "auto":
        result = detect_license("LICENSE")
        if result["detected"]:
            license = result["license"]
        else:
            fail("Cannot detect license")

    holder = cfg.holder
    if not holder:
        fail("Set lint.copyright.holder in star.yaml")

    # Check or fix files
    files = file.glob(path + "/**/*.go")

    if fix_mode:
        fixed = []
        for f in files:
            if fix_file(f, license, holder):
                fixed.append(f)
        if len(fixed) > 0:
            success("Fixed " + str(len(fixed)) + " files")
    else:
        issues = []
        for f in files:
            result = check_file(f, license, holder)
            if not result["ok"]:
                issues.append(f)
        if len(issues) == 0:
            success("All files have correct headers")
        else:
            for f in issues:
                error(f + ": missing header")
            fail("Found issues")

command(
    name = "lint.copyright",
    help = "Check or fix copyright headers",
    run = run,
)
```

## Runtime Context

When an extension command executes, the runtime provides a context object.

### Context Structure

```python
def run(ctx):
    # Flags - resolved via CLI → ENV → Config → Default
    fix = ctx.args.get("fix") == "true"
    path = ctx.args.get("path")

    # Configuration - merged defaults + star.yaml
    cfg = config.get().lint.copyright

    # Dry run mode
    if ctx.dry_run:
        note("Would fix files...")
```

### What the Runtime Provides

| Component | Source | Access |
|-----------|--------|--------|
| **Flags** | Command `flags:` section | `ctx.args.get("flag_name")` |
| **Config** | Extension `config:` + `star.yaml` | `config.get().lint.copyright` |
| **Metadata** | Global flags (`--dry-run`) | `ctx.dry_run` |

## Files

| File | Purpose |
|------|---------|
| `internal/extension/spec.go` | ExtensionSpec, CommandSpec, validation |
| `internal/extension/discovery.go` | Extension discovery and loading |
| `internal/extension/registry.go` | Global extension registry |
| `internal/config/config.go` | Config loading/saving |
| `internal/starlark/runtime.go` | Starlark runtime, extension loading |
| `internal/starlark/receiver.go` | BaseReceiver, HasAttrs interface |
| `internal/starlark/receiver_file.go` | FileReceiver (fs operations) |
| `internal/starlark/receiver_config.go` | ConfigReceiver (config access) |
| `internal/starlark/receiver_regexp.go` | RegexpReceiver (regex operations) |
| `internal/wasm/host.go` | WASM host runtime |

## Design Principles

1. **Reverse domain naming** — Extension identifier = directory name
2. **One file per command** — Commands in `commands/` subdirectory
3. **Per-receiver capabilities** — WASM sandboxing at receiver level
4. **Convention over configuration** — Command name determines CLI paths
5. **Starlark-first** — Commands implemented in Starlark
6. **Go escape hatch** — Receivers for performance/system access
7. **No backwards compatibility** — Clean break from legacy formats
