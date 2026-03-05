// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestStarlarkToGo(t *testing.T) {
	tests := []struct {
		name string
		val  starlark.Value
		want any
	}{
		{"none", starlark.None, nil},
		{"bool_true", starlark.Bool(true), true},
		{"bool_false", starlark.Bool(false), false},
		{"int", starlark.MakeInt(42), int64(42)},
		{"float", starlark.Float(3.14), float64(3.14)},
		{"string", starlark.String("hello"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := starlarkToGo(tt.val)
			if got != tt.want {
				t.Errorf("starlarkToGo(%v) = %v (%T), want %v (%T)", tt.val, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestStarlarkToGo_Dict(t *testing.T) {
	d := starlark.NewDict(2)
	_ = d.SetKey(starlark.String("name"), starlark.String("test"))
	_ = d.SetKey(starlark.String("count"), starlark.MakeInt(5))

	got := starlarkToGo(d)

	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	if m["name"] != "test" {
		t.Errorf("m[name] = %v, want test", m["name"])
	}
	if m["count"] != int64(5) {
		t.Errorf("m[count] = %v, want 5", m["count"])
	}
}

func TestStarlarkToGo_List(t *testing.T) {
	l := starlark.NewList([]starlark.Value{starlark.String("a"), starlark.String("b")})

	got := starlarkToGo(l)

	sl, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(sl) != 2 || sl[0] != "a" || sl[1] != "b" {
		t.Errorf("got %v, want [a, b]", sl)
	}
}

func TestStarlarkToGo_Struct(t *testing.T) {
	s := starlarkstruct.FromStringDict(starlark.String("test"), starlark.StringDict{
		"foo": starlark.String("bar"),
		"num": starlark.MakeInt(10),
	})

	got := starlarkToGo(s)

	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	if m["foo"] != "bar" {
		t.Errorf("m[foo] = %v, want bar", m["foo"])
	}
}

func TestStarlarkToGo_Nested(t *testing.T) {
	inner := starlark.NewDict(1)
	_ = inner.SetKey(starlark.String("key"), starlark.String("value"))

	outer := starlark.NewDict(1)
	_ = outer.SetKey(starlark.String("nested"), inner)

	got := starlarkToGo(outer)

	m := got.(map[string]any)
	nested := m["nested"].(map[string]any)
	if nested["key"] != "value" {
		t.Errorf("nested[key] = %v, want value", nested["key"])
	}
}

func TestRenderCamelToSnake(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"FooBar", "foo_bar"},
		{"HTTPServer", "http_server"},
		{"WriteText", "write_text"},
		{"CompensateInstall", "compensate_install"},
		{"URL", "url"},
		{"HTMLParser", "html_parser"},
	}
	for _, tt := range tests {
		got := renderCamelToSnake(tt.in)
		if got != tt.want {
			t.Errorf("renderCamelToSnake(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderLCFirst(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Foo", "foo"},
		{"", ""},
		{"a", "a"},
		{"ABC", "aBC"},
	}
	for _, tt := range tests {
		got := renderLCFirst(tt.in)
		if got != tt.want {
			t.Errorf("renderLCFirst(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGoRender_SimpleTemplate(t *testing.T) {
	r := NewGoReceiver()

	data := starlark.NewDict(2)
	_ = data.SetKey(starlark.String("Package"), starlark.String("foo"))
	_ = data.SetKey(starlark.String("Name"), starlark.String("Bar"))

	tmpl := `package {{.Package}}

func New{{.Name}}() {}
`
	result, err := r.goRender(nil, nil, starlark.Tuple{starlark.String(tmpl), data}, nil)
	if err != nil {
		t.Fatalf("goRender error: %v", err)
	}

	code := string(result.(starlark.String))
	if code != "package foo\n\nfunc NewBar() {}\n" {
		t.Errorf("unexpected output:\n%s", code)
	}
}

func TestGoRender_WithRange(t *testing.T) {
	r := NewGoReceiver()

	methods := starlark.NewList([]starlark.Value{
		starlark.String("Foo"),
		starlark.String("Bar"),
	})

	data := starlark.NewDict(2)
	_ = data.SetKey(starlark.String("Package"), starlark.String("test"))
	_ = data.SetKey(starlark.String("Methods"), methods)

	tmpl := `package {{.Package}}
{{range .Methods}}
func {{.}}() {}
{{end}}`
	result, err := r.goRender(nil, nil, starlark.Tuple{starlark.String(tmpl), data}, nil)
	if err != nil {
		t.Fatalf("goRender error: %v", err)
	}

	code := string(result.(starlark.String))
	if !strings.Contains(code, "func Foo()") || !strings.Contains(code, "func Bar()") {
		t.Errorf("expected Foo and Bar functions in output:\n%s", code)
	}
}

func TestGoRender_NestedDict(t *testing.T) {
	r := NewGoReceiver()

	inner := starlark.NewDict(1)
	_ = inner.SetKey(starlark.String("Type"), starlark.String("string"))

	data := starlark.NewDict(2)
	_ = data.SetKey(starlark.String("Package"), starlark.String("pkg"))
	_ = data.SetKey(starlark.String("Field"), inner)

	tmpl := `package {{.Package}}

type S struct {
	Name {{.Field.Type}}
}
`
	result, err := r.goRender(nil, nil, starlark.Tuple{starlark.String(tmpl), data}, nil)
	if err != nil {
		t.Fatalf("goRender error: %v", err)
	}

	code := string(result.(starlark.String))
	if !strings.Contains(code, "Name string") {
		t.Errorf("expected 'Name string' in output:\n%s", code)
	}
}

func TestGoRender_CamelToSnake(t *testing.T) {
	r := NewGoReceiver()

	data := starlark.NewDict(2)
	_ = data.SetKey(starlark.String("Package"), starlark.String("test"))
	_ = data.SetKey(starlark.String("Name"), starlark.String("WriteText"))

	tmpl := `package {{.Package}}

// {{camelToSnake .Name}}
func {{.Name}}() {}
`
	result, err := r.goRender(nil, nil, starlark.Tuple{starlark.String(tmpl), data}, nil)
	if err != nil {
		t.Fatalf("goRender error: %v", err)
	}

	code := string(result.(starlark.String))
	if !strings.Contains(code, "write_text") {
		t.Errorf("expected 'write_text' in output:\n%s", code)
	}
}

func TestGoFormat(t *testing.T) {
	r := NewGoReceiver()

	code := `package   foo

func   Bar(  )   {  }
`
	result, err := r.goFormat(nil, nil, starlark.Tuple{starlark.String(code)}, nil)
	if err != nil {
		t.Fatalf("goFormat error: %v", err)
	}

	formatted := string(result.(starlark.String))
	if formatted != "package foo\n\nfunc Bar() {}\n" {
		t.Errorf("unexpected format output:\n%q", formatted)
	}
}

func TestGoFormat_InvalidCode(t *testing.T) {
	r := NewGoReceiver()

	_, err := r.goFormat(nil, nil, starlark.Tuple{starlark.String("not valid go")}, nil)
	if err == nil {
		t.Fatal("expected error for invalid Go code")
	}
}
