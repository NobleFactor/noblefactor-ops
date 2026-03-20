// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/NobleFactor/noblefactor-ops/internal/provider/goast/doctaxonomy"
)

// =============================================================================
// FILE NODE TREE
// =============================================================================

type nodeKind int

const (
	nodeComment nodeKind = iota
	nodePackage
	nodeCode
)

// fileNode is one item in the ordered tree at any level.
type fileNode struct {
	kind nodeKind

	// gap: whitespace from the original source that precedes this node.
	gap string

	// nodeComment: the comment text (original or replacement).
	text string

	// nodeCode: the source text for this code fragment.
	code string

	// nodePackage: the package name.
	pkgName string

	// Children: for blocks that contain interleaved code and comments.
	children []fileNode
}

// =============================================================================
// BUILD TREE
// =============================================================================

// buildFileTree walks the Go AST and original source to build an ordered list
// of fileNode items. Positions are used during construction only — never during
// output. Each declaration with a doc comment becomes two items (doc comment
// and code) so the newline between them is captured as a gap.
func buildFileTree(src string, fset *token.FileSet, node *ast.File, width int) []fileNode {
	var items []fileNode

	// Map doc comments to their owners for replacement logic.
	docOwner := map[*ast.CommentGroup]ast.Node{}
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Doc != nil {
				docOwner[d.Doc] = d
			}
		case *ast.GenDecl:
			if d.Doc != nil {
				docOwner[d.Doc] = d
			}
		}
	}

	type positioned struct {
		startOff int
		endOff   int
		emit     func()
	}

	var all []positioned
	off := func(p token.Pos) int { return fset.Position(p).Offset }

	// Classify comments into doc, body, or floating.
	docCGs := map[*ast.CommentGroup]bool{}
	for cg := range docOwner {
		docCGs[cg] = true
	}

	bodyCGs := map[*ast.CommentGroup]bool{}
	for _, decl := range node.Decls {
		for _, cg := range node.Comments {
			if !docCGs[cg] && cg.Pos() >= decl.Pos() && cg.End() <= decl.End() {
				bodyCGs[cg] = true
			}
		}
	}

	// Floating comments (not doc, not body).
	for _, cg := range node.Comments {
		if docCGs[cg] || bodyCGs[cg] {
			continue
		}
		cg := cg // capture
		all = append(all, positioned{startOff: off(cg.Pos()), endOff: off(cg.End()), emit: func() {
			items = append(items, fileNode{
				kind: nodeComment,
				text: rewriteComment(cg, nil, width),
			})
		}})
	}

	// Package clause.
	pkgEnd := off(node.Name.End())
	all = append(all, positioned{startOff: off(node.Package), endOff: pkgEnd, emit: func() {
		items = append(items, fileNode{kind: nodePackage, pkgName: node.Name.Name})
	}})

	// Declarations: doc comment and code are separate positioned items so the
	// newline between them is captured as the code item's gap.
	for _, decl := range node.Decls {
		decl := decl // capture

		var doc *ast.CommentGroup
		switch d := decl.(type) {
		case *ast.FuncDecl:
			doc = d.Doc
		case *ast.GenDecl:
			doc = d.Doc
		}

		// Doc comment as a separate positioned item.
		if doc != nil {
			doc := doc // capture
			all = append(all, positioned{startOff: off(doc.Pos()), endOff: off(doc.End()), emit: func() {
				items = append(items, fileNode{
					kind: nodeComment,
					text: rewriteComment(doc, decl, width),
				})
			}})
		}

		// Code part — starts at the keyword (func/type/var/const), not the doc.
		declStartOff := off(decl.Pos())
		declEndOff := off(decl.End())
		all = append(all, positioned{startOff: declStartOff, endOff: declEndOff, emit: func() {
			var children []fileNode

			// Body comments for this declaration.
			var bodyCmts []*ast.CommentGroup
			for _, cg := range node.Comments {
				if bodyCGs[cg] && cg.Pos() >= decl.Pos() && cg.End() <= decl.End() {
					bodyCmts = append(bodyCmts, cg)
				}
			}

			if len(bodyCmts) == 0 {
				children = append(children, fileNode{
					kind: nodeCode,
					code: src[declStartOff:declEndOff],
				})
			} else {
				codeStart := declStartOff
				for _, cg := range bodyCmts {
					cmtStart := fset.Position(cg.Pos()).Offset
					cmtEnd := fset.Position(cg.End()).Offset

					if cmtStart > codeStart {
						children = append(children, fileNode{
							kind: nodeCode,
							code: src[codeStart:cmtStart],
						})
					}
					// Use source text for body comments to preserve
					// indentation of multi-line comment groups.
					children = append(children, fileNode{
						kind: nodeComment,
						text: src[cmtStart:cmtEnd],
					})
					codeStart = cmtEnd
				}

				if codeStart < declEndOff {
					children = append(children, fileNode{
						kind: nodeCode,
						code: src[codeStart:declEndOff],
					})
				}
			}

			items = append(items, fileNode{kind: nodeCode, children: children})
		}})
	}

	// Sort by start offset — used once, then discarded.
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].startOff < all[i].startOff {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	// Emit in order, capturing gaps between items.
	cursor := 0
	for _, p := range all {
		p.emit()
		gap := ""
		if p.startOff > cursor {
			gap = src[cursor:p.startOff]
		}
		items[len(items)-1].gap = gap
		cursor = p.endOff
	}

	// Capture trailing content after the last item.
	if cursor < len(src) {
		items = append(items, fileNode{kind: nodeCode, code: "", gap: src[cursor:]})
	}

	return items
}

