---
title: "Writing Extensions"
description: "A step-by-step guide to creating star extensions"
---

# Writing Extensions

This guide explains how to create extensions for the `star` CLI. Extensions add new commands and capabilities to star.

## What is an Extension?

An extension is a package that provides one or both of:

- **Command**: A CLI subcommand implemented in Starlark (e.g., `star lint copyright`)
- **Receivers**: Go binding functions that Starlark commands can call

Extensions are defined by a YAML specification file (`extension.yaml`) and optionally a Starlark implementation file.

## Extension Types

| Type | Provides | Example |
|------|----------|---------|
| **Command-only** | CLI command, no new bindings | `lint.all` - orchestrates other linters |
| **Binding-only** | Reusable functions, no command | `copyright` - provides `copyright.check()` for other extensions |
| **Full** | Both command and bindings | `lint.copyright` - command AND binding functions |

## Quick Start

Create a minimal command-only extension in 5 minutes:

### 1. Create the extension directory

```bash
mkdir -p extensions/hello
```

### 2. Create extension.yaml

```yaml
# extensions/hello/extension.yaml
extension: hello
description: Say hello

command:
  help: Print a greeting message
  implementation: hello.star

flags:
  - name: name
    type: string
    default: "World"
    help: Name to greet
```

### 3. Create the Starlark implementation

```python
# extensions/hello/hello.star

def run(ctx):
    """Entry point for the command."""
    name = ctx.args.get("name", "World")
    success("Hello, " + name + "!")
```

### 4. Run it

```bash
star hello              # Hello, World!
star hello --name=Alice # Hello, Alice!
```

## Extension YAML Specification

The `extension.yaml` file defines your extension's metadata, command, flags, and configuration.

### Required Fields

```yaml
extension: lint.copyright          # Unique identifier (determines command path)
description: Check copyright headers  # Brief description
```

### Command Section

Define a CLI subcommand:

```yaml
command:
  help: |
    Check or fix copyright headers in source files.

    Examples:
      star lint copyright
      star lint copyright --fix
  implementation: lint-copyright.star  # Starlark file with run(ctx)
```

### Flags Section

Define command-line flags:

```yaml
flags:
  - name: fix
    type: bool
    default: "false"
    help: Add missing headers
  - name: path
    type: string
    default: "."
    help: Path to check
```

Supported types: `bool`, `string`, `int`, `float`, `glob`

### Receivers Section

Declare binding functions (for full extensions):

```yaml
receivers:
  - name: copyright
    builtin: true                    # Compiled into star binary
    type: CopyrightChecker           # Go type name
    description: Copyright header checking
    functions:
      check: Verify files have correct headers
      fix: Add or update headers
      detect_license: Detect SPDX from LICENSE file
```

### Config Section

Define extension configuration schema:

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

## Naming Conventions

The extension name determines several paths:

| Extension Name | Command | Config Path | Env Var Prefix |
|----------------|---------|-------------|----------------|
| `lint.copyright` | `star lint copyright` | `lint.copyright` | `STAR_LINT_COPYRIGHT_` |
| `setup.hooks` | `star setup hooks` | `setup.hooks` | `STAR_SETUP_HOOKS_` |
| `hello` | `star hello` | `hello` | `STAR_HELLO_` |

## Starlark Command Implementation

Every command needs a `run(ctx)` function:

```python
def run(ctx):
    """Entry point for the command."""
    # Your implementation here
```

### Accessing Flags

```python
def run(ctx):
    fix = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")
```

### Loading Configuration

```python
def run(ctx):
    cfg = config.get()
    copyright_cfg = cfg.lint.copyright

    if not copyright_cfg.enabled:
        warn("Copyright checking is disabled")
        return

    license = copyright_cfg.license
    holder = copyright_cfg.holder
```

### Calling Binding Functions

Receivers are available as global modules:

```python
def run(ctx):
    # Call the copyright receiver
    result = copyright.check(
        paths=files,
        license="MIT",
        holder="Noble Factor",
    )

    if result.passed:
        success("All files pass")
    else:
        for issue in result.issues:
            error(issue.file + ": " + issue.message)
        fail("Found " + str(result.count) + " issues")
```

### Output Functions

Use these functions to communicate with the user:

| Function | Purpose | Example |
|----------|---------|---------|
| `success(msg)` | Report success (green) | `success("All tests pass")` |
| `fail(msg)` | Report failure and exit (red) | `fail("3 errors found")` |
| `error(msg)` | Report error (red, continues) | `error("file.go: missing header")` |
| `warn(msg)` | Report warning (yellow) | `warn("Config not set")` |
| `note(msg)` | Informational message | `note("Checking 42 files...")` |

### Filesystem Operations

Use the `fs` receiver for file operations:

```python
def run(ctx):
    # Find files matching a pattern
    go_files = fs.glob("**/*.go")
    star_files = fs.glob("**/*.star")

    for f in go_files:
        note("Found: " + f)
```

## Built-in Receivers

These receivers are always available:

