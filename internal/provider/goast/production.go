// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

import (
	"go/doc/comment"
	"strings"

	"github.com/NobleFactor/noblefactor-ops/internal/provider/goast/doctaxonomy"
)

// Production transforms a slice of comment blocks according to a schema element.
// It consumes blocks starting at cursor, produces output blocks, and returns the
// new cursor position.
type Production interface {
	Execute(blocks []comment.Block, cursor int, elem doctaxonomy.SchemaElement, ctx styleContext) (output []comment.Block, next int)
}

// MultiCommentProduction is an optional interface for productions that emit additional
// comments beyond the primary one. After Execute, styleDoc checks for this interface
// and appends extra DocComments to its return slice.
type MultiCommentProduction interface {
	ExtraComments() []DocComment
}

// blockTypeName returns the type name of a comment.Block for matching against Consumes.Types.
func blockTypeName(b comment.Block) string {
	switch b.(type) {
	case *comment.Paragraph:
		return "Paragraph"
	case *comment.Heading:
		return "Heading"
	case *comment.Code:
		return "Code"
	case *comment.List:
		return "List"
	default:
		return ""
	}
}

// itemProduction consumes zero or more blocks of specified types, optionally matching a prefix.
type itemProduction struct {
	consumes Consumes
}

// Execute scans blocks from cursor, consuming those whose type matches the consumes spec.
// If a prefix is specified on the schema element, only the first matching block must have
// that prefix. Returns consumed blocks and the new cursor.
func (p *itemProduction) Execute(blocks []comment.Block, cursor int, elem doctaxonomy.SchemaElement, ctx styleContext) ([]comment.Block, int) {
	var output []comment.Block
	pos := cursor
	count := 0

	for pos < len(blocks) {
		if p.consumes.Max >= 0 && count >= p.consumes.Max {
			break
		}

		b := blocks[pos]
		if !p.consumes.Matches(blockTypeName(b)) {
			break
		}

		// Prefix check: for single-match (Max=1), only check the first block.
		// For multi-match (Max=-1), check every block.
		if elem.Prefix != "" {
			if !blockMatchesPrefix(b, elem.Prefix, ctx) {
				break
			}
		}

		// Sentence splitting: extract first sentence, replace block with remainder.
		if count == 0 && elem.Split == "sentence" {
			text := blockText(b)
			if text != "" {
				summaryText, remainderText := splitSentence(text)
				if remainderText != "" {
					output = append(output, &comment.Paragraph{
						Text: []comment.Text{comment.Plain(summaryText)},
					})
					blocks[pos] = &comment.Paragraph{
						Text: []comment.Text{comment.Plain(remainderText)},
					}
					count++
					continue
				}
			}
		}

		output = append(output, b)
		pos++
		count++
	}

	// If required and nothing matched, emit a stub.
	if count == 0 && elem.Required == "true" {
		stub := makeStubParagraph(ctx.name, elem)
		output = append(output, stub)
	}

	return output, pos
}

// nilProduction consumes matching blocks and emits nothing. Used for comment removal.
type nilProduction struct {
	consumes Consumes
}

// Execute advances past all matching blocks and returns empty output.
func (p *nilProduction) Execute(blocks []comment.Block, cursor int, _ doctaxonomy.SchemaElement, _ styleContext) ([]comment.Block, int) {
	i := cursor
	for i < len(blocks) && p.consumes.Matches(blockTypeName(blocks[i])) {
		i++
	}
	return nil, i
}

// resizeProduction consumes matching blocks, detects delineator form (line/banner),
// applies a target style, and resizes to the configured line width.
type resizeProduction struct {
	consumes Consumes
}

// Execute processes delineator blocks: detects form, applies style, resizes.
func (p *resizeProduction) Execute(blocks []comment.Block, cursor int, elem doctaxonomy.SchemaElement, ctx styleContext) ([]comment.Block, int) {
	var output []comment.Block
	pos := cursor

	// Content width is lineWidth minus "// " prefix (3 chars).
	contentWidth := ctx.lineWidth - 3
	if contentWidth < 10 {
		contentWidth = 77 // fallback
	}

	// Parse per-form styles from elem.Style.
	lineStyle, bannerStyle, _ := parseFormStyles(elem.Style)

	for pos < len(blocks) && p.consumes.Matches(blockTypeName(blocks[pos])) {
		para, ok := blocks[pos].(*comment.Paragraph)
		if !ok {
			// Non-paragraph blocks pass through unchanged.
			output = append(output, blocks[pos])
			pos++
			continue
		}

		text := paragraphPlainText(para)
		resized := resizeDelineatorText(text, contentWidth, lineStyle, bannerStyle)
		output = append(output, &comment.Paragraph{
			Text: []comment.Text{comment.Plain(resized)},
		})
		pos++
	}

	return output, pos
}

