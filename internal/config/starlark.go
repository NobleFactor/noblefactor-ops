// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package config

import (
	"errors"
	"fmt"
	"reflect"

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

// mapToStarlarkStructTopLevel converts a map to a Starlark struct at the top levels.
// After depth 2 (e.g., lint.copyright.patterns), nested maps become dicts for .get access.
func mapToStarlarkStructTopLevel(m map[string]interface{}, depth int) starlark.Value {
	if m == nil {
		return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{})
	}

	fields := starlark.StringDict{}
	for k, v := range m {
		fields[k] = valueToStarlarkHybrid(v, depth+1)
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, fields)
}

// valueToStarlarkHybrid converts values with smart struct/dict handling.
// Config sections (depth <= 2) use structs for attribute access.
// Deeper nested maps (depth > 2) use dicts for .get()/.items() access.
func valueToStarlarkHybrid(v interface{}, depth int) starlark.Value {
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
			items = append(items, valueToStarlarkHybrid(item, depth+1))
		}
		return starlark.NewList(items)
	case []string:
		var items []starlark.Value
		for _, item := range val {
			items = append(items, starlark.String(item))
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		// Config sections (depth <= 2) use struct for attribute access
		// Deeper maps (patterns, config inline) use dict for .get()/.items()
		if depth <= 2 {
			return mapToStarlarkStructTopLevel(val, depth)
		}
		// Convert to dict - values inside become structs for attribute access
		return mapToStarlarkDictWithStructValues(val)
	default:
		return starlark.None
	}
}

// mapToStarlarkDictWithStructValues converts a map to a Starlark dict,
// but nested maps become structs for attribute access on the values.
func mapToStarlarkDictWithStructValues(m map[string]interface{}) starlark.Value {
	if m == nil {
		return starlark.NewDict(0)
	}

	dict := starlark.NewDict(len(m))
	for k, v := range m {
		// Values in the dict become structs for attribute access
		_ = dict.SetKey(starlark.String(k), valueToStarlarkAsStruct(v))
	}
	return dict
}

// valueToStarlarkAsStruct converts a value, turning maps into structs.
func valueToStarlarkAsStruct(v interface{}) starlark.Value {
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
			items = append(items, valueToStarlarkAsStruct(item))
		}
		return starlark.NewList(items)
	case []string:
		var items []starlark.Value
		for _, item := range val {
			items = append(items, starlark.String(item))
		}
		return starlark.NewList(items)
	case map[string]interface{}:
		fields := starlark.StringDict{}
		for k, inner := range val {
			fields[k] = valueToStarlarkAsStruct(inner)
		}
		return starlarkstruct.FromStringDict(starlarkstruct.Default, fields)
	default:
		return starlark.None
	}
}

// ===========================================================================
// ConfigValue: Reflection-based Starlark adapter for dynamic config types
// ===========================================================================
//
// ConfigValue wraps any Go struct (including those generated at runtime via
// reflect.StructOf) and implements starlark.HasAttrs for attribute access.
// This allows dynamically-generated extension configs to be accessed in
// Starlark scripts using dot notation: cfg.lint.copyright.enabled
//
// Unlike the type-specific ToStarlark() methods above, ConfigValue uses
// reflection to handle any struct type, including those created at runtime.

// ConfigValue wraps a Go struct for Starlark attribute access.
// It implements starlark.Value and starlark.HasAttrs.
type ConfigValue struct {
	elem interface{} // any struct or pointer to struct
}

// Ensure ConfigValue implements the required interfaces.
var (
	_ starlark.Value    = (*ConfigValue)(nil)
	_ starlark.HasAttrs = (*ConfigValue)(nil)
)

// WrapAsStarlarkValue wraps any Go value for Starlark access.
// Returns a ConfigValue that provides attribute access via reflection.
func WrapAsStarlarkValue(v interface{}) *ConfigValue {
	return &ConfigValue{elem: v}
}

// String returns a string representation of the ConfigValue.
func (v *ConfigValue) String() string {
	if v.elem == nil {
		return "config(nil)"
	}
	return fmt.Sprintf("config(%T)", v.elem)
}

// Type returns the Starlark type name.
func (v *ConfigValue) Type() string {
	return "config"
}

// Freeze makes the ConfigValue immutable. This is a no-op since Go structs
// don't have a concept of mutability that matches Starlark's.
func (v *ConfigValue) Freeze() {}

// Truth returns the Starlark truth value. ConfigValue is always truthy.
func (v *ConfigValue) Truth() starlark.Bool {
	return starlark.True
}

// Hash returns a hash for the ConfigValue. Config values are not hashable.
func (v *ConfigValue) Hash() (uint32, error) {
	return 0, errors.New("config is not hashable")
}

