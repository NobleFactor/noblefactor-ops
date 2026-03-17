---
title: "Comment Taxonomy Implementation"
status: draft
created: 2026-03-17
updated: 2026-03-17
tracking: TBD
---

# Plan: Comment Taxonomy Implementation

## Summary

Implement the comment taxonomy defined in [docs/architecture/star-goast-linter.md](../architecture/star-goast-linter.md).
Regex named groups parse doc comments into typed slots; Go templates render deterministic output. The goast provider
gains a `.Comment` field on every result type, and Starlark linter rules validate/fix via typed slots instead of
ad-hoc text parsing.

## Goals

1. **Taxonomy engine** — schema registry, two-level regex parser, template renderer with `reflow` function
2. **Go FuncDecl schema** — regex + template for the Go function/method doc comment format
3. **Provider integration** — `.Comment` field on `MethodResult`, `FuncResult`, `StructResult`, `FieldDetail`
4. **Linter rule migration** — `doc-comments.star` and `line-width.star` rewritten to use typed slots
5. **Fix mode that works** — `format_comment` renders from slots, producing correct output that compiles

## Current State

| Component | Status | Notes |
| --- | --- | --- |
| goast provider | Working | 17 methods, codegen verified |
| Linter extension | Working | 7 rules, check mode finds 1,523 violations |
| Fix mode | Broken | Merges copyright lines, inserts spurious stubs, breaks builds |
| Comment parsing | Ad-hoc | `parseParamDocs` in Go, `_extract_doc_lines` in Starlark |
| Comment rendering | Ad-hoc | `rewrapCommentGroup` + 4 helpers in Go, naive text in Starlark |

## Implementation Phases

### Phase 1: Taxonomy engine (pending)

Build the schema registry, regex parser, and template renderer as new files in the goast provider package.
No changes to existing provider methods yet — this phase is additive.

- [ ] Create `internal/provider/goast/taxonomy.go`:
      - `CommentSchema` struct (NodeType, Format, Pattern, Template, Slots)
      - `SlotDef` struct (Name, Type, Required, ItemPattern)
      - `SlotType` and `RequiredPolicy` enums
      - Schema registry: `RegisterSchema(schema)`, `LookupSchema(nodeType, format)`
- [ ] Create `internal/provider/goast/comment_parse.go`:
      - `ParseFuncComment(raw string, schema *CommentSchema) (*FuncComment, error)` — level 1 regex → named
        groups, level 2 item patterns → `ParamDoc`/`ReturnDoc`/`Directive` slices
      - `ParseTypeComment(raw string, schema *CommentSchema) (*TypeComment, error)`
      - `ParseFileComment(raw string, schema *CommentSchema) (*FileComment, error)`
      - `ParseFieldComment(raw string) (*FieldComment, error)` — trivial: inline text
- [ ] Create `internal/provider/goast/comment_render.go`:
      - `RenderFuncComment(comment *FuncComment, schema *CommentSchema, width int) (string, error)` — execute
        Go template with slot data and `reflow`/`wrap` template functions
      - `RenderTypeComment`, `RenderFileComment`, `RenderFieldComment` — same pattern
      - `reflow(width int, text string) string` — backed by `go/doc/comment.Printer`
      - `wrap(width int, prefix, text string) string` — word-wrap with continuation prefix
- [ ] Register Go FuncDecl schema:
      - Section regex with named groups: `summary`, `body`, `directives`, `parameters`, `returns`
      - Level-2 item patterns for `ParamListSlot`, `ReturnListSlot`, `DirectiveSlot`
      - Go template per the architecture doc
- [ ] Register Go TypeSpec schema (Summary + Body)
- [ ] Register Go File schema (Copyright + PackageDoc)
- [ ] Unit tests:
      - Round-trip: parse → render produces deterministic output
      - Partial match: missing optional slots → graceful degradation
      - Directive extraction: `//+devlore:defaults key=value` stripped and preserved
      - Item parsing: params, returns, directives correctly split
      - Reflow: paragraphs fill to column width, code blocks preserved
      - List rendering: `//   - name: long description wraps correctly`
- [ ] `make build` and `make test` pass

