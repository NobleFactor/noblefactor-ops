// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

import (
	"go/doc/comment"
	"strings"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/provider/goast/doctaxonomy"
)

func makeParagraph(text string) *comment.Paragraph {
	return &comment.Paragraph{Text: []comment.Text{comment.Plain(text)}}
}

func makeCode(text string) *comment.Code {
	return &comment.Code{Text: text}
}

func makeHeading(text string) *comment.Heading {
	return &comment.Heading{Text: []comment.Text{comment.Plain(text)}}
}

func makeList(items ...string) *comment.List {
	list := &comment.List{}
	for _, item := range items {
		list.Items = append(list.Items, &comment.ListItem{
			Content: []comment.Block{makeParagraph(item)},
		})
	}
	return list
}

// --- itemProduction tests ---

func TestItemProduction_SingleParagraph(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("Backup creates a backup."),
		makeParagraph("Extended description."),
	}

	elem := doctaxonomy.SchemaElement{Name: "summary", Consumes: "Paragraph / Heading"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{name: "Backup"})
	if len(output) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(output))
	}
	if next != 1 {
		t.Errorf("expected cursor at 1, got %d", next)
	}
}

func TestItemProduction_ZeroOrMore(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("First paragraph."),
		makeCode("code block"),
		makeParagraph("Second paragraph."),
		makeList("not consumed"),
	}

	elem := doctaxonomy.SchemaElement{Name: "description", Consumes: "*(Paragraph / Code)"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if len(output) != 3 {
		t.Fatalf("expected 3 output blocks, got %d", len(output))
	}
	if next != 3 {
		t.Errorf("expected cursor at 3, got %d", next)
	}
}

