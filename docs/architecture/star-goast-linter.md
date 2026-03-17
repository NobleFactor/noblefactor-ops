---
title: "Comment Taxonomy"
description: "Architecture for format-agnostic comment schemas: regex named groups parse into typed slots, Go templates render deterministic output"
status: draft
created: 2026-03-17
updated: 2026-03-17
---

# Comment Taxonomy

This document defines a comment taxonomy — a per-node-type schema of typed comment slots that makes doc comment
formatting fully deterministic. You fill the slots, the format is predictable.

The taxonomy is format-agnostic: regex patterns with named capture groups parse comments into slots, and Go
templates render slots back to text. The same slot types work across comment formats (`//`, `#`, `--`) by swapping
the pattern and template while keeping the data model unchanged.

## Motivation

The Go AST already attaches doc comments to every relevant node via `.Doc` fields on `FuncDecl`, `TypeSpec`,
`GenDecl`, and `Field`. Today, the goast provider extracts node metadata (name, params, returns) but treats the
doc comment as an opaque string. Linter rules re-parse that string with ad-hoc line walking — scanning backwards
from function line numbers, searching for section headers like `Parameters:`, extracting parameter names by
splitting on colons.

This creates three problems:

1. **Fragile parsing.** Line-number walking (`_extract_doc_lines`) assumes comments are contiguous and immediately
   above the declaration. Ad-hoc section detection (`_has_section`) relies on exact string matching.
2. **Non-deterministic formatting.** Hand-rolled reflow helpers (`rewrapCommentGroup`, `fillWords`) produce output
   that varies depending on input formatting. Two semantically identical comments can produce different output.
3. **Duplicated logic.** The Starlark linter rule and the Go provider both parse doc comments — differently.

## Design

### Core idea: mine the AST's existing comment attachment

`go/ast` already does the hard work of associating doc comments with their owning nodes. The taxonomy **mines**
those existing attachments, parsing the raw text into typed slots. Provider result types (`MethodResult`,
`FuncResult`, `StructResult`, `FieldDetail`) gain a `.Comment` field carrying the parsed taxonomy. Starlark rules
access typed slots directly — no line-number walking, no text scanning.

### Parallel AST–comment structure

Every `go/ast` node that carries a `.Doc` field gets a parallel parsed comment object with typed slots. The goast
provider creates both in a single pass — node metadata and parsed comment travel together.

```text
go/ast nodes                 Provider results              Comment taxonomy
═══════════                  ════════════════              ════════════════

ast.File ──────────────────► FileResult ────────────────► FileComment
  .Doc ─── comment text ──►   .Comment ──── parse ────►     .Copyright  ← VerbatimSlot
  .Comments                    .Path                         .PackageDoc ← ParagraphSlot
                               .Package

ast.FuncDecl ──────────────► MethodResult ──────────────► FuncComment
  .Doc ─── comment text ──►   .Comment ──── parse ────►     .Summary    ← ParagraphSlot
  .Name ──────────────────►   .Name                          .Body       ← BlockSlot
  .Type.Params ───────────►   .Params                        .Directives ← DirectiveSlot
  .Type.Results ──────────►   .Returns                       .Parameters ← ParamListSlot
                              .Line                          .Returns    ← ReturnListSlot

ast.FuncDecl ──────────────► FuncResult ────────────────► FuncComment
  .Doc ─── comment text ──►   .Comment ──── parse ────►     (same schema as above)

ast.TypeSpec ──────────────► StructResult ──────────────► TypeComment
  .Doc ─── comment text ──►   .Comment ──── parse ────►     .Summary    ← ParagraphSlot
                              .Name                          .Body       ← BlockSlot
                              .Fields

ast.Field ─────────────────► FieldDetail ───────────────► FieldComment
  .Comment ── comment ─────►   .Comment ──── parse ────►     .Inline     ← VerbatimSlot
                               .Name
                               .Type
```

### Comment schemas

Each Go node type has a comment schema with named, typed slots:

```text
File
  ├─ Copyright     (VerbatimSlot — required, SPDX + Copyright lines)
  └─ PackageDoc    (ParagraphSlot — optional, "// Package foo provides...")

FuncDecl
  ├─ Summary       (ParagraphSlot — required, imperative verb phrase)
  ├─ Body          (BlockSlot — optional, extended description: paragraphs, code, headings)
  ├─ Directives    (DirectiveSlot — optional, +devlore:key=value)
  ├─ Parameters    (ParamListSlot — required if func has params)
  └─ Returns       (ReturnListSlot — required if func has returns)

TypeSpec
  ├─ Summary       (ParagraphSlot — required, type description)
  └─ Body          (BlockSlot — optional, extended description)

Field
  └─ Inline        (VerbatimSlot — optional, trailing comment on same line)
```