**Files:**

| File | Action | Purpose |
| --- | --- | --- |
| `internal/provider/goast/taxonomy.go` | Create | Schema types, registry |
| `internal/provider/goast/comment_parse.go` | Create | Two-level regex parser |
| `internal/provider/goast/comment_render.go` | Create | Template renderer with reflow |
| `internal/provider/goast/taxonomy_test.go` | Create | Engine unit tests |
| `internal/provider/goast/comment_proto_test.go` | Delete | Q2 prototype (served its purpose) |

### Phase 2: Provider integration (pending)

Wire the taxonomy into the goast provider. Every result type gains a `.Comment` field populated during extraction.
Replace `parseParamDocs` and `RewrapComments` internals. Run codegen.

- [ ] Add `FuncComment`, `TypeComment`, `FileComment`, `FieldComment` types to `types.go`
      (with `starlark:"..."` tags)
- [ ] Add `Directive`, `ParamDoc`, `ReturnDoc` shared sub-types to `types.go`
- [ ] Add `.Comment` field to `MethodResult`, `FuncResult`, `StructResult`, `FieldDetail`
- [ ] Update `Methods()`: parse `fn.Doc` → `FuncComment`, populate `result.Comment`
- [ ] Update `Funcs()`: same pattern
- [ ] Update `Structs()`: parse `ts.Doc` → `TypeComment`, populate `result.Comment`;
      parse `field.Comment` → `FieldComment`, populate `field.Comment`
- [ ] Replace `parseParamDocs` callers — `ParamDetail.Doc` now populated from
      `FuncComment.Params` (remove `parseParamDocs` entirely)
- [ ] Rewrite `RewrapComments` to use taxonomy parse → render internally
- [ ] Add `FormatComment(nodeType, comment, width)` provider method for Starlark write path
- [ ] Rebuild star binary (`make build`) — new method in provider
- [ ] Run codegen (new method `FormatComment` needs params.gen.go entry)
- [ ] `make build` and `make test` pass
- [ ] `make check-codegen` passes against develop

**Files:**

| File | Action | Purpose |
| --- | --- | --- |
| `internal/provider/goast/types.go` | Modify | Add comment types and `.Comment` fields |
| `internal/provider/goast/provider.go` | Modify | Wire taxonomy into Methods/Funcs/Structs, add FormatComment |
| `internal/provider/goast/helpers.go` | Modify | Remove `parseParamDocs`, `rewrapCommentGroup`, `rewrapTextParagraph`, `rewrapBulletItem`, `fillWords`, `stringSlicesEqual`, `commentBodyPrefix` |
| `internal/provider/goast/gen/params.gen.go` | Regenerate | FormatComment entry |
| `internal/provider/goast/gen/receiver.gen.go` | Regenerate | Updated method count |
| `internal/provider/goast/gen/receiver_gen_test.go` | Regenerate | Updated method count |

### Phase 3: Linter rule migration (pending)

Rewrite `doc-comments.star` and `line-width.star` to use `.comment` slots. Fix mode calls `goast.format_comment`
instead of doing text manipulation. Other rules (`formatting`, `receivers`, `file-layout`, `regions`, `method-order`)
are unaffected — they don't deal with doc comment content.

- [ ] Rewrite `doc-comments.star` check:
      - `m.comment.summary` existence check (replaces `_extract_doc_lines` + first-line scan)
      - `m.comment.params` vs `m.params` sync (replaces `_extract_param_names` + `_has_section`)
      - `m.comment.returns` vs `m.returns` sync
      - `m.comment.directives` presence (informational)
      - Remove `_extract_doc_lines`, `_has_section`, `_extract_param_names`, `_check_fill_width`
- [ ] Rewrite `doc-comments.star` fix:
      - Populate missing slots (summary TODO, param stubs, return stubs)
      - Call `goast.format_comment(node_type="FuncDecl", comment=slots, width=120)` to render
      - Write rendered comment back to file at the correct position
- [ ] Rewrite `line-width.star` check:
      - Under-filled detection moves to checking whether `goast.format_comment` output differs from
        current comment (a reformat would fill under-filled lines)
      - Over-long code lines remain as-is (check-only)