func TestItemProduction_ZeroOrMore_Empty(t *testing.T) {
	blocks := []comment.Block{
		makeList("not a paragraph"),
	}

	elem := doctaxonomy.SchemaElement{Name: "description", Consumes: "*(Paragraph / Code)"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if len(output) != 0 {
		t.Fatalf("expected 0 output blocks, got %d", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor at 0, got %d", next)
	}
}

func TestItemProduction_RequiredMissing(t *testing.T) {
	blocks := []comment.Block{
		makeList("not a paragraph"),
	}

	elem := doctaxonomy.SchemaElement{Name: "summary", Required: "true", Consumes: "Paragraph / Heading"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{name: "Backup"})
	if len(output) != 1 {
		t.Fatalf("expected 1 stub block, got %d", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor unchanged at 0, got %d", next)
	}
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	if text != "Backup TODO(go-style): add summary" {
		t.Errorf("unexpected stub: %q", text)
	}
}

func TestItemProduction_PrefixMatch(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("Backup creates a copy."),
		makeParagraph("Other text."),
	}

	elem := doctaxonomy.SchemaElement{Name: "summary", Prefix: "{name}", Consumes: "Paragraph / Heading"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{name: "Backup"})
	if len(output) != 1 {
		t.Fatalf("expected 1 block, got %d", len(output))
	}
	if next != 1 {
		t.Errorf("expected cursor at 1, got %d", next)
	}
}

func TestItemProduction_PrefixNoMatch(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("creates a copy."),
	}

	elem := doctaxonomy.SchemaElement{Name: "summary", Prefix: "{name}", Consumes: "Paragraph / Heading"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{name: "Backup"})
	if len(output) != 0 {
		t.Fatalf("expected 0 blocks (prefix mismatch), got %d", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor unchanged, got %d", next)
	}
}

func TestItemProduction_DirectivePrefix(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("+devlore:defaults overwrite=true"),
		makeParagraph("+devlore:access=both"),
		makeParagraph("Not a directive."),
	}

	elem := doctaxonomy.SchemaElement{Name: "directives", Prefix: "+", Consumes: "*Paragraph"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if len(output) != 2 {
		t.Fatalf("expected 2 directive blocks, got %d", len(output))
	}
	if next != 2 {
		t.Errorf("expected cursor at 2, got %d", next)
	}
}

// --- listProduction tests ---

func TestListProduction_HeadingAndList(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("Parameters:"),
		makeList("path: the file path", "name: the name"),
	}

	elem := doctaxonomy.SchemaElement{
		Name:       "parameters",
		Production: "list",
		Header:     "Parameters:",
		Condition:  "params",
		Consumes:   "List",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	ctx := styleContext{name: "Backup", paramNames: []string{"path", "name"}}
	output, next := prod.Execute(blocks, 0, elem, ctx)
	if len(output) != 2 {
		t.Fatalf("expected 2 blocks (heading + list), got %d", len(output))
	}
	if next != 2 {
		t.Errorf("expected cursor at 2, got %d", next)
	}
}

func TestListProduction_ConditionFalse(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("Parameters:"),
		makeList("path: the file path"),
	}

	elem := doctaxonomy.SchemaElement{
		Name:       "parameters",
		Production: "list",
		Header:     "Parameters:",
		Condition:  "params",
		Consumes:   "List",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	ctx := styleContext{name: "Backup"} // no params
	output, next := prod.Execute(blocks, 0, elem, ctx)
	if len(output) != 0 {
		t.Fatalf("expected 0 blocks (condition false), got %d", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor unchanged, got %d", next)
	}
}

func TestListProduction_RequiredStub(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("Some other text."),
	}

	elem := doctaxonomy.SchemaElement{
		Name:       "parameters",
		Production: "list",
		Header:     "Parameters:",
		Condition:  "params",
		Required:   "if_condition",
		Slots:      "params",
		Consumes:   "List",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	ctx := styleContext{name: "Backup", paramNames: []string{"path", "suffix"}}
	output, next := prod.Execute(blocks, 0, elem, ctx)
	if len(output) != 2 {
		t.Fatalf("expected 2 stub blocks (heading + list), got %d", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor unchanged (stubs inserted, input not consumed), got %d", next)
	}

	// Verify stub list has items for each param.
	list, ok := output[1].(*comment.List)
	if !ok {
		t.Fatal("expected List as second output block")
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 list items, got %d", len(list.Items))
	}
}

// --- sentence splitting tests ---

func TestItemProduction_SplitSentence(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("NewAccessor creates a ConfigAccessor. The value should be a struct."),
		makeList("not consumed"),
	}

	elem := doctaxonomy.SchemaElement{
		Name:     "summary",
		Consumes: "Paragraph / Heading",
		Prefix:   "{name}",
		Split:    "sentence",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{name: "NewAccessor"})
	if len(output) != 1 {
		t.Fatalf("expected 1 output block (summary), got %d", len(output))
	}

	summaryText := paragraphPlainText(output[0].(*comment.Paragraph))
	if summaryText != "NewAccessor creates a ConfigAccessor." {
		t.Errorf("summary = %q, want first sentence only", summaryText)
	}

	// The remainder should replace the original block for body to consume.
	if next != 0 {
		t.Errorf("expected cursor at 0 (remainder replaces block), got %d", next)
	}
	remainderText := paragraphPlainText(blocks[0].(*comment.Paragraph))
	if remainderText != "The value should be a struct." {
		t.Errorf("remainder = %q, want second sentence", remainderText)
	}
}

func TestItemProduction_SplitSentence_SingleSentence(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("NewAccessor creates a ConfigAccessor."),
	}

	elem := doctaxonomy.SchemaElement{
		Name:     "summary",
		Consumes: "Paragraph / Heading",
		Prefix:   "{name}",
		Split:    "sentence",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{name: "NewAccessor"})
	if len(output) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(output))
	}
	if next != 1 {
		t.Errorf("expected cursor at 1 (no remainder), got %d", next)
	}
}