### Slot types

| Slot type      | Content                           | Parsing                                  | Rendering                               |
| -------------- | --------------------------------- | ---------------------------------------- | --------------------------------------- |
| ParagraphSlot  | Plain text                        | Named group → raw text                   | `{{ reflow }}` template function        |
| BlockSlot      | Paragraphs, code blocks, headings | Named group → raw text                   | `{{ reflow }}` preserving code blocks   |
| DirectiveSlot  | `+devlore:key=value` lines        | Named group → sub-parse into Key/Value   | `{{ range }}` verbatim output           |
| ParamListSlot  | `[]ParamDoc{Name, Desc}`          | Named group → sub-parse into Name/Desc   | `{{ range }}` with `{{ reflow }}` desc  |
| ReturnListSlot | `[]ReturnDoc{Type, Desc}`         | Named group → sub-parse into Type/Desc   | `{{ range }}` with `{{ reflow }}` desc  |
| VerbatimSlot   | Raw text lines                    | Named group → raw text                   | Passed through unchanged                |

## Parse and render

### Two-level parsing

Parsing uses a two-level regex hierarchy:

**Level 1 — Section pattern.** One regex per schema with named capture groups. Each group captures one
section's raw text. The group name **is** the slot name. Unmatched optional groups produce empty strings —
the slot's `Required` policy distinguishes "missing because optional" from "missing but required." The linter
reports the latter; fix mode can populate the missing slot.

**Level 2 — Item pattern.** For repeating slots (ParamList, ReturnList, Directive), the slot type defines an
item pattern that splits the captured section text into individual items. The slot type determines the data
model (`ParamDoc{Name, Desc}`); the item pattern determines how to extract those fields from each line.

```text
Level 1: Section regex (one per schema)
──────────────────────────────────────────────────────
  (?P<summary>...)  (?P<body>...)  (?P<parameters>...)
         │                │               │
         ▼                ▼               ▼
     ParagraphSlot    BlockSlot     ParamListSlot
     (no level 2)    (no level 2)  (has level 2)
                                        │
Level 2: Item regex (on slot type)      ▼
──────────────────────────────────────────────────────
  Item pattern: (?P<name>\w+):\s+(?P<desc>.+)
         │              │
         ▼              ▼
  ParamDoc{Name, Desc}  (repeated for each item)
```

Non-repeating slots (Paragraph, Block, Verbatim) have no level-2 pattern — the captured text is the value.
Repeating slots carry an `ItemPattern` on the `SlotDef` that can override the slot type's default, enabling
format-specific item syntax while keeping the data model unchanged.

### Comment prefix backreference

Multi-line comment patterns use a backreference to ensure prefix consistency within a single comment block.
The first group captures the comment prefix (`//`, `#`, `--`, etc.), and subsequent lines backreference it:

```regex
^(?P<prefix>//|#)\s+(?P<summary>[^\n]+)
(?:\n(?P=prefix)\s*\n(?P=prefix)\s+(?P<body>[^\n]+))?
```

This guarantees that a comment starting with `//` can't accidentally match a `#` line mid-comment. The
copyright linter already uses this pattern with `(?P<prefix>//|#)` and `(?P=prefix)`.

### Parse: section regex with named capture groups

Go FuncDecl section pattern:

```regex
(?s)
^(?P<prefix>//)\s+(?P<summary>[^\n]+)
(?:\n(?P=prefix)\s*\n(?P<body>(?:(?P=prefix)\s+[^\n]+\n?)+))?
(?:\n(?P=prefix)\s*\n(?P<directives>(?:(?P=prefix)\s+\+[^\n]+\n?)+))?
(?:\n(?P=prefix)\s*\n(?P=prefix)\s+Parameters:\n(?P<parameters>(?:(?P=prefix)\s+\s+-\s+[^\n]+\n?)+))?
(?:\n(?P=prefix)\s*\n(?P=prefix)\s+Returns:\n(?P<returns>(?:(?P=prefix)\s+\s+-\s+[^\n]+\n?)+))?
```

