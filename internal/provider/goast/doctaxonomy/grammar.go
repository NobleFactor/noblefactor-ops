// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"github.com/alecthomas/participle/v2"
)

// DocElement is the wrapper struct for unordered alternation. Exactly one
// field is non-nil after each parse iteration. Order matters: more specific
// productions (Directive, ParamSection, ReturnSection) are tried before the
// catch-all Paragraph.
type DocElement struct {
	Directive     *Directive     `parser:"  @@"`
	ParamSection  *ParamSection  `parser:"| @@"`
	ReturnSection *ReturnSection `parser:"| @@"`
	CodeBlock     *CodeBlock     `parser:"| @@"`
	Heading       *Heading       `parser:"| @@"`
	Paragraph     *Paragraph     `parser:"| @@"`
}

// FuncDoc collects doc elements in whatever order they appear. BlankLine
// tokens serve as element separators — they are NOT elided so that Paragraph
// stops consuming at section boundaries.
type FuncDoc struct {
	Elements []*DocElement `parser:"(BlankLine* @@)* BlankLine*"`
}

// TypeDocElement is the wrapper struct for type doc alternation.
// Type docs contain only paragraphs, code blocks, and headings.
type TypeDocElement struct {
	CodeBlock *CodeBlock `parser:"  @@"`
	Heading   *Heading   `parser:"| @@"`
	Paragraph *Paragraph `parser:"| @@"`
}

// TypeDoc collects type doc elements in any order.
type TypeDoc struct {
	Elements []*TypeDocElement `parser:"(BlankLine* @@)* BlankLine*"`
}

// CopyrightDoc represents a fixed-order SPDX + Copyright header.
type CopyrightDoc struct {
	SPDX      string   `parser:"'SPDX-License-Identifier' Colon @Word"`
	Copyright []string `parser:"'Copyright' @(Word | Colon)+"`
}

// NewFuncParser constructs a participle parser for FuncDoc using a
// context-aware lexer built from the given parameter names and return types.
// BlankLine is NOT elided — it acts as an element separator so that
// Paragraph does not consume across section boundaries.
func NewFuncParser(paramNames, returnTypes []string) *participle.Parser[FuncDoc] {
	lex := NewDocLexer(paramNames, returnTypes)
	p := participle.MustBuild[FuncDoc](
		participle.Lexer(lex),
		participle.Elide("whitespace", "Newline"),
	)
	return p
}

// NewTypeParser constructs a participle parser for TypeDoc.
func NewTypeParser() *participle.Parser[TypeDoc] {
	lex := NewDocLexer(nil, nil)
	p := participle.MustBuild[TypeDoc](
		participle.Lexer(lex),
		participle.Elide("whitespace", "Newline"),
	)
	return p
}

// NewCopyrightParser constructs a participle parser for CopyrightDoc.
// BlankLine IS elided here — copyright headers are two adjacent lines
// with no blank line between them.
func NewCopyrightParser() *participle.Parser[CopyrightDoc] {
	lex := NewDocLexer(nil, nil)
	p := participle.MustBuild[CopyrightDoc](
		participle.Lexer(lex),
		participle.Elide("whitespace", "Newline", "BlankLine"),
	)
	return p
}