// =============================================================================
// COMMENT FORMATTING
// =============================================================================

// rewriteComment produces the text for a comment group — reformatted or
// original. Skips copyright headers and delineator blocks.
func rewriteComment(cg *ast.CommentGroup, decl ast.Node, width int) string {
	raw := commentGroupRaw(cg)
	if raw == "" {
		return commentText(cg)
	}

	// Skip copyright and delineators.
	if strings.HasPrefix(raw, "SPDX-License-Identifier") {
		return commentText(cg)
	}
	if isDelineatorBlock(raw) {
		return commentText(cg)
	}

	// FuncDecl doc — taxonomy.
	if fn, ok := decl.(*ast.FuncDecl); ok {
		pNames := astParamNames(fn.Type.Params)
		rTypes := astReturnTypes(fn.Type.Results)
		funcDoc := parseFuncDocSafe(raw, pNames, rTypes)
		normalized := funcDoc.Normalize()
		return strings.TrimRight(doctaxonomy.Format(normalized, width), "\n")
	}

	// GenDecl type doc — TypeDoc parser.
	if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
		tp := doctaxonomy.NewTypeParser()
		typeDoc, err := tp.ParseString("", raw)
		if err == nil {
			normalized := typeDoc.Normalize()
			return strings.TrimRight(doctaxonomy.Format(normalized, width), "\n")
		}
	}

	// Floating prose or other — reflow.
	return strings.TrimRight(doctaxonomy.Format(raw, width), "\n")
}

// commentText returns the original text of a comment group with // prefixes.
func commentText(cg *ast.CommentGroup) string {
	var b strings.Builder
	for i, c := range cg.List {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(c.Text)
	}
	return b.String()
}

// isDelineatorBlock returns true if the raw comment text contains a delineator
// line (3+ repeated =, -, ~, or * characters).
func isDelineatorBlock(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		s := strings.TrimSpace(line)
		if len(s) >= 3 {
			first := s[0]
			if first == '=' || first == '-' || first == '~' || first == '*' {
				allSame := true
				for i := 1; i < len(s); i++ {
					if s[i] != first {
						allSame = false
						break
					}
				}
				if allSame {
					return true
				}
			}
		}
	}
	return false
}

// =============================================================================
// PRINT TREE
// =============================================================================

func printTree(nodes []fileNode) string {
	var b strings.Builder
	for _, n := range nodes {
		printNode(&b, n)
	}
	return b.String()
}

func printNode(b *strings.Builder, n fileNode) {
	b.WriteString(n.gap)

	switch n.kind {
	case nodePackage:
		b.WriteString("package ")
		b.WriteString(n.pkgName)

	case nodeComment:
		b.WriteString(n.text)

	case nodeCode:
		if len(n.children) > 0 {
			for _, child := range n.children {
				printNode(b, child)
			}
		} else {
			b.WriteString(n.code)
		}
	}
}

// =============================================================================
// ENTRY POINT
// =============================================================================

// rewriteFileFromSource parses Go source, rewrites comments through the
// taxonomy pipeline, and returns the modified source. Code is copied from the
// original source verbatim — no format.Node.
func rewriteFileFromSource(filename, src string, width int) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("rewrite %s: %w", filename, err)
	}

	tree := buildFileTree(src, fset, node, width)
	return printTree(tree), nil
}
