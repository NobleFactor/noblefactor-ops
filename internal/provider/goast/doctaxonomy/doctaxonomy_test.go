// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"strings"
	"testing"
)

// canonicalInput is the raw comment text (with // prefix stripped) from the
// canonical Backup method example in the architecture doc.
const canonicalInput = `Backup creates a timestamped copy of the resource. Existing backups
are overwritten.

+devlore:defaults overwrite=true

Parameters:
  - resource: The file to back up.
  - opts: Backup options (default: nil).

Returns:
  - Resource: The backup copy.
  - Tombstone: Compensation state.
  - error: Non-nil if the backup failed.`

var (
	backupParams  = []string{"resource", "opts"}
	backupReturns = []string{"Resource", "Tombstone", "error"}
)

// TestCanonicalBackup_Parse parses the canonical Backup comment and verifies
// the parse tree matches the architecture doc Step 6.
func TestCanonicalBackup_Parse(t *testing.T) {
	p := NewFuncParser(backupParams, backupReturns)
	doc, err := p.ParseString("", canonicalInput)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(doc.Elements) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(doc.Elements))
	}

	// Element 0: Paragraph (summary).
	el := doc.Elements[0]
	if el.Paragraph == nil {
		t.Fatal("element 0: expected Paragraph")
	}
	summary := el.Paragraph.Normalize()
	if !strings.HasPrefix(summary, "Backup creates") {
		t.Errorf("summary = %q, want prefix 'Backup creates'", summary)
	}
	if !strings.HasSuffix(summary, "are overwritten.") {
		t.Errorf("summary = %q, want suffix 'are overwritten.'", summary)
	}

	// Element 1: Directive.
	el = doc.Elements[1]
	if el.Directive == nil {
		t.Fatal("element 1: expected Directive")
	}
	if el.Directive.Key != "devlore:defaults" {
		t.Errorf("directive key = %q, want 'devlore:defaults'", el.Directive.Key)
	}
	if len(el.Directive.Value) != 1 || el.Directive.Value[0] != "overwrite=true" {
		t.Errorf("directive value = %v, want ['overwrite=true']", el.Directive.Value)
	}

	// Element 2: ParamSection.
	el = doc.Elements[2]
	if el.ParamSection == nil {
		t.Fatal("element 2: expected ParamSection")
	}
	if len(el.ParamSection.Items) != 2 {
		t.Fatalf("param items = %d, want 2", len(el.ParamSection.Items))
	}
	if el.ParamSection.Items[0].Name != "resource" {
		t.Errorf("param 0 name = %q, want 'resource'", el.ParamSection.Items[0].Name)
	}
	if el.ParamSection.Items[1].Name != "opts" {
		t.Errorf("param 1 name = %q, want 'opts'", el.ParamSection.Items[1].Name)
	}

	// Element 3: ReturnSection.
	el = doc.Elements[3]
	if el.ReturnSection == nil {
		t.Fatal("element 3: expected ReturnSection")
	}
	if len(el.ReturnSection.Items) != 3 {
		t.Fatalf("return items = %d, want 3", len(el.ReturnSection.Items))
	}
	if el.ReturnSection.Items[0].Type != "Resource" {
		t.Errorf("return 0 type = %q, want 'Resource'", el.ReturnSection.Items[0].Type)
	}
	if el.ReturnSection.Items[1].Type != "Tombstone" {
		t.Errorf("return 1 type = %q, want 'Tombstone'", el.ReturnSection.Items[1].Type)
	}
	if el.ReturnSection.Items[2].Type != "error" {
		t.Errorf("return 2 type = %q, want 'error'", el.ReturnSection.Items[2].Type)
	}
}

// TestCanonicalBackup_Normalize verifies the normalize output matches the
// architecture doc Step 7.
func TestCanonicalBackup_Normalize(t *testing.T) {
	p := NewFuncParser(backupParams, backupReturns)
	doc, err := p.ParseString("", canonicalInput)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	normalized := doc.Normalize()

	expected := `Backup creates a timestamped copy of the resource. Existing backups are overwritten.

+devlore:defaults overwrite=true

Parameters:
  - resource: The file to back up.
  - opts: Backup options (default: nil).

Returns:
  - Resource: The backup copy.
  - Tombstone: Compensation state.
  - error: Non-nil if the backup failed.`

	if normalized != expected {
		t.Errorf("normalized output mismatch:\n--- got ---\n%s\n--- want ---\n%s", normalized, expected)
	}
}

