---
title: "Go Style Linter Extension"
status: proposal
created: 2026-03-14
updated: 2026-03-15
---

# Plan: Go Style Linter Extension

## Summary

A `star lint go-style` command that enforces the NobleFactor Go Style Guidelines on Go source files. The linter is
built as a **pluggable rule engine**: each rule is an independent Starlark script with a known contract. The extension
ships default rules, and any rule can be replaced by dropping an override script in the project's configuration
directory.

## Goals

1. **Pluggable rules** — each rule is a standalone `.star` script implementing a two-function contract (`check` and
   `fix`). Replace any rule by providing an override script.
2. **Enforce doc comment structure** — summary line, blank separator, paragraph fill to 120 columns,
   Parameters/Returns sections, signature-to-doc synchronization
3. **Enforce region hierarchy** — every struct with methods has the required region markers
4. **Enforce line-width filling** — all text (code and comments) fills to column 120 before wrapping
5. **Enforce file layout** — copyright, package, imports, init, guards, vars, struct, types, methods in correct order
6. **Enforce receiver consistency** — no mixed pointer/value receivers on the same type
7. **Fix mode** — rewrite files to comply, not just report

## Non-Goals

- Replacing `gofmt` (whitespace, indentation, brace placement)
- Replacing `golangci-lint` (static analysis, bug detection, unused code)
- Replacing `go vet` (correctness checks)
- Enforcing naming conventions (would require semantic understanding of intent)

## Architecture

### Rule Engine

The command `star lint go-style` is a thin orchestrator that discovers rules, loads them, and runs them against each
file. Rules are independent — they don't know about each other.

```text
star lint go-style
  │
  ├─ discover rules (built-in + overrides)
  ├─ collect .go files (respecting --path, --exclude, --generated, --tests)
  │
  └─ for each file:
       └─ for each rule:
            ├─ check mode: rule.check(ctx) → []Violation
            └─ fix mode:   rule.fix(ctx) → modified content or None
```

### Rule Contract

Every rule is a Starlark script that defines two functions:

```python
def check(ctx):
    """Check a single file for violations.

    Parameters:
        ctx: RuleContext with fields:
            - path:     string — absolute file path
            - lines:    list[string] — file content split by newline
            - funcs:    list — go.funcs(path) result
            - methods:  dict — {receiver_type: go.methods(path, receiver_type=...)}
            - structs:  list — go.structs(path) result
            - type_doc: func(type) — go.type_doc(path, type)
            - config:   dict — merged config for this rule

    Returns:
        list[Violation] — each is {"line": int, "message": string, "rule": string}
    """

def fix(ctx):
    """Fix violations in a single file.

    Parameters:
        ctx: RuleContext (same as check)

    Returns:
        string — the fixed file content, or None if no changes needed
    """
```

The orchestrator calls `check` in check mode and `fix` in fix mode. A rule that cannot auto-fix returns `None` from
`fix` and relies on `check` to report the violation.

### Rule Discovery and Override Precedence

Rules are resolved using the standard config precedence chain:

1. **CLI** — `--rule=doc-comments` selects which rules to run
2. **Environment** — `STAR_LINT_GO_STYLE_DISABLED_RULES=regions,formatting` disables specific rules
3. **Project config** — `star.yaml` under `lint.go-style.disabled_rules` and `lint.go-style.overrides`
4. **Global config** — `~/.config/star/star.yaml` same keys

Override scripts are loaded from the project's `.star/lint/go-style/` directory. An override completely replaces the
built-in rule of the same name — it must implement the full contract.

### Rule Naming

Each rule has a short kebab-case name that matches its script filename:

| Rule           | Script                    | Description                                                  |
| -------------- | ------------------------- | ------------------------------------------------------------ |
| `doc-comments` | `rules/doc-comments.star` | Doc comment structure, fill width, Parameters/Returns, stale |
| `regions`      | `rules/regions.star`      | Method region hierarchy markers                              |
| `method-order` | `rules/method-order.star` | Alphabetical ordering within regions and delineators         |
| `file-layout`  | `rules/file-layout.star`  | Top-level declaration ordering                               |
| `receivers`    | `rules/receivers.star`    | Consistent pointer/value receiver types                      |
| `line-width`   | `rules/line-width.star`   | Maximum 120-column line length                               |
| `formatting`   | `rules/formatting.star`   | Blank line rules (after signature, before return)            |

### Go Receiver API Extensions

