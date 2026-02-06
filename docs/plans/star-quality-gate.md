# Plan: Star as Unified Quality Gate

## Summary

Make `star` the single source of truth for pre-commit hooks and quality gates across all NobleFactor repositories, replacing direct configuration of individual linters.

## Goals

1. **Single tool**: `star` manages all quality checks
2. **Unified config**: `star.yaml` replaces `.golangci.yaml`, `.markdownlint-cli2.yaml`, etc.
3. **Replace pre-commit**: `star` hooks replace pre-commit framework
4. **Cross-repo consistency**: Same checks enforced everywhere
5. **Extensible**: New checks added via Starlark scripts

## Current State

### Implemented

| Command | Tool | Status |
|---------|------|--------|
| `star lint go` | golangci-lint | ✅ Working |
| `star lint shell` | shellcheck, shfmt | ✅ Working |
| `star lint markdown` | markdownlint-cli2 | ✅ Working |
| `star setup hooks` | pre-commit install | ✅ Working |
| `star setup tools` | Check tool availability | ✅ Working |

### Not Implemented

| Feature | Purpose |
|---------|---------|
| Copyright header checks | Ensure SPDX headers on all source files |
| Frontmatter validation | Validate YAML frontmatter in markdown guides |
| YAML/JSON lint | Schema validation for config files |
| Native git hooks | Replace pre-commit with star-managed hooks |
| `star lint all` | Run all applicable linters |

## Requirements

### Copyright Header Checks

All source files must have SPDX license headers:

```go
// SPDX-License-Identifier: SSPL-1.0
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.
```

**Configuration** (`star.yaml`):
```yaml
copyright:
  license: SSPL-1.0
  holder: Noble Factor
  year_range: 2025-2026
  patterns:
    go: "// SPDX-License-Identifier: {license}\n// Copyright (c) {years} {holder}. All rights reserved."
    star: "# SPDX-License-Identifier: {license}\n# Copyright (c) {years} {holder}. All rights reserved."
    shell: "# SPDX-License-Identifier: {license}\n# Copyright (c) {years} {holder}. All rights reserved."
  exclude:
    - "**/testdata/**"
    - "**/vendor/**"
    - "**/*_test.go"  # Optional: exclude test files
```

**Commands**:
```
star lint copyright           # Check all files
star lint copyright --fix     # Add missing headers
```

### Frontmatter Validation

Markdown guides must have valid YAML frontmatter:

```yaml
---
title: Guide Title
description: One-line description
category: guides/lore
tags: [deployment, packages]
---
```

**Configuration** (`star.yaml`):
```yaml
frontmatter:
  required_fields:
    - title
    - description
  optional_fields:
    - category
    - tags
    - author
    - date
  paths:
    - "docs/guides/**/*.md"
    - "docs/cli/**/*.md"
  exclude:
    - "**/README.md"
    - "docs/index.md"
```

**Commands**:
```
star lint frontmatter         # Validate frontmatter
star lint frontmatter --fix   # Generate missing frontmatter (interactive)
```

### Native Git Hooks

Replace pre-commit framework with star-managed hooks:

**Hook installation**:
```
star setup hooks              # Install star as git hook
```

**Generated `.git/hooks/pre-commit`**:
```bash
#!/bin/sh
exec star hook pre-commit "$@"
```

**Hook execution**:
```
star hook pre-commit          # Run pre-commit checks
star hook pre-push            # Run pre-push checks (optional)
```

**Configuration** (`star.yaml`):
```yaml
hooks:
  pre-commit:
    - lint go
    - lint shell
    - lint markdown
    - lint copyright
    - lint frontmatter
  pre-push:
    - test go
```

### Unified Lint Command

```
star lint all                 # Run all configured linters
star lint all --fix           # Run all with auto-fix where supported
```

## Implementation Phases

### Phase 1: Copyright Checks

1. Add `builtin_copyright.go` with:
   - `copyright.check(paths, config)` - Check files for headers
   - `copyright.fix(paths, config)` - Add missing headers
   - `copyright.template(lang, config)` - Generate header for language

2. Add `ops/lint-copyright.star`:
   - Read config from `star.yaml`
   - Glob source files
   - Check/fix headers

3. Update `ops/lint.star`:
   - Add `star lint copyright` command

### Phase 2: Frontmatter Validation

1. Add `builtin_frontmatter.go` with:
   - `frontmatter.parse(content)` - Extract YAML frontmatter
   - `frontmatter.validate(content, schema)` - Validate against schema
   - `frontmatter.generate(title, description)` - Generate frontmatter

2. Add `ops/lint-frontmatter.star`:
   - Read config from `star.yaml`
   - Glob markdown files
   - Validate frontmatter

3. Update `ops/lint.star`:
   - Add `star lint frontmatter` command

### Phase 3: Native Git Hooks

1. Update `builtin_setup.go`:
   - `setup.install_hook(name, commands)` - Install star as hook
   - `setup.uninstall_hook(name)` - Remove hook

2. Add `ops/hook.star`:
   - `star hook pre-commit` - Execute pre-commit checks
   - `star hook pre-push` - Execute pre-push checks

3. Update `ops/setup.star`:
   - `star setup hooks` installs star hooks (not pre-commit)

### Phase 4: Replace Pre-commit

1. Remove `.pre-commit-config.yaml` from repos
2. Update CI to use `star lint all`
3. Update documentation

## Configuration Schema

Full `star.yaml` schema:

```yaml
# star.yaml - Unified quality gate configuration

lint:
  go:
    enabled: true
    config: .golangci.yaml  # Or inline config
  shell:
    enabled: true
    shellcheck:
      severity: warning
    shfmt:
      indent: 4
  markdown:
    enabled: true
    config: .markdownlint-cli2.yaml  # Or inline config
  copyright:
    enabled: true
    license: SSPL-1.0
    holder: Noble Factor
    year_range: 2025-2026
  frontmatter:
    enabled: true
    required_fields: [title, description]
    paths: ["docs/guides/**/*.md"]

hooks:
  pre-commit:
    - lint go
    - lint shell
    - lint markdown
    - lint copyright
    - lint frontmatter
```

## Migration Path

### For devlore-cli

1. Add `star.yaml` with current lint config
2. Update `.pre-commit-config.yaml` to call `star lint all`
3. Validate equivalent behavior
4. Remove `.pre-commit-config.yaml`
5. Update CI to use `star lint all`

### For noblefactor-ops

1. Already has `star.yaml`
2. Add copyright and frontmatter config
3. Test hooks
4. Remove `.pre-commit-config.yaml`

### For devlore-registry

1. Add `star.yaml`
2. Configure for YAML/JSON validation
3. Migrate from pre-commit

## Related Documents

- [PLAN-starlark-extensions.md](../../PLAN-starlark-extensions.md) - Overall star architecture
- Issue #15 - `star gh rotate-secrets`
- Issue #23 - `star setup` design (closed)

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/starlark/builtin_copyright.go` | Create |
| `internal/starlark/builtin_frontmatter.go` | Create |
| `ops/lint-copyright.star` | Create |
| `ops/lint-frontmatter.star` | Create |
| `ops/hook.star` | Create |
| `ops/lint.star` | Modify - add copyright, frontmatter, all |
| `ops/setup.star` | Modify - native hooks |
| `cmd/star/main.go` | Modify - add hook command |
