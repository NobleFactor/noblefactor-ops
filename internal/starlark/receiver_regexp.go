// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"regexp"
	"sync"

	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/pkg/op"
)

// RegexpReceiver provides regular expression operations with pattern caching.
// Implements starlark.Value and starlark.HasAttrs.
type RegexpReceiver struct {
	op.Receiver
	cache sync.Map // pattern string → *regexp.Regexp
}

// NewRegexpReceiver creates a new RegexpReceiver.
func NewRegexpReceiver() *RegexpReceiver {
	return &RegexpReceiver{Receiver: op.NewReceiver("regexp")}
}

// compile returns a compiled regexp, using the cache if available.
func (r *RegexpReceiver) compile(pattern string) (*regexp.Regexp, error) {
	if cached, ok := r.cache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	// Store in cache (race is fine, we just compile twice at worst)
	r.cache.Store(pattern, re)
	return re, nil
}

// Attr implements starlark.HasAttrs.
func (r *RegexpReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "match":
		return op.MakeAttr("regexp.match", r.match), nil
	case "find":
		return op.MakeAttr("regexp.find", r.find), nil
	case "find_all":
		return op.MakeAttr("regexp.find_all", r.findAll), nil
	case "find_submatch":
		return op.MakeAttr("regexp.find_submatch", r.findSubmatch), nil
	case "find_all_submatch":
		return op.MakeAttr("regexp.find_all_submatch", r.findAllSubmatch), nil
	case "replace":
		return op.MakeAttr("regexp.replace", r.replace), nil
	case "replace_literal":
		return op.MakeAttr("regexp.replace_literal", r.replaceLiteral), nil
	case "split":
		return op.MakeAttr("regexp.split", r.split), nil
	default:
		return nil, op.NoSuchAttrError("regexp", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *RegexpReceiver) AttrNames() []string {
	return []string{
		"find",
		"find_all",
		"find_all_submatch",
		"find_submatch",
		"match",
		"replace",
		"replace_literal",
		"split",
	}
}

// match returns true if the pattern matches the text.
// regexp.match(pattern, text) -> bool
func (r *RegexpReceiver) match(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	if err := starlark.UnpackArgs("regexp.match", args, kwargs, "pattern", &pattern, "text", &text); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	return starlark.Bool(re.MatchString(text)), nil
}

// find returns the first match or None.
// regexp.find(pattern, text) -> string | None
func (r *RegexpReceiver) find(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	if err := starlark.UnpackArgs("regexp.find", args, kwargs, "pattern", &pattern, "text", &text); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	match := re.FindString(text)
	if match == "" {
		// Check if it's actually an empty match or no match
		if re.MatchString(text) {
			return starlark.String(""), nil
		}
		return starlark.None, nil
	}
	return starlark.String(match), nil
}

// findAll returns all matches.
// regexp.find_all(pattern, text, n=-1) -> list[string]
func (r *RegexpReceiver) findAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	n := -1
	if err := starlark.UnpackArgs("regexp.find_all", args, kwargs, "pattern", &pattern, "text", &text, "n?", &n); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	matches := re.FindAllString(text, n)
	result := make([]starlark.Value, len(matches))
	for i, m := range matches {
		result[i] = starlark.String(m)
	}
	return starlark.NewList(result), nil
}

// findSubmatch returns the first match with capture groups.
// regexp.find_submatch(pattern, text) -> list[string] | None
// Returns [full_match, group1, group2, ...]
func (r *RegexpReceiver) findSubmatch(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	if err := starlark.UnpackArgs("regexp.find_submatch", args, kwargs, "pattern", &pattern, "text", &text); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	matches := re.FindStringSubmatch(text)
	if matches == nil {
		return starlark.None, nil
	}

	result := make([]starlark.Value, len(matches))
	for i, m := range matches {
		result[i] = starlark.String(m)
	}
	return starlark.NewList(result), nil
}

// findAllSubmatch returns all matches with capture groups.
// regexp.find_all_submatch(pattern, text, n=-1) -> list[list[string]]
func (r *RegexpReceiver) findAllSubmatch(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	n := -1
	if err := starlark.UnpackArgs("regexp.find_all_submatch", args, kwargs, "pattern", &pattern, "text", &text, "n?", &n); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	allMatches := re.FindAllStringSubmatch(text, n)
	result := make([]starlark.Value, len(allMatches))
	for i, matches := range allMatches {
		inner := make([]starlark.Value, len(matches))
		for j, m := range matches {
			inner[j] = starlark.String(m)
		}
		result[i] = starlark.NewList(inner)
	}
	return starlark.NewList(result), nil
}

// replace replaces all matches with the replacement string.
// Supports $1, $2, ${name} for capture group references.
// regexp.replace(pattern, text, repl) -> string
func (r *RegexpReceiver) replace(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text, repl string
	if err := starlark.UnpackArgs("regexp.replace", args, kwargs, "pattern", &pattern, "text", &text, "repl", &repl); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	result := re.ReplaceAllString(text, repl)
	return starlark.String(result), nil
}

// replaceLiteral replaces all matches with the literal replacement string.
// Does not interpret $1, $2, etc.
// regexp.replace_literal(pattern, text, repl) -> string
func (r *RegexpReceiver) replaceLiteral(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text, repl string
	if err := starlark.UnpackArgs("regexp.replace_literal", args, kwargs, "pattern", &pattern, "text", &text, "repl", &repl); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	result := re.ReplaceAllLiteralString(text, repl)
	return starlark.String(result), nil
}

// split splits the text by the pattern.
// regexp.split(pattern, text, n=-1) -> list[string]
func (r *RegexpReceiver) split(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern, text string
	n := -1
	if err := starlark.UnpackArgs("regexp.split", args, kwargs, "pattern", &pattern, "text", &text, "n?", &n); err != nil {
		return nil, err
	}

	re, err := r.compile(pattern)
	if err != nil {
		return nil, err
	}

	parts := re.Split(text, n)
	result := make([]starlark.Value, len(parts))
	for i, p := range parts {
		result[i] = starlark.String(p)
	}
	return starlark.NewList(result), nil
}