// parseFormStyles parses the Style field into per-form BoxStyle pointers.
// Formats:
//
//	"double"                                → all forms use "double"
//	"line:ascii-=,banner:heavy,box:double"  → per-form styles
//
// Returns nil for any form not specified (preserve original).
func parseFormStyles(style string) (line, banner, box *BoxStyle) {
	if style == "" {
		return nil, nil, nil
	}

	// Check for per-form syntax (contains ":").
	if strings.Contains(style, ":") {
		for _, part := range strings.Split(style, ",") {
			part = strings.TrimSpace(part)
			kv := strings.SplitN(part, ":", 2)
			if len(kv) != 2 {
				continue
			}
			s := LookupBoxStyle(strings.TrimSpace(kv[1]))
			switch strings.TrimSpace(kv[0]) {
			case "line":
				line = s
			case "banner":
				banner = s
			case "box":
				box = s
			}
		}
		return
	}

	// Bare name: applies to all forms.
	s := LookupBoxStyle(style)
	return s, s, s
}

// resizeDelineatorText resizes delineator text to the target width.
// Handles multi-line text (box comments) by processing each line independently.
func resizeDelineatorText(text string, width int, lineStyle, bannerStyle *BoxStyle) string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, "")
			continue
		}

		runes := []rune(trimmed)
		first := runes[0]

		if isDelineatorRune(first) && isPureRepeatedLine(runes, first) {
			// Pure repeated line.
			ch := first
			if lineStyle != nil {
				ch = lineStyle.Horizontal
			}
			result = append(result, string(repeatRune(ch, width)))
		} else if isDelineatorRune(first) && isBannerLine(runes, first) {
			// Centered-text banner.
			content := extractBannerText(runes, first)
			ch := first
			if bannerStyle != nil {
				ch = bannerStyle.Horizontal
			}
			result = append(result, buildBanner(content, ch, width))
		} else {
			// Non-delineator line (content in a box) — pass through.
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}

// isPureRepeatedLine checks if all runes are the same.
func isPureRepeatedLine(runes []rune, filler rune) bool {
	for _, r := range runes {
		if r != filler {
			return false
		}
	}
	return true
}

// isBannerLine checks if runes form a centered-text banner with the given filler.
func isBannerLine(runes []rune, filler rune) bool {
	return isCenteredBanner(runes, filler)
}

// extractBannerText extracts the text content from between leading and trailing filler runs.
func extractBannerText(runes []rune, filler rune) string {
	n := len(runes)
	lead := 0
	for lead < n && runes[lead] == filler {
		lead++
	}
	trail := 0
	for trail < n-lead && runes[n-1-trail] == filler {
		trail++
	}
	return strings.TrimSpace(string(runes[lead : n-trail]))
}

// buildBanner constructs a centered-text banner with the given filler and width.
func buildBanner(text string, filler rune, width int) string {
	textLen := len([]rune(text))
	// " text " takes textLen + 2 spaces.
	remaining := width - textLen - 2
	if remaining < 6 {
		// Not enough room — just fill.
		return string(repeatRune(filler, width))
	}
	left := remaining / 2
	right := remaining - left
	return string(repeatRune(filler, left)) + " " + text + " " + string(repeatRune(filler, right))
}

// repeatRune creates a slice of n copies of r.
func repeatRune(r rune, n int) []rune {
	if n <= 0 {
		return nil
	}
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return out
}

// regionProduction consumes delineator blocks, extracts the section name, and emits
// a "region Name" paragraph. The endregion comment is returned via ExtraComments.
type regionProduction struct {
	consumes Consumes
	extra    []DocComment
}