Two new Go receiver methods support the rules that need performance-sensitive operations:

**`go.rewrap_comments(path, width)`** — rewraps all doc comment paragraphs in a Go file to fill to the specified column
width. Skips indented code blocks (4+ spaces or tab-indented after `//`). Returns the modified file content.

**`go.sort_declarations(path, scope, order)`** — reorders Go declarations within a scope. Parameters:
- `path` — the file to modify
- `scope` — `"file"` (top-level), `"struct:<Name>"` (methods of a struct), or `"region:<name>"` (within a region)
- `order` — `"alpha"` (alphabetical), `"kind"` (by delineator: compensable → fallible → actions)

Returns the modified file content. Preserves doc comments and blank lines attached to each declaration.

## Command

```
star lint go-style [flags] [path]
```

### Flags

| Flag          | Type   | Default | Description                                            |
| ------------- | ------ | ------- | ------------------------------------------------------ |
| `--fix`       | bool   | false   | Rewrite files to fix violations (otherwise check-only) |
| `--path`      | string | "."     | Root directory to scan                                 |
| `--exclude`   | string | ""      | Comma-separated glob patterns to exclude               |
| `--generated` | bool   | true    | Include generated files (`DO NOT EDIT`)                |
| `--tests`     | bool   | true    | Include test files (`_test.go`)                        |
| `--rule`      | string | ""      | Run only this rule (e.g., `doc-comments`)              |
| `--verbose`   | bool   | false   | Print each file as it is checked                       |

### Exit Codes

| Code | Meaning                                                       |
| ---- | ------------------------------------------------------------- |
| 0    | All files compliant (or all violations fixed in `--fix` mode) |
| 1    | Violations found (check mode)                                 |
| 2    | Internal error (parse failure, I/O error)                     |

## Rules

### `doc-comments` — Doc comment structure and freshness

Checks every exported method and unexported helper for:

1. **Summary line**: first line is an imperative verb phrase
2. **Blank separator**: blank `//` line between summary and extended description
3. **Fill width**: all comment text fills to column 120 before wrapping. A line that could absorb the next line's first
   word without exceeding 120 is under-filled. Only indented code blocks (4+ spaces after `//`) are exempt from
   rewrapping.
4. **Parameters section**: present on every method/function with parameters. Each parameter in the Go signature must
   have a corresponding `//   - paramName:` entry.
5. **Returns section**: present on every method/function with return values. Void methods omit Returns. Each return
   type in the Go signature must have a corresponding entry.
6. **Signature synchronization**: the doc comment's Parameters and Returns sections must match the current Go
   signature. Violations:
   - Parameter added to signature but missing from doc
   - Parameter removed from signature but still documented
   - Parameter renamed in signature but doc has old name
   - Return type added, removed, or changed but doc doesn't reflect it

**Fix behavior**: rewraps all comment text to fill to 120 (except indented code blocks). Inserts missing Parameters
and Returns sections with TODO placeholders for each parameter name and return type. Removes stale parameter/return
entries that no longer match the signature. Updates renamed parameters.

### `regions` — Method region hierarchy

Checks every struct with methods for:

1. `// region EXPORTED METHODS` / `// endregion` wrapping all exported methods
2. `// region UNEXPORTED METHODS` / `// endregion` wrapping all unexported methods
3. `// region State management` / `// endregion` sub-region (if getters/setters exist)
4. `// region Behaviors` / `// endregion` sub-region (if non-getter methods exist)
5. Empty sub-regions are omitted (not flagged as missing)

**Fix behavior**: inserts region markers around method groups. Does not reorder methods.

### `method-order` — Method sort order

Checks within each region/delineator group:

1. State management before Behaviors
2. Within Behaviors: Compensable actions → Fallible actions → Actions
3. Within each group: alphabetical (CompensateX immediately follows X)

**Fix behavior**: calls `go.sort_declarations` to reorder methods within their groups. Preserves attached doc comments
and blank lines.

### `file-layout` — File element ordering

Checks the order of top-level declarations:

1. Copyright header → package → imports → init → guards → vars → main struct → types → methods → other structs

**Fix behavior**: reports violations only. Reordering top-level declarations is too risky for auto-fix.

### `receivers` — Consistent receiver types

Checks that all methods on a type use the same receiver kind (pointer or value). Exception: `UnmarshalJSON` and other
`encoding` interface methods that require pointer receivers on value types.

**Fix behavior**: reports violations only. Changing receiver types can break interface satisfaction.