Same slot names, different section pattern for Starlark:

```regex
(?s)
^(?P<prefix>#)\s+(?P<summary>[^\n]+)
(?:\n(?P=prefix)\s*\n(?P<body>(?:(?P=prefix)\s+[^\n]+\n?)+))?
(?:\n(?P=prefix)\s*\n(?P=prefix)\s+Args:\n(?P<parameters>(?:(?P=prefix)\s+\s+\w+:\s+[^\n]+\n?)+))?
```

The slots are identical — `summary`, `body`, `parameters` — only the section regex and section headers differ.

### Level-2 item patterns

Each repeating slot type has a default item pattern. The schema can override it for format-specific syntax.

| Slot type      | Default item pattern                             | Produces            |
| -------------- | ------------------------------------------------ | ------------------- |
| ParamListSlot  | `(?P<name>\w+):\s+(?P<desc>.+)`                 | `[]ParamDoc`        |
| ReturnListSlot | `(?P<type>\w+):\s+(?P<desc>.+)`                 | `[]ReturnDoc`       |
| DirectiveSlot  | `\+(?P<key>[\w:]+)\s+(?P<value>.+)`             | `[]Directive`       |

The item pattern uses the same named-group-to-field mapping as the section pattern. Group names map to struct
fields in the item type.

### Graceful degradation on partial match

When the section regex matches some groups but not others, the taxonomy populates whatever slots **did** match
and leaves unmatched optional slots empty. This is the normal case for comments that don't have all sections:

```go
// Exists checks whether a resource exists.        ← summary matches
//                                                   ← no body, no directives
// Parameters:                                      ← parameters matches
//   - resource: The resource to check.
//                                                   ← no returns (returns bool, error)
```

The linter evaluates `Required` policy against the populated slots:
- `summary`: present → ok
- `body`: absent, `Required: Never` → ok
- `parameters`: present → validate item sync against signature
- `returns`: absent, `Required: IfReturns`, function returns `(bool, error)` → violation

### Render: Go templates

Each schema has a Go template (`text/template`). Slot values are template data. Custom template functions handle
reflow.

Go FuncDecl template:

```go
{{ .Summary | reflow 117 }}
//
{{- if .Body }}
{{ .Body | reflow 117 }}
//
{{- end }}
{{- range .Directives }}
// +{{ .Key }} {{ .Value }}
{{- end }}
{{- if .Directives }}
//
{{- end }}
{{- if .Params }}
// Parameters:
{{- range .Params }}
//   - {{ .Name }}: {{ .Desc | reflow 113 }}
{{- end }}
{{- end }}
{{- if .Returns }}
//
// Returns:
{{- range .Returns }}
//   - {{ .Type }}: {{ .Desc | reflow 113 }}
{{- end }}
{{- end }}
```

`reflow` is a custom template function backed by `go/doc/comment.Printer`. The width argument accounts for the
prefix: `117 = 120 - len("// ")`, `113 = 120 - len("//   - ")`.

Same slots, different template for Starlark:

```go
{{ .Summary | reflow 118 }}
#
{{- if .Params }}
# Args:
{{- range .Params }}
#   {{ .Name }}: {{ .Desc | reflow 114 }}
{{- end }}
{{- end }}
```

Same data, different output format. The template controls the comment prefix, section headers, and layout.

### Cross-format summary

```text
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Go pattern   │     │  Star pattern │     │  Shell pattern│
│  // prefix    │     │  # prefix     │     │  # prefix     │
│  Parameters:  │     │  Args:        │     │  Arguments:   │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       ▼                    ▼                    ▼
  Named groups:        Named groups:        Named groups:
  summary, body,       summary, body,       summary,
  directives,          parameters           parameters
  parameters,
  returns
       │                    │                    │
       ▼                    ▼                    ▼
┌──────────────────────────────────────────────────────────┐
│            Shared slot types and data model               │
│  FuncComment { Summary, Body, Directives, Params, ... }  │
└──────────────────────────┬───────────────────────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
       ┌────────────┐ ┌──────────┐ ┌──────────┐
       │ Go template │ │ Star tmpl │ │ Shell tmpl│
       │ // prefix   │ │ # prefix  │ │ # prefix  │
       └─────┬──────┘ └────┬─────┘ └────┬─────┘
             ▼             ▼             ▼
        Formatted      Formatted     Formatted
        Go comment     Star comment  Shell comment
```