// Execute extracts the section name from delineator text and emits "region Name".
func (p *regionProduction) Execute(blocks []comment.Block, cursor int, _ doctaxonomy.SchemaElement, _ styleContext) ([]comment.Block, int) {
	// Consume all matching blocks, collecting text.
	var allText []string
	pos := cursor
	for pos < len(blocks) && p.consumes.Matches(blockTypeName(blocks[pos])) {
		if para, ok := blocks[pos].(*comment.Paragraph); ok {
			allText = append(allText, paragraphPlainText(para))
		}
		pos++
	}

	name := extractSectionName(allText)

	// Primary output: region comment.
	regionText := "region " + name
	output := []comment.Block{
		&comment.Paragraph{Text: []comment.Text{comment.Plain(regionText)}},
	}

	// Extra output: endregion comment with TODO.
	endregionText := "endregion " + name + "  TODO(go-style): move to end of section"
	p.extra = []DocComment{{
		doc:     &comment.Doc{Content: []comment.Block{&comment.Paragraph{Text: []comment.Text{comment.Plain(endregionText)}}}},
		present: true,
		style:   StyleRegionMarker,
	}}

	return output, pos
}

// ExtraComments returns the endregion DocComment for insertion after the primary comment.
func (p *regionProduction) ExtraComments() []DocComment {
	return p.extra
}

// extractSectionName extracts a human-readable section name from delineator text.
func extractSectionName(texts []string) string {
	for _, text := range texts {
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			runes := []rune(trimmed)
			first := runes[0]

			if isDelineatorRune(first) {
				// Check for banner: extract text between filler runs.
				if isCenteredBanner(runes, first) {
					name := extractBannerText(runes, first)
					if name != "" {
						return name
					}
				}
				// Pure repeated line — skip.
				continue
			}

			// Non-delineator line — this is the content (box comment text).
			return trimmed
		}
	}
	return "TODO(go-style): add section name"
}

// listProduction consumes an optional heading paragraph followed by a list block.
// Slot filling is deferred to Step 8c.
type listProduction struct {
	consumes Consumes
}

// Execute scans for a heading paragraph matching the schema's Header field, followed by a List.
// If condition is specified and not met, skips entirely. Slot filling is a placeholder until Step 8c.
func (p *listProduction) Execute(blocks []comment.Block, cursor int, elem doctaxonomy.SchemaElement, ctx styleContext) ([]comment.Block, int) {
	// Check condition.
	if elem.Condition != "" && !evaluateCondition(elem.Condition, ctx) {
		return nil, cursor
	}

	var output []comment.Block
	pos := cursor

	// Look for heading paragraph.
	if pos < len(blocks) {
		if para, ok := blocks[pos].(*comment.Paragraph); ok {
			if paragraphTextStartsWith(para, elem.Header) {
				output = append(output, para)
				pos++
			}
		}
	}

	// Look for list.
	if pos < len(blocks) {
		if list, ok := blocks[pos].(*comment.List); ok {
			output = append(output, list)
			pos++
		}
	}

	// If we found nothing and the element is required, emit stubs.
	if len(output) == 0 && (elem.Required == "true" || elem.Required == "if_condition") {
		output = append(output, makeHeaderParagraph(elem.Header))
		output = append(output, makeStubList(ctx, elem))
	}

	return output, pos
}

// evaluateCondition checks a named condition against the style context.
func evaluateCondition(cond string, ctx styleContext) bool {
	switch cond {
	case "params":
		return len(ctx.paramNames) > 0
	case "returns":
		return len(ctx.returnTypes) > 0
	case "exported":
		return len(ctx.name) > 0 && ctx.name[0] >= 'A' && ctx.name[0] <= 'Z'
	case "receiver":
		// Would need receiver info in styleContext — not yet available.
		return false
	default:
		return false
	}
}

// blockMatchesPrefix checks if a block's text starts with the given prefix pattern.
// Currently does exact prefix matching. Fuzzy matching deferred to Step 8b.
func blockMatchesPrefix(b comment.Block, prefix string, ctx styleContext) bool {
	para, ok := b.(*comment.Paragraph)
	if !ok {
		return false
	}
	text := paragraphPlainText(para)
	target := expandPrefix(prefix, ctx)
	return len(text) >= len(target) && text[:len(target)] == target
}

// expandPrefix substitutes {name} in a prefix pattern.
func expandPrefix(prefix string, ctx styleContext) string {
	if ctx.name != "" {
		return replaceAll(prefix, "{name}", ctx.name)
	}
	return prefix
}

// replaceAll is strings.ReplaceAll without importing strings (already imported in sourcefile.go).
func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// paragraphPlainText extracts the plain text from a paragraph, joining all Text elements.
func paragraphPlainText(p *comment.Paragraph) string {
	var result string
	for _, t := range p.Text {
		switch v := t.(type) {
		case comment.Plain:
			result += string(v)
		case comment.Italic:
			result += string(v)
		}
	}
	return result
}

