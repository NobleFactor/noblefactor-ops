---
title: "Comment Taxonomy Implementation"
status: in-progress
created: 2026-03-17
updated: 2026-03-20
tracking: TBD
---

# Plan: Comment Taxonomy Implementation

## Summary

Implement the comment taxonomy defined in [docs/architecture/star-goast-linter.md](../architecture/star-goast-linter.md).
A [participle](https://github.com/alecthomas/participle)-based parser with a context-aware lexer parses doc comments
into typed elements (Paragraph, Directive, ParamSection, ReturnSection, CodeBlock, Heading). Elements can appear in
any order but print in schema-defined order. Each element type has its own Printer. Schemas are YAML, loaded at runtime.

The canonical `Backup` method example in the architecture doc is the reference for implementation and test development.

## Goals

1. **Participle grammar** — element types with struct tags, context-aware lexer that injects parameter names and
   return types as first-class tokens, unordered `@@*` collection
2. **Normalize-then-format pipeline** — each element type owns `Normalize()` producing a single unwrapped line;
   elements are concatenated with correct blank-line separators; the assembled text is passed through
   `go/doc/comment.Printer` in one final pass for line wrapping, indentation, and `//` prefix
3. **YAML schemas** — element types, cardinality, required policy, and print order defined declaratively
4. **Provider integration** — `.Comment` field on `MethodResult`, `FuncResult`, `StructResult`, `FieldDetail`
5. **Linter rule migration** — `doc-comments.star` and `line-width.star` rewritten to use typed elements
6. **Fix mode that works** — renders from parsed elements, producing correct compilable output

## Current State

| Component         | Status  | Notes                                                          |
| ----------------- | ------- | -------------------------------------------------------------- |
| goast provider    | Working | 17 methods, codegen verified                                   |
| Linter extension  | Working | 7 rules, check mode finds 1,523 violations                     |
| Fix mode          | Broken  | Merges copyright lines, inserts spurious stubs, breaks builds  |
| Comment parsing   | Ad-hoc  | `parseParamDocs` in Go, `_extract_doc_lines` in Starlark       |
| Comment rendering | Ad-hoc  | `rewrapCommentGroup` + 4 helpers in Go, naive text in Starlark |

## Implementation Phases

### Phase 1: Element types and participle grammar (complete)

Add participle as a dependency. Define the element types, grammar structs, context-aware lexer factory, and
per-element Printers. The canonical `Backup` method from the architecture doc is the primary test case.

No changes to existing provider methods — this phase is purely additive.

- [ ] `go get github.com/alecthomas/participle/v2`
- [ ] Create `internal/provider/goast/doctaxonomy/lexer.go`: - `NewDocLexer(paramNames, returnTypes []string) *lexer.StatefulDefinition` — dynamically built lexer
      with `ParamName` and `ReturnType` rules injected before `Word` - Token types: `BlankLine`, `DirectiveMark`, `SectionHeader`, `ListMarker`, `CodeLine`, `ParamName`,
      `ReturnType`, `Colon`, `Word`, `Newline`, whitespace (elided) - Directive sub-state: `DirectiveKey`, `DirectiveValue` with Push/Pop
- [ ] Create `internal/provider/goast/doctaxonomy/elements.go`: - `Paragraph` — `@(Word | ParamName | ReturnType | Colon)+` - `Directive` — `DirectiveMark @DirectiveKey @DirectiveValue` - `ParamItem` — `ListMarker @ParamName Colon @@` (Desc is Paragraph) - `ParamSection` — `"Parameters" Colon @@*` - `ReturnItem` — `ListMarker @(ReturnType | Word) Colon @@` - `ReturnSection` — `"Returns" Colon @@*` - `CodeBlock` — `@CodeLine+` - `Heading` — `"#" @(Word | ParamName | ReturnType)+`
- [ ] Create `internal/provider/goast/doctaxonomy/grammar.go`: - `DocElement` wrapper struct — one pointer field per element type, joined by `|` - `FuncDoc` — `Elements []*DocElement` collected via `@@*` - `TypeDoc` — `Elements []*TypeDocElement` (Paragraph, CodeBlock, Heading only) - `CopyrightDoc` — fixed-order SPDX + Copyright fields - `NewParser(paramNames, returnTypes []string) *participle.Parser[FuncDoc]` — constructs lexer and parser
- [ ] Create `internal/provider/goast/doctaxonomy/normalize.go`:
      - `Paragraph.Normalize() string` — join words into single unwrapped line
      - `Directive.Normalize() string` — `+key value` (single line, never wrapped)
      - `ParamSection.Normalize() string` — `Parameters:` header + one `  - name: desc` line per item
      - `ReturnSection.Normalize() string` — `Returns:` header + one `  - type: desc` line per item
      - `CodeBlock.Normalize() string` — indented lines (4+ spaces), verbatim
      - `Heading.Normalize() string` — `# Title` (single line)
      - `FuncDoc.Normalize() string` — sort elements by schema order, concatenate with correct blank-line
        separators (one empty line between sections, items on consecutive lines)
      - `TypeDoc.Normalize() string` — summary + body
      - `CopyrightDoc.Normalize() string` — SPDX + Copyright (two lines)
- [ ] Create `internal/provider/goast/doctaxonomy/format.go`:
      - `Format(normalized string, prefix string, width int) string` — single-pass formatting via
        `go/doc/comment.Parser.Parse(normalized)` → `go/doc/comment.Printer.Comment(doc)` with
        `TextWidth = width - len(prefix)`. The Printer handles line wrapping, list indentation,
        code block pass-through, and `//` prefix on every line.
      - Directive lines (`+devlore:...`) pass through as short single-line paragraphs — reflow is a no-op
- [ ] Create `internal/provider/goast/doctaxonomy/doctaxonomy_test.go`:
      - **Canonical test: Backup method** — parse the exact comment from the architecture doc, verify
        parse tree matches Step 6, verify Normalize → Format pipeline output matches Step 8
      - Lexer token test: `resource` → `ParamName`, `foo` → `Word`
      - Stale param test: `- path:` does not match `ParamItem` (Word, not ParamName)
      - Missing section test: comment with summary only → `ParamSection` is nil
      - Unordered input test: directives before summary → parse succeeds, Normalize outputs canonical order
      - Copyright test: round-trip parse → normalize → format
      - Type doc test: summary-only and summary + body
      - Normalize test: each element produces exactly one unwrapped line
      - Format test: normalized text through `go/doc/comment` produces correct wrapping, indentation, and
        `//` prefix; under-filled paragraph fills to width; code blocks preserved
      - Directive pass-through test: `+devlore:defaults overwrite=true` survives format unchanged
      - Multi-paragraph body test: heading + paragraph + code block in body
      - Fix mode test: missing summary → stub `FuncName <TODO description>` inserted; missing Returns →
        section added with `<TODO description>` per return value
- [ ] `make build` and `make test` pass

**Files:**

| File                                                      | Action | Purpose                                   |
| --------------------------------------------------------- | ------ | ----------------------------------------- |
| `go.mod`                                                  | Modify | Add participle/v2 dependency              |
| `internal/provider/goast/doctaxonomy/lexer.go`            | Create | Context-aware lexer factory               |
| `internal/provider/goast/doctaxonomy/elements.go`         | Create | Element types with participle struct tags |
| `internal/provider/goast/doctaxonomy/grammar.go`          | Create | Grammar structs, parser factory           |
| `internal/provider/goast/doctaxonomy/normalize.go`        | Create | Per-element Normalize methods             |
| `internal/provider/goast/doctaxonomy/format.go`           | Create | Single-pass go/doc/comment formatting     |
| `internal/provider/goast/doctaxonomy/doctaxonomy_test.go` | Create | Tests driven by canonical Backup example  |

### Phase 2: Schema YAML and registry (complete)

Define the schema YAML format. Implement the schema loader and registry. Register Go schemas for FuncDecl,
TypeSpec, and File. Validation of `required` and `cardinality` constraints.

- [ ] Create `internal/provider/goast/doctaxonomy/schema.go`: - `SchemaElement` struct — `Name`, `Type`, `Required`, `Cardinality`, `Order`, `Header`, `ItemTokens` - `CommentSchema` struct — `Name`, `Format`, `NodeType`, `Elements []SchemaElement` - `SchemaRegistry` — `Register(schema)`, `Lookup(nodeType, format)` - `LoadSchemas(yamlPath string) ([]CommentSchema, error)` — deserialize YAML into schema structs
- [ ] Create `internal/provider/goast/doctaxonomy/schemas/go.yaml`: - `func_doc` schema (summary, body, directives, parameters, returns) - `type_doc` schema (summary, body) - `copyright` schema (spdx, copyright)
- [ ] Wire schema into `FuncDoc.Normalize` — element order driven by `SchemaElement.Order` field,
      not hardcoded method logic
- [ ] Implement validation: `Validate(doc *FuncDoc, schema *CommentSchema, params, returns []string) []Diagnostic` - Required element missing → diagnostic - Cardinality violated (e.g., 2 ParamSections) → diagnostic - ParamSection items vs. actual params → sync diagnostics - ReturnSection items vs. actual returns → sync diagnostics
- [ ] Create `internal/provider/goast/doctaxonomy/schema_test.go`: - YAML round-trip: load → marshal → load produces identical schemas - Validation: missing required summary → diagnostic - Validation: undocumented parameter → diagnostic - Validation: stale documented parameter → diagnostic
- [ ] `make build` and `make test` pass

**Files:**

| File                                                  | Action | Purpose                                    |
| ----------------------------------------------------- | ------ | ------------------------------------------ |
| `internal/provider/goast/doctaxonomy/schema.go`       | Create | Schema types, registry, loader, validation |
| `internal/provider/goast/doctaxonomy/schemas/go.yaml` | Create | Go FuncDecl, TypeSpec, File schemas        |
| `internal/provider/goast/doctaxonomy/schema_test.go`  | Create | Schema loading and validation tests        |

### Phase 3: Provider integration (complete)

Wire the taxonomy into the goast provider. Every result type gains a `.Comment` field populated during
extraction. Replace `parseParamDocs` and `RewrapComments` internals. Run codegen.

- [ ] Add `FuncDoc`, `TypeDoc`, `CopyrightDoc` references to `types.go` (re-export from doctaxonomy
      package or add `starlark:"..."` accessor types)
- [ ] Add `.Comment` field to `MethodResult`, `FuncResult`, `StructResult`, `FieldDetail`
- [ ] Update `Methods()`: - Extract `paramNames` and `returnTypes` from `ast.FuncDecl.Type` - Call `doctaxonomy.NewParser(paramNames, returnTypes)` - Parse `fn.Doc` text → `FuncDoc` - Populate `result.Comment`
- [ ] Update `Funcs()`: same pattern
- [ ] Update `Structs()`: - Parse `typeSpec.Doc` → `TypeDoc`, populate `result.Comment` - Parse `field.Comment` → inline text, populate `field.Comment`
- [ ] Replace `parseParamDocs` callers — `ParamDetail.Doc` now populated from `FuncDoc.ParamSection.Items`
- [ ] Remove `parseParamDocs` from `helpers.go`
- [ ] Rewrite `RewrapComments`: - For each comment group attached to a FuncDecl/TypeSpec: parse → print - For other comment groups: use `go/doc/comment.Printer` directly for reflow
- [ ] Add `FormatComment(nodeType, comment, width)` provider method for Starlark write path
- [ ] Remove dead helpers from `helpers.go`: - `rewrapCommentGroup`, `rewrapTextParagraph`, `rewrapBulletItem`, `fillWords` - `stringSlicesEqual`, `commentBodyPrefix`
- [ ] Run codegen (`FormatComment` needs `params.gen.go` entry)
- [ ] `make build` and `make test` pass
- [ ] `make check-codegen` passes

**Files:**

| File                                               | Action     | Purpose                                                     |
| -------------------------------------------------- | ---------- | ----------------------------------------------------------- |
| `internal/provider/goast/types.go`                 | Modify     | Add `.Comment` fields, accessor types                       |
| `internal/provider/goast/provider.go`              | Modify     | Wire taxonomy into Methods/Funcs/Structs, add FormatComment |
| `internal/provider/goast/helpers.go`               | Modify     | Remove 7 dead helpers (~230 lines)                          |
| `internal/provider/goast/gen/params.gen.go`        | Regenerate | FormatComment entry                                         |
| `internal/provider/goast/gen/receiver.gen.go`      | Regenerate | Updated method count                                        |
| `internal/provider/goast/gen/receiver_gen_test.go` | Regenerate | Updated test count                                          |

### Phase 4: Linter rule migration (complete)

Rewrite `doc-comments.star` and `line-width.star` to use `.comment` elements. Fix mode calls
`goast.format_comment`. Other rules (`formatting`, `receivers`, `file-layout`, `regions`, `method-order`)
are unaffected — they don't deal with doc comment content.

- [ ] Rewrite `doc-comments.star` check: - `m.comment` existence → summary present - `m.comment.param_section.items` vs `m.params` → sync check - `m.comment.return_section.items` vs `m.returns` → sync check - Remove `_extract_doc_lines`, `_has_section`, `_extract_param_names`, `_check_fill_width`
- [ ] Rewrite `doc-comments.star` fix: - Populate missing elements (summary TODO, param stubs, return stubs) - Call `goast.format_comment(...)` to render - Write rendered comment to file at correct position
- [ ] Rewrite `line-width.star` check: - Under-filled detection: compare current comment against `format_comment` output - Over-long code lines remain check-only
- [ ] Rewrite `line-width.star` fix: - Call `goast.rewrap_comments(path, width)` — now taxonomy-backed
- [ ] Run fix mode on goast provider files — verify correct, compilable output
- [ ] `make build` and `make test` pass

**Files:**

| File                                                                       | Action  | Purpose                     |
| -------------------------------------------------------------------------- | ------- | --------------------------- |
| `star/extensions/com.noblefactor.star.LintGoStyle/rules/doc-comments.star` | Rewrite | Element-based check and fix |
| `star/extensions/com.noblefactor.star.LintGoStyle/rules/line-width.star`   | Rewrite | Taxonomy-backed reflow      |

### Phase 5: Fix the codebase and CI (in-progress)

With correct fix mode, run across noblefactor-ops. Wire into CI and pre-commit.

#### Phase 5a: AST rewrite of RewrapComments (complete)

Build a parallel ordered tree from the Go AST. Walk the tree to print: comments are
reformatted, code is copied from original source. Schema-driven formatting enforces
doc comment structure: summary splitting, element ordering, TODO stub insertion for
missing required sections.

- [x] `buildFileTree` splits declarations into doc comment + code positioned items
- [x] Body comments use source text to preserve indentation
- [x] DoubleColon token, compound Word pattern, `\b` word boundaries in lexer
- [x] Directive order changed to last (after returns)
- [x] `rewriteFileFromSource` wired into `RewrapComments`
- [x] Schema loaded from embedded `go.yaml` via `DefaultRegistry`
- [x] `splitSummary` separates summary from body at first sentence boundary
- [x] `NormalizeWithContext` inserts TODO stubs for missing required elements
- [x] `type_doc` renamed to `gen_decl` for uniform GenDecl handling (type, var, const)
- [x] `TypeDoc.NormalizeWithSchema` for schema-driven GenDecl formatting
- [x] `doc-comments.star` fix reduced to rewrap-only (no line-based TODO insertion)

#### Phase 5b: Config-driven schemas (complete)

Move doc comment schemas from embedded `go.yaml` into the extension config system
so they are user-configurable.

- [x] Add `comment_schemas` field to extension config with nested `CommentSchema` and
      `SchemaElement` type definitions. Defaults match current `go.yaml`.
- [x] Add `NestedTypeDef` to `ConfigDef` and wire through `ToConfigSpec` so nested
      type definitions in extension.yaml flow to the config type generator.
- [x] Fix sibling nested type resolution: `generateNestedType` now receives the full
      parent nested map so types like `CommentSchema` can reference `SchemaElement`.
- [x] Add `Config.Navigate()` forwarding method so provider can access config hierarchy.
- [x] Add `Config.MergeYAML()` for loading config overrides from bytes.
- [x] Provider reads schemas from config context internally via `schemaRegistry()` →
      `configSchemas()`. No method signature change or codegen regen needed.
- [x] `DefaultRegistry()` built programmatically (no embedded YAML dependency).
      Embedded `go.yaml` can be deleted.
- [x] Tests: defaults match `DefaultRegistry()`, project config overrides work.
- [x] `make build` and `make test` pass

#### Phase 5c: SourceFile semantic tree and in-memory fix pipeline (in-progress)

The old rule-based fix pipeline is removed. It was broken by design: multiple
fixers reading from disk, writing between passes, corrupting source. The
replacement is a semantic tree (`SourceFile`) that the orchestrator queries
and styles in memory. Read once, query, style, write once.

**Programming model:** `LoadSourceFile` parses Go source into a semantic tree
organized by declaration kind (types, functions, variables, constants). Each
declaration carries a reference to its AST node (for navigation), a mutable
`DocComment` (for styling), and its verbatim code text (extracted once at
construction time). Code is never structurally modified. Styling config
(spacing rules, comment schemas, line width) is injected by the provider
at construction time from the extension config.

Three operations on the tree:

- `reformat()` — applies comment styling (taxonomy pipeline) and spacing
  rules to the tree in memory. Pure mutation, no I/O.
- `save()` — serializes the tree to disk. Walks `allDecls`, emits each
  node's code and restyled comment, inserts blank lines per spacing rules.
- `check_style()` — reports violations (missing comments, missing sections,
  under-filled lines). No mutation, no I/O.

**Starlark orchestrator:**

```python
def run(ctx):
    ast = goast.load_source_file(path)
    if ctx.args.get("fix", "false") == "true":
        ast.cleanup()
        ast.save()
        return
    ast.check_compliance()
```

**Query API (for inspection and custom checks):**

```python
ast = goast.load_source_file(path)

for decl in ast.decls:
    decl.name, decl.kind, decl.comment

for t in ast.types:
    t.name, t.comment
    for m in t.methods:
        m.name, m.comment, m.returns
        for p in m.params:
            p.name, p.type

for f in ast.funcs:
    f.name, f.comment, f.returns

for c in ast.consts:
    c.name, c.comment
    for entry in c.entries:
        entry.name, entry.value

for v in ast.vars:
    v.name, v.comment

provider = ast.types["Provider"]
backup = provider.methods["Backup"]
```

**Object model:**

```
SourceFile
  allDecls      []Decl              # source order
  typeDecls     []*TypeDecl         # indexed by name
  funcDecls     []*FuncDecl         # indexed by name
  varDecls      []*VarDecl
  constDecls    []*ConstDecl
  spacing       SpacingRules        # injected by provider
  registry      *SchemaRegistry     # injected by provider
  lineWidth     int                 # injected by provider
  filename      string              # for save()

  Query:    Decls, Types, GetType, Funcs, GetFunc, Vars, Consts, PackageName
  Actions:  Cleanup, Save, CheckCompliance

TypeDecl (adapts *ast.TypeSpec)
  comment       DocComment
  code          string              # verbatim, extracted at build time
  methods       []*FuncDecl         # indexed by name
  Query:    Name, Comment, Methods, GetMethod

FuncDecl (adapts *ast.FuncDecl)
  comment       DocComment
  code          string              # verbatim, extracted at build time
  Query:    Name, Comment, ReceiverType, Params, Returns

VarDecl (adapts *ast.ValueSpec)
  comment       DocComment
  code          string
  Query:    Name, Comment

ConstDecl (adapts *ast.GenDecl)
  comment       DocComment
  code          string
  entries       []*ConstEntry
  Query:    Name, Comment, Entries

ConstEntry (adapts *ast.ValueSpec)
  Query:    Name, Value

CommentDecl (floating comment)
  text          string
  Query:    Text

DocComment
  text          string
  present       bool
  Query:    Text → string or nil

SpacingRules
  AfterPackage, AfterImports, BetweenFunctions, BetweenMethods,
  BeforeTypeMethods, AroundRegionMarkers, AroundDelineators
```

**Step 1: FormatComment provider method (complete → removed)**

- [x] ~~`FormatComment` and `RewrapComments` provider methods~~ — removed.
      Replaced by the SourceFile pipeline (`LoadSourceFile` → `Cleanup` →
      `Save`). `doctaxonomy.Format()`, `formatComment`, `rewriteComment`,
      `rewriteFileFromSource`, `buildFileTree`, `printTree` all removed.
      `astrewrite.go` retains only config helpers (`schemasFromConfig`,
      `genDeclName`, `splitSummary`, `isDelineatorBlock`).
- [x] Codegen regenerated: 18 methods (was 20).

**Step 2: SourceFile semantic tree (complete)**

Build the type hierarchy and `LoadSourceFile` builder. Each declaration node
adapts a Go AST node for navigation and carries a mutable `DocComment`.

- [x] `SourceFile` — root: holds `decls` (source order), `types`, `funcs`,
      `vars`, `consts` (indexed by name).
- [x] `TypeDecl` — adapts `*ast.TypeSpec`. Holds `methods` (indexed by name).
- [x] `FuncDecl` — adapts `*ast.FuncDecl`. Used for both functions and methods.
      Exposes `params`, `returns`, `receiver_type`.
- [x] `VarDecl` — adapts `*ast.ValueSpec`.
- [x] `ConstDecl` — adapts `*ast.GenDecl`. Holds `entries` (individual constants).
- [x] `ConstEntry` — adapts `*ast.ValueSpec`. Exposes `name`, `value`.
- [x] `DocComment` — mutable comment text. The only writable part of the tree.
- [x] `Decl` interface — `DeclName()`, `DeclKind()`, `DeclComment()` for
      uniform iteration over `decls`.
- [x] `CommentDecl` — floating comments (region markers, delineators) positioned
      in source order among declarations.
- [x] `LoadSourceFile(content)` — parses source, builds tree, classifies
      comments (doc/body/floating), associates methods with types.
- [x] Tests covering iteration, lookup, floating comment positioning.
- [x] `make build` and `make test` pass.

**Step 3: Starlark integration (complete)**

Expose `LoadSourceFile` as a provider method. Register `SourceFile` and its
child types so Starlark can call methods on returned objects.

- [x] `LoadSourceFile` provider method on `Provider`.
- [x] Codegen auto-discovered dependent types: `SourceFile`, `TypeDecl`,
      `FuncDecl`, `DocComment`. Generated type registrations.
- [x] Starlark integration test: full iteration and lookup coverage.
- [x] `DocComment.Text()` returns `nil` (Starlark `None`) when no comment
      is present, matching Python API conventions.
- [x] `make build` and `make test` pass.

**Step 4: Comment classification and single pipeline (complete)**

Every comment is classified into one of seven styles at `LoadSourceFile`
time. Each style has exactly one handler — no fallbacks. The provider
validates at construction that all taxonomy styles have schemas. Missing
schemas are repaired from defaults with a warning.

Seven comment styles:

1. **Copyright** — SPDX + Copyright lines. Verbatim.
2. **Delineator** — `// =====...=====`. Verbatim.
3. **Region marker** — `// region X` / `// endregion`. Verbatim.
4. **Section header** — `// Fallible actions`. Verbatim.
5. **Prose** — multi-line floating comment. `go/doc/comment` fill and wrap.
6. **FuncDecl doc** — taxonomy pipeline, `func_doc` schema.
7. **GenDecl doc** — taxonomy pipeline, `gen_decl` schema.

- [x] `CommentStyle` enum with seven values. `style` field on `DocComment`
      and `CommentDecl`. `DeclStyle()` on the `Decl` interface — queryable
      on every declaration.
- [x] `classifyFloatingComment` — classifies by content at tree construction.
      Doc comment styles are hard-wired from tree position.
- [x] ~~`NewProvider` validates taxonomy schemas~~ — removed. Validation
      ran before config was available, producing noise. Schema lookup at
      `LoadSourceFile` time handles missing schemas gracefully.
- [x] Removed `go/doc/comment` fallback in `rewriteComment`. Each of the
      seven styles has an explicit handler. Prose uses `go/doc/comment`
      intentionally — not as a fallback.
- [x] Apostrophe bug fixed: contractions no longer split because prose
      comments go through `go/doc/comment` directly (which handles them
      correctly), not through the taxonomy lexer.
- [x] `make build` and `make test` pass.

**Step 5: SourceFile operations — Cleanup, Save, CheckCompliance**

Implement the three operations on SourceFile. The text pipeline:

1. **Read** — `LoadSourceFile` reads the file, builds the tree, stores
   raw comment text without `// ` prefix.
2. **Recompose** — `Cleanup()` walks the tree, applies taxonomy (reorder
   elements, add stubs, split summary). Produces plain text per element —
   no line breaks, no indentation. Just logical structure with blank line
   separators and list markers.
3. **Render** — `Save()` runs composed text through `go/doc/comment.Printer`
   for word wrapping and indent normalization, then adds `// ` prefix to
   every line, then writes to disk.

`go/doc/comment` is the renderer, not the formatter. It touches text once,
at the end, on the way out. The taxonomy owns structure. The Printer owns
presentation. `Save()` owns the `// ` prefix.

All comment text is stored WITHOUT `// ` prefix throughout the tree.
`doctaxonomy.Format()` no longer adds prefixes. `Save()` is the single
place that adds `// ` to every comment line.

- [x] Add `code` field to each declaration type. `LoadSourceFile` extracts
      verbatim code text at build time.
- [x] `filename` on SourceFile. Config read from context (not stored on
      SourceFile).
- [x] `Cleanup()` — walks declarations, restyles comments through the
      taxonomy pipeline, updates `DocComment.text` in place. Pure mutation.
- [x] `Save()` / `SaveAs(path)` — serializes the tree to disk.
- [x] `CheckCompliance()` — reports violations. No mutation, no I/O.
- [x] `SpacingRules` config type with named settings (JetBrains IDEA model).
- [x] Add `spacing_rules` field to extension config with defaults (all 1).
- [x] Provider loads config into context. SourceFile reads from context.
- [x] Codegen registration for `Cleanup`, `Save`, `SaveAs`, `CheckCompliance`.
- [x] Fix text pipeline: `docFromCommentGroup` always sets style on absent
      comments. `Cleanup()` dispatches on `DeclStyle()` instead of Go types.
      Single `renderComment` path via `go/doc/comment.Printer` — stylers
      produce text conforming to `go/doc/comment` syntax.
- [x] Fix Save() preamble: copyright and package doc before package clause.
- [x] Tests: round-trip parse→cleanup→save produces valid Go on synthetic
      and real files (accessor.go, config.go, element.go, root.go).
- [x] `go test ./internal/provider/goast/...` passes.

**Step 6: Orchestrator — lint-go-style.star (complete)**

The orchestrator is minimal — config and job control only. File discovery,
verbose logging, fix/check dispatch.

- [x] Rewrite orchestrator to the three-line model.
- [x] File discovery loop via `file.find`.
- [x] Verbose flag for filename echoing.
- [x] Delete all rule files.
- [x] `make build` and `make test` pass.

**Step 7: Stabilize on `*go/doc/comment.Doc` storage**

Discovery: `go/doc/comment` already parses comments into structured blocks
(`Paragraph`, `Heading`, `Code`, `List`). Storing raw text and re-parsing
is redundant. The normalize dispatch has a type mismatch: config uses
`type: section` but the code expects `param_section` / `return_section`.

**Design change:** `DocComment` stores `*go/doc/comment.Doc` instead of
`string`. One parse at load time, one render at save time, block tree
in between. See [doc-comment-styling.md](../architecture/doc-comment-styling.md).

- [x] `DocComment.doc *go/doc/comment.Doc` replaces `DocComment.text string`.
- [x] `LoadSourceFile` parses via `go/doc/comment.Parser.Parse()`.
- [x] `Save()` renders via `go/doc/comment.Printer.Text()`.
- [x] `CommentDecl` stores `*go/doc/comment.Doc`.
- [x] Single `styleDoc` styler: takes DocComment + styleContext, returns DocComment.
- [x] Unified `GenDeclNode` replaces TypeDecl/VarDecl/ConstDecl/ImportDecl.
      One entry per GenDecl — fixes multi-spec tripling bug.
- [x] Removed `Comment` fields from StructResult/MethodResult/FuncResult
      (unused by Starlark). Participle consumers reduced to transitional styler only.
- [x] `renderComment` replaced by `renderDoc`. `commentGroupRaw` retained
      for legacy provider methods only.
- [x] Tests: round-trip produces valid Go, star fix mode produces compilable output.
- [x] `make clean build test` passes.

**Step 8: Production-based styler**

Replace the transitional normalize dispatch with a production model operating
directly on `[]go/doc/comment.Block`. Each schema element defines a production
rule: what block types it consumes (ABNF notation), what prefix it expects
(fuzzy matching), and what it emits.

See [doc-comment-styling.md](../architecture/doc-comment-styling.md) for
full design: production model, ABNF consumes notation, conditions, fuzzy
matcher with normalization pipeline and synonym table, and greedy slot
assignment algorithm.

**Step 8a: ABNF parser and production types**

- [ ] `Consumes` struct: Min, Max, Types. Parsed from ABNF string.
- [ ] ABNF parser: recursive descent, ~50 lines.
- [ ] `Production` interface: `Execute(blocks []comment.Block, cursor int,
      schema SchemaElement, ctx styleContext) (output []comment.Block, next int)`.
- [ ] `itemProduction` — consumes blocks by type, matches prefix.
- [ ] `listProduction` — consumes heading + list, fills slots.
- [ ] Schema element gains `production`, `consumes`, `condition`,
      `prefix`, `split`, `slots`, `slot_prefix` fields.
- [ ] Tests: ABNF parsing, production matching on synthetic blocks.
- [ ] `make build` and `make test` pass.

**Step 8b: Fuzzy matcher**

- [ ] Normalization pipeline: strip demarcation, normalize case, strip
      trailing punctuation, synonym resolution.
- [ ] Levenshtein distance (~20 lines).
- [ ] Match scoring: exact → prefix → substring → edit distance.
- [ ] Hardcoded synonym table (Parameters/Returns/Deprecated).
- [ ] Tests: normalization, scoring, synonym resolution.
- [ ] `make build` and `make test` pass.

**Step 8c: Slot assignment**

- [ ] Greedy assignment algorithm with forced elimination.
- [ ] Score matrix: `fuzzyScore(normalize(item), normalize(slot))`.
- [ ] Threshold 0.3 for voluntary matches. Forced when 1:1 remains.
- [ ] TODO stub generation for unmatched slots.
- [ ] TODO flag for unmatched items.
- [ ] Tests: exact matches, fuzzy matches, renames, forced assignment.
- [ ] `make build` and `make test` pass.

**Step 8d: Wire productions into styleDoc**

- [ ] Replace transitional `switch ctx.nodeType` with production dispatcher.
- [ ] Dispatcher iterates schema elements, evaluates conditions, calls
      productions in order, assembles output block list.
- [ ] `styleDoc` operates on `*comment.Doc` directly — no text round-trip.
- [ ] Update config schema format: `consumes`, `production`, `condition`,
      `prefix`, `split`, `slots`, `slot_prefix` fields.
- [ ] Tests: full pipeline on NobleFactor func_doc schema.
- [ ] `make build` and `make test` pass.

**Step 8d-fix: Parameters stub missing for single-param functions**

`NewAccessor(v interface{})` gets a Returns stub but no Parameters stub.
The `condition: params` evaluates to true (the function has a param), but
the list production doesn't emit a stub. Likely cause: `astParamNames`
returns `["v"]` but the production's condition check or stub generation
has a subtle bug for single-param cases. Investigate and fix.

- [ ] Reproduce with a test: single-param function, no existing Parameters
      section → should get a Parameters stub with one TODO item.
- [ ] Fix the production or condition evaluation.
- [ ] `make build` and `make test` pass.

**Step 8d-fix2: Summary sentence splitting**

Multi-sentence doc comments like `"NewAccessor creates a ConfigAccessor
for the given value. The value should be a struct or pointer to struct."`
should be split into summary (first sentence) + body (remainder). The
`split: sentence` field on the summary production element is defined in
the design but not implemented. The item production currently consumes
the whole paragraph.

- [ ] Implement `split: sentence` in `itemProduction.Execute`.
- [ ] Extract first sentence, emit as summary paragraph.
- [ ] Push remainder as a new paragraph into the block stream for the
      body production to consume.
- [ ] Tests: multi-sentence paragraph split, single-sentence no-op.
- [ ] `make build` and `make test` pass.

**Step 8e: Remove participle**

- [ ] Remove `doctaxonomy/grammar.go`, `elements.go`, `lexer.go`,
      `normalize.go`.
- [ ] Remove `parseFuncDocSafe` from helpers.go.
- [ ] Remove `splitSummary` from astrewrite.go (absorbed by paragraph
      production).
- [ ] `go mod tidy` to drop participle dependency.
- [ ] Remove debug `cli.Warn` from `configSchemas`.
- [ ] Tests pass, `make clean build test` passes.

**~~Step 9: Remove dead code~~ — obsolete.** The `CommentStyle` enum removal
happens naturally as part of the ongoing refactor toward schema-driven behavior.
Not a separate step.

**Step 10: Body-level spacing rules**

Normalize blank lines inside function bodies. Same config model as top-level
spacing rules — named settings with blank line counts, loaded from config,
applied at save time. JetBrains IDEA model:

- [ ] Add body spacing settings to `SpacingRules` config type:
      `before_if`, `after_if`, `before_for`, `after_for`,
      `before_switch`, `after_switch`, `before_return`,
      `after_local_vars`, `around_body_comments`.
- [ ] Add defaults to extension config.
- [ ] `Cleanup()` applies body spacing rules within each declaration's
      code block.
- [ ] Tests: body spacing normalized in function bodies.
- [ ] `make build` and `make test` pass.

**Step 11: Fix codebase and CI**

- [ ] Run `star lint go-style --fix=true` on noblefactor-ops
- [ ] `make build` and `make test` pass after fix
- [ ] Review diff — no corrupted source, no duplicate methods
- [ ] Run check mode — violation count drops significantly
- [ ] Commit formatting changes
- [ ] Add `go-style` target to Makefile
- [ ] Add `Lint Go Style` step to `.github/workflows/ci.yaml` (check mode)
- [ ] Add go-style to pre-commit hook (`hook-pre-commit.star`) in fix mode
- [ ] Verify CI passes on a test PR

**Open issue:** `checkLineWidth` takes entire file content and hardcodes
skip conditions (SPDX, copyright, delineators). Redesign to use structured
declarations from SourceFile instead. Tracked in #119.

**Files (5b):**

| File                                                                              | Action | Purpose                                       |
| --------------------------------------------------------------------------------- | ------ | --------------------------------------------- |
| `star/extensions/com.noblefactor.star.LintGoStyle/extension.yaml`                 | Modify | Add comment_schemas config with defaults      |
| `internal/extension/spec.go`                                                      | Modify | Add NestedTypeDef, wire through ToConfigSpec   |
| `internal/config/types.go`                                                        | Modify | Fix sibling nested type resolution             |
| `internal/config/unified.go`                                                      | Modify | Add Navigate and MergeYAML methods             |
| `internal/provider/goast/astrewrite.go`                                           | Modify | Config-to-registry conversion, accept registry |
| `internal/provider/goast/provider.go`                                             | Modify | schemaRegistry/configSchemas from context       |
| `internal/provider/goast/doctaxonomy/schema.go`                                   | Modify | Programmatic DefaultRegistry, remove embed      |
| `internal/provider/goast/config_schema_test.go`                                   | Create | Defaults and override tests                    |

**Files (5c):**

| File                                                                               | Action     | Purpose                                     |
| ---------------------------------------------------------------------------------- | ---------- | ------------------------------------------- |
| `internal/provider/goast/sourcefile.go`                                            | Create     | SourceFile tree, declaration types, builder  |
| `internal/provider/goast/sourcefile_test.go`                                       | Create     | Iteration and lookup tests                   |
| `internal/provider/goast/provider.go`                                              | Modify     | LoadSourceFile + FormatComment methods       |
| `internal/provider/goast/gen/params.gen.go`                                        | Regenerate | LoadSourceFile entry                         |
| `star/extensions/com.noblefactor.star.LintGoStyle/commands/lint-go-style.star`      | Rewrite    | SourceFile-based orchestrator                |
| `star/extensions/com.noblefactor.star.LintGoStyle/rules/*.star`                     | Delete     | Replaced by inline orchestrator logic        |
| `Makefile`                                                                         | Modify     | Add go-style target                          |
| `.github/workflows/ci.yaml`                                                        | Modify     | Add lint step                                |
| `star/extensions/com.noblefactor.star.HookPreCommit/commands/hook-pre-commit.star`  | Modify     | Pre-commit hook                              |

### Phase 6: Cleanup (pending)

Remove dead code and verify no orphaned artifacts remain.

- [ ] Verify no orphaned helpers in `helpers.go`
- [ ] `make build` and `make test` pass

## Migration Path

**Provider consumers (Starlark scripts):**

- `.doc` field remains for backward compatibility
- `.comment` field is additive — scripts can adopt it incrementally
- No breaking changes to existing Starlark scripts

**Linter rules:**

- `doc-comments.star` and `line-width.star` are rewritten (Phase 4)
- Other 5 rules unchanged
- Orchestrator unchanged

**New comment formats:**

- Adding a format (Starlark, shell) requires a new YAML schema + lexer prefix config
- No changes to element types, grammar, or Printers

## Files to Create/Modify

| File                                                      | Action     | Phase | Purpose                            |
| --------------------------------------------------------- | ---------- | ----- | ---------------------------------- |
| `go.mod`                                                  | Modify     | 1     | Add participle/v2                  |
| `internal/provider/goast/doctaxonomy/lexer.go`            | Create     | 1     | Context-aware lexer factory        |
| `internal/provider/goast/doctaxonomy/elements.go`         | Create     | 1     | Element types with participle tags |
| `internal/provider/goast/doctaxonomy/grammar.go`          | Create     | 1     | Grammar structs, parser factory    |
| `internal/provider/goast/doctaxonomy/normalize.go`        | Create     | 1     | Per-element Normalize methods      |
| `internal/provider/goast/doctaxonomy/format.go`           | Create     | 1     | go/doc/comment formatting pass     |
| `internal/provider/goast/doctaxonomy/doctaxonomy_test.go` | Create     | 1     | Canonical example tests            |
| `internal/provider/goast/doctaxonomy/schema.go`           | Create     | 2     | Schema types, registry, validation |
| `internal/provider/goast/doctaxonomy/schemas/go.yaml`     | Create     | 2     | Go schemas (func, type, copyright) |
| `internal/provider/goast/doctaxonomy/schema_test.go`      | Create     | 2     | Schema and validation tests        |
| `internal/provider/goast/types.go`                        | Modify     | 3     | Add `.Comment` fields              |
| `internal/provider/goast/provider.go`                     | Modify     | 3     | Wire taxonomy, add FormatComment   |
| `internal/provider/goast/helpers.go`                      | Modify     | 3     | Remove 7 dead helpers              |
| `internal/provider/goast/gen/*.go`                        | Regenerate | 3     | Codegen for FormatComment          |
| `rules/doc-comments.star`                                 | Rewrite    | 4     | Element-based check and fix        |
| `rules/line-width.star`                                   | Rewrite    | 4     | Taxonomy-backed reflow             |
| `Makefile`                                                | Modify     | 5     | go-style target                    |
| `.github/workflows/ci.yaml`                               | Modify     | 5     | CI lint step                       |

## Open Questions

### Resolved

- [x] **Q1: `go/doc/comment` role — resolved: normalize-then-format pipeline.** Elements produce one unwrapped
      line each via `Normalize()`. Concatenated with correct blank-line separators. The assembled text goes
      through `go/doc/comment.Parser` + `Printer` in one final pass for line wrapping, indentation, and `//`
      prefix. No per-element formatting. `go/doc/comment` owns all line wrapping and indentation.

- [x] **Q2: Fix mode strategy — resolved: normalize, don't rewrite.** Fix mode adds missing required sections
      with `<TODO description>` placeholders. It does not rewrite existing content. Summary stubs use the
      function name: `FuncName <TODO description>`. Missing Returns sections are added with one stub per return
      value. Stale entries (e.g., a return name documented under Parameters) are flagged with
      `<TODO description>` but not moved. Warnings are issued on all TODOs.

      **Canonical fix example** — input:

      ```go
      // moves the file at "path" to a timestamped backup location.
      //
      // Parameters:
      //   - path: Absolute path to the file to back up
      //   - backupSuffix: Suffix appended before the timestamp (default: .devlore-backup)
      //   - err: any error
      func (p *Provider) Backup(path Resource, backupSuffix string) (result Resource, undo Tombstone, err error) {
      ```

      Fixed output:

      ```go
      // Backup TODO(go-style): add summary describing what this method does
      //
      // moves the file at "path" to a timestamped backup location.
      //
      // Parameters:
      //   - path: Absolute path to the file to back up
      //   - backupSuffix: Suffix appended before the timestamp (default: .devlore-backup)
      //   - err: TODO(go-style): 'err' is not a parameter of Backup; remove or move to Returns
      //
      // Returns:
      //   - result: TODO(go-style): add description for return value
      //   - undo: TODO(go-style): add description for return value
      //   - err: TODO(go-style): add description for return value
      ```

- [x] **Q3: Lexer token ambiguity — resolved: accepted risk.** Parameter names in prose match as `ParamName`
      tokens but `Paragraph` grammar accepts both `Word` and `ParamName`. We care about matching in headings
      and structured sections. Misidentification in prose paragraphs is harmless. Defer if it becomes a problem.

- [x] **Q4: Continuation lines — resolved: flat token stream.** Participle operates on a flat token stream;
      it does not inherently need line boundaries. We can elide `Newline` tokens entirely — the grammar matches
      `ListMarker @ParamName Colon @@` regardless of whether the description spans one line or three.
      `go/doc/comment.Printer` normalizes indentation and wrapping on the output side. No `ContinuationLine`
      token type needed.

- [x] **Q5: Directive handling — resolved: pass through `go/doc/comment`.** Directive lines (`+devlore:...`)
      are short single-line paragraphs. `go/doc/comment.Printer` reflow is a no-op on lines that fit within
      `TextWidth`. No strip-and-reinsert logic needed.

- [x] **Q6: Summary detection — resolved: exact function name match.** If the first paragraph doesn't start
      with the exact function name, it's not a valid summary. Fix mode generates a stub
      (`// FuncName TODO(go-style): add summary`) and demotes the original to body. No synonym matching —
      we'd never get it right with static tools. Drawing attention is the goal; a human (or an AI they
      delegate to) writes the real summary.

- [x] **Q8: TODO marker format — resolved: `TODO(go-style): `.** Standard Go convention, grep-able. Followed
      by an actionable description of what needs to be done. Example:
      `// Backup TODO(go-style): add summary describing what this method does`

- [x] **Q7: Stale param diagnostic — resolved: actionable message.** TODOs must be self-contained — a human
      or LLM should know exactly what to do without additional context. When a documented name doesn't match
      the section it's in, the TODO says what's wrong and what to do:
      `TODO(go-style): 'err' is not a parameter of Backup; remove or move to Returns`

### Open

No open questions remain. All design decisions have been resolved.

## Related Documents

- [Comment Taxonomy Architecture](../architecture/star-goast-linter.md) — design, element types, canonical example
- [Go Style Linter Plan](./lint-go-style.md) — Phases 0–3 (complete), linter extension scaffold
- [Go Style Guidelines](../guides/go-style-guidelines.md) — Section 4: Doc Comment Format
- [participle](https://github.com/alecthomas/participle) — PEG parser via Go struct tags
- [`go/doc/comment` stdlib](https://pkg.go.dev/go/doc/comment) — paragraph reflow engine