### Copyright example

The existing copyright linter already uses this pattern. One regex, named groups map to slot values:

```regex
^(?P<prefix>//|#)\s+SPDX-License-Identifier:\s+(?P<license>\S+)\s*\n(?P=prefix)\s+Copyright\s+(?P<holder>[^.]+)\.\s+All rights reserved\.
```

Named groups: `prefix`, `license`, `holder`. Template:

```go
{{ .Prefix }} SPDX-License-Identifier: {{ .License }}
{{ .Prefix }} Copyright {{ .Holder }}. All rights reserved.
```

Same regex captures, same template rendering — regardless of whether the prefix is `//` or `#`.

## Data flow

### Source → AST → provider result → Starlark rule

```text
┌─────────────────────────────────────────────────────────────┐
│  Source file                                                 │
│                                                              │
│  // Backup creates a timestamped copy of the resource.       │
│  //                                                          │
│  // +devlore:defaults overwrite=true                         │
│  //                                                          │
│  // Parameters:                                              │
│  //   - resource: The file to back up.                       │
│  //   - opts: Backup options (default: nil).                 │
│  //                                                          │
│  // Returns:                                                 │
│  //   - Resource: The backup copy.                           │
│  //   - Tombstone: Compensation state.                       │
│  //   - error: Non-nil if the backup failed.                 │
│  func (p *Provider) Backup(resource Resource,                │
│      opts *BackupOpts) (Resource, Tombstone, error) {        │
└────────────────────────────┬────────────────────────────────┘
                             │
                      go/parser.ParseFile
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│  ast.FuncDecl                                                │
│    .Name = "Backup"                                          │
│    .Doc  = *ast.CommentGroup{ ... }  ──── regex match ────►  │
│    .Recv, .Type.Params, .Type.Results    named groups fill   │
│                                          slot values         │
└────────────────────────────┬────────────────────────────────┘
                             │
                    goast provider builds
                    MethodResult with
                    .Comment populated
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│  MethodResult                                                │
│    .Name         = "Backup"                                  │
│    .ReceiverType = "*Provider"                               │
│    .Params       = [{resource, Resource}, {opts, *BackupOpts}]
│    .Returns      = "(Resource, Tombstone, error)"            │
│    .Comment      = FuncComment {                             │
│        .Summary    = "Backup creates a timestamped copy..."  │
│        .Directives = [{Key: "devlore:defaults",              │
│                        Value: "overwrite=true"}]             │
│        .Params     = [{Name: "resource",                     │
│                        Desc: "The file to back up."},        │
│                       {Name: "opts",                         │
│                        Desc: "Backup options (default: nil)."}]
│        .Returns    = [{Type: "Resource",                     │
│                        Desc: "The backup copy."},            │
│                       {Type: "Tombstone",                    │
│                        Desc: "Compensation state."},         │
│                       {Type: "error",                        │
│                        Desc: "Non-nil if backup failed."}]   │
│    }                                                         │
└────────────────────────────┬────────────────────────────────┘
                             │
                    Starlark accesses
                    typed slots directly
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│  doc-comments.star                                           │
│                                                              │
│  for m in goast.methods(path=path):                          │
│      c = m.comment                                           │
│      if not c.summary:                                       │
│          violation("missing summary")                        │
│      sig_params = [p.name for p in m.params]                 │
│      doc_params = [p.name for p in c.params]                 │
│      for s in sig_params:                                    │
│          if s not in doc_params:                              │
│              violation("param '" + s + "' not documented")   │
│                                                              │
│  # No _extract_doc_lines. No _has_section. No line walking. │
└─────────────────────────────────────────────────────────────┘
```

## Layers

```text
┌─────────────────────────────────────────────────────────────────┐
│  Starlark rules (.star)                                         │
│  m.comment.summary, m.comment.params, ...                       │
│  goast.format_comment(comment, width) → rendered string         │
├─────────────────────────────────────────────────────────────────┤
│  goast Provider (provider.go)                                   │
│  Methods(), Funcs(), Structs() → results with .Comment          │
│  FormatComment(), RewrapComments()                              │
├───────────────┬─────────────────┬───────────────────────────────┤
│  Regex parse  │  Go templates   │  go/ast                       │
│  named groups │  render output  │  .Doc fields on nodes          │
│  → slot data  │  + reflow funcs │  Read-only AST queries        │
└───────────────┴─────────────────┴───────────────────────────────┘
```

