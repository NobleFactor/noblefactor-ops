// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

// Paragraph represents one or more text tokens (words, param names, return
// types, colons) forming a prose paragraph.
type Paragraph struct {
	Words []string `parser:"@(Word | ParamName | ReturnType | Colon)+"`
}

// Directive represents a +key value line (e.g., +devlore:defaults overwrite=true).
type Directive struct {
	Key   string   `parser:"DirectiveMark @DirectiveToken"`
	Value []string `parser:"@DirectiveToken*"`
}

// ParamItem represents a single parameter list entry. Accepts both ParamName
// and Word so that stale parameter names parse without error — validation
// detects the mismatch post-parse.
type ParamItem struct {
	Name string     `parser:"ListMarker @(ParamName | Word) Colon"`
	Desc *Paragraph `parser:"@@"`
}

// ParamSection represents a "Parameters:" section with zero or more items.
type ParamSection struct {
	Items []*ParamItem `parser:"'Parameters' Colon @@*"`
}

// ReturnItem represents a single return value list entry.
type ReturnItem struct {
	Type string     `parser:"ListMarker @(ReturnType | Word) Colon"`
	Desc *Paragraph `parser:"@@"`
}

// ReturnSection represents a "Returns:" section with zero or more items.
type ReturnSection struct {
	Items []*ReturnItem `parser:"'Returns' Colon @@*"`
}

// CodeBlock represents indented lines passed through verbatim.
type CodeBlock struct {
	Lines []string `parser:"@CodeLine+"`
}

// Heading represents a markdown-style heading in a doc comment.
type Heading struct {
	Text []string `parser:"HashMark @(Word | ParamName | ReturnType)+"`
}
