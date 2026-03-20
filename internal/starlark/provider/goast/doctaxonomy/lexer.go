// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 Noble Factor. All rights reserved.

package doctaxonomy

import (
	"regexp"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
)

// neverMatch is a regex pattern that cannot match any input. Used to define
// token rules that must exist for the grammar but have no values to match.
const neverMatch = `\x00`

// NewDocLexer constructs a context-aware stateful lexer definition. Parameter
// names and return types from the AST are injected as token rules before the
// generic Word rule so they match first.
func NewDocLexer(paramNames, returnTypes []string) *lexer.StatefulDefinition {
	root := []lexer.Rule{
		{Name: "BlankLine", Pattern: `\n[ \t]*\n`, Action: nil},
		{Name: "DirectiveMark", Pattern: `\+`, Action: lexer.Push("Directive")},
		{Name: "ListMarker", Pattern: `-[ \t]+`, Action: nil},
		{Name: "CodeLine", Pattern: `    .+`, Action: nil},
		{Name: "HashMark", Pattern: `#`, Action: nil},
	}

	// ParamName and ReturnType rules must always exist because the grammar
	// references them. When the lists are empty, use a never-match pattern.
	paramPattern := neverMatch
	if len(paramNames) > 0 {
		paramPattern = buildAlternation(paramNames)
	}
	root = append(root, lexer.Rule{
		Name:    "ParamName",
		Pattern: paramPattern,
		Action:  nil,
	})

	returnPattern := neverMatch
	if len(returnTypes) > 0 {
		returnPattern = buildAlternation(returnTypes)
	}
	root = append(root, lexer.Rule{
		Name:    "ReturnType",
		Pattern: returnPattern,
		Action:  nil,
	})

	root = append(root,
		lexer.Rule{Name: "Colon", Pattern: `:`, Action: nil},
		lexer.Rule{Name: "Word", Pattern: `[^\s:]+`, Action: nil},
		lexer.Rule{Name: "whitespace", Pattern: `[ \t]+`, Action: nil},
		lexer.Rule{Name: "Newline", Pattern: `\n`, Action: nil},
	)

	directive := []lexer.Rule{
		{Name: "DirectiveToken", Pattern: `\S+`, Action: nil},
		{Name: "whitespace", Pattern: `[ \t]+`, Action: nil},
		{Name: "Newline", Pattern: `\n`, Action: lexer.Pop()},
	}

	return lexer.MustStateful(lexer.Rules{
		"Root":      root,
		"Directive": directive,
	})
}

// buildAlternation creates a regex alternation pattern from the given words,
// escaping any regex metacharacters. Longer names are placed first so that
// a prefix doesn't shadow a longer name (e.g., "err" vs "error").
func buildAlternation(words []string) string {
	sorted := make([]string, len(words))
	copy(sorted, words)

	// Sort by length descending so longer names match first.
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if len(sorted[j]) > len(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	escaped := make([]string, len(sorted))
	for i, w := range sorted {
		escaped[i] = regexp.QuoteMeta(w)
	}

	return strings.Join(escaped, "|")
}