// paragraphTextStartsWith checks if a paragraph's plain text starts with the given string.
func paragraphTextStartsWith(p *comment.Paragraph, prefix string) bool {
	if prefix == "" {
		return false
	}
	text := paragraphPlainText(p)
	return len(text) >= len(prefix) && text[:len(prefix)] == prefix
}

// blockText extracts the plain text from any block that has text content.
func blockText(b comment.Block) string {
	switch v := b.(type) {
	case *comment.Paragraph:
		return paragraphPlainText(v)
	case *comment.Heading:
		return paragraphPlainText(&comment.Paragraph{Text: v.Text})
	default:
		return ""
	}
}

// splitSentence splits text at the first sentence boundary (". " or ".\n").
// Returns the first sentence and the remainder. If there's only one sentence,
// remainder is empty.
func splitSentence(text string) (string, string) {
	for i := 0; i < len(text)-1; i++ {
		if text[i] == '.' && (text[i+1] == ' ' || text[i+1] == '\n') {
			summary := strings.TrimSpace(text[:i+1])
			remainder := strings.TrimSpace(text[i+1:])
			return summary, remainder
		}
	}
	return text, ""
}

// makeStubParagraph creates a TODO stub paragraph for a missing required element.
func makeStubParagraph(name string, elem doctaxonomy.SchemaElement) *comment.Paragraph {
	text := name + " TODO(go-style): add summary"
	return &comment.Paragraph{
		Text: []comment.Text{comment.Plain(text)},
	}
}

// makeHeaderParagraph creates a paragraph containing just a section header (e.g., "Parameters:").
func makeHeaderParagraph(header string) *comment.Paragraph {
	return &comment.Paragraph{
		Text: []comment.Text{comment.Plain(header)},
	}
}

// makeStubList creates a stub list with TODO items for each slot name.
func makeStubList(ctx styleContext, elem doctaxonomy.SchemaElement) *comment.List {
	var names []string
	switch elem.Slots {
	case "params":
		names = ctx.paramNames
	case "returns":
		names = ctx.returnTypes
	}

	list := &comment.List{}
	for _, name := range names {
		item := &comment.ListItem{
			Content: []comment.Block{
				&comment.Paragraph{
					Text: []comment.Text{
						comment.Plain(name + ": TODO(go-style): add description"),
					},
				},
			},
		}
		list.Items = append(list.Items, item)
	}
	return list
}

// ProductionFactory creates a Production from a parsed Consumes spec and the full SchemaElement.
type ProductionFactory func(consumes Consumes, elem doctaxonomy.SchemaElement) Production

var productions = map[string]ProductionFactory{}

// RegisterProduction adds a named production type to the registry.
func RegisterProduction(name string, factory ProductionFactory) {
	productions[name] = factory
}

func init() {
	RegisterProduction("item", func(c Consumes, _ doctaxonomy.SchemaElement) Production {
		return &itemProduction{consumes: c}
	})
	RegisterProduction("list", func(c Consumes, _ doctaxonomy.SchemaElement) Production {
		return &listProduction{consumes: c}
	})
	RegisterProduction("nil", func(c Consumes, _ doctaxonomy.SchemaElement) Production {
		return &nilProduction{consumes: c}
	})
	RegisterProduction("resize", func(c Consumes, _ doctaxonomy.SchemaElement) Production {
		return &resizeProduction{consumes: c}
	})
	RegisterProduction("region", func(c Consumes, _ doctaxonomy.SchemaElement) Production {
		return &regionProduction{consumes: c}
	})
}

// NewProduction creates a Production from a schema element via the production registry.
func NewProduction(elem doctaxonomy.SchemaElement) (Production, error) {
	consumesStr := elem.Consumes
	if consumesStr == "" {
		// Default consumes based on legacy type field.
		switch elem.Type {
		case "paragraph":
			consumesStr = "Paragraph / Heading"
		case "block":
			consumesStr = "*(Paragraph / Code / Heading)"
		case "section":
			consumesStr = "List"
		case "directive":
			consumesStr = "*Paragraph"
		default:
			consumesStr = "Paragraph"
		}
	}

	c, err := ParseConsumes(consumesStr)
	if err != nil {
		return nil, err
	}

	prod := elem.Production
	if prod == "" {
		// Default production based on legacy type field.
		switch elem.Type {
		case "section":
			prod = "list"
		default:
			prod = "item"
		}
	}

	factory, ok := productions[prod]
	if !ok {
		factory = productions["item"]
	}
	return factory(c, elem), nil
}