## Go types

### Comment types (one per node kind)

```go
// FuncComment holds the parsed doc comment for a FuncDecl (function or method).
// Populated by parsing ast.FuncDecl.Doc through the taxonomy.
type FuncComment struct {
    Summary    string          `starlark:"summary"`     // imperative verb phrase
    Body       string          `starlark:"body"`        // extended description
    Directives []Directive     `starlark:"directives"`  // +devlore:key=value entries
    Params     []ParamDoc      `starlark:"params"`      // documented parameters
    Returns    []ReturnDoc     `starlark:"returns"`     // documented return values
}

// TypeComment holds the parsed doc comment for a TypeSpec (struct, interface, etc.).
// Populated by parsing ast.TypeSpec.Doc through the taxonomy.
type TypeComment struct {
    Summary string `starlark:"summary"`  // type description
    Body    string `starlark:"body"`     // extended description
}

// FileComment holds the parsed file-level comments for an ast.File.
// Populated from ast.File.Comments (copyright) and ast.File.Doc (package doc).
type FileComment struct {
    Copyright  string `starlark:"copyright"`    // SPDX + Copyright lines
    PackageDoc string `starlark:"package_doc"`  // "Package foo provides..."
}

// FieldComment holds the parsed comment for a struct field.
// Populated from ast.Field.Comment (trailing inline comment).
type FieldComment struct {
    Inline string `starlark:"inline"`  // trailing comment on same line
}
```

### Shared sub-types

```go
type Directive struct {
    Key   string `starlark:"key"`     // e.g., "devlore:defaults"
    Value string `starlark:"value"`   // e.g., "key=value"
}

type ParamDoc struct {
    Name string `starlark:"name"`
    Desc string `starlark:"desc"`     // may include "(default: value)"
}

type ReturnDoc struct {
    Type string `starlark:"type"`
    Desc string `starlark:"desc"`
}
```

### Updated provider result types

Existing result types gain a `.Comment` field. The raw `.Doc` string is kept for backward compatibility.

```go
type MethodResult struct {
    Name         string        `starlark:"name"`
    ReceiverType string        `starlark:"receiver_type"`
    Returns      string        `starlark:"returns"`
    Params       []ParamDetail `starlark:"params"`
    File         string        `starlark:"file"`
    Line         int           `starlark:"line"`
    Doc          string        `starlark:"doc"`      // raw text (kept for compat)
    Comment      *FuncComment  `starlark:"comment"`  // parsed taxonomy
    Scope        string        `starlark:"scope"`
}

type FuncResult struct {
    Name    string        `starlark:"name"`
    Returns string        `starlark:"returns"`
    Params  []ParamDetail `starlark:"params"`
    File    string        `starlark:"file"`
    Line    int           `starlark:"line"`
    Doc     string        `starlark:"doc"`
    Comment *FuncComment  `starlark:"comment"`
    Scope   string        `starlark:"scope"`
}

type StructResult struct {
    Name    string        `starlark:"name"`
    File    string        `starlark:"file"`
    Line    int           `starlark:"line"`
    Fields  []FieldDetail `starlark:"fields"`
    Comment *TypeComment  `starlark:"comment"`
}

type FieldDetail struct {
    Name        string        `starlark:"name"`
    JSONName    string        `starlark:"json_name"`
    Type        string        `starlark:"type"`
    Required    bool          `starlark:"required"`
    Description string        `starlark:"description"`  // kept for compat
    Embedded    bool          `starlark:"embedded"`
    Comment     *FieldComment `starlark:"comment"`
}
```

### Schema types

```go
// CommentSchema defines the expected comment structure for a node type in a specific format.
type CommentSchema struct {
    NodeType string            // "File", "FuncDecl", "TypeSpec", "Field"
    Format   string            // "go", "star", "shell", etc.
    Pattern  *regexp.Regexp    // one regex with named capture groups
    Template *template.Template // Go template for rendering
    Slots    []SlotDef         // named group → slot type mapping
}

type SlotDef struct {
    Name        string          // matches a regex named group (level 1)
    Type        SlotType        // data model + default item pattern (level 2)
    Required    RequiredPolicy  // when this slot must be present
    ItemPattern string          // override level-2 item regex (optional; slot type has default)
}

type SlotType int

const (
    SlotParagraph  SlotType = iota  // plain text, reflowed via reflow template function
    SlotBlock                       // multiple blocks, each reflowed/preserved
    SlotDirective                   // sub-parsed into []Directive, rendered verbatim
    SlotParamList                   // sub-parsed into []ParamDoc
    SlotReturnList                  // sub-parsed into []ReturnDoc
    SlotVerbatim                    // passed through unchanged
)

type RequiredPolicy int

const (
    Always    RequiredPolicy = iota
    IfParams
    IfReturns
    Never
)
```

