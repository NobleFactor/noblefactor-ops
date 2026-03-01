// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func TestGoCallable(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"provider.go": `package example

import "os"

// +devlore:callable swallow=stack handle=DirEntry:Name,IsDir
type Visitor func(initial any, path string, dirEntry os.DirEntry, stack *RecoveryStack) (result any, err error)

type RecoveryStack struct{}
`,
	})

	r := NewGoReceiver()

	result := callMethod(t, r, "callable",
		starlark.Tuple{starlark.String(dir), starlark.String("Visitor")},
		nil,
	)

	// Verify it's a struct
	s, ok := result.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("expected struct, got %T", result)
	}

	// Verify name
	name := getStructAttr(t, s, "name")
	if name != "Visitor" {
		t.Errorf("name: got %q, want %q", name, "Visitor")
	}

	// Verify doc preserves directive
	doc := getStructAttr(t, s, "doc")
	if !contains(doc, "+devlore:callable") {
		t.Errorf("doc should contain +devlore:callable directive, got %q", doc)
	}
	if !contains(doc, "swallow=stack") {
		t.Errorf("doc should contain swallow=stack, got %q", doc)
	}

	// Verify returns
	returns := getStructAttr(t, s, "returns")
	if returns != "(any, error)" {
		t.Errorf("returns: got %q, want %q", returns, "(any, error)")
	}

	// Verify params count
	paramsAttr, err := s.Attr("params")
	if err != nil {
		t.Fatalf("params attr: %v", err)
	}
	params := paramsAttr.(*starlark.List)
	if params.Len() != 4 {
		t.Fatalf("expected 4 params, got %d", params.Len())
	}

	expected := []struct {
		name string
		typ  string
	}{
		{"initial", "any"},
		{"path", "string"},
		{"dirEntry", "os.DirEntry"},
		{"stack", "*RecoveryStack"},
	}

	for i, exp := range expected {
		p := params.Index(i)
		if got := getStructAttr(t, p, "name"); got != exp.name {
			t.Errorf("param[%d] name: got %q, want %q", i, got, exp.name)
		}
		if got := getStructAttr(t, p, "type"); got != exp.typ {
			t.Errorf("param[%d] type: got %q, want %q", i, got, exp.typ)
		}
	}
}

func TestGoCallable_NotFound(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"empty.go": `package example
`,
	})

	r := NewGoReceiver()
	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("callable")
	fn := attr.(*starlark.Builtin)
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String(dir), starlark.String("NonExistent")}, nil)
	if err == nil {
		t.Error("expected error for non-existent function type")
	}
}

func TestGoCallable_StructNotFunc(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"types.go": `package example

type Config struct {
	Name string
}
`,
	})

	r := NewGoReceiver()
	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("callable")
	fn := attr.(*starlark.Builtin)
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String(dir), starlark.String("Config")}, nil)
	if err == nil {
		t.Error("expected error when looking for function type but found struct")
	}
}

func TestGoCallable_SimpleFunc(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"types.go": `package example

// Handler processes a request.
type Handler func(path string, isDir bool) error
`,
	})

	r := NewGoReceiver()

	result := callMethod(t, r, "callable",
		starlark.Tuple{starlark.String(dir), starlark.String("Handler")},
		nil,
	)

	name := getStructAttr(t, result, "name")
	if name != "Handler" {
		t.Errorf("name: got %q, want %q", name, "Handler")
	}

	doc := getStructAttr(t, result, "doc")
	if !contains(doc, "processes a request") {
		t.Errorf("doc should contain 'processes a request', got %q", doc)
	}

	returns := getStructAttr(t, result, "returns")
	if returns != "error" {
		t.Errorf("returns: got %q, want %q", returns, "error")
	}

	s := result.(*starlarkstruct.Struct)
	paramsAttr, _ := s.Attr("params")
	params := paramsAttr.(*starlark.List)
	if params.Len() != 2 {
		t.Fatalf("expected 2 params, got %d", params.Len())
	}

	if got := getStructAttr(t, params.Index(0), "name"); got != "path" {
		t.Errorf("param[0] name: got %q, want %q", got, "path")
	}
	if got := getStructAttr(t, params.Index(0), "type"); got != "string" {
		t.Errorf("param[0] type: got %q, want %q", got, "string")
	}
	if got := getStructAttr(t, params.Index(1), "name"); got != "isDir" {
		t.Errorf("param[1] name: got %q, want %q", got, "isDir")
	}
	if got := getStructAttr(t, params.Index(1), "type"); got != "bool" {
		t.Errorf("param[1] type: got %q, want %q", got, "bool")
	}
}