- [ ] Rewrite `line-width.star` fix:
      - Call `goast.rewrap_comments(path, width)` — now taxonomy-backed, produces correct output
      - Remove `_check_under_filled`, `_is_delineator` helpers
- [ ] Run fix mode on goast provider files — verify it produces correct, compilable output
- [ ] `make build` and `make test` pass

**Files:**

| File | Action | Purpose |
| --- | --- | --- |
| `rules/doc-comments.star` | Rewrite | Slot-based check and fix |
| `rules/line-width.star` | Rewrite | Taxonomy-backed reflow |

### Phase 4: Fix the codebase (pending)

With correct fix mode, run `star lint go-style --fix=true` across noblefactor-ops. Verify the build passes.
Commit the formatting changes. This is the prerequisite for wiring into CI.

- [ ] Run `star lint go-style --fix=true --generated=false --tests=false` on noblefactor-ops
- [ ] `make build` and `make test` pass after fix
- [ ] Review diff for correctness — no merged copyright lines, no spurious stubs, no broken delineators
- [ ] Run `star lint go-style --generated=false --tests=false` — violation count should drop significantly
      (remaining violations are check-only rules: file-layout, code-line-width)
- [ ] Commit formatting changes

### Phase 5: CI and Makefile integration (pending)

Wire `star lint go-style` into Makefile and CI. Add to pre-commit hook in fix mode.

- [ ] Add `go-style` target to noblefactor-ops Makefile
- [ ] Add `Lint Go Style` step to `.github/workflows/ci.yaml` (check mode, no `--fix`)
- [ ] Add go-style to pre-commit hook (`hook-pre-commit.star`) in fix mode
- [ ] Verify CI passes on a test PR
- [ ] `make build` and `make test` pass

**Files:**

| File | Action | Purpose |
| --- | --- | --- |
| `Makefile` | Modify | Add `go-style` target |
| `.github/workflows/ci.yaml` | Modify | Add lint step |
| `star/extensions/com.noblefactor.star.HookPreCommit/commands/hook-pre-commit.star` | Modify | Add go-style to pre-commit |

### Phase 6: Cleanup (pending)

Remove dead code, the prototype test file, and any remaining ad-hoc helpers.

- [ ] Remove `comment_proto_test.go` (Q2 prototype)
- [ ] Verify no orphaned helpers remain in `helpers.go`
- [ ] Verify `doc-comments.star` has no text-parsing helpers
- [ ] Verify `line-width.star` has no text-parsing helpers
- [ ] `make build` and `make test` pass

## Migration Path

**Provider consumers (Starlark scripts):**
- `.doc` field remains for backward compatibility
- `.comment` field is additive — scripts can adopt it incrementally
- No breaking changes to existing Starlark scripts

**Linter rules:**
- `doc-comments.star` and `line-width.star` are rewritten (Phase 3)
- Other 5 rules unchanged
- Orchestrator unchanged

## Open Questions

- [x] Q1: Directive handling — strip before parse, render verbatim at canonical position (`//+devlore:...`)
- [x] Q2: List formatting — Printer matches `  - name: desc` style but doesn't wrap; render lists ourselves
- [x] Q3: Section headers — `Parameters:` survives round-trip unchanged
- [ ] Q4: Should `FormatComment` accept a Starlark dict of slot values, or a more structured input?
      The Starlark side needs to construct the slot data for fix mode.
- [ ] Q5: Should the Go FuncDecl regex handle multi-line function signatures (signature spans 2+ lines
      with the doc comment ending before the first line)? Current `go/ast` handles this — the regex only
      parses the `.Doc` text, not the signature. Confirm this is a non-issue.

## Related Documents

- [Comment Taxonomy Architecture](../architecture/star-goast-linter.md) — design, schemas, data flow
- [Go Style Linter Plan](./lint-go-style.md) — Phases 0–3 (complete), Phase 4 (pending)
- [Go Style Guidelines](../guides/go-style-guidelines.md) — Section 4: Doc Comment Format
- [`go/doc/comment` stdlib](https://pkg.go.dev/go/doc/comment) — paragraph reflow engine
