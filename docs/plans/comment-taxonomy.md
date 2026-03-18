---
title: "Comment Taxonomy Implementation"
status: in-progress
created: 2026-03-17
updated: 2026-03-17
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

### Phase 5: Fix the codebase and CI (pending)

With correct fix mode, run across noblefactor-ops. Wire into CI and pre-commit.

- [ ] Run `star lint go-style --fix=true --generated=false --tests=false` on noblefactor-ops
- [ ] `make build` and `make test` pass after fix
- [ ] Review diff — no merged copyright lines, no spurious stubs, no broken delineators
- [ ] Run check mode — violation count should drop significantly
- [ ] Commit formatting changes
- [ ] Add `go-style` target to Makefile
- [ ] Add `Lint Go Style` step to `.github/workflows/ci.yaml` (check mode)
- [ ] Add go-style to pre-commit hook (`hook-pre-commit.star`) in fix mode
- [ ] Verify CI passes on a test PR

**Files:**

| File                                                                               | Action | Purpose               |
| ---------------------------------------------------------------------------------- | ------ | --------------------- |
| `Makefile`                                                                         | Modify | Add `go-style` target |
| `.github/workflows/ci.yaml`                                                        | Modify | Add lint step         |
| `star/extensions/com.noblefactor.star.HookPreCommit/commands/hook-pre-commit.star` | Modify | Pre-commit hook       |

### Phase 6: Cleanup (pending)

Remove dead code and verify no orphaned artifacts remain.

- [ ] Verify no orphaned helpers in `helpers.go` (should be ~820 lines smaller)
- [ ] Verify `doc-comments.star` has no text-parsing helpers
- [ ] Verify `line-width.star` has no text-parsing helpers
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
