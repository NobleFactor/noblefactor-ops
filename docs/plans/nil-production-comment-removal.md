# Nil Production and Delineator Detection

## Context

The doc comment styling system is feature complete for formatting (summary/body/sections/directives). The next capabilities are **removal** and eventually **transformation** of classified comments. The immediate use case is delineator comments (box-style separators with repeated characters):

- **Removal** — delete delineators entirely via a nil production
- **Transformation** (future) — reshape delineators to a different style via a template production

Both depend on correct classification, so widening `isDelineatorBlock` is prerequisite work.

## Design

### 1. Widen delineator detection

`isDelineatorBlock()` in `astrewrite.go:166-188` currently recognizes only 4 filler characters
(`=`, `-`, `~`, `*`) and requires the entire line to be one repeated character. This misses
most real-world patterns.

**Filler characters to add:**

Based on patterns from [Comment Divider](https://github.com/stackbreak/comment-divider),
[Comment Bars](https://marketplace.visualstudio.com/items?itemName=zfzackfrost.commentbars),
[comment-box.nvim](https://github.com/LudoPinelli/comment-box.nvim) (22 box styles, 17 line
styles), and common Go codebases:

| Current | Add |
|---|---|
| `=` `-` `~` `*` | `#` `+` `^` `/` `@` `%` `_` |

Plus Unicode box-drawing characters: `━` `─` `═` `╌` `┄` `┅` `┈` `┉`

**Pattern variants to detect:**

1. **Pure repeated line** (current, just more characters):
   ```
   // =============================================================================
   // #############################################################################
   ```

2. **Centered-text banner** — line starts and ends with 3+ of the same filler character
   with text in between:
   ```
   // ========================= Section Name ======================================
   // ------------------------------ Helpers ---------------------------------------
   // ******************** Public API *********************************************
   ```

3. **Multi-line box** — recognized as a unit when first and last lines are pure repeated
   lines (already handled by classifying any comment containing a delineator line):
   ```
   // =============================================================================
   // Section Name
   // =============================================================================
   ```

**Implementation:**

Replace the hardcoded character check with a set lookup. Add a second detection path
for centered-text banners: strip the text content from the middle, check that both sides
are 3+ of the same filler character.

```go
var delineatorChars = map[byte]bool{
    '=': true, '-': true, '~': true, '*': true,
    '#': true, '+': true, '^': true, '/': true,
    '@': true, '%': true, '_': true,
}

func isDelineatorLine(s string) bool {
    // Pure repeated: all same character, 3+ long
    // Centered banner: 3+ filler, text, 3+ same filler
}
```

For Unicode box-drawing, check the first rune against a set of known box-drawing characters.

### 2. Nil production

A new production type that consumes blocks and emits nothing.

- Schema YAML: `production: "nil"`
- New `nilProduction` struct implementing `Production` interface
- `Execute()` advances cursor past all blocks matching the `consumes` spec, returns empty
  `[]comment.Block`
- `NewProduction()` recognizes `"nil"` and returns a `nilProduction`

```go
type nilProduction struct {
    consumes Consumes
}

func (p *nilProduction) Execute(blocks []comment.Block, cursor int, elem doctaxonomy.SchemaElement, ctx styleContext) ([]comment.Block, int) {
    // Advance past all matching blocks, emit nothing.
    i := cursor
    for i < len(blocks) && p.consumes.Matches(blockTypeName(blocks[i])) {
        i++
    }
    return nil, i
}
```

### 3. Removal flag on CommentDecl

`CommentDecl` needs a way to signal "I was removed." Currently it has no `present` or
`removed` field. Without this, `SaveAs()` would emit spacing for a phantom comment.

- Add `removed bool` to `CommentDecl`
- In `SaveAs()`, skip `CommentDecl` entries where `removed == true` (before spacing
  calculation)

### 4. styleDoc returns []DocComment

Change `styleDoc` signature to return a slice:

```go
func (sf *SourceFile) styleDoc(dc DocComment, ctx styleContext) []DocComment
```

- Most productions return a single-element slice (identical to current behavior).
- Productions that generate new comments (e.g., region) return multiple elements.
  The first replaces the original; additional elements become new `CommentDecl`
  entries inserted after it in `allDecls`.

All existing callers (`StyleFuncDoc`, `StyleGenDeclDoc`, `StylePackageDoc`) take `[0]`
from the returned slice — minimal change.

### 5. Route classified styles through styleDoc when a schema exists

Change `Cleanup()` so that styles currently skipped (delineator, section header, prose,
etc.) get routed through `styleDoc()` **if** a matching schema is registered. No schema
registered = preserve verbatim (backward compatible).

Node type mapping for currently-skipped styles:

| CommentStyle | Node type for schema lookup |
|---|---|
| `StyleDelineator` | `"Delineator"` |
| `StyleSectionHeader` | `"SectionHeader"` |
| `StyleRegionMarker` | `"RegionMarker"` |
| `StyleProse` | `"Floating"` |

`Cleanup()` collects insertions during iteration and splices them into `allDecls`
after the loop (to avoid mutating the slice while iterating):

```go
type insertion struct {
    afterIndex int
    decls      []*CommentDecl
}

var inserts []insertion

for i, decl := range sf.allDecls {
    switch decl.DeclStyle() {
    // ... existing cases take results[0] ...

    case StyleDelineator, StyleSectionHeader, StyleRegionMarker, StyleProse:
        cd, ok := decl.(*CommentDecl)
        if !ok {
            continue
        }
        nodeType := floatingNodeType(cd.style)
        if schema := sf.schemaRegistry().Lookup(nodeType, "go"); schema == nil {
            continue
        }
        results := sf.styleDoc(
            DocComment{doc: cd.doc, present: true, style: cd.style},
            styleContext{nodeType: nodeType},
        )
        if len(results) == 0 || len(results[0].doc.Content) == 0 {
            cd.removed = true
            continue
        }
        cd.doc = results[0].doc
        if len(results) > 1 {
            var newDecls []*CommentDecl
            for _, r := range results[1:] {
                newDecls = append(newDecls, &CommentDecl{
                    doc:   r.doc,
                    style: r.style,
                })
            }
            inserts = append(inserts, insertion{afterIndex: i, decls: newDecls})
        }
    }
}

// Splice insertions in reverse order to preserve indices.
for i := len(inserts) - 1; i >= 0; i-- {
    ins := inserts[i]
    sf.spliceDecls(ins.afterIndex, ins.decls)
}
```

`StyleImportDoc` and `StyleCopyright` remain skipped (they already have specialized
handling or aren't candidates for removal).

### 6. Config example

```yaml
comment_schemas:
  delineator:
    format: go
    node_type: Delineator
    elements:
      - name: content
        production: nil
        consumes: "*(Paragraph / Code / Heading / List)"
        order: 1
```

This says: for any comment classified as a delineator, consume all blocks and emit
nothing. Net effect: the comment is removed from the file.

To *keep* delineators, simply don't register a Delineator schema (current default
behavior).

### 7. Resize production (phase 2): extend delineators to line width

The resize production detects the input form (line, banner, or box), applies a target
character set per form, and resizes to the configured line width.

**Three forms (detected from input):**

1. **Line** — pure repeated characters: `// ====`
2. **Banner** — text centered between filler: `// === foo bar ===`
3. **Box** — multi-line with border lines and content:
   ```
   // =============
   // foo bar
   // =============
   ```

**Style catalog — box styles (have corners, usable for all three forms):**

| Name | H | V | TL | TR | BL | BR |
|---|---|---|---|---|---|---|
| `light` | `─` | `│` | `┌` | `┐` | `└` | `┘` |
| `heavy` | `━` | `┃` | `┏` | `┓` | `┗` | `┛` |
| `double` | `═` | `║` | `╔` | `╗` | `╚` | `╝` |
| `rounded` | `─` | `│` | `╭` | `╮` | `╰` | `╯` |

**Style catalog — line-only styles (no corners, lines and banners only):**

| Name | H | V |
|---|---|---|
| `light-triple-dash` | `┄` | `┆` |
| `heavy-triple-dash` | `┅` | `┇` |
| `light-quad-dash` | `┈` | `┊` |
| `heavy-quad-dash` | `┉` | `┋` |
| `light-double-dash` | `╌` | `╎` |
| `heavy-double-dash` | `╍` | `╏` |

**Style catalog — ASCII styles (single repeated character):**

| Name | Char |
|---|---|
| `ascii` | `-` |
| `ascii-=` | `=` |
| `ascii-*` | `*` |
| `ascii-#` | `#` |
| `ascii-~` | `~` |
| `ascii-+` | `+` |
| `ascii-_` | `_` |

4 box + 6 line-only + 7 ASCII = **17 named styles**.

**Config: independent style per form.** Omit a form to preserve its original characters.

```yaml
- name: content
  production: resize
  style:
    line: ascii-=
    banner: heavy
    box: double
  consumes: "*(Paragraph / Code / Heading / List)"
  order: 1
```

- Input `// ====` (line) → `// =============...=` resized with `=`
- Input `// === foo ===` (banner) → `// ━━━━━━ foo ━━━━━━━━...━` resized with heavy
- Input multi-line box → resized with `═` `║` `╔╗╚╝` double chars

**BoxStyle struct:**

```go
type BoxStyle struct {
    Name        string
    Horizontal  rune
    Vertical    rune   // zero for line-only/ASCII styles
    TopLeft     rune   // zero for line-only/ASCII styles
    TopRight    rune
    BottomLeft  rune
    BottomRight rune
}
```

Line-only and ASCII styles have zero-value corners/verticals. If the input is a box
but the target style has no corners, fall back to the ASCII `+` for corners.

`styleContext` needs a `lineWidth int` field (already available via `sf.lineWidth()`).

**Config examples summary:**

```yaml
# Remove delineators (phase 1)
production: nil

# Resize, preserve original characters
production: resize

# Resize with style conversion
production: resize
style:
  line: double
  banner: double
  box: double
```

**Caveat:** `go/doc/comment` merges consecutive non-blank lines into a single Paragraph.
A 3-line box comment (border / text / border) may parse as one Paragraph, so the resize
production needs to work at the text-line level within a paragraph, not just at the block
level. It splits the paragraph's plain text on newlines, detects which lines are delineator
lines vs content, resizes the delineator lines, and reassembles the paragraph.

### 8. Region production (phase 3): convert delineators to region markers

Convert a delineator to a `// region Name` / `// endregion` pair. Since a delineator
is a single comment that visually groups code, but region markers are two comments that
bracket a section, we can't determine where the region ends. The endregion gets a TODO.

**Editor support for region markers:**

| Editor | Syntax | Status |
|---|---|---|
| **GoLand** | `// region Name` / `// endregion` | Native support |
| **VS Code** | `// region Name` or `// #region Name` / `// endregion` | Built-in folding; breaks when gopls controls folding |
| **Neovim** | Tree-sitter based | Requires plugin for region markers |
| **Zed** | Tree-sitter / LSP based | No native region markers |
| **Helix** | No code folding yet | N/A |

GoLand and VS Code are the primary consumers. Use `// region` / `// endregion` syntax
(without `#`) for maximum compatibility.

**Behavior:**

1. Extract the section name from the delineator:
   - **Centered-text banner** (`// === Section Name ===`): extract "Section Name"
   - **Multi-line box**: extract the content line(s) between border lines
   - **Pure repeated line** (no text): use `TODO(go-style): add section name`
2. Replace the delineator `CommentDecl` with `// region Name`
3. Insert a new `CommentDecl` immediately after with
   `// endregion Name  // TODO(go-style): move to end of section`

**Return value:** `styleDoc()` returns `[]DocComment` with two elements:

```go
[]DocComment{
    {doc: textToDoc("region " + sectionName), style: StyleRegionMarker},
    {doc: textToDoc("endregion " + sectionName + "  TODO(go-style): move to end of section"), style: StyleRegionMarker},
}
```

The first replaces the original delineator. The second is inserted as a new
`CommentDecl` immediately after by the `Cleanup()` insertion logic (section 5).

**Config example:**

```yaml
comment_schemas:
  delineator:
    format: go
    node_type: Delineator
    elements:
      - name: content
        production: region
        consumes: "*(Paragraph / Code / Heading / List)"
        order: 1
```

## Config summary: one schema, three production choices

All three transformations target the same `Delineator` node type. The project picks one:

```yaml
# Remove delineators (phase 1)
production: nil

# Resize delineators to line width (phase 2)
production: resize

# Convert delineators to region markers (phase 3)
production: region
```

## Files to modify

### Phase 1: detection + nil production

| File | Change |
|---|---|
| `internal/provider/goast/astrewrite.go` | Widen `isDelineatorBlock` — more characters, centered-text banners, Unicode box-drawing |
| `internal/provider/goast/production.go` | Add `nilProduction` struct + `NewProduction` case |
| `internal/provider/goast/sourcefile.go` | Add `removed` field to `CommentDecl`; update `Cleanup()` routing; update `SaveAs()` to skip removed decls |
| `internal/provider/goast/astrewrite_test.go` | Tests for widened delineator detection |
| `internal/provider/goast/production_test.go` | Tests for `nilProduction` |
| `star/config.yaml` | Add delineator schema with nil production |
| `docs/architecture/doc-comment-styling.md` | Document nil production, removal semantics, widened detection |

### Phase 2: resize production

| File | Change |
|---|---|
| `internal/provider/goast/production.go` | Add `resizeProduction` struct + `NewProduction` case |
| `internal/provider/goast/sourcefile.go` | Pass `lineWidth` through `styleContext` |
| `internal/provider/goast/production_test.go` | Tests for `resizeProduction` — pure lines, centered banners, multi-line boxes |

### Phase 3: region production

| File | Change |
|---|---|
| `internal/provider/goast/production.go` | Add `regionProduction` struct + `NewProduction` case; section name extraction logic |
| `internal/provider/goast/sourcefile.go` | Cleanup routing for region: insert new `CommentDecl` into `allDecls` |
| `internal/provider/goast/production_test.go` | Tests for `regionProduction` — name extraction from banners, boxes, pure lines |

## Verification

### Phase 1
1. Unit tests for `isDelineatorLine`: pure repeated lines (all filler chars), centered-text
   banners, Unicode box-drawing, negative cases (prose, code, short lines)
2. Unit test: `nilProduction.Execute()` consumes all blocks, returns nil output, advances cursor
3. Unit test: `styleDoc()` with an all-nil schema produces empty Content
4. Integration test: `Cleanup()` + `SaveAs()` on a file with delineator comments — delineators
   removed, no orphan blank lines, surrounding code preserved
5. Backward compat: existing configs without Delineator schema — delineators preserved verbatim

### Phase 2
6. Unit test: `resizeProduction` on a pure line — output matches `lineWidth - 3`
7. Unit test: `resizeProduction` on centered-text banner — text preserved, filler adjusted
8. Unit test: `resizeProduction` on multi-line box — borders resized, content lines untouched
9. Integration test: full pipeline with resize schema — delineators normalized to line width

### Phase 3
10. Unit test: section name extraction from centered-text banner
11. Unit test: section name extraction from multi-line box
12. Unit test: pure repeated line → TODO placeholder name
13. Integration test: delineator → region + endregion with TODO, correct placement in allDecls
