// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"fmt"
	"sort"
	"strings"
)

// Normalize joins words into a single unwrapped line, handling punctuation
// that was split into separate tokens by the lexer (e.g., ParamName followed
// by a period).
func (p *Paragraph) Normalize() string {
	var b strings.Builder
	var prev string
	for i, w := range p.Words {
		w = strings.TrimLeft(w, " \t")
		if i > 0 && !startsWithClosePunct(w) && !joinsToFollowing(prev) {
			b.WriteByte(' ')
		}
		b.WriteString(w)
		prev = w
	}
	return b.String()
}

// startsWithClosePunct returns true for tokens whose first character is
// closing or trailing punctuation.
func startsWithClosePunct(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.ContainsAny(s[:1], ".,;:!?)]}")
}

// joinsToFollowing returns true for tokens that join to the following word
// without a space.
func joinsToFollowing(s string) bool {
	return s == "::"
}

// Normalize formats as "+key value" on a single line.
func (d *Directive) Normalize() string {
	if len(d.Value) == 0 {
		return "+" + d.Key
	}
	return "+" + d.Key + " " + strings.Join(d.Value, " ")
}

// Normalize formats the section header followed by one indented line per item.
func (s *ParamSection) Normalize() string {
	lines := make([]string, 0, len(s.Items)+1)
	lines = append(lines, "Parameters:")
	for _, item := range s.Items {
		lines = append(lines, "  - "+item.Name+": "+item.Desc.Normalize())
	}
	return strings.Join(lines, "\n")
}

// Normalize formats the section header followed by one indented line per item.
func (s *ReturnSection) Normalize() string {
	lines := make([]string, 0, len(s.Items)+1)
	lines = append(lines, "Returns:")
	for _, item := range s.Items {
		lines = append(lines, "  - "+item.Type+": "+item.Desc.Normalize())
	}
	return strings.Join(lines, "\n")
}

// Normalize returns indented lines verbatim, joined by newlines.
func (c *CodeBlock) Normalize() string {
	return strings.Join(c.Lines, "\n")
}

// Normalize formats as "# Title" on a single line.
func (h *Heading) Normalize() string {
	return "# " + strings.Join(h.Text, " ")
}

// defaultFuncDocOrder is the hardcoded element order used when no schema is
// provided. Matches the Go func_doc schema: summary=1, body=2, parameters=3,
// returns=4, directives=5.
var defaultFuncDocOrder = []SchemaElement{
	{Name: "summary", Type: "paragraph", Order: 1},
	{Name: "body", Type: "block", Cardinality: "*", Order: 2},
	{Name: "parameters", Type: "param_section", Order: 3},
	{Name: "returns", Type: "return_section", Order: 4},
	{Name: "directives", Type: "directive", Cardinality: "*", Order: 5},
}

// Normalize assembles elements in default schema order with correct blank-line
// separators. For schema-driven ordering, use NormalizeWithSchema.
func (d *FuncDoc) Normalize() string {
	return d.NormalizeWithSchema(defaultFuncDocOrder)
}

// NormalizeWithSchema assembles elements in the order defined by the schema
// elements, separated by blank lines.
func (d *FuncDoc) NormalizeWithSchema(schemaElements []SchemaElement) string {
	// Sort schema elements by Order.
	ordered := make([]SchemaElement, len(schemaElements))
	copy(ordered, schemaElements)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Order < ordered[j].Order
	})

	// Classify parsed elements.
	var paragraphs []*Paragraph
	var directives []*Directive
	var paramSection *ParamSection
	var returnSection *ReturnSection

	// bodyElements tracks headings, code blocks, and non-summary paragraphs
	// in parse order.
	type bodyElement struct {
		text string
	}
	var bodyElements []bodyElement

	paragraphIdx := 0
	for _, el := range d.Elements {
		switch {
		case el.Paragraph != nil:
			paragraphs = append(paragraphs, el.Paragraph)
			if paragraphIdx > 0 {
				bodyElements = append(bodyElements, bodyElement{el.Paragraph.Normalize()})
			}
			paragraphIdx++
		case el.Heading != nil:
			bodyElements = append(bodyElements, bodyElement{el.Heading.Normalize()})
		case el.CodeBlock != nil:
			bodyElements = append(bodyElements, bodyElement{el.CodeBlock.Normalize()})
		case el.Directive != nil:
			directives = append(directives, el.Directive)
		case el.ParamSection != nil:
			paramSection = el.ParamSection
		case el.ReturnSection != nil:
			returnSection = el.ReturnSection
		}
	}

	// Emit elements in schema order.
	var sections []string
	for _, se := range ordered {
		switch se.Type {
		case "paragraph":
			if len(paragraphs) > 0 {
				sections = append(sections, paragraphs[0].Normalize())
			}
		case "block":
			for _, be := range bodyElements {
				sections = append(sections, be.text)
			}
		case "directive":
			for _, dir := range directives {
				sections = append(sections, dir.Normalize())
			}
		case "param_section":
			if paramSection != nil {
				sections = append(sections, paramSection.Normalize())
			}
		case "return_section":
			if returnSection != nil {
				sections = append(sections, returnSection.Normalize())
			}
		}
	}

	return strings.Join(sections, "\n\n")
}