func TestListProduction_SingleParamStub(t *testing.T) {
	// No heading, no list — just a paragraph that doesn't match the header.
	blocks := []comment.Block{
		makeParagraph("Unrelated text."),
	}

	elem := doctaxonomy.SchemaElement{
		Name:       "parameters",
		Production: "list",
		Header:     "Parameters:",
		Condition:  "params",
		Required:   "if_condition",
		Slots:      "params",
		Consumes:   "List",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	ctx := styleContext{name: "NewAccessor", paramNames: []string{"v"}}
	output, next := prod.Execute(blocks, 0, elem, ctx)
	if len(output) != 2 {
		t.Fatalf("expected 2 stub blocks (heading + list), got %d", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor unchanged (stubs inserted, input not consumed), got %d", next)
	}

	// Verify stub list has one item.
	list, ok := output[1].(*comment.List)
	if !ok {
		t.Fatal("expected List as second output block")
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 list item, got %d", len(list.Items))
	}
}

// --- nilProduction tests ---

func TestNilProduction_ConsumesAll(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("============================================================================="),
		makeParagraph("Section Name"),
		makeParagraph("============================================================================="),
	}

	elem := doctaxonomy.SchemaElement{
		Name:       "content",
		Production: "nil",
		Consumes:   "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}
	if _, ok := prod.(*nilProduction); !ok {
		t.Fatal("expected nilProduction")
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if len(output) != 0 {
		t.Errorf("expected empty output, got %d blocks", len(output))
	}
	if next != 3 {
		t.Errorf("expected cursor at 3 (all consumed), got %d", next)
	}
}

func TestNilProduction_EmptyInput(t *testing.T) {
	var blocks []comment.Block

	elem := doctaxonomy.SchemaElement{
		Name:       "content",
		Production: "nil",
		Consumes:   "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if len(output) != 0 {
		t.Errorf("expected empty output, got %d blocks", len(output))
	}
	if next != 0 {
		t.Errorf("expected cursor at 0, got %d", next)
	}
}

func TestNilProduction_StopsAtNonMatching(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("first"),
		makeParagraph("second"),
		makeCode("code block"),
	}

	// Only consumes Paragraphs, not Code.
	elem := doctaxonomy.SchemaElement{
		Name:       "content",
		Production: "nil",
		Consumes:   "*Paragraph",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if len(output) != 0 {
		t.Errorf("expected empty output, got %d blocks", len(output))
	}
	if next != 2 {
		t.Errorf("expected cursor at 2 (stopped at Code), got %d", next)
	}
}

func TestNilProduction_FromCursor(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("first"),
		makeParagraph("second"),
		makeParagraph("third"),
	}

	elem := doctaxonomy.SchemaElement{
		Name:       "content",
		Production: "nil",
		Consumes:   "*Paragraph",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 1, elem, styleContext{})
	if len(output) != 0 {
		t.Errorf("expected empty output, got %d blocks", len(output))
	}
	if next != 3 {
		t.Errorf("expected cursor at 3, got %d", next)
	}
}

// --- resizeProduction tests ---

func TestResizeProduction_PureLine_PreserveChar(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("====="),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "resize",
		Consumes: "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{lineWidth: 80})
	if next != 1 {
		t.Fatalf("expected cursor at 1, got %d", next)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 block, got %d", len(output))
	}
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	// 80 - 3 = 77 characters of '='
	if len(text) != 77 {
		t.Errorf("expected 77 chars, got %d: %q", len(text), text)
	}
	for _, r := range text {
		if r != '=' {
			t.Errorf("expected all '=', got %q", text)
			break
		}
	}
}

func TestResizeProduction_PureLine_WithStyle(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("-----"),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "resize",
		Consumes: "*(Paragraph / Code / Heading / List)",
		Style:    "double",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, _ := prod.Execute(blocks, 0, elem, styleContext{lineWidth: 20})
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	// 20 - 3 = 17 chars of '═'
	expected := "═════════════════"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestResizeProduction_Banner_PreserveChar(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("=== Section Name ==="),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "resize",
		Consumes: "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, _ := prod.Execute(blocks, 0, elem, styleContext{lineWidth: 80})
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	// Should contain "Section Name" centered in '=' chars, total 77 chars.
	if len([]rune(text)) != 77 {
		t.Errorf("expected 77 runes, got %d: %q", len([]rune(text)), text)
	}
	if !strings.Contains(text, "Section Name") {
		t.Errorf("expected 'Section Name' in output: %q", text)
	}
	if text[0] != '=' {
		t.Errorf("expected leading '=', got %q", text)
	}
}

func TestResizeProduction_Banner_WithStyle(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("--- Helpers ---"),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "resize",
		Consumes: "*(Paragraph / Code / Heading / List)",
		Style:    "banner:heavy",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, _ := prod.Execute(blocks, 0, elem, styleContext{lineWidth: 40})
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	// Should contain "Helpers" centered in '━' chars, total 37 chars.
	if !strings.Contains(text, "Helpers") {
		t.Errorf("expected 'Helpers' in output: %q", text)
	}
	if []rune(text)[0] != '━' {
		t.Errorf("expected leading '━', got %c in %q", []rune(text)[0], text)
	}
}

func TestResizeProduction_BoxComment(t *testing.T) {
	// go/doc/comment preserves newlines in merged paragraph.
	blocks := []comment.Block{
		makeParagraph("=============\nSection Name\n============="),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "resize",
		Consumes: "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, _ := prod.Execute(blocks, 0, elem, styleContext{lineWidth: 40})
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	lines := strings.Split(text, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), text)
	}
	// First and last lines should be 37 '=' chars.
	if len(lines[0]) != 37 {
		t.Errorf("border line length: expected 37, got %d: %q", len(lines[0]), lines[0])
	}
	// Content line preserved.
	if lines[1] != "Section Name" {
		t.Errorf("content line: expected 'Section Name', got %q", lines[1])
	}
}

func TestResizeProduction_PerFormStyle(t *testing.T) {
	// Line with '=' and banner with '-' in same comment won't happen in practice,
	// but test per-form style parsing.
	elem := doctaxonomy.SchemaElement{
		Style: "line:double,banner:heavy,box:light",
	}
	lineS, bannerS, boxS := parseFormStyles(elem.Style)
	if lineS == nil || lineS.Name != "double" {
		t.Errorf("line style: expected 'double', got %v", lineS)
	}
	if bannerS == nil || bannerS.Name != "heavy" {
		t.Errorf("banner style: expected 'heavy', got %v", bannerS)
	}
	if boxS == nil || boxS.Name != "light" {
		t.Errorf("box style: expected 'light', got %v", boxS)
	}
}

func TestResizeProduction_BareStyle(t *testing.T) {
	lineS, bannerS, boxS := parseFormStyles("rounded")
	if lineS == nil || lineS.Name != "rounded" {
		t.Errorf("line style: expected 'rounded', got %v", lineS)
	}
	if bannerS == nil || bannerS.Name != "rounded" {
		t.Errorf("banner style: expected 'rounded', got %v", bannerS)
	}
	if boxS == nil || boxS.Name != "rounded" {
		t.Errorf("box style: expected 'rounded', got %v", boxS)
	}
}

func TestResizeProduction_EmptyStyle(t *testing.T) {
	lineS, bannerS, boxS := parseFormStyles("")
	if lineS != nil || bannerS != nil || boxS != nil {
		t.Error("empty style should return all nil")
	}
}

// --- regionProduction tests ---

func TestRegionProduction_Banner(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("=== Public API ==="),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "region",
		Consumes: "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, next := prod.Execute(blocks, 0, elem, styleContext{})
	if next != 1 {
		t.Fatalf("expected cursor at 1, got %d", next)
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 block, got %d", len(output))
	}
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	if text != "region Public API" {
		t.Errorf("expected 'region Public API', got %q", text)
	}

	// Check extra comments.
	mcp, ok := prod.(MultiCommentProduction)
	if !ok {
		t.Fatal("expected MultiCommentProduction interface")
	}
	extras := mcp.ExtraComments()
	if len(extras) != 1 {
		t.Fatalf("expected 1 extra comment, got %d", len(extras))
	}
	endText := paragraphPlainText(extras[0].doc.Content[0].(*comment.Paragraph))
	if !strings.Contains(endText, "endregion Public API") {
		t.Errorf("expected endregion with section name, got %q", endText)
	}
	if !strings.Contains(endText, "TODO(go-style)") {
		t.Errorf("expected TODO marker, got %q", endText)
	}
}

func TestRegionProduction_BoxComment(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("=============\nSection Name\n============="),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "region",
		Consumes: "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, _ := prod.Execute(blocks, 0, elem, styleContext{})
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	if text != "region Section Name" {
		t.Errorf("expected 'region Section Name', got %q", text)
	}
}

func TestRegionProduction_PureLine(t *testing.T) {
	blocks := []comment.Block{
		makeParagraph("============================================================================="),
	}
	elem := doctaxonomy.SchemaElement{
		Name: "content", Production: "region",
		Consumes: "*(Paragraph / Code / Heading / List)",
	}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}

	output, _ := prod.Execute(blocks, 0, elem, styleContext{})
	text := paragraphPlainText(output[0].(*comment.Paragraph))
	if !strings.Contains(text, "region TODO(go-style)") {
		t.Errorf("expected TODO placeholder name, got %q", text)
	}
}

func TestExtractSectionName(t *testing.T) {
	tests := []struct {
		name  string
		texts []string
		want  string
	}{
		{"banner", []string{"=== Public API ==="}, "Public API"},
		{"box", []string{"=============\nSection Name\n============="}, "Section Name"},
		{"pure line", []string{"============="}, "TODO(go-style): add section name"},
		{"multiple paragraphs", []string{"=====", "Content Here"}, "Content Here"},
		{"empty", []string{}, "TODO(go-style): add section name"},
		{"dashes banner", []string{"--- Helpers ---"}, "Helpers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSectionName(tt.texts)
			if got != tt.want {
				t.Errorf("extractSectionName(%v) = %q, want %q", tt.texts, got, tt.want)
			}
		})
	}
}

// --- NewProduction from legacy type field ---

func TestNewProduction_LegacyParagraph(t *testing.T) {
	elem := doctaxonomy.SchemaElement{Type: "paragraph"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}
	if _, ok := prod.(*itemProduction); !ok {
		t.Error("expected itemProduction for paragraph type")
	}
}

func TestNewProduction_LegacySection(t *testing.T) {
	elem := doctaxonomy.SchemaElement{Type: "section"}
	prod, err := NewProduction(elem)
	if err != nil {
		t.Fatalf("NewProduction: %v", err)
	}
	if _, ok := prod.(*listProduction); !ok {
		t.Error("expected listProduction for section type")
	}
}