// TestCanonicalBackup_Format verifies the full pipeline (parse → normalize →
// format) produces the expected output matching architecture doc Step 8.
func TestCanonicalBackup_Format(t *testing.T) {
	p := NewFuncParser(backupParams, backupReturns)
	doc, err := p.ParseString("", canonicalInput)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	normalized := doc.Normalize()
	output := Format(normalized, 120)

	// The output should contain the comment prefix and all sections.
	if !strings.Contains(output, "// Backup creates a timestamped copy") {
		t.Error("output missing summary line")
	}
	if !strings.Contains(output, "// +devlore:defaults overwrite=true") {
		t.Error("output missing directive")
	}
	if !strings.Contains(output, "// Parameters:") {
		t.Error("output missing Parameters section")
	}
	if !strings.Contains(output, "//   - resource: The file to back up.") {
		t.Error("output missing resource param")
	}
	if !strings.Contains(output, "// Returns:") {
		t.Error("output missing Returns section")
	}
	if !strings.Contains(output, "//   - error: Non-nil if the backup failed.") {
		t.Error("output missing error return")
	}

	// Verify no line exceeds width.
	for i, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if len(line) > 120 {
			t.Errorf("line %d exceeds 120 columns (%d): %q", i+1, len(line), line)
		}
	}
}

// TestLexerTokenTypes verifies that parameter names and return types are
// lexed as their respective token types.
func TestLexerTokenTypes(t *testing.T) {
	lex := NewDocLexer([]string{"resource"}, []string{"Resource"})

	iter, err := lex.LexString("", "resource foo Resource")
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}

	syms := lex.Symbols()
	// Build reverse map: id → name.
	idToName := make(map[int]string)
	for name, id := range syms {
		idToName[int(id)] = name
	}

	var names []string
	for {
		tok, err := iter.Next()
		if err != nil {
			t.Fatalf("next error: %v", err)
		}
		name := idToName[int(tok.Type)]
		if name == "EOF" {
			break
		}
		if name != "whitespace" {
			names = append(names, name)
		}
	}

	// Expect: ParamName, Word, ReturnType.
	if len(names) < 3 {
		t.Fatalf("expected at least 3 named tokens, got %d: %v", len(names), names)
	}
	if names[0] != "ParamName" {
		t.Errorf("token 0 = %q, want ParamName", names[0])
	}
	if names[1] != "Word" {
		t.Errorf("token 1 = %q, want Word", names[1])
	}
	if names[2] != "ReturnType" {
		t.Errorf("token 2 = %q, want ReturnType", names[2])
	}
}

// TestStaleParam verifies that a word not in the parameter list parses as a
// ParamItem but the name is captured as a Word (not ParamName). Post-parse
// validation detects the mismatch.
func TestStaleParam(t *testing.T) {
	p := NewFuncParser([]string{"resource", "opts"}, nil)

	// "path" is not a parameter — it's a Word, not ParamName.
	// The grammar accepts both so parsing succeeds, but validation
	// should flag "path" as stale.
	input := `Summary line.

Parameters:
  - path: The file.`

	doc, err := p.ParseString("", input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// ParamSection should have 1 item — the stale name "path" was parsed.
	var paramSection *ParamSection
	for _, el := range doc.Elements {
		if el.ParamSection != nil {
			paramSection = el.ParamSection
		}
	}
	if paramSection == nil {
		t.Fatal("expected ParamSection")
	}
	if len(paramSection.Items) != 1 {
		t.Fatalf("expected 1 param item, got %d", len(paramSection.Items))
	}
	if paramSection.Items[0].Name != "path" {
		t.Errorf("param name = %q, want 'path'", paramSection.Items[0].Name)
	}
}

// TestMissingSection verifies that a comment with summary only produces
// nil ParamSection and ReturnSection.
func TestMissingSection(t *testing.T) {
	p := NewFuncParser(nil, nil)

	doc, err := p.ParseString("", "Summary only.")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	for _, el := range doc.Elements {
		if el.ParamSection != nil {
			t.Error("unexpected ParamSection")
		}
		if el.ReturnSection != nil {
			t.Error("unexpected ReturnSection")
		}
	}

	if len(doc.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(doc.Elements))
	}
	if doc.Elements[0].Paragraph == nil {
		t.Error("expected Paragraph element")
	}
}

