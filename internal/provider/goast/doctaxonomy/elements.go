// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

// Paragraph represents one or more text tokens (words, param names, return
// types, colons) forming a prose paragraph.
type Paragraph struct {
	Words []string `@(Word | ParamName | ReturnType | Colon)+`
}

// Directive represents a +key value line (e.g., +devlore:defaults overwrite=true).
type Directive struct {
	Key   string   `DirectiveMark @DirectiveToken`
	Value []string `@DirectiveToken*`
}

// ParamItem represents a single parameter list entry. Accepts both ParamName
// and Word so that stale parameter names parse without error — validation
// detects the mismatch post-parse.
type ParamItem struct {
	Name string     `ListMarker @(ParamName | Word) Colon`
	Desc *Paragraph `@@`
}

// ParamSection represents a "Parameters:" section with zero or more items.
type ParamSection struct {
	Items []*ParamItem `"Parameters" Colon @@*`
}

// ReturnItem represents a single return value list entry.
type ReturnItem struct {
	Type string     `ListMarker @(ReturnType | Word) Colon`
	Desc *Paragraph `@@`
}

// ReturnSection represents a "Returns:" section with zero or more items.
type ReturnSection struct {
	Items []*ReturnItem `"Returns" Colon @@*`
}

// CodeBlock represents indented lines passed through verbatim.
type CodeBlock struct {
	Lines []string `@CodeLine+`
}

// Heading represents a markdown-style heading in a doc comment.
type Heading struct {
	Text []string `HashMark @(Word | ParamName | ReturnType)+`
}
