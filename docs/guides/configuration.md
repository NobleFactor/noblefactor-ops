---
title: "Configuration Guide"
description: "Understanding star configuration with the extension model"
---

# Configuration Guide

This guide explains how configuration works in `star` with the extension-based model.

## Overview

Star uses a layered configuration system where each extension defines its own configuration schema. Configuration values are resolved from multiple sources in priority order.

## Configuration Hierarchy

Values are resolved in this order (highest priority first):

1. **CLI flags**: `--fix`, `--path=./src`
2. **Environment variables**: `STAR_LINT_COPYRIGHT_FIX=true`
3. **User config file**: `star.yaml`
4. **Extension defaults**: Defined in `extension.yaml`

### Example Resolution

For the `lint.copyright` extension with `--fix` flag:

```
CLI:         star lint copyright --fix
             ↓ (highest priority)
Env:         STAR_LINT_COPYRIGHT_FIX=true
             ↓
star.yaml:   lint.copyright.fix: false
             ↓
extension:   default: "false"
             ↓ (lowest priority)
Result:      fix = true (from CLI)
```

## star.yaml Format

The `star.yaml` file is the primary configuration file. Its structure mirrors extension names.

### Basic Structure

```yaml
# star.yaml - Unified quality gate configuration

lint:
  go:
    path: "./..."
  shell:
    path: "."
    severity: warning
  copyright:
    enabled: true
    license: MIT
    holder: Noble Factor
```

### Extension Name to Config Path

| Extension | Config Path | YAML Location |
|-----------|-------------|---------------|
| `lint.go` | `lint.go` | `lint.go:` |
| `lint.copyright` | `lint.copyright` | `lint.copyright:` |
| `setup.hooks` | `setup.hooks` | `setup.hooks:` |

### Complete Example

```yaml
# star.yaml

lint:
  go:
    path: "./..."

  shell:
    path: "."
    severity: warning
    indent: 4

  markdown:
    path: "."
    exclude:
      - "vendor/**"
      - "node_modules/**"
    frontmatter:
      required:
        - title
        - description
    config:
      MD013: false
      MD033: false

  copyright:
    enabled: true
    license: MIT
    holder: Noble Factor
    patterns:
      go:
        match: '// SPDX-License-Identifier: \S+\s*\n// Copyright.*'
        replace: |
          // SPDX-License-Identifier: {license}
          // Copyright {holder}. All rights reserved.
      star:
        match: '# SPDX-License-Identifier: \S+\s*\n# Copyright.*'
        replace: |
          # SPDX-License-Identifier: {license}
          # Copyright {holder}. All rights reserved.
    exclude:
      - "**/testdata/**"
      - "**/vendor/**"

precommit:
  hooks:
    - id: star-lint-all
      name: Star quality gate
      entry: star lint all --
      language: system
```

## Environment Variables

Environment variables override config file values. The naming convention:

```
STAR_<EXTENSION>_<FLAG>=value
```

### Naming Rules

1. Start with `STAR_`
2. Replace dots with underscores
3. Convert to uppercase

| Extension | Flag | Environment Variable |
|-----------|------|---------------------|
| `lint.copyright` | `fix` | `STAR_LINT_COPYRIGHT_FIX` |
| `lint.copyright` | `path` | `STAR_LINT_COPYRIGHT_PATH` |
| `lint.go` | `skip_mod_tidy` | `STAR_LINT_GO_SKIP_MOD_TIDY` |

### Usage Examples

```bash
# Override a single flag
STAR_LINT_COPYRIGHT_FIX=true star lint copyright

# Override multiple flags
STAR_LINT_GO_PATH="./cmd/..." STAR_LINT_GO_CONFIG=".golangci.yml" star lint go

# In CI environments
export STAR_LINT_COPYRIGHT_ENABLED=true
star lint all
```

### Boolean Values

Boolean environment variables accept:

- **True**: `true`, `1`, `yes`, `on`
- **False**: `false`, `0`, `no`, `off`

```bash
STAR_LINT_COPYRIGHT_ENABLED=true   # enabled
STAR_LINT_COPYRIGHT_ENABLED=1      # enabled
STAR_LINT_COPYRIGHT_ENABLED=false  # disabled
STAR_LINT_COPYRIGHT_ENABLED=0      # disabled
```

## Per-Project vs Per-User Configuration

Star looks for configuration in multiple locations:

### Project Configuration (Recommended)

```
./star.yaml          # Project root
```

Project configuration is checked into version control and shared by all developers.

### User Configuration

```
~/.config/star/star.yaml    # Linux/macOS
%APPDATA%\star\star.yaml    # Windows
```

User configuration provides personal defaults that don't override project settings.

### Resolution Order

1. CLI flags (highest)
2. Environment variables
3. Project `star.yaml`
4. User `~/.config/star/star.yaml`
5. Extension defaults (lowest)

## Configuration Types

### Boolean

```yaml
lint:
  copyright:
    enabled: true    # or false
```

### String

```yaml
lint:
  copyright:
    license: MIT
    holder: "Noble Factor"
```

### Integer

```yaml
lint:
  shell:
    indent: 4
```

### List

```yaml
lint:
  copyright:
    exclude:
      - "**/vendor/**"
      - "**/testdata/**"
```

### Map

```yaml
lint:
  copyright:
    patterns:
      go:
        match: '// Copyright.*'
        replace: '// Copyright {holder}'
      star:
        match: '# Copyright.*'
        replace: '# Copyright {holder}'
```

## Troubleshooting

### View Resolved Configuration

```bash
star config show
```

This displays the final merged configuration from all sources.

### Common Issues

**Config not being applied:**

1. Check YAML syntax: `yamllint star.yaml`
2. Verify indentation (YAML is whitespace-sensitive)
3. Check extension name matches config path exactly

**Environment variable not working:**

1. Verify naming: `STAR_` prefix, underscores, uppercase
2. Check for typos in extension or flag name
3. Ensure variable is exported: `export STAR_...=value`

**Extension defaults not appearing:**

1. Run `star config show` to see resolved values
2. Check the extension's `extension.yaml` for default definitions

### Debug Configuration Loading

```bash
# Show verbose output
star --verbose config show

# Check a specific extension's config
star config show | grep -A 10 "lint.copyright"
```

## Next Steps

- See [Writing Extensions](./writing-extensions.md) to create your own extensions
- Check `extensions/*/extension.yaml` for configuration schema examples
- Read [Architecture](../architecture/star-extensions.md) for technical details
