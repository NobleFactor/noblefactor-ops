// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"fmt"
	"strings"
)

// Normalize joins words into a single unwrapped line, handling punctuation
// that was split into separate tokens by the lexer (e.g., ParamName followed
// by a period).
func (p *Paragraph) Normalize() string {
	var b strings.Builder
	for i, w := range p.Words {
		if i > 0 && !isTrailingPunctuation(w) {
			b.WriteByte(' ')
		}
		b.WriteString(w)
	}
	return b.String()
}

// isTrailingPunctuation returns true for single-character tokens that should
// be joined to the preceding word without a space.
func isTrailingPunctuation(s string) bool {
	if len(s) != 1 {
		return false
	}
	return strings.ContainsAny(s, ".,;:!?)]}")
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

// Normalize assembles elements in schema order with correct blank-line
// separators. Order: summary, body paragraphs/headings/code, directives,
// parameters, returns.
func (d *FuncDoc) Normalize() string {
	var paragraphs []*Paragraph
	var directives []*Directive
	var paramSection *ParamSection
	var returnSection *ReturnSection

	for _, el := range d.Elements {
		switch {
		case el.Paragraph != nil:
			paragraphs = append(paragraphs, el.Paragraph)
		case el.Directive != nil:
			directives = append(directives, el.Directive)
		case el.ParamSection != nil:
			paramSection = el.ParamSection
		case el.ReturnSection != nil:
			returnSection = el.ReturnSection
		}
	}

	var sections []string

	// 1. Summary — first paragraph.
	if len(paragraphs) > 0 {
		sections = append(sections, paragraphs[0].Normalize())
	}

	// 2. Body — remaining paragraphs, headings, code blocks (in parse order).
	// Track which body paragraphs we've emitted.
	bodyIdx := 0
	for _, el := range d.Elements {
		switch {
		case el.Heading != nil:
			sections = append(sections, el.Heading.Normalize())
		case el.CodeBlock != nil:
			sections = append(sections, el.CodeBlock.Normalize())
		case el.Paragraph != nil:
			// Skip the first paragraph (already emitted as summary).
			if bodyIdx > 0 {
				sections = append(sections, el.Paragraph.Normalize())
			}
			bodyIdx++
		}
	}

	// 3. Directives.
	for _, dir := range directives {
		sections = append(sections, dir.Normalize())
	}

	// 4. Parameters section.
	if paramSection != nil {
		sections = append(sections, paramSection.Normalize())
	}

	// 5. Returns section.
	if returnSection != nil {
		sections = append(sections, returnSection.Normalize())
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

// Normalize formats as two lines: SPDX identifier and copyright notice.
func (c *CopyrightDoc) Normalize() string {
	return fmt.Sprintf("SPDX-License-Identifier: %s\nCopyright %s", c.SPDX, strings.Join(c.Copyright, " "))
}