| Receiver | Functions | Purpose |
|----------|-----------|---------|
| `config` | `get()` | Load merged configuration |
| `fs` | `glob(pattern)` | Find files matching pattern |
| `go` | `structs()`, `const_groups()`, `methods()`, `funcs()`, `calls()`, `composites()`, `return_string()`, `raw_string()`, `metrics()`, `deps()` | Go source analysis and AST queries |
| `lint` | `go()`, `shell()`, `markdown()` | Run linters |
| `copyright` | `check()`, `fix()`, `detect_license()` | Copyright header operations |

### Go Source Analysis (`go`)

The `go` receiver provides Go AST query primitives for static analysis of Go source code.

#### Struct and type queries

- `go.structs(path)` — list struct definitions with fields, tags, comments
- `go.const_groups(path, type=None)` — list typed const groups, optionally filtered by type name

#### Function and method queries

- `go.methods(path, name=None, receiver_type=None, returns=None)` — find methods with optional filters
- `go.funcs(path, name=None)` — find functions (non-method)

#### Scope queries (within a function/method body)

Methods and funcs return a `.scope` attribute — an opaque string. Pass it to these functions to query within the function body:

- `go.calls(scope, name=None)` — find function calls, each with `.args[]` containing `.string_value` and `.ident_name`
- `go.composites(scope, type=None)` — find composite literals with `.fields`
- `go.return_string(scope)` — extract string from `return "..."` statement
- `go.raw_string(scope)` — extract first backtick string literal

#### Code metrics

- `go.metrics(path)` — LOC, functions, structs, etc.
- `go.deps(path)` — import analysis

## Configuration Schema

### Simple Fields

```yaml
config:
  type: MyConfig
  fields:
    enabled: bool
    name: string
    count: int
  defaults:
    enabled: true
    name: "default"
    count: 10
```

### Complex Types

```yaml
config:
  type: LintConfig
  fields:
    exclude: "[]string"           # List of strings
    patterns: "map[string]any"    # Map with any values
  defaults:
    exclude:
      - "**/vendor/**"
      - "**/testdata/**"
    patterns:
      go:
        match: "// Copyright.*"
```

### Accessing Nested Config

```python
def run(ctx):
    cfg = config.get()

    # Access list
    exclude = list(cfg.lint.copyright.exclude)

    # Access map
    patterns = cfg.lint.copyright.patterns
    go_pattern = patterns.get("go")
```

## Complete Example

Here's the full `lint.copyright` extension:

**extension.yaml:**

```yaml
extension: lint.copyright
description: Check or fix copyright headers in source files

receivers:
  - name: copyright
    builtin: true
    type: CopyrightChecker
    description: Copyright header checking
    functions:
      check: Verify files have correct headers
      fix: Add or update headers
      detect_license: Detect SPDX from LICENSE file
  - name: config
    builtin: true
    type: ConfigReceiver
    description: Configuration access
    functions:
      get: Load merged config
  - name: fs
    builtin: true
    type: FSReceiver
    description: Filesystem operations
    functions:
      glob: Find files matching pattern

command:
  help: Check or fix copyright headers in source files
  implementation: lint-copyright.star

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
    patterns: map[string]any
    exclude: "[]string"
  defaults:
    enabled: false
    license: "auto"
```

**lint-copyright.star:**

```python
def collect_source_files(path, exclude_patterns):
    """Collect source files from path, excluding patterns."""
    files = []

    for pattern in ["**/*.go", "**/*.star", "**/*.sh"]:
        for f in fs.glob(path + "/" + pattern):
            files.append(f)

    # Filter excluded patterns
    filtered = []
    for f in files:
        excluded = False
        for pattern in exclude_patterns:
            if pattern.endswith("/**") and pattern[:-3] in f:
                excluded = True
                break
        if not excluded:
            filtered.append(f)

    return filtered

def run(ctx):
    """Check or fix copyright headers in source files."""
    fix = ctx.args.get("fix", "false") == "true"
    path = ctx.args.get("path", ".")

    # Load config
    cfg = config.get()
    copyright_cfg = cfg.lint.copyright

    if not copyright_cfg.enabled:
        warn("Copyright checking is disabled in star.yaml")
        return

    license = copyright_cfg.license
    holder = copyright_cfg.holder
    exclude = list(copyright_cfg.exclude)

    # Collect files
    files = collect_source_files(path, exclude)
    note("Checking " + str(len(files)) + " source files...")

    if fix:
        result = copyright.fix(paths=files, license=license, holder=holder)
        if result.count > 0:
            success("Fixed " + str(result.count) + " files")
        else:
            success("All files have correct headers")
    else:
        result = copyright.check(paths=files, license=license, holder=holder)
        if result.passed:
            success("All files have correct headers")
        else:
            for issue in result.issues:
                error(issue.file + ": " + issue.message)
            fail("Found " + str(result.count) + " issues")
```

## Testing Extensions

### Run Your Extension

```bash
# Test with defaults
star <your-command>

# Test with flags
star <your-command> --flag=value

# Test with environment variable override
STAR_YOUR_COMMAND_FLAG=value star <your-command>
```

### Check Configuration Resolution

```bash
# Show resolved config values
star config show
```

### Verify YAML Syntax

```bash
# Lint your extension.yaml
yamllint extensions/your-extension/extension.yaml
```

## Next Steps

- See existing extensions in `extensions/` for more examples
- Read [Configuration Guide](./configuration.md) for configuration details
- Check [Architecture](../architecture/devlore-extension-model.md) for full technical details
