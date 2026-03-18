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
	Directive     *Directive     `parser:"  @@" starlark:"directive"`
	ParamSection  *ParamSection  `parser:"| @@" starlark:"param_section"`
	ReturnSection *ReturnSection `parser:"| @@" starlark:"return_section"`
	CodeBlock     *CodeBlock     `parser:"| @@" starlark:"code_block"`
	Heading       *Heading       `parser:"| @@" starlark:"heading"`
	Paragraph     *Paragraph     `parser:"| @@" starlark:"paragraph"`
}

// String returns the normalized text of whichever element is non-nil.
func (e *DocElement) String() string {
	switch {
	case e.Directive != nil:
		return e.Directive.String()
	case e.ParamSection != nil:
		return e.ParamSection.String()
	case e.ReturnSection != nil:
		return e.ReturnSection.String()
	case e.CodeBlock != nil:
		return e.CodeBlock.String()
	case e.Heading != nil:
		return e.Heading.String()
	case e.Paragraph != nil:
		return e.Paragraph.String()
	default:
		return ""
	}
}

// FuncDoc collects doc elements in whatever order they appear. BlankLine
// tokens serve as element separators — they are NOT elided so that Paragraph
// stops consuming at section boundaries.
type FuncDoc struct {
	Elements []*DocElement `parser:"(BlankLine* @@)* BlankLine*" starlark:"elements"`
}

// String returns the normalized text of the function doc comment.
func (d *FuncDoc) String() string { return d.Normalize() }

// GetParamSection returns the first ParamSection from the parsed elements,
// or nil if none exists.
func (d *FuncDoc) GetParamSection() *ParamSection {
	for _, el := range d.Elements {
		if el.ParamSection != nil {
			return el.ParamSection
		}
	}
	return nil
}

// GetReturnSection returns the first ReturnSection from the parsed elements,
// or nil if none exists.
func (d *FuncDoc) GetReturnSection() *ReturnSection {
	for _, el := range d.Elements {
		if el.ReturnSection != nil {
			return el.ReturnSection
		}
	}
	return nil
}

// ParamDocs builds a map of parameter name → description from the parsed
// ParamSection. Returns nil if no ParamSection exists.
func (d *FuncDoc) ParamDocs() map[string]string {
	ps := d.GetParamSection()
	if ps == nil || len(ps.Items) == 0 {
		return nil
	}
	docs := make(map[string]string, len(ps.Items))
	for _, item := range ps.Items {
		docs[item.Name] = item.Desc.Normalize()
	}
	return docs
}

// TypeDocElement is the wrapper struct for type doc alternation.
// Type docs contain only paragraphs, code blocks, and headings.
type TypeDocElement struct {
	CodeBlock *CodeBlock `parser:"  @@" starlark:"code_block"`
	Heading   *Heading   `parser:"| @@" starlark:"heading"`
	Paragraph *Paragraph `parser:"| @@" starlark:"paragraph"`
}

// String returns the normalized text of whichever element is non-nil.
func (e *TypeDocElement) String() string {
	switch {
	case e.CodeBlock != nil:
		return e.CodeBlock.String()
	case e.Heading != nil:
		return e.Heading.String()
	case e.Paragraph != nil:
		return e.Paragraph.String()
	default:
		return ""
	}
}

// TypeDoc collects type doc elements in any order.
type TypeDoc struct {
	Elements []*TypeDocElement `parser:"(BlankLine* @@)* BlankLine*" starlark:"elements"`
}

// String returns the normalized text of the type doc comment.
func (d *TypeDoc) String() string { return d.Normalize() }

// CopyrightDoc represents a fixed-order SPDX + Copyright header.
type CopyrightDoc struct {
	SPDX      string   `parser:"'SPDX-License-Identifier' Colon @Word" starlark:"spdx"`
	Copyright []string `parser:"'Copyright' @(Word | Colon)+" starlark:"copyright"`
}

// String returns the normalized copyright text.
func (c *CopyrightDoc) String() string { return c.Normalize() }

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