// FuncContext provides the function signature context needed to insert stubs
// for missing required elements.
type FuncContext struct {
	Name        string
	ParamNames  []string
	ReturnTypes []string
}

// NormalizeWithContext assembles elements in schema order, inserting TODO stubs
// for any required elements that are missing.
func (d *FuncDoc) NormalizeWithContext(schemaElements []SchemaElement, ctx FuncContext) string {
	ordered := make([]SchemaElement, len(schemaElements))
	copy(ordered, schemaElements)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Order < ordered[j].Order
	})

	// Classify parsed elements.
	var paragraphs []*Paragraph
	var directives []*Directive
	var paramSection *ParamSection
	var returnSection *ReturnSection

	type bodyElement struct {
		text string
	}
	var bodyElements []bodyElement

	paragraphIdx := 0
	for _, el := range d.Elements {
		switch {
		case el.Paragraph != nil:
			paragraphs = append(paragraphs, el.Paragraph)
			if paragraphIdx > 0 {
				bodyElements = append(bodyElements, bodyElement{el.Paragraph.Normalize()})
			}
			paragraphIdx++
		case el.Heading != nil:
			bodyElements = append(bodyElements, bodyElement{el.Heading.Normalize()})
		case el.CodeBlock != nil:
			bodyElements = append(bodyElements, bodyElement{el.CodeBlock.Normalize()})
		case el.Directive != nil:
			directives = append(directives, el.Directive)
		case el.ParamSection != nil:
			paramSection = el.ParamSection
		case el.ReturnSection != nil:
			returnSection = el.ReturnSection
		}
	}

	// Emit elements in schema order, inserting stubs for missing required elements.
	var sections []string
	for _, se := range ordered {
		switch se.Type {
		case "paragraph":
			if len(paragraphs) > 0 {
				sections = append(sections, paragraphs[0].Normalize())
			} else if se.Required == "true" {
				sections = append(sections, ctx.Name+" TODO(go-style): add summary describing what this method does")
			}
		case "block":
			for _, be := range bodyElements {
				sections = append(sections, be.text)
			}
		case "directive":
			for _, dir := range directives {
				sections = append(sections, dir.Normalize())
			}
		case "param_section":
			if paramSection != nil {
				sections = append(sections, paramSection.Normalize())
			} else if se.Required == "if_params" && len(ctx.ParamNames) > 0 {
				lines := []string{"Parameters:"}
				for _, name := range ctx.ParamNames {
					lines = append(lines, "  - "+name+": TODO(go-style): add description for parameter")
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		case "return_section":
			if returnSection != nil {
				sections = append(sections, returnSection.Normalize())
			} else if se.Required == "if_returns" && len(ctx.ReturnTypes) > 0 {
				lines := []string{"Returns:"}
				for _, rt := range ctx.ReturnTypes {
					lines = append(lines, "  - "+rt+": TODO(go-style): add description for return value")
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	return strings.Join(sections, "\n\n")
}

// Normalize assembles type doc elements: summary first, then body in parse
// order.
func (d *TypeDoc) Normalize() string {
	if len(d.Elements) == 0 {
		return ""
	}

	var sections []string

	for _, el := range d.Elements {
		switch {
		case el.Paragraph != nil:
			sections = append(sections, el.Paragraph.Normalize())
		case el.Heading != nil:
			sections = append(sections, el.Heading.Normalize())
		case el.CodeBlock != nil:
			sections = append(sections, el.CodeBlock.Normalize())
		}
	}

	return strings.Join(sections, "\n\n")
}

// NormalizeWithSchema assembles type doc elements in schema order. The summary
// is the first paragraph; remaining paragraphs, headings, and code blocks are
// body elements.
func (d *TypeDoc) NormalizeWithSchema(schemaElements []SchemaElement) string {
	ordered := make([]SchemaElement, len(schemaElements))
	copy(ordered, schemaElements)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Order < ordered[j].Order
	})

	var summary string
	var bodyElements []string

	paragraphIdx := 0
	for _, el := range d.Elements {
		switch {
		case el.Paragraph != nil:
			if paragraphIdx == 0 {
				summary = el.Paragraph.Normalize()
			} else {
				bodyElements = append(bodyElements, el.Paragraph.Normalize())
			}
			paragraphIdx++
		case el.Heading != nil:
			bodyElements = append(bodyElements, el.Heading.Normalize())
		case el.CodeBlock != nil:
			bodyElements = append(bodyElements, el.CodeBlock.Normalize())
		}
	}

	var sections []string
	for _, se := range ordered {
		switch se.Type {
		case "paragraph":
			if summary != "" {
				sections = append(sections, summary)
			}
		case "block":
			for _, be := range bodyElements {
				sections = append(sections, be)
			}
		}
	}

	return strings.Join(sections, "\n\n")
}

// Normalize formats as two lines: SPDX identifier and copyright notice.
func (c *CopyrightDoc) Normalize() string {
	return fmt.Sprintf("SPDX-License-Identifier: %s\nCopyright %s", c.SPDX, strings.Join(c.Copyright, " "))
}
