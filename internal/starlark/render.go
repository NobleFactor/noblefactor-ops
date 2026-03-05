// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"
	"unicode"

	"go.starlark.net/starlark"
)

// =============================================================================
// go.render() — general-purpose Go template rendering
// =============================================================================

// goRender executes a Go text/template against a Starlark dict and returns
// go/format-formatted Go source code. No project-specific validation or
// descriptor parsing — this is a general-purpose template engine.
//
// Usage from Starlark:
//
//	code = go.render(template_string, data_dict)
func (r *GoReceiver) goRender(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var templateContent string
	var dataVal starlark.Value
	if err := starlark.UnpackArgs("go.render", args, kwargs, "template", &templateContent, "data", &dataVal); err != nil {
		return nil, err
	}

	tmpl, err := template.New("render").Funcs(renderFuncs).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("go.render: template parse: %w", err)
	}

	data := starlarkToGo(dataVal)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("go.render: template execution: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("go.render: format error: %w\nraw output:\n%s", err, buf.String())
	}

	return starlark.String(string(formatted)), nil
}

// =============================================================================
// go.format() — standalone go/format
// =============================================================================

// goFormat formats Go source code via go/format.
func (r *GoReceiver) goFormat(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var code string
	if err := starlark.UnpackArgs("go.format", args, kwargs, "code", &code); err != nil {
		return nil, err
	}

	formatted, err := format.Source([]byte(code))
	if err != nil {
		return nil, fmt.Errorf("go.format: %w", err)
	}

	return starlark.String(string(formatted)), nil
}

// =============================================================================
// TEMPLATE FUNCTIONS
// =============================================================================

// renderFuncs provides general-purpose template functions for go.render().
var renderFuncs = template.FuncMap{
	"camelToSnake": renderCamelToSnake,
	"lcFirst":      renderLCFirst,
	"join":         strings.Join,
}

// renderCamelToSnake converts CamelCase Go names to snake_case.
func renderCamelToSnake(s string) string {
	runes := []rune(s)
	var result []rune
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) {
					result = append(result, '_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result = append(result, '_')
				}
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// renderLCFirst lowercases the first character of a string.
func renderLCFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