// Attr returns the value of the named attribute.
// Implements starlark.HasAttrs.
func (v *ConfigValue) Attr(name string) (starlark.Value, error) {
	if v.elem == nil {
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("config has no .%s attribute", name))
	}

	// Try to use ConfigElement interface if available
	if elem := getConfigElement(v.elem); elem != nil {
		if child := elem.Get(name); child != nil {
			return goToStarlarkReflect(child)
		}
	}

	rv := reflect.ValueOf(v.elem)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	// Look for struct field
	if rv.Kind() == reflect.Struct {
		fieldName := toPascalCase(name)
		field := rv.FieldByName(fieldName)
		if field.IsValid() {
			return goToStarlarkReflect(field.Interface())
		}
	}

	return nil, starlark.NoSuchAttrError(fmt.Sprintf("config has no .%s attribute", name))
}

// getConfigElement extracts the ConfigElement from various types.
func getConfigElement(v interface{}) *ConfigElement {
	if v == nil {
		return nil
	}

	// Direct *ConfigElement
	if elem, ok := v.(*ConfigElement); ok {
		return elem
	}

	// ExtensibleConfig has embedded ConfigElement
	if cfg, ok := v.(*ExtensibleConfig); ok {
		return &cfg.ConfigElement
	}

	// Try to extract via reflection for other types
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	// Look for embedded ConfigElement field
	elemField := rv.FieldByName("ConfigElement")
	if !elemField.IsValid() {
		return nil
	}

	// Get the address of the embedded field if possible
	if elemField.CanAddr() {
		if elem, ok := elemField.Addr().Interface().(*ConfigElement); ok {
			return elem
		}
	}

	return nil
}

// AttrNames returns the names of all available attributes.
// Implements starlark.HasAttrs.
func (v *ConfigValue) AttrNames() []string {
	if v.elem == nil {
		return nil
	}

	var names []string

	// Add ConfigElement children
	if elem := getConfigElement(v.elem); elem != nil {
		for name := range elem.Children() {
			names = append(names, name)
		}
	}

	rv := reflect.ValueOf(v.elem)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return names
	}

	// Add struct fields
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		// Skip embedded ConfigElement
		if field.Anonymous && field.Name == "ConfigElement" {
			continue
		}
		names = append(names, toSnakeCase(field.Name))
	}

	return names
}

// goToStarlarkReflect converts a Go value to a Starlark value using reflection.
// This handles runtime-generated types that can't be matched in type switches.
func goToStarlarkReflect(v interface{}) (starlark.Value, error) {
	if v == nil {
		return starlark.None, nil
	}

	rv := reflect.ValueOf(v)
	return reflectToStarlark(rv)
}

// reflectToStarlark converts a reflect.Value to a Starlark value.
func reflectToStarlark(rv reflect.Value) (starlark.Value, error) {
	// Handle interface{}
	if rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return starlark.None, nil
		}
		rv = rv.Elem()
	}

	// Handle pointers - wrap pointer to struct as ConfigValue to preserve reference
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return starlark.None, nil
		}
		// If it's a pointer to a struct, wrap it directly (preserve pointer)
		if rv.Elem().Kind() == reflect.Struct {
			return WrapAsStarlarkValue(rv.Interface()), nil
		}
		// For other pointer types, dereference
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool:
		return starlark.Bool(rv.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint64(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return starlark.Float(rv.Float()), nil
	case reflect.String:
		return starlark.String(rv.String()), nil
	case reflect.Slice:
		return sliceToStarlark(rv)
	case reflect.Map:
		return mapToStarlarkReflect(rv)
	case reflect.Struct:
		// Wrap struct as ConfigValue - try to get addressable version
		if rv.CanAddr() {
			return WrapAsStarlarkValue(rv.Addr().Interface()), nil
		}
		return WrapAsStarlarkValue(rv.Interface()), nil
	default:
		return starlark.None, nil
	}
}

// sliceToStarlark converts a slice to a Starlark list.
func sliceToStarlark(rv reflect.Value) (starlark.Value, error) {
	n := rv.Len()
	items := make([]starlark.Value, n)
	for i := 0; i < n; i++ {
		item, err := reflectToStarlark(rv.Index(i))
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return starlark.NewList(items), nil
}

// mapToStarlarkReflect converts a map to a Starlark dict using reflection.
func mapToStarlarkReflect(rv reflect.Value) (starlark.Value, error) {
	dict := starlark.NewDict(rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		key, err := reflectToStarlark(iter.Key())
		if err != nil {
			return nil, err
		}
		val, err := reflectToStarlark(iter.Value())
		if err != nil {
			return nil, err
		}
		if err := dict.SetKey(key, val); err != nil {
			return nil, err
		}
	}
	return dict, nil
}
