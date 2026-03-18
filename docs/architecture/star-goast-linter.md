---
title: "Comment Taxonomy"
description: "Architecture for parsing doc comments with participle: context-aware lexer, unordered element collection, per-element printers, YAML-defined schemas"
status: draft
created: 2026-03-17
updated: 2026-03-17
---

# Comment Taxonomy

This document defines a comment taxonomy — a grammar-based system for parsing, validating, and printing doc comments
deterministically. Comments are parsed into typed elements (Paragraph, Section, List, CodeBlock, Directive, etc.)
that can appear in any order but print in a defined order. Each element type has its own Printer.

The taxonomy uses [participle](https://github.com/alecthomas/participle) for parsing. A context-aware lexer injects
code element names (parameter names, return types) as first-class tokens. The grammar can then match these tokens
directly — a Parameters list item must reference an actual parameter name, or it fails to parse. Schemas are defined
in YAML and loaded by goast extensions at runtime.

## Motivation

`go/ast` attaches doc comments to every relevant node (`.Doc` on `FuncDecl`, `TypeSpec`, `GenDecl`, `Field`). Today,
the goast provider extracts node metadata but treats the doc comment as an opaque string. Linter rules re-parse that
string with ad-hoc line walking — scanning backwards from function line numbers, searching for section headers,
extracting parameter names by splitting on colons.

This creates three problems:

1. **Fragile parsing.** Line-number walking assumes comments are contiguous and immediately above the declaration.
   Section detection relies on exact string matching.
2. **Non-deterministic formatting.** Hand-rolled reflow helpers produce output that varies depending on input
   formatting. Two semantically identical comments can produce different output.
3. **No connection to code.** The comment parser doesn't know what parameters exist, what types are returned, or
   what the function is called. Validation is a separate step that re-derives this information.

## Design overview

### Three-part architecture

```text
┌─────────────────────────────────────────────────────────────────────┐
│  1. LEXER (context-aware, per-function)                              │
│     Input: raw comment text + code element names from AST            │
│     Output: token stream with ParamName, ReturnType as token types   │
├─────────────────────────────────────────────────────────────────────┤
│  2. PARSER (participle, grammar via struct tags)                     │
│     Input: token stream                                              │
│     Output: []*DocElement — unordered collection of typed elements   │
├─────────────────────────────────────────────────────────────────────┤
│  3. NORMALIZE (one method per element type)                          │
│     Input: typed elements                                            │
│     Output: single-line-per-element text in canonical order          │
│     No wrapping, no prefix — just structure and ordering             │
├─────────────────────────────────────────────────────────────────────┤
│  4. FORMAT (go/doc/comment — single pass)                            │
│     Input: normalized text                                           │
│     Output: final formatted comment with // prefix, line wrapping,   │
│             list indentation, code block pass-through                │
└─────────────────────────────────────────────────────────────────────┘
```

### Element types — the building blocks

Doc comments are composed from these element types. They can appear in any order in the input but print in a
schema-defined order.

| Element     | Content                                    | Cardinality in FuncDecl   |
| ----------- | ------------------------------------------ | ------------------------- |
| Paragraph   | Plain text, reflowed to column width       | 1+ (first = summary)      |
| Heading     | `# Title` line                             | 0+                        |
| CodeBlock   | Indented lines, passed through verbatim    | 0+                        |
| Section     | Named section with child elements          | 0+ (Parameters, Returns)  |
| List        | Bullet items (`- text`)                    | 0+                        |
| Directive   | `+devlore:key value`, verbatim             | 0+                        |
| Table       | Aligned columns (future)                   | 0+                        |
| Diagram     | ASCII art block (future)                   | 0+                        |

### Unordered parsing, ordered printing

Participle collects elements via a wrapper struct with alternation (`@@*`). Each iteration tries each element
type in order and populates exactly one field. After parsing, the schema defines the print order.

```text
Input (any order)              Parse                     Print (canonical order)
─────────────────              ─────                     ──────────────────────
Paragraph "Backup..."    ──►   Element{Paragraph}   ──►  1. Summary paragraph
Directive "+devlore:..."  ──►   Element{Directive}   ──►  2. Body paragraphs
Section "Parameters:"    ──►   Element{Section}     ──►  3. Directives
                                                          4. Parameters section
                                                          5. Returns section
```

### Context-aware lexer

The lexer is constructed **per-function** with code element names injected as token types. Because participle
lexer rules are tried in order, named tokens match before generic `Word` tokens.

```text
Function signature:  func (p *Provider) Backup(resource Resource, opts *BackupOpts) (Resource, Tombstone, error)
                                                ────────           ────               ────────  ─────────  ─────
Injected tokens:                                ParamName          ParamName           ReturnType ReturnType ReturnType

Comment text:        "- resource: The file to back up."
Token stream:        ListMarker  ParamName("resource")  Colon  Word("The")  Word("file")  ...

Grammar match:       ListMarker @ParamName Colon @@    ← ParamItem matches because "resource" is a ParamName token
                     ListMarker @ParamName Colon @@    ← "path" would be Word, not ParamName → no match → diagnostic
```

### Schema-driven via YAML

Schemas are data, not compiled Go. The goast provider loads schema YAML at startup and uses it to configure the
participle parser per node type and format.

```yaml
schemas:
  func_doc:
    format: go
    elements:
      - {name: summary, type: paragraph, required: true, order: 1}
      - {name: body, type: paragraph, cardinality: "*", order: 2}
      - {name: directives, type: directive, cardinality: "*", order: 3}
      - {name: parameters, type: param_section, required: if_params, order: 4,
         header: "Parameters:", item_tokens: param_names}
      - {name: returns, type: return_section, required: if_returns, order: 5,
         header: "Returns:", item_tokens: return_types}
```

## Parallel AST–comment structure

`go/ast` already attaches `.Doc` to every relevant node. The taxonomy **mines** those attachments, parsing the
raw text into typed elements. Provider result types gain a `.Comment` field carrying the parsed structure.

```text
go/ast nodes                 Provider results              Parsed comment
═══════════                  ════════════════              ══════════════

ast.File ──────────────────► FileResult ────────────────► CopyrightDoc
  .Doc ─── comment text ──►   .Comment ──── parse ────►     .SPDX
  .Comments                    .Path                         .Copyright

ast.FuncDecl ──────────────► MethodResult ──────────────► FuncDoc
  .Doc ─── comment text ──►   .Comment ──── parse ────►     .Elements[]*DocElement
  .Name ──────────────────►   .Name                          (Paragraph, Directive,
  .Type.Params ───────────►   .Params  ─── inject ────►      Section{ParamItem...},
  .Type.Results ──────────►   .Returns ─── tokens ────►      Section{ReturnItem...})

ast.TypeSpec ──────────────► StructResult ──────────────► TypeDoc
  .Doc ─── comment text ──►   .Comment ──── parse ────►     .Elements[]*DocElement

ast.Field ─────────────────► FieldDetail ───────────────► FieldDoc
  .Comment ── comment ─────►   .Comment ──── parse ────►     .Inline
```

## Canonical example: `Backup` method

This example is the reference for implementation and test development.

### Source

```go
// Backup creates a timestamped copy of the resource. Existing backups
// are overwritten.
//
// +devlore:defaults overwrite=true
//
// Parameters:
//   - resource: The file to back up.
//   - opts: Backup options (default: nil).
//
// Returns:
//   - Resource: The backup copy.
//   - Tombstone: Compensation state.
//   - error: Non-nil if the backup failed.
func (p *Provider) Backup(resource Resource, opts *BackupOpts) (Resource, Tombstone, error) {
```

### Step 1: AST extraction

`go/parser.ParseFile` produces:

```go
ast.FuncDecl{
    Name: &ast.Ident{Name: "Backup"},
    Doc:  &ast.CommentGroup{/* 13 comment lines */},
    Recv: /* *Provider */,
    Type: &ast.FuncType{
        Params:  /* resource Resource, opts *BackupOpts */,
        Results: /* Resource, Tombstone, error */,
    },
}
```

### Step 2: Lexer construction

The goast provider extracts parameter names and return types from the AST, then constructs the lexer:

```go
paramNames  := []string{"resource", "opts"}
returnTypes := []string{"Resource", "Tombstone", "error"}
lex := newDocLexer(paramNames, returnTypes)
```

The lexer definition (dynamically built):

```go
lexer.MustStateful(lexer.Rules{
    "Root": {
        {"BlankLine",     `\n\s*\n`, nil},
        {"DirectiveMark", `\+`, lexer.Push("Directive")},
        {"SectionHeader", `(?:Parameters|Returns):`, nil},
        {"ListMarker",    `-\s+`, nil},
        {"CodeLine",      `    .+`, nil},
        {"ParamName",     `resource|opts`, nil},          // ← injected from AST
        {"ReturnType",    `Resource|Tombstone|error`, nil}, // ← injected from AST
        {"Colon",         `:`, nil},
        {"Word",          `\S+`, nil},
        {"whitespace",    `[ \t]+`, nil},
        {"Newline",       `\n`, nil},
    },
    "Directive": {
        {"DirectiveKey",   `[\w:]+`, nil},
        {"DirectiveValue", `.+`, lexer.Pop()},
    },
})
```

`ParamName` and `ReturnType` rules appear **before** `Word`. The lexer tries rules in order, so `resource`
matches as `ParamName`, not `Word`. An undocumented name like `path` would match only as `Word`.

### Step 3: Comment text preparation

Strip `// ` prefix from each line of `ast.CommentGroup`:

```text
Backup creates a timestamped copy of the resource. Existing backups
are overwritten.

+devlore:defaults overwrite=true

Parameters:
  - resource: The file to back up.
  - opts: Backup options (default: nil).

Returns:
  - Resource: The backup copy.
  - Tombstone: Compensation state.
  - error: Non-nil if the backup failed.
```

### Step 4: Lexer produces token stream

```text
Word("Backup")  Word("creates")  Word("a")  Word("timestamped")  Word("copy")
Word("of")  Word("the")  ParamName("resource")  ...  Word("overwritten.")
BlankLine
DirectiveMark("+")  DirectiveKey("devlore:defaults")  DirectiveValue("overwrite=true")
BlankLine
SectionHeader("Parameters:")
ListMarker("- ")  ParamName("resource")  Colon(":")  Word("The")  Word("file")  ...
ListMarker("- ")  ParamName("opts")      Colon(":")  Word("Backup")  Word("options")  ...
BlankLine
SectionHeader("Returns:")
ListMarker("- ")  ReturnType("Resource")   Colon(":")  Word("The")  Word("backup")  ...
ListMarker("- ")  ReturnType("Tombstone")  Colon(":")  Word("Compensation")  ...
ListMarker("- ")  ReturnType("error")      Colon(":")  Word("Non-nil")  ...
```

Note: `resource` in the summary paragraph also matches as `ParamName`. The grammar handles this — `Paragraph`
accepts both `Word` and `ParamName` tokens as text. Only `ParamItem` requires `@ParamName` specifically.

### Step 5: Participle grammar parses tokens

Grammar structs (with participle struct tags):

```go
// Paragraph — one or more text tokens (words, param names, return types, colons).
type Paragraph struct {
    Words []string `@(Word | ParamName | ReturnType | Colon)+`
}

// Directive — +key value.
type Directive struct {
    Key   string `DirectiveMark @DirectiveKey`
    Value string `@DirectiveValue`
}

// ParamItem — "- resource: The file to back up."
// @ParamName only matches tokens in the function's actual parameter list.
type ParamItem struct {
    Name string     `ListMarker @ParamName Colon`
    Desc *Paragraph `@@`
}

// ParamSection — "Parameters:" followed by zero or more ParamItems.
type ParamSection struct {
    Items []*ParamItem `"Parameters" Colon @@*`
}

// ReturnItem — "- Resource: The backup copy."
type ReturnItem struct {
    Type string     `ListMarker @ReturnType Colon`
    Desc *Paragraph `@@`
}

// ReturnSection — "Returns:" followed by zero or more ReturnItems.
type ReturnSection struct {
    Items []*ReturnItem `"Returns" Colon @@*`
}

// CodeBlock — indented lines, verbatim.
type CodeBlock struct {
    Lines []string `@CodeLine+`
}

// Heading — markdown-style heading in a doc comment.
type Heading struct {
    Text []string `"#" @(Word | ParamName | ReturnType)+`
}

// DocElement — wrapper struct for unordered alternation.
// Exactly one field is non-nil after each parse iteration.
type DocElement struct {
    Directive     *Directive     `  @@`
    ParamSection  *ParamSection  `| @@`
    ReturnSection *ReturnSection `| @@`
    CodeBlock     *CodeBlock     `| @@`
    Heading       *Heading       `| @@`
    Paragraph     *Paragraph     `| @@`
}

// FuncDoc — collect elements in whatever order they appear.
type FuncDoc struct {
    Elements []*DocElement `@@*`
}
```

Participle's `@@*` tries each alternative per iteration. The first match wins. `Directive` is tried before
`Paragraph` because `+` at the start of a line is unambiguous. `ParamSection` and `ReturnSection` are tried
before `Paragraph` because `SectionHeader` is unambiguous.

### Step 6: Parse result

```go
FuncDoc{
    Elements: []*DocElement{
        {Paragraph: &Paragraph{
            Words: ["Backup", "creates", "a", "timestamped", "copy", "of", "the",
                    "resource.", "Existing", "backups", "are", "overwritten."],
        }},
        {Directive: &Directive{
            Key:   "devlore:defaults",
            Value: "overwrite=true",
        }},
        {ParamSection: &ParamSection{
            Items: []*ParamItem{
                {Name: "resource", Desc: &Paragraph{
                    Words: ["The", "file", "to", "back", "up."],
                }},
                {Name: "opts", Desc: &Paragraph{
                    Words: ["Backup", "options", "(default:", "nil)."],
                }},
            },
        }},
        {ReturnSection: &ReturnSection{
            Items: []*ReturnItem{
                {Type: "Resource",  Desc: &Paragraph{Words: ["The", "backup", "copy."]}},
                {Type: "Tombstone", Desc: &Paragraph{Words: ["Compensation", "state."]}},
                {Type: "error",     Desc: &Paragraph{Words: ["Non-nil", "if", "the",
                                                              "backup", "failed."]}},
            },
        }},
    },
}
```

### Step 7: Normalize — one line per element, canonical order

Each element type has a `Normalize() string` method that produces a single unwrapped line. `FuncDoc.Normalize`
sorts elements by schema order and concatenates with correct blank-line separators.

```go
func (p *Paragraph) Normalize() string {
    return strings.Join(p.Words, " ")
}

func (d *Directive) Normalize() string {
    return "+" + d.Key + " " + d.Value
}

func (s *ParamSection) Normalize() string {
    var lines []string
    lines = append(lines, "Parameters:")
    for _, item := range s.Items {
        lines = append(lines, "  - "+item.Name+": "+item.Desc.Normalize())
    }
    return strings.Join(lines, "\n")
}

func (s *ReturnSection) Normalize() string {
    var lines []string
    lines = append(lines, "Returns:")
    for _, item := range s.Items {
        lines = append(lines, "  - "+item.Type+": "+item.Desc.Normalize())
    }
    return strings.Join(lines, "\n")
}
```

`FuncDoc.Normalize` assembles elements in schema order, one empty line between sections:

```go
func (d *FuncDoc) Normalize() string {
    // 1. Summary — first paragraph (one unwrapped line)
    // 2. Body — remaining paragraphs, headings, code blocks
    // 3. Directives (one line each)
    // 4. Parameters section (header + one line per item)
    // 5. Returns section (header + one line per item)
    // Sections separated by one empty line.
    // Items within a section on consecutive lines.
}
```

Normalized output for the Backup example:

```text
Backup creates a timestamped copy of the resource. Existing backups are overwritten.

+devlore:defaults overwrite=true

Parameters:
  - resource: The file to back up.
  - opts: Backup options (default: nil).

Returns:
  - Resource: The backup copy.
  - Tombstone: Compensation state.
  - error: Non-nil if the backup failed.
```

Each element is one unwrapped line. No `//` prefix. No line wrapping. No indentation beyond list formatting.
This is the canonical intermediate form.

### Step 8: Format — `go/doc/comment` handles everything else

The normalized text is passed through `go/doc/comment.Parser` + `Printer` in a single pass:

```go
func Format(normalized string, width int) string {
    var p comment.Parser
    doc := p.Parse(normalized)

    var pr comment.Printer
    pr.TextWidth = width - 3  // 120 - len("// ")
    return string(pr.Comment(doc))
}
```

`go/doc/comment.Printer` handles:
- Line wrapping paragraphs to `TextWidth`
- List item continuation indentation
- Code block pass-through (indented lines)
- `// ` prefix on every line
- Blank line separators between blocks

Directive lines (`+devlore:defaults overwrite=true`) are short enough to fit on one line — reflow is a no-op.

Output with `width = 120`:

```go
// Backup creates a timestamped copy of the resource. Existing backups are overwritten.
//
// +devlore:defaults overwrite=true
//
// Parameters:
//   - resource: The file to back up.
//   - opts: Backup options (default: nil).
//
// Returns:
//   - Resource: The backup copy.
//   - Tombstone: Compensation state.
//   - error: Non-nil if the backup failed.
```

Deterministic. Same input data always produces the same output, regardless of the order elements appeared in
the original comment. Our code owns structure and ordering; `go/doc/comment` owns formatting.

## Copyright schema

```go
// CopyrightDoc — fixed-order, both fields required, verbatim.
type CopyrightDoc struct {
    SPDX      string `"SPDX-License-Identifier" Colon @Word`
    Copyright string `"Copyright" @(Word | Colon)+`
}
```

Parses:

```text
SPDX-License-Identifier: SSPL-1.0
Copyright (c) 2025-2026 Noble Factor. All rights reserved.
```

Normalize:

```go
func (c *CopyrightDoc) Normalize() string {
    return fmt.Sprintf("SPDX-License-Identifier: %s\nCopyright %s", c.SPDX, c.Copyright)
}
```

Passed through `Format()` to add `// ` prefix.

## Type schema

```go
// TypeDocElement — wrapper for unordered alternation.
type TypeDocElement struct {
    CodeBlock *CodeBlock `  @@`
    Heading   *Heading   `| @@`
    Paragraph *Paragraph `| @@`
}

// TypeDoc — collect elements in any order.
type TypeDoc struct {
    Elements []*TypeDocElement `@@*`
}
```

First paragraph is Summary. Remaining elements are Body. Print order: Summary, then Body elements in parse order.

Example:

```go
// Resource represents a handle to data that can be streamed.
type Resource struct { ... }
```

Parse result:

```go
TypeDoc{
    Elements: []*TypeDocElement{
        {Paragraph: &Paragraph{
            Words: ["Resource", "represents", "a", "handle", "to", "data",
                    "that", "can", "be", "streamed."],
        }},
    },
}
```

## Parameter validation via token types

The context-aware lexer makes parameter validation structural rather than post-hoc. Consider three scenarios:

### Correct: all parameters documented

```go
// Parameters:
//   - resource: The file to back up.
//   - opts: Backup options.
```

Token stream: `ListMarker ParamName("resource") Colon ... ListMarker ParamName("opts") Colon ...`

`ParamItem` grammar matches both. Parse succeeds. Linter compares documented names against signature — all present.

### Error: stale parameter name

```go
// Parameters:
//   - path: The file to back up.       ← "path" is not a parameter
//   - opts: Backup options.
```

Token stream: `ListMarker Word("path") Colon ...`

`ParamItem` expects `@ParamName` but gets `Word`. The grammar does not match a `ParamItem`. The parser can
report this as a diagnostic: `"path" is not a parameter of Backup`.

### Error: missing parameter

```go
// Parameters:
//   - resource: The file to back up.
//                                       ← "opts" not documented
```

Parse succeeds (the grammar allows zero or more `ParamItem`). The linter compares `ParamSection.Items` names
against `MethodResult.Params` names and reports: `parameter 'opts' not documented`.

## Schema YAML format

Schemas serialize as YAML, loaded by goast extensions at runtime. Adding a new comment format requires a new
YAML file, not Go code changes.

```yaml
# doc-comment-schemas.yaml
schemas:
  copyright:
    format: go
    node_type: File
    elements:
      - name: spdx
        type: verbatim
        required: true
        order: 1
      - name: copyright
        type: verbatim
        required: true
        order: 2

  type_doc:
    format: go
    node_type: TypeSpec
    elements:
      - name: summary
        type: paragraph
        required: true
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2

  func_doc:
    format: go
    node_type: FuncDecl
    elements:
      - name: summary
        type: paragraph
        required: true
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2
      - name: directives
        type: directive
        cardinality: "*"
        order: 3
      - name: parameters
        type: param_section
        required: if_params
        order: 4
        header: "Parameters:"
        item_tokens: param_names
      - name: returns
        type: return_section
        required: if_returns
        order: 5
        header: "Returns:"
        item_tokens: return_types

  # Same slots, different format
  func_doc_star:
    format: star
    node_type: FuncDecl
    elements:
      - name: summary
        type: paragraph
        required: true
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2
      - name: parameters
        type: param_section
        required: if_params
        order: 3
        header: "Args:"
        item_tokens: param_names
```

The `item_tokens` field tells the schema engine which injected token type to use for matching list items.
`param_names` means the lexer injects the function's parameter names as `ParamName` tokens. `return_types`
injects return types as `ReturnType` tokens.

## Graceful degradation

When the parser matches some elements but not others, it populates whatever **did** match. The schema's
`required` and `cardinality` fields drive validation:

| Situation                        | Parser behavior              | Linter behavior                        |
| -------------------------------- | ---------------------------- | -------------------------------------- |
| Summary present, no Parameters   | Parses summary only          | Checks `required: if_params`           |
| Parameters present, stale name   | `ParamItem` fails to match   | Reports stale name diagnostic          |
| All elements present, wrong order | Parses all elements          | Printers output in canonical order     |
| Empty doc comment                | No elements parsed           | Reports missing required `summary`     |

## Provider result types

Existing result types gain a `.Comment` field carrying the parsed comment structure:

```go
type MethodResult struct {
    Name         string        `starlark:"name"`
    ReceiverType string        `starlark:"receiver_type"`
    Returns      string        `starlark:"returns"`
    Params       []ParamDetail `starlark:"params"`
    File         string        `starlark:"file"`
    Line         int           `starlark:"line"`
    Doc          string        `starlark:"doc"`      // raw text (kept for compat)
    Comment      *FuncDoc      `starlark:"comment"`  // parsed structure
    Scope        string        `starlark:"scope"`
}

type FuncResult struct {
    Name    string        `starlark:"name"`
    Returns string        `starlark:"returns"`
    Params  []ParamDetail `starlark:"params"`
    File    string        `starlark:"file"`
    Line    int           `starlark:"line"`
    Doc     string        `starlark:"doc"`
    Comment *FuncDoc      `starlark:"comment"`
    Scope   string        `starlark:"scope"`
}

type StructResult struct {
    Name    string        `starlark:"name"`
    File    string        `starlark:"file"`
    Line    int           `starlark:"line"`
    Fields  []FieldDetail `starlark:"fields"`
    Comment *TypeDoc      `starlark:"comment"`
}
```

## Starlark integration

Starlark rules access parsed comment elements directly — no line walking, no text scanning:

```python
for m in goast.methods(path=path):
    c = m.comment
    if not c.summary:
        violation("missing summary")

    # Parameter sync — compare documented params against signature
    sig_params = [p.name for p in m.params if p.name]
    doc_params = [item.name for item in c.param_section.items] if c.param_section else []

    for s in sig_params:
        if s not in doc_params:
            violation("parameter '" + s + "' not documented")
    for d in doc_params:
        if d not in sig_params:
            violation("documented parameter '" + d + "' not in signature")
```

The `_extract_doc_lines`, `_has_section`, `_extract_param_names`, and `_check_fill_width` functions in the
current `doc-comments.star` are eliminated entirely.

## Layers

```text
┌─────────────────────────────────────────────────────────────────────┐
│  Starlark rules (.star)                                              │
│  m.comment.summary, m.comment.param_section.items, ...               │
│  goast.format_comment(comment, width) → rendered string              │
├─────────────────────────────────────────────────────────────────────┤
│  goast Provider (provider.go)                                        │
│  Methods(), Funcs(), Structs() → results with .Comment               │
│  FormatComment(), RewrapComments()                                   │
├──────────────┬──────────────────┬────────────────────────────────────┤
│  Participle  │  Normalize       │  go/ast                            │
│  parser +    │  (one line per   │  .Doc fields → raw comment text    │
│  context-    │  element, canon  │  .Type.Params → param names        │
│  aware lexer │  order)          │  .Type.Results → return types      │
├──────────────┴──────────────────┴────────────────────────────────────┤
│  go/doc/comment (single formatting pass)                             │
│  Line wrapping, list indentation, code block pass-through, // prefix │
├─────────────────────────────────────────────────────────────────────┤
│  Schema YAML                                                         │
│  Defines element types, cardinality, required policy, element order  │
└─────────────────────────────────────────────────────────────────────┘
```

## Scope

### What the taxonomy covers

- Doc comments attached to AST nodes: functions, methods, types, fields, files
- Parsing raw comment text into typed elements via participle
- Context-aware validation: parameter names and return types as lexer tokens
- Deterministic rendering via per-element Printers
- Paragraph reflow via `go/doc/comment.Printer`
- Schema definitions in YAML, loaded at runtime

### What the taxonomy does not cover

- **Structural comments** — region markers (`// region EXPORTED METHODS`), delineators
  (`// Compensable actions`), endregion markers. These are positional, not attached to AST nodes. Handled by
  the `regions` and `method-order` linter rules.
- **Code formatting** — indentation, brace placement, blank lines. That's `go/format` and the `formatting`
  linter rule.
- **Declaration reordering** — `SortDeclarations` is a separate concern.

## Design decisions

1. **Participle for parsing.** A PEG parser via Go struct tags gives us a real grammar with typed parse trees,
   alternation, cardinality constraints, and composable element types. Replaces fragile regex extraction.

2. **Context-aware lexer.** The lexer is constructed per-function with parameter names and return types injected
   as first-class token types. The grammar matches these tokens directly — parameter validation is structural,
   not a post-hoc string comparison.

3. **Unordered collection, ordered normalization.** Participle's `@@*` with a wrapper struct collects elements
   in whatever order they appear. Each element type has a `Normalize()` method producing a single unwrapped
   line. The schema defines canonical output order.

4. **Normalize-then-format pipeline.** Elements produce single-line content via `Normalize()`. Concatenated
   with correct blank-line separators. The assembled text goes through `go/doc/comment.Printer` in one final
   pass for line wrapping, list indentation, code block pass-through, and `// ` prefix. Our code owns
   structure and ordering; `go/doc/comment` owns formatting. Directive lines are short enough that reflow
   is a no-op — no strip-and-reinsert needed.

5. **Schema as data.** Schemas are YAML, not compiled Go. Element types, cardinality, required policy, and print
   order are defined declaratively. Adding a new comment format (Starlark, shell) is a new YAML file, not a code
   change.

6. **Mine the AST.** `go/ast` already attaches `.Doc` to nodes. The taxonomy parses that existing attachment.
   Parameter names and return types come from `.Type.Params` and `.Type.Results` on the same AST node. No
   separate correlation step.

7. **Graceful degradation.** Partial matches populate whatever elements parsed successfully. The schema's
   `required` and `cardinality` fields drive validation of what's present vs. what's expected.

8. **Format-agnostic element types.** `Paragraph`, `ParamItem`, `ReturnItem`, `Directive` are format-agnostic
   data. Only the lexer (comment prefix rules) and Printers (prefix strings) are format-specific.

## References

- [participle](https://github.com/alecthomas/participle) — PEG parser via Go struct tags
- [`go/doc/comment` stdlib](https://pkg.go.dev/go/doc/comment) — paragraph reflow engine
- [Go style guidelines](docs/guides/go-style-guidelines.md) — Section 4: Doc Comment Format
- [Go Doc Comments specification](https://go.dev/doc/comment) — the format our schemas implement
