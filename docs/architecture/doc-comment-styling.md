---
title: "Doc Comment Styling: Block-Based Slot Filling"
description: "Architecture for styling doc comments using go/doc/comment blocks, schema-driven primitives, and fuzzy slot filling"
status: draft
created: 2026-03-21
updated: 2026-03-21
supersedes: star-goast-linter.md (comment taxonomy sections)
---

# Doc Comment Styling: Block-Based Slot Filling

## Background

The original comment taxonomy used participle to parse doc comments into typed elements, then
a normalize-then-format pipeline to reorder and render them. This design had fundamental problems:

1. **Duplicate parsing.** `go/doc/comment` already parses comments into structured blocks
   (Paragraph, Heading, Code, List). Participle re-parsed the same text with a separate grammar.
2. **Type mismatch.** The schema defined element types (`section`) that the normalize dispatcher
   didn't understand (`param_section`, `return_section`). Config-driven schemas were silently
   dropped.
3. **No compositional model.** The dispatcher had hardcoded knowledge of Parameters vs Returns.
   Adding a new section type required code changes, not just schema changes.
4. **Raw text storage.** `DocComment` stored a `string`. Every operation re-parsed it. No
   structured representation survived between parse and render.

## Go doc comment standards

The defaults for our schemas are derived from the [Google Go Style Guide](https://google.github.io/styleguide/go/best-practices.html)
and [Go Doc Comments](https://go.dev/doc/comment). Our project adds structured extensions
(Parameters/Returns sections, directives) via config.

### What the standard says per element

| Element | Go standard | Source |
|---|---|---|
| **Package summary** | Must start with `Package <name>`. Complete sentence. | [Go Doc Comments](https://go.dev/doc/comment): "the first sentence begins with 'Package '" |
| **Func/method summary** | Starts with the function name. Explains what it returns or does. Complete sentence. | [Go Doc Comments](https://go.dev/doc/comment): "A function's doc comment should explain what the function returns or, for functions called for side effects, what it does." |
| **Type summary** | Explains what each instance represents. Complete sentence starting with name. | [Go Doc Comments](https://go.dev/doc/comment): "A type's doc comment should explain what each instance of that type represents or provides." |
| **Var/const group summary** | Complete sentence starting with name. Introduces the group. | [Go Doc Comments](https://go.dev/doc/comment): "Go's declaration syntax allows grouping of declarations, in which case a single doc comment can introduce a group of related constants." |
| **Var/const entry summary** | Complete sentence starting with name. Documents one entry within a group. | [Go Doc Comments](https://go.dev/doc/comment): "individual constants only documented by short end-of-line comments" (but full doc comments are also valid per entry). |
| **Body** | Additional paragraphs, headings, code blocks, lists. No required structure. | Standard Go doc comment syntax. |
| **Parameters** | Documented inline in prose. **No structured section.** | [Google Style Guide](https://google.github.io/styleguide/go/best-practices.html): "Not every parameter must be enumerated in the documentation. Document the error-prone or non-obvious fields and parameters." |
| **Returns** | Documented inline in prose. `reports whether` for bools. **No structured section.** | [Go Doc Comments](https://go.dev/doc/comment): "Named parameters and results can be referred to directly in the comment, without any special syntax." |
| **Directives** | Not part of doc comments. `//go:` are compiler pragmas outside the doc. | Not in any standard. |
| **Deprecation** | `Deprecated:` paragraph. Recognized by godoc and pkg.go.dev. | [Go Doc Comments](https://go.dev/doc/comment): "A paragraph beginning with 'Deprecated:' is used to document that an identifier should no longer be used." |
| **Notes** | `TODO(user):`, `BUG(user):` — uppercase markers with uid. | [Go Doc Comments](https://go.dev/doc/comment) |

### Named stylings

Stylings are named. The name identifies a coherent set of schemas for all declaration types.

- **Google** — the default. Summary + body only for all declarations. Matches the
  [Google Go Style Guide](https://google.github.io/styleguide/go/best-practices.html)
  and standard Go doc comment conventions. No Parameters/Returns sections, no directives.
- **NobleFactor** — a refinement of Google. Adds structured `Parameters:` and `Returns:`
  sections for functions/methods, `+devlore:` directives, and stricter summary prefix
  enforcement. Defined in `star/config.yaml`.

A project selects a styling by name in config. The styling name resolves to a complete
set of schemas. Individual schemas can still be overridden per-element.

Projects that don't configure a styling get **Google** by default.

### Defaults vs project extensions

The **Google** default schema for all declaration types is: `summary` + `body`.
That's it. No Parameters, no Returns, no Directives.

`Parameters:`, `Returns:`, and `+devlore:` directives are **NobleFactor** extensions
layered on via config (`star/config.yaml`). Projects that don't configure these get
Google-standard Go doc comment formatting only.

### Missing schema policy

Every declaration that goes through the styler must have a schema. If the registry does not
have a schema for a given node type, this is a configuration error. The styler repairs from
`DefaultRegistry()` with a warning. It never silently skips styling.

## Design

### Principle: one styler, one schema lookup, one code path

There is no `CommentStyle` enum. There is no verbatim vs taxonomy distinction in code. The
styler checks: does a schema exist for this declaration's node type? If yes, style it using
the schema's elements. If no, repair from defaults with a warning, then style.

The schema IS the style. A func_doc schema with five elements (summary, body, parameters,
returns, directives) and a gen_decl schema with two elements (summary, body) are different
styles — distinguished entirely by data. The styler code is identical for both.

### Principle: `go/doc/comment` is the only parser and renderer

`go/doc/comment.Parser.Parse()` produces a `*go/doc/comment.Doc` containing a `Content` field
of type `[]go/doc/comment.Block`. The block types are:

- `*go/doc/comment.Paragraph` — prose text
- `*go/doc/comment.Heading` — `# Heading` line
- `*go/doc/comment.Code` — indented code block
- `*go/doc/comment.List` — bulleted list, containing `[]go/doc/comment.ListItem`

Each `go/doc/comment.ListItem` has its own `Content []go/doc/comment.Block`.

`go/doc/comment.Printer.Text()` renders a `*go/doc/comment.Doc` back to `// `-prefixed text
with line wrapping, list indentation, and code block preservation.

**No other parser or renderer exists.** The styling layer works entirely with
`*go/doc/comment.Doc` and its block tree.

### Storage: `DocComment` holds `*go/doc/comment.Doc`

```go
type DocComment struct {
    doc     *go/doc/comment.Doc
    present bool
    style   CommentStyle
}
```

- `LoadSourceFile` parses each comment group via `go/doc/comment.Parser.Parse()` and stores
  the resulting `*go/doc/comment.Doc`.
- `Cleanup()` reads and mutates the block tree in place.
- `Save()` renders via `go/doc/comment.Printer.Text()`.

One parse. One render. Blocks in between.

### Styling: cursor-based primitive execution

A **primitive** is a match function. It takes:

1. A cursor position in the `[]go/doc/comment.Block` stream
2. A schema element definition (name, type, header, item_tokens, required)
3. A declaration context (function name, param names, return types)

It returns:

1. Whether it matched (and optionally edited)
2. The new cursor position

The dispatcher iterates schema elements in order, passing the cursor through each primitive.
There is no switch on type names. No hardcoded knowledge of what Parameters or Returns look
like. The schema element carries everything the primitive needs.

```text
Schema elements (ordered):     Block stream (from go/doc/comment):
  1. summary (paragraph)   →     Paragraph: "Backup creates a timestamped copy..."
  2. body (block)          →     Paragraph: "The backup location is..."
  3. parameters (section)  →     Paragraph: "Parameters:"
                                 List: [path: ..., backupSuffix: ...]
  4. returns (section)     →     Paragraph: "Returns:"
                                 List: [result: ..., undo: ..., err: ...]
  5. directives (directive)→     Paragraph: "+devlore:defaults overwrite=true"
```

Each primitive advances the cursor past the blocks it consumes.

### Primitives

The primitives are the four `go/doc/comment.Block` types. Every schema element maps
to one of these four. There are no other block types.

1. **Paragraph** (`*go/doc/comment.Paragraph`) — prose text composed of `[]comment.Text`
   (Plain, Italic, Link, DocLink).
2. **Heading** (`*go/doc/comment.Heading`) — a `# Heading` line, also composed of
   `[]comment.Text`.
3. **Code** (`*go/doc/comment.Code`) — preformatted text. A single `Text string` field.
4. **List** (`*go/doc/comment.List`) — contains `Items []*comment.ListItem`. Each ListItem
   has `Content []comment.Block`. The type system allows any Block in a ListItem, but the
   [Go Doc Comments spec](https://go.dev/doc/comment) restricts it: "List items only contain
   paragraphs, not code blocks or nested lists." So in practice, `ListItem.Content` holds
   only `*comment.Paragraph` blocks.

### Production model

A style operates on a tree of blocks. The input is the `[]comment.Block` from a parsed
comment. The output is a new `[]comment.Block` conforming to the schema's element ordering.

Each schema element defines a **production** — a rule that says what block types it
consumes from the input stream, what prefix it expects, and what it emits. Productions
execute in schema order. Each consumes from the input and emits to the output.

There are two kinds of productions:

- **`itemProduction`** — consumes zero or more blocks of specified types. May consume
  one (summary), zero-or-one (section heading), or zero-or-more (description body).
- **`listProduction`** — consumes a List block and does slot filling on its items.

Unmatched input is never lost. Anything the productions don't claim gets emitted with
a TODO flag.

### Consumes notation (ABNF-like)

The `consumes` field uses ABNF-like notation to specify what block types a production
accepts and how many. Parsed into a `Consumes` struct at config load time.

**Block type names:** `Paragraph`, `Heading`, `Code`, `List`

**Cardinality:**
- `Paragraph` — exactly one
- `*Paragraph` — zero or more
- `1*Paragraph` — one or more
- `0*1Paragraph` — zero or one (optional)

**Alternatives:**
- `Paragraph / Heading` — one Paragraph or one Heading
- `*(Paragraph / Code)` — zero or more of either
- `*(Paragraph / Code / Heading)` — zero or more of any

**Sequences:**
- `0*1Paragraph List` — optional Paragraph then exactly one List

**Resulting struct:**

```go
type Consumes struct {
    Min   int      // minimum count (0 for optional, 1 for required)
    Max   int      // maximum count (-1 for unbounded)
    Types []string // allowed block types: "Paragraph", "Heading", "Code", "List"
}
```

### Conditions

The `condition` field gates a production on declaration context. Conditions are named
presence checks — no expression language.

| Condition | Meaning |
|---|---|
| `params` | Declaration has parameters (`len(params) > 0`) |
| `returns` | Declaration has return values (`len(returns) > 0`) |
| `exported` | Declaration name starts with uppercase |
| `receiver` | Declaration is a method (has a receiver) |

The styler has a map of condition evaluators that take the declaration context and
return bool. If the condition is false, the production is skipped entirely — no output,
no TODO stub.

Compound conditions (`params AND exported`) can be added later with `all`/`any`
combinators. Not needed now.

### Prefix matching

The `prefix` and `slot_prefix` fields define fuzzy match patterns for the first tokens
of a block or list item.

- `{name}` — substitute the declaration name, fuzzy match
- `{slot}` — substitute the slot name (param name, return name), fuzzy match
- `"Parameters:"` — literal string, exact match
- `"+"` — literal prefix, exact match

Variable references (`{name}`, `{slot}`) are always fuzzy-matched. A fuzzy match means:
prefix/substring match first, then edit distance as fallback. The separator after a
variable is any non-word token (colon, dash, em-dash, whitespace).

### Fully-worked example: NobleFactor func_doc

```yaml
elements:
  - name: summary
    production: item
    consumes: "Paragraph / Heading"
    prefix: "{name}"
    split: sentence
    required: true

  - name: description
    production: item
    consumes: "*(Paragraph / Code / Heading)"

  - name: parameters
    production: list
    condition: params
    heading: "Parameters:"
    consumes: "0*1Paragraph List"
    slots: params
    slot_prefix: "{slot}"
    required: if_condition

  - name: returns
    production: list
    condition: returns
    heading: "Returns:"
    consumes: "0*1Paragraph List"
    slots: returns
    slot_prefix: "{slot}"
    required: if_condition

  - name: directives
    production: item
    consumes: "*Paragraph"
    prefix: "+"
```

**Production behavior:**

1. **summary** — consumes the first Paragraph or Heading whose text fuzzy-matches
   `{name}`. Extracts the first sentence (`split: sentence`). Emits the sentence as
   the summary paragraph. Pushes the remainder to description. If no match and
   `required: true`, emits `Name TODO(go-style): add summary`.

2. **description** — consumes zero or more unclaimed Paragraphs, Code blocks, and
   Headings. No prefix match. Emits them in order.

3. **parameters** — activated only if `condition: params` is true. Looks for a
   Paragraph matching `"Parameters:"` followed by a List. Assigns list items to
   parameter name slots using fuzzy `slot_prefix: "{slot}"`. Unmatched slots get
   TODO stubs. Unmatched items are emitted with a TODO flag. If no match and
   `required: if_condition`, emits the heading and a stub list.

4. **returns** — same pattern as parameters, with return name slots.

5. **directives** — consumes Paragraphs starting with `+`. No slot filling. Emits
   them in order.

### Slot filling for list productions

A list production's items fill **documentation slots**. The slot names come from the
declaration context (parameter names, return type names). The matcher assigns list
items to slots using fuzzy matching on `slot_prefix`.

**Matching strategy (ordered by priority):**

1. **Prefix/substring match.** The list item text starts with a token that contains
   or closely resembles a slot name. `path:` matches slot `path`.

2. **Edit distance.** If no prefix/substring match, compute edit distance between
   the first token of the list item and each unmatched slot name. Accept if distance
   is within a threshold proportional to name length.

3. **Unmatched slots** get TODO stubs:
   `backupSuffix: TODO(go-style): add description for parameter`

4. **Unmatched items** are emitted with a TODO flag:
   `err: TODO(go-style): 'err' is not a parameter of Backup; remove or move to Returns`

This is an **assignment problem**, not a string comparison. The matcher produces the
best assignment of M list items to N slots, maximizing match quality.

### Fuzzy matcher

The fuzzy matcher recognizes misspellings, synonyms, punctuation variations, and
demarcated names. It is used for both prefix matching (headings, summaries) and
slot filling (list items).

**Normalization pipeline (applied before comparison):**

1. **Strip demarcation** — remove backticks, square brackets, quotes.
   `` `path` `` → `path`, `[path]` → `path`
2. **Normalize case** — case-insensitive comparison.
   `PARAMETERS` → `parameters`
3. **Strip trailing punctuation** — remove colons, dashes, em-dashes.
   `path:` → `path`, `Returns —` → `Returns`
4. **Synonym resolution** — map known alternatives to canonical forms.

**Synonym table (hardcoded initially, configurable later):**

| Canonical | Synonyms |
|---|---|
| `Parameters` | `Params`, `Arguments`, `Args`, `Inputs` |
| `Returns` | `Return values`, `Return`, `Result`, `Results`, `Output`, `Outputs` |
| `Deprecated` | `Deprecation` |

The synonym table is per-styling. "Google" and "NobleFactor" share the same defaults.
Projects can extend the table in config in a future version.

**Match scoring (after normalization):**

1. **Exact match** — normalized candidate equals normalized target. Score: 1.0
2. **Prefix match** — target is a prefix of candidate. Score: 0.9
3. **Substring match** — target appears within candidate. Score: 0.7
4. **Edit distance** — Levenshtein distance within threshold. Score: `1.0 - (distance / max(len(target), len(candidate)))`. Accept if score > 0.5.
5. **No match** — score: 0.0

For heading matching, a single score above 0.5 is sufficient.

### Slot assignment algorithm

For list productions, list items must be assigned to named slots (parameter names,
return type names). The algorithm is greedy with forced assignment for eliminations.

**Inputs:**
- `slots []string` — the expected names from the function signature
- `items []string` — the first tokens of each list item (after normalization)

**Algorithm:**

1. **Build score matrix.** For each (item, slot) pair, compute `fuzzyScore(normalize(item), normalize(slot))`.
2. **Greedy assignment.** Pick the highest-scoring pair in the matrix. Assign it. Remove both from contention. Repeat until no pair scores above 0.3.
3. **Forced assignment by elimination.** If exactly one unmatched slot and one unmatched item remain, assign them regardless of score. This catches renames where the new name bears no resemblance to the old — the combinatorics force the assignment. The match is flagged as `Forced: true`.
4. **Collect unmatched.** Remaining slots get TODO stubs. Remaining items are emitted with a TODO flag.

**Properties:**
- Greedy, not optimal. O(N*M) per round, at most min(N,M) rounds. Fast enough for doc comments (N and M are typically < 10).
- Threshold 0.3 prevents garbage matches when multiple slots and items are free.
- Forced assignment detects renames: `[source, destination]` documenting `[src, dst]` where `source` matches `src` by substring, leaving `destination` → `dst` as the only remaining pair.
- Forced matches produce a TODO note: `dst: TODO(go-style): was documented as 'destination'; verify after rename`.

```go
type Match struct {
    Slot   string
    Item   string
    Score  float64
    Forced bool // assigned by elimination, not by score
}
```

### Config-driven schemas

Ten node types, each with a schema. The schema's elements define the style.

| # | Schema name | Node type | AST node | Google default | NobleFactor adds |
|---|---|---|---|---|---|
| 1 | `package_doc` | `Package` | `*ast.File.Doc` | summary (`Package <name>`) + body | — |
| 2 | `func_doc` | `FuncDecl` | `*ast.FuncDecl.Doc` | summary (`<name>`) + body | parameters, returns, directives |
| 3 | `gen_decl_type` | `GenDeclType` | `*ast.GenDecl.Doc` (token.TYPE) | summary (`<name>`) + body | — |
| 4 | `gen_decl_var` | `GenDeclVar` | `*ast.GenDecl.Doc` (token.VAR) | summary (`<name>`) + body | — |
| 5 | `gen_decl_const` | `GenDeclConst` | `*ast.GenDecl.Doc` (token.CONST) | summary (`<name>`) + body | — |
| 6 | `gen_decl_import` | `GenDeclImport` | `*ast.GenDecl.Doc` (token.IMPORT) | summary + body | — |
| 7 | `value_spec` | `ValueSpec` | `*ast.ValueSpec.Doc` | summary (`<name>`) + body | — |
| 8 | `type_spec` | `TypeSpec` | `*ast.TypeSpec.Doc` | summary (`<name>`) + body | — |
| 9 | `field` | `Field` | `*ast.Field.Doc` | summary (`<name>`) + body | — |
| 10 | `import_spec` | `ImportSpec` | `*ast.ImportSpec.Doc` | summary + body | — |
| 11 | `body_comment` | `BodyComment` | `*ast.CommentGroup` inside decl body | summary | — |
| 12 | `copyright` | `Copyright` | floating comment | spdx + copyright | — |
| 13 | `floating` | `Floating` | floating comment | summary + body (prose) | — |

Schemas 1–8 bind to AST nodes. Schemas 9–10 bind to floating comments classified
by content.

**Example: NobleFactor func_doc schema**

```yaml
comment_schemas:
  func_doc:
    format: go
    node_type: FuncDecl
    summary_prefix: "{name}\\b"
    elements:
      - name: summary
        type: paragraph
        required: "true"
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2
      - name: parameters
        type: section
        header: "Parameters:"
        item_tokens: ParamName
        order: 3
      - name: returns
        type: section
        header: "Returns:"
        item_tokens: ReturnType
        order: 4
      - name: directives
        type: directive
        cardinality: "*"
        order: 5
```

**Example: Google gen_decl_type schema (default)**

```yaml
  gen_decl_type:
    format: go
    node_type: GenDeclType
    summary_prefix: "{name}\\b"
    elements:
      - name: summary
        type: paragraph
        required: "true"
        order: 1
      - name: body
        type: block
        cardinality: "*"
        order: 2
```

The `type` field selects the primitive. The primitive reads `header`, `item_tokens`,
`required`, and `summary_prefix` from the schema element. No code change needed to add
new section types — just add a schema element with `type: section` and appropriate
`header` and `item_tokens`.

## Implementation plan

### Step 1: Stabilize on `*go/doc/comment.Doc` storage

Change `DocComment` to store `*go/doc/comment.Doc` instead of `string`. Update
`LoadSourceFile` to parse via `go/doc/comment.Parser.Parse()`. Update `Save()` to
render via `go/doc/comment.Printer.Text()`. Cleanup stylers work with the block tree
temporarily using the existing normalize logic (still broken for sections, but summary
and body work). Tests pass, output is valid Go.

**Key changes:**
- `DocComment.doc *go/doc/comment.Doc` replaces `DocComment.text string`
- `commentGroupRaw` replaced by `go/doc/comment.Parser.Parse()`
- `renderComment(text, width)` replaced by `go/doc/comment.Printer.Text(doc)`
- `CommentDecl.text` replaced by `CommentDecl.doc *go/doc/comment.Doc`
- Stylers receive and return `*go/doc/comment.Doc`

### Step 2: Slot-filling styler redesign

Replace the normalize dispatch with cursor-based primitive execution. Implement the
four primitives (paragraph, block, section, directive). Implement fuzzy slot filling
for section list items. Remove participle dependency.

**Design questions to resolve:**
- How do we represent slots? A slice of `Slot{Name, Filled bool, Block}` per section?
- How do we map slots to blocks? Assignment algorithm — greedy with backtracking, or
  Hungarian algorithm for optimal assignment?
- How do we update the doc before printing? Mutate the `[]go/doc/comment.Block` slice
  in place — insert, replace, reorder blocks. Or rebuild the slice from scratch based
  on matched slots?
- What threshold for edit distance? Fixed threshold or proportional to name length?

## What gets removed

- `doctaxonomy/normalize.go` — all Normalize methods replaced by primitive execution
- `doctaxonomy/grammar.go` — participle grammar structs
- `doctaxonomy/elements.go` — participle element types
- `doctaxonomy/lexer.go` — participle context-aware lexer
- `astrewrite.go` — `splitSummary` moves to the paragraph primitive
- `parseFuncDocSafe` in helpers.go — no longer needed
- `go get github.com/alecthomas/participle/v2` — dependency removed

## What stays

- `doctaxonomy/schema.go` — schema types, registry, loading (unchanged)
- `SourceFile` tree structure — unchanged (only `DocComment` representation changes)
- Config system — unchanged
- `go/doc/comment` — promoted from renderer to sole parser and renderer

## What changes

- `CommentStyle` enum — removed. Schema presence determines styling behavior.
- `classifyFloatingComment` — simplified. Only needs to determine node type for
  schema lookup, not classify into nine styles.
- `Cleanup` dispatch — replaced by single `styleDoc` call per declaration.
  Schema lookup by node type; missing schema repaired from defaults with warning.