### `line-width` — Maximum line length

Checks that no line exceeds 120 columns. Applies to code and comments equally.

**Fix behavior**: calls `go.rewrap_comments` for comment lines. Code lines require manual restructuring.

### `formatting` — Blank line rules

Checks:

1. Blank line after function signature for multi-statement bodies
2. No blank line after function signature for single-statement bodies
3. No blank line before `return` in short functions

**Fix behavior**: adds or removes blank lines as needed.

## Extension Structure

```text
com.noblefactor.star.LintGoStyle/
├── extension.yaml
├── commands/
│   └── lint-go-style.star          # orchestrator
└── rules/
    ├── doc-comments.star
    ├── regions.star
    ├── method-order.star
    ├── file-layout.star
    ├── receivers.star
    ├── line-width.star
    └── formatting.star
```

### Overriding a Rule

To replace the `doc-comments` rule in a project, create:

```text
.star/lint/go-style/doc-comments.star
```

The script must implement `check(ctx)` and `fix(ctx)` with the same contract. The orchestrator loads it instead of the
built-in `doc-comments.star`.

### extension.yaml

```yaml
extension: com.noblefactor.star.LintGoStyle
description: Enforce Go style guidelines with pluggable rules

receivers:
  - name: go
    builtin: true
    type: GoReceiver
    description: Go AST operations
  - name: file
    builtin: true
    type: FileReceiver
    description: Filesystem operations
  - name: regexp
    builtin: true
    type: RegexpReceiver
    description: Regular expression operations
  - name: config
    builtin: true
    type: ConfigReceiver
    description: Configuration access

commands:
  - name: lint.go-style
    help: Enforce Go style guidelines (pluggable rule engine)
    implementation: commands/lint-go-style.star
    flags:
      - name: fix
        type: bool
        default: "false"
        help: Rewrite files to fix violations
      - name: path
        type: string
        default: "."
        help: Root directory to scan
      - name: exclude
        type: string
        default: ""
        help: Comma-separated glob patterns to exclude
      - name: generated
        type: bool
        default: "true"
        help: Include generated files (DO NOT EDIT)
      - name: tests
        type: bool
        default: "true"
        help: Include test files (_test.go)
      - name: rule
        type: string
        default: ""
        help: Run only this rule
      - name: verbose
        type: bool
        default: "false"
        help: Print each file as it is checked

config:
  path: lint.go-style
  type: GoStyleConfig
  fields:
    enabled: bool
    line_width: int
    exclude: "[]string"
    disabled_rules: "[]string"
  defaults:
    enabled: true
    line_width: 120
    exclude:
      - "**/vendor/**"
      - "**/testdata/**"
    disabled_rules: []
```

## Integration

### Pre-commit Hook

```yaml
hooks:
  pre-commit:
    - lint.go-style --path=. --fix
```

### CI

Check mode (no `--fix`). Violations fail the build.

### make check

```makefile
go-style: ## Enforce Go style guidelines
    $(STAR) lint go-style --path=.

check: vet lint shell-lint complexity go-style test
```

## Limitations

1. **Code block detection** — the rewrapper skips indented code blocks (4+ spaces after `//`). Other manual formatting
   (ASCII tables, aligned columns) outside of code blocks will be reflowed. Authors should use indented code blocks for
   any content that must preserve exact formatting.

2. **Override contract enforcement** — no compile-time check that an override script implements `check` and `fix`. The
   orchestrator validates at load time and fails fast with a clear error.

## Decisions

1. **Comment rewrapping scope**: rewrap everything — summary, extended description, Parameters, Returns. Only indented
   code blocks are exempt.

2. **TODO stubs**: `--fix` inserts TODO stubs with placeholders for each parameter name and return type when
   Parameters/Returns sections are missing.

3. **Signature synchronization**: `doc-comments` cross-references the Go signature against the doc comment. Stale,
   missing, or renamed parameters and returns are violations. `--fix` updates the doc to match the signature.

4. **Method reordering**: implemented via `go.sort_declarations` Go receiver API, not in Starlark. The receiver
   handles the AST-level reordering; the rule script calls it.

5. **Comment rewrapping**: implemented via `go.rewrap_comments` Go receiver API for performance. The receiver handles
   the text processing; the rule script calls it.

6. **Per-project rule disabling**: `disabled_rules` in `star.yaml` config allows disabling specific rules per-project.

7. **Override resolution**: follows standard config precedence — CLI → environment → project config → global config.