// TestUnorderedInput verifies that directives before summary parse
// successfully and Normalize outputs canonical order.
func TestUnorderedInput(t *testing.T) {
	p := NewFuncParser([]string{"x"}, nil)

	input := `+devlore:test value

FuncName does something.

Parameters:
  - x: The input.`

	doc, err := p.ParseString("", input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	normalized := doc.Normalize()

	// Summary should come first in normalized output.
	lines := strings.Split(normalized, "\n")
	if !strings.HasPrefix(lines[0], "FuncName does") {
		t.Errorf("first line = %q, want 'FuncName does...'", lines[0])
	}

	// Directive should come after summary.
	if !strings.Contains(normalized, "+devlore:test value") {
		t.Error("missing directive in normalized output")
	}

	// Parameters should come after directive.
	dirIdx := strings.Index(normalized, "+devlore:test")
	paramIdx := strings.Index(normalized, "Parameters:")
	if paramIdx < dirIdx {
		t.Error("Parameters should come after directive in canonical order")
	}
}

// TestCopyright verifies round-trip parse → normalize for copyright headers.
func TestCopyright(t *testing.T) {
	p := NewCopyrightParser()

	input := `SPDX-License-Identifier: MIT
Copyright (c) 2025-2026 Noble Factor. All rights reserved.`

	doc, err := p.ParseString("", input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if doc.SPDX != "MIT" {
		t.Errorf("SPDX = %q, want 'MIT'", doc.SPDX)
	}

	normalized := doc.Normalize()
	if !strings.Contains(normalized, "SPDX-License-Identifier: MIT") {
		t.Error("normalized missing SPDX line")
	}
	if !strings.Contains(normalized, "Copyright (c) 2025-2026 Noble Factor.") {
		t.Error("normalized missing Copyright line")
	}
}

// TestTypeDoc verifies summary-only and summary + body type docs.
func TestTypeDoc(t *testing.T) {
	p := NewTypeParser()

	t.Run("summary only", func(t *testing.T) {
		doc, err := p.ParseString("", "Resource represents a handle to data.")
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if len(doc.Elements) != 1 {
			t.Fatalf("expected 1 element, got %d", len(doc.Elements))
		}
		if doc.Elements[0].Paragraph == nil {
			t.Error("expected Paragraph")
		}
		normalized := doc.Normalize()
		if normalized != "Resource represents a handle to data." {
			t.Errorf("normalized = %q", normalized)
		}
	})

	t.Run("summary and body", func(t *testing.T) {
		input := `Resource represents a handle to data.

It can be streamed or read in full.`
		doc, err := p.ParseString("", input)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if len(doc.Elements) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(doc.Elements))
		}
		normalized := doc.Normalize()
		if !strings.Contains(normalized, "Resource represents") {
			t.Error("missing summary")
		}
		if !strings.Contains(normalized, "It can be streamed") {
			t.Error("missing body")
		}
	})
}

// TestNormalizeSingleLine verifies that each element type produces exactly
// one unwrapped line (or structured multi-line for sections).
func TestNormalizeSingleLine(t *testing.T) {
	t.Run("paragraph", func(t *testing.T) {
		p := &Paragraph{Words: []string{"Hello", "world", "foo", "bar"}}
		got := p.Normalize()
		if strings.Contains(got, "\n") {
			t.Errorf("paragraph contains newline: %q", got)
		}
		if got != "Hello world foo bar" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("paragraph punctuation", func(t *testing.T) {
		// Simulates ParamName("resource") + Word(".") from the lexer.
		p := &Paragraph{Words: []string{"the", "resource", ".", "Existing"}}
		got := p.Normalize()
		if got != "the resource. Existing" {
			t.Errorf("got %q, want 'the resource. Existing'", got)
		}
	})

	t.Run("directive", func(t *testing.T) {
		d := &Directive{Key: "devlore:defaults", Value: []string{"overwrite=true"}}
		got := d.Normalize()
		if got != "+devlore:defaults overwrite=true" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("heading", func(t *testing.T) {
		h := &Heading{Text: []string{"Example", "Usage"}}
		got := h.Normalize()
		if got != "# Example Usage" {
			t.Errorf("got %q", got)
		}
	})
}

// TestFormatWidth verifies that Format produces lines within the target
// width and adds the // prefix.
func TestFormatWidth(t *testing.T) {
	// A long paragraph that should be wrapped.
	words := strings.Repeat("word ", 30)
	normalized := "Summary " + strings.TrimSpace(words) + "."

	output := Format(normalized, 80)

	if !strings.HasPrefix(output, "// ") {
		t.Errorf("output missing // prefix, got: %q", output[:min(40, len(output))])
	}

	for i, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if len(line) > 80 {
			t.Errorf("line %d exceeds 80 columns (%d): %q", i+1, len(line), line)
		}
	}
}

// TestFormatCodeBlockPreserved verifies that code blocks pass through
// format unchanged.
func TestFormatCodeBlockPreserved(t *testing.T) {
	normalized := `Summary line.

    code := "preserved"
    fmt.Println(code)`

	output := Format(normalized, 120)

	if !strings.Contains(output, "code :=") {
		t.Errorf("code block not preserved in output:\n%s", output)
	}
}

// TestDirectivePassThrough verifies that directive lines survive format
// unchanged.
func TestDirectivePassThrough(t *testing.T) {
	normalized := `Summary line.

+devlore:defaults overwrite=true`

	output := Format(normalized, 120)

	if !strings.Contains(output, "+devlore:defaults overwrite=true") {
		t.Errorf("directive not preserved in output:\n%s", output)
	}
}

// TestMultiParagraphBody verifies heading + paragraph + code block in body.
func TestMultiParagraphBody(t *testing.T) {
	p := NewFuncParser(nil, nil)

	input := `Summary line.

# Example

This is the body paragraph.

    code := "example"

Another body paragraph.`

	doc, err := p.ParseString("", input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	normalized := doc.Normalize()

	if !strings.HasPrefix(normalized, "Summary line.") {
		t.Error("summary not first")
	}
	if !strings.Contains(normalized, "# Example") {
		t.Error("missing heading")
	}
	if !strings.Contains(normalized, "This is the body paragraph.") {
		t.Error("missing body paragraph")
	}
	if !strings.Contains(normalized, "    code := \"example\"") {
		t.Error("missing code block")
	}
}
