// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package config

import (
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// ToStarlark converts Config to a Starlark struct with attribute access.
func (c *Config) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"lint":      c.Lint.ToStarlark(),
		"precommit": c.Precommit.ToStarlark(),
	})
}

// ToStarlark converts LintConfig to a Starlark struct.
func (l *LintConfig) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"go":        l.Go.ToStarlark(),
		"shell":     l.Shell.ToStarlark(),
		"markdown":  l.Markdown.ToStarlark(),
		"copyright": l.Copyright.ToStarlark(),
	})
}

// ToStarlark converts GoLintConfig to a Starlark struct.
func (g *GoLintConfig) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":          starlark.String(g.Path),
		"skip_mod_tidy": starlark.Bool(g.SkipModTidy),
		"config":        mapToStarlarkDict(g.Config),
	})
}

// ToStarlark converts ShellLintConfig to a Starlark struct.
func (s *ShellLintConfig) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":     starlark.String(s.Path),
		"severity": starlark.String(s.Severity),
		"indent":   starlark.MakeInt(s.Indent),
	})
}

// ToStarlark converts MarkdownLintConfig to a Starlark struct.
func (m *MarkdownLintConfig) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"path":        starlark.String(m.Path),
		"exclude":     stringSliceToStarlark(m.Exclude),
		"config":      mapToStarlarkDict(m.Config),
		"frontmatter": m.Frontmatter.ToStarlark(),
	})
}

// ToStarlark converts FrontmatterConfig to a Starlark struct.
func (f *FrontmatterConfig) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"required": stringSliceToStarlark(f.Required),
		"optional": stringSliceToStarlark(f.Optional),
	})
}

// ToStarlark converts CopyrightLintConfig to a Starlark struct.
func (c *CopyrightLintConfig) ToStarlark() starlark.Value {
	// Convert patterns map to Starlark dict of structs
	patternsDict := starlark.NewDict(len(c.Patterns))
	for k, v := range c.Patterns {
		patternStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"match":   starlark.String(v.Match),
			"replace": starlark.String(v.Replace),
		})
		_ = patternsDict.SetKey(starlark.String(k), patternStruct)
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"enabled":  starlark.Bool(c.Enabled),
		"license":  starlark.String(c.License),
		"holder":   starlark.String(c.Holder),
		"patterns": patternsDict,
		"exclude":  stringSliceToStarlark(c.Exclude),
	})
}

// ToStarlark converts PrecommitConfig to a Starlark struct.
func (p *PrecommitConfig) ToStarlark() starlark.Value {
	var hooks []starlark.Value
	for _, h := range p.Hooks {
		hooks = append(hooks, h.ToStarlark())
	}

	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"hooks": starlark.NewList(hooks),
	})
}

// ToStarlark converts PrecommitHook to a Starlark struct.
func (h *PrecommitHook) ToStarlark() starlark.Value {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"id":             starlark.String(h.ID),
		"name":           starlark.String(h.Name),
		"entry":          starlark.String(h.Entry),
		"language":       starlark.String(h.Language),
		"pass_filenames": starlark.Bool(h.PassFilenames),
		"types":          stringSliceToStarlark(h.Types),
		"stages":         stringSliceToStarlark(h.Stages),
	})
}

// stringSliceToStarlark converts a Go string slice to a Starlark list.
func stringSliceToStarlark(s []string) starlark.Value {
	var items []starlark.Value
	for _, item := range s {
		items = append(items, starlark.String(item))
	}
	return starlark.NewList(items)
}

// mapToStarlarkDict converts a Go map to a Starlark dict.
// Handles nested maps and various value types.
func mapToStarlarkDict(m map[string]interface{}) starlark.Value {
	if m == nil {
		return starlark.NewDict(0)
	}

	dict := starlark.NewDict(len(m))
	for k, v := range m {
		_ = dict.SetKey(starlark.String(k), valueToStarlark(v))
	}
	return dict
}

// valueToStarlark converts a Go interface{} value to a Starlark value.
func valueToStarlark(v interface{}) starlark.Value {
	switch val := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []interface{}:
		var items []starlark.Value
		for _, item := range val {
			items = append(items, valueToStarlark(item))
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		return mapToStarlarkDict(val)
	default:
		// Fallback: convert to string
		return starlark.None
	}
}