### Template functions

Custom functions available in all comment templates:

| Function | Signature                 | Behavior                                                          |
| -------- | ------------------------- | ----------------------------------------------------------------- |
| `reflow` | `reflow width text`       | Reflows text to fill to `width` columns using `go/doc/comment`    |
| `wrap`   | `wrap width prefix text`  | Wraps text with a specific continuation prefix                    |
| `join`   | `join sep items`          | Joins a slice with a separator                                    |

`go/doc/comment.Printer` is the engine behind `reflow` — it handles paragraphs, headings, code blocks, and lists
natively with configurable `TextWidth`. It's not exposed directly; it's a template function implementation detail.

## Scope

### What the taxonomy covers

- Doc comments attached to nodes: functions, methods, types, fields, files
- Parsing raw comment text into typed, named slots
- Deterministic rendering from slot contents
- Paragraph reflow via `go/doc/comment`

### What the taxonomy does not cover

- **Structural comments** — region markers (`// region EXPORTED METHODS`), delineators (`// Compensable actions`),
  and endregion markers are positional, not attached to any AST node. They are handled by the `regions` and
  `method-order` linter rules, not the comment taxonomy.
- **Code formatting** — indentation, brace placement, blank lines between statements. That's `go/format` and the
  `formatting` linter rule.
- **Declaration reordering** — reordering functions within a file is a separate concern (`SortDeclarations`). The
  taxonomy ensures comments are correctly structured; reordering ensures they're in the right position.

## Design decisions

1. **Mine the AST, don't duplicate it.** `go/ast` already attaches `.Doc` to every relevant node. The taxonomy
   parses that existing attachment into typed slots. No new comment-to-node correlation is needed.

2. **Regex named groups are slots.** One regex per schema, named capture groups map directly to slot names. The
   group name is the slot name — no separate mapping, no per-slot regex. The regex defines structure; the slot
   type defines semantics.

3. **Two-level parsing.** Level 1: section regex captures named groups (one per slot). Level 2: repeating slots
   use an item pattern to split captured text into structured items. Non-repeating slots have no level 2. The
   slot type defines the data model and a default item pattern; the schema can override it for format-specific
   syntax.

4. **Prefix backreference.** Multi-line patterns capture the comment prefix (`//`, `#`) in a named group and
   backreference it for subsequent lines. This guarantees prefix consistency within a comment block and follows
   the same pattern the copyright linter already uses.

5. **Graceful degradation on partial match.** When the section regex matches some groups but not others, the
   taxonomy populates whatever slots did match and leaves unmatched optional slots empty. The linter evaluates
   `Required` policy to distinguish "missing because optional" from "missing but required."

6. **Go templates for rendering.** `text/template` with custom functions (`reflow`, `wrap`) produces deterministic
   output. The template controls format-specific details (comment prefix, section headers, indentation). Slot data
   is format-agnostic.

7. **Format-agnostic slot types.** The same `FuncComment` struct — with `Summary`, `Params`, `Returns` — works for
   Go (`// `), Starlark (`# `), and any other comment format. Only the regex pattern and template change. This is
   the same pattern the copyright linter already uses across 30+ file extensions.

8. **Schema registration by format.** A registry mapping `(nodeType, format)` → `CommentSchema` lets the provider
   look up the right pattern + template at parse time. The goast provider registers Go schemas; a future Starlark
   linter registers Starlark schemas. Same taxonomy engine, different registered schemas.

9. **Stdlib for paragraph reflow.** `go/doc/comment.Printer` is the engine behind the `reflow` template function.
   It handles paragraphs, headings, code blocks, and lists natively with configurable `TextWidth`. Not exposed
   directly — it's a template function implementation detail.

10. **Uniformity over specifics.** The exact indentation of list items is flexible — what matters is that every
    list item uses the same format. The template is the single source of truth for output format.
