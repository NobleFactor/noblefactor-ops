// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

// Paragraph represents one or more text tokens (words, param names, return
// types, colons) forming a prose paragraph. CodeLine is included so that
// list item continuation lines (4-space indent) are captured as part of
// the description rather than creating spurious CodeBlock elements.
type Paragraph struct {
	Words []string `parser:"@(Word | ParamName | ReturnType | Colon | CodeLine)+" starlark:"words"`
}

// String returns the normalized text of the paragraph.
func (p *Paragraph) String() string { return p.Normalize() }

// Directive represents a +key value line (e.g., +devlore:defaults overwrite=true).
type Directive struct {
	Key   string   `parser:"DirectiveMark @DirectiveToken" starlark:"key"`
	Value []string `parser:"@DirectiveToken*" starlark:"value"`
}

// String returns the normalized directive text.
func (d *Directive) String() string { return d.Normalize() }

// ParamItem represents a single parameter list entry. Accepts both ParamName
// and Word so that stale parameter names parse without error — validation
// detects the mismatch post-parse.
type ParamItem struct {
	Name string     `parser:"ListMarker @(ParamName | Word) Colon" starlark:"name"`
	Desc *Paragraph `parser:"@@" starlark:"desc"`
}

// String returns "name: desc".
func (i *ParamItem) String() string { return i.Name + ": " + i.Desc.Normalize() }

// ParamSection represents a "Parameters:" section with zero or more items.
type ParamSection struct {
	Items []*ParamItem `parser:"'Parameters' Colon @@*" starlark:"items"`
}

// String returns the normalized section text.
func (s *ParamSection) String() string { return s.Normalize() }

// ReturnItem represents a single return value list entry.
type ReturnItem struct {
	Type string     `parser:"ListMarker @(ReturnType | Word) Colon" starlark:"type"`
	Desc *Paragraph `parser:"@@" starlark:"desc"`
}

// String returns "type: desc".
func (i *ReturnItem) String() string { return i.Type + ": " + i.Desc.Normalize() }

// ReturnSection represents a "Returns:" section with zero or more items.
type ReturnSection struct {
	Items []*ReturnItem `parser:"'Returns' Colon @@*" starlark:"items"`
}

// String returns the normalized section text.
func (s *ReturnSection) String() string { return s.Normalize() }

// CodeBlock represents indented lines passed through verbatim.
type CodeBlock struct {
	Lines []string `parser:"@CodeLine+" starlark:"lines"`
}

// String returns the normalized code block text.
func (c *CodeBlock) String() string { return c.Normalize() }

// Heading represents a markdown-style heading in a doc comment.
type Heading struct {
	Text []string `parser:"HashMark @(Word | ParamName | ReturnType)+" starlark:"text"`
}

// String returns the normalized heading text.
func (h *Heading) String() string { return h.Normalize() }
