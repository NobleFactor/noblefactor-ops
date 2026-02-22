// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"os"
	"path/filepath"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// callMethod calls a GoReceiver method by name with the given args/kwargs.
func callMethod(t *testing.T, r *GoReceiver, method string, args starlark.Tuple, kwargs []starlark.Tuple) starlark.Value {
	t.Helper()
	attr, err := r.Attr(method)
	if err != nil {
		t.Fatalf("Attr(%q): %v", method, err)
	}
	fn, ok := attr.(*starlark.Builtin)
	if !ok {
		t.Fatalf("Attr(%q) returned %T, want *starlark.Builtin", method, attr)
	}
	thread := &starlark.Thread{Name: "test"}
	result, err := fn.CallInternal(thread, args, kwargs)
	if err != nil {
		t.Fatalf("%s() failed: %v", method, err)
	}
	return result
}

// getStructAttr extracts a string attribute from a Starlark struct.
func getStructAttr(t *testing.T, v starlark.Value, name string) string {
	t.Helper()
	s, ok := v.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("expected struct, got %T", v)
	}
	attr, err := s.Attr(name)
	if err != nil {
		t.Fatalf("Attr(%q): %v", name, err)
	}
	str, ok := starlark.AsString(attr)
	if !ok {
		t.Fatalf("Attr(%q) = %v, want string", name, attr)
	}
	return str
}

// getListLen returns the length of a Starlark list.
func getListLen(t *testing.T, v starlark.Value) int {
	t.Helper()
	list, ok := v.(*starlark.List)
	if !ok {
		t.Fatalf("expected list, got %T", v)
	}
	return list.Len()
}

// getListItem returns item i from a Starlark list.
func getListItem(t *testing.T, v starlark.Value, i int) starlark.Value {
	t.Helper()
	list, ok := v.(*starlark.List)
	if !ok {
		t.Fatalf("expected list, got %T", v)
	}
	return list.Index(i)
}

// getBoolAttr extracts a bool attribute from a Starlark struct.
func getBoolAttr(t *testing.T, v starlark.Value, name string) bool {
	t.Helper()
	s, ok := v.(*starlarkstruct.Struct)
	if !ok {
		t.Fatalf("expected struct, got %T", v)
	}
	attr, err := s.Attr(name)
	if err != nil {
		t.Fatalf("Attr(%q): %v", name, err)
	}
	b, ok := attr.(starlark.Bool)
	if !ok {
		t.Fatalf("Attr(%q) = %v (%T), want bool", name, attr, attr)
	}
	return bool(b)
}

// writeTempDir creates a temp dir with Go source files.
func writeTempDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestGoStructs(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"types.go": `package example

// Node represents an execution graph node.
type Node struct {
	ID     string            ` + "`json:\"id\"`" + `         // Unique identifier
	Name   string            ` + "`json:\"name\"`" + `       // Display name
	Slots  map[string]string ` + "`json:\"slots,omitempty\"`" + `
	hidden int
	Skip   bool              ` + "`json:\"-\"`" + `
}

type Edge struct {
	From string ` + "`json:\"from\"`" + `
	To   string ` + "`json:\"to\"`" + `
}
`,
	})

	r := NewGoReceiver()
	result := callMethod(t, r, "structs", starlark.Tuple{starlark.String(dir)}, nil)

	count := getListLen(t, result)
	if count != 2 {
		t.Fatalf("expected 2 structs, got %d", count)
	}

	// Find Node struct
	var node starlark.Value
	for i := 0; i < count; i++ {
		item := getListItem(t, result, i)
		if getStructAttr(t, item, "name") == "Node" {
			node = item
			break
		}
	}
	if node == nil {
		t.Fatal("Node struct not found")
	}

	s := node.(*starlarkstruct.Struct)
	fieldsAttr, _ := s.Attr("fields")
	fields := fieldsAttr.(*starlark.List)

	// Node has 5 fields total, but Skip (json:"-") is excluded and hidden has no json tag
	// Expected: ID, Name, Slots, hidden (no json tag → json_name="")
	if fields.Len() != 4 {
		t.Errorf("expected 4 fields (excluding json:\"-\"), got %d", fields.Len())
		for i := 0; i < fields.Len(); i++ {
			t.Logf("  field: %s", getStructAttr(t, fields.Index(i), "name"))
		}
	}

	// Check ID field
	idField := fields.Index(0).(*starlarkstruct.Struct)
	idJSON, _ := idField.Attr("json_name")
	if str, _ := starlark.AsString(idJSON); str != "id" {
		t.Errorf("ID json_name: got %q, want %q", str, "id")
	}
	idReq, _ := idField.Attr("required")
	if idReq != starlark.Bool(true) {
		t.Errorf("ID required: got %v, want true", idReq)
	}
	idDesc, _ := idField.Attr("description")
	if str, _ := starlark.AsString(idDesc); str != "Unique identifier" {
		t.Errorf("ID description: got %q, want %q", str, "Unique identifier")
	}

	// Check Slots field (omitempty → not required)
	slotsField := fields.Index(2).(*starlarkstruct.Struct)
	slotsReq, _ := slotsField.Attr("required")
	if slotsReq != starlark.Bool(false) {
		t.Errorf("Slots required: got %v, want false", slotsReq)
	}
}

func TestGoConstGroups(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"consts.go": `package example

type SourceSystem string
type EncryptionSystem string

const (
	SourceGit SourceSystem = "git"
	SourceSVN SourceSystem = "svn"
	SourceHG  SourceSystem = "hg"
)

const (
	EncryptNone EncryptionSystem = "none"
	EncryptAES  EncryptionSystem = "aes"
)

const UntypedConst = "ignored"
`,
	})

	r := NewGoReceiver()

	// Test without filter
	result := callMethod(t, r, "const_groups", starlark.Tuple{starlark.String(dir)}, nil)
	if getListLen(t, result) != 2 {
		t.Fatalf("expected 2 const groups, got %d", getListLen(t, result))
	}

	// Test with type filter
	result = callMethod(t, r, "const_groups",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("type"), starlark.String("SourceSystem")}},
	)
	count := getListLen(t, result)
	if count != 1 {
		t.Fatalf("expected 1 group for SourceSystem filter, got %d", count)
	}

	group := getListItem(t, result, 0).(*starlarkstruct.Struct)
	typeName, _ := group.Attr("type_name")
	if str, _ := starlark.AsString(typeName); str != "SourceSystem" {
		t.Errorf("type_name: got %q, want %q", str, "SourceSystem")
	}
	consts, _ := group.Attr("constants")
	if getListLen(t, consts) != 3 {
		t.Errorf("expected 3 constants, got %d", getListLen(t, consts))
	}

	// Check first constant
	first := getListItem(t, consts, 0).(*starlarkstruct.Struct)
	nameAttr, _ := first.Attr("name")
	valAttr, _ := first.Attr("value")
	if str, _ := starlark.AsString(nameAttr); str != "SourceGit" {
		t.Errorf("first const name: got %q, want %q", str, "SourceGit")
	}
	if str, _ := starlark.AsString(valAttr); str != "git" {
		t.Errorf("first const value: got %q, want %q", str, "git")
	}
}

func TestGoMethods(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"ops.go": `package example

// CopyOp copies files.
type CopyOp struct{}

// Name returns the operation name.
func (c *CopyOp) Name() string {
	return "copy"
}

func (c *CopyOp) Execute() error {
	return nil
}

type MoveOp struct{}

func (m MoveOp) Name() string {
	return "move"
}
`,
	})

	r := NewGoReceiver()

	// All methods
	result := callMethod(t, r, "methods", starlark.Tuple{starlark.String(dir)}, nil)
	if getListLen(t, result) != 3 {
		t.Fatalf("expected 3 methods, got %d", getListLen(t, result))
	}

	// Filter by name
	result = callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("Name")}},
	)
	if getListLen(t, result) != 2 {
		t.Errorf("expected 2 Name methods, got %d", getListLen(t, result))
	}

	// Filter by receiver_type (without pointer matches both)
	result = callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("receiver_type"), starlark.String("CopyOp")}},
	)
	if getListLen(t, result) != 2 {
		t.Errorf("expected 2 CopyOp methods, got %d", getListLen(t, result))
	}

	// Filter by receiver_type (with pointer matches only pointer)
	result = callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("receiver_type"), starlark.String("*CopyOp")}},
	)
	if getListLen(t, result) != 2 {
		t.Errorf("expected 2 *CopyOp methods, got %d", getListLen(t, result))
	}

	// Filter by returns
	result = callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("returns"), starlark.String("string")}},
	)
	if getListLen(t, result) != 2 {
		t.Errorf("expected 2 methods returning string, got %d", getListLen(t, result))
	}

	// Check scope is set
	first := getListItem(t, result, 0)
	scope := getStructAttr(t, first, "scope")
	if scope == "" {
		t.Error("expected non-empty scope")
	}

	// Check doc comment
	doc := getStructAttr(t, first, "doc")
	if doc == "" {
		t.Log("doc may be empty depending on which method is first")
	}

	// MoveOp.Name has value receiver
	result = callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("receiver_type"), starlark.String("*MoveOp")}},
	)
	if getListLen(t, result) != 0 {
		t.Errorf("expected 0 *MoveOp methods (MoveOp uses value receiver), got %d", getListLen(t, result))
	}
}

func TestGoFuncs(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"funcs.go": `package example

// buildSystemPrompt creates the system prompt.
func buildSystemPrompt() string {
	return "hello"
}

func helperFunc(a, b int) (int, error) {
	return a + b, nil
}

type Foo struct{}
func (f *Foo) Method() {}
`,
	})

	r := NewGoReceiver()

	// All functions (excludes methods)
	result := callMethod(t, r, "funcs", starlark.Tuple{starlark.String(dir)}, nil)
	if getListLen(t, result) != 2 {
		t.Fatalf("expected 2 functions, got %d", getListLen(t, result))
	}

	// Filter by name
	result = callMethod(t, r, "funcs",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("buildSystemPrompt")}},
	)
	if getListLen(t, result) != 1 {
		t.Fatalf("expected 1 function, got %d", getListLen(t, result))
	}

	fn := getListItem(t, result, 0)
	if name := getStructAttr(t, fn, "name"); name != "buildSystemPrompt" {
		t.Errorf("name: got %q, want %q", name, "buildSystemPrompt")
	}
	if returns := getStructAttr(t, fn, "returns"); returns != "string" {
		t.Errorf("returns: got %q, want %q", returns, "string")
	}
	if doc := getStructAttr(t, fn, "doc"); doc != "buildSystemPrompt creates the system prompt." {
		t.Errorf("doc: got %q", doc)
	}
	if scope := getStructAttr(t, fn, "scope"); scope == "" {
		t.Error("expected non-empty scope")
	}
}

func TestGoCalls(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"handler.go": `package example

type Builder struct{}

func (b *Builder) Attr(name string) {
	switch name {
	case "install":
		NewBuiltin("plan.install", b.handleInstall)
	case "remove":
		NewBuiltin("plan.remove", b.handleRemove)
	}
}

func NewBuiltin(name string, handler interface{}) {}
`,
	})

	r := NewGoReceiver()

	// Get the Attr method's scope
	methods := callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("Attr")}},
	)
	if getListLen(t, methods) != 1 {
		t.Fatalf("expected 1 Attr method, got %d", getListLen(t, methods))
	}
	scope := getStructAttr(t, getListItem(t, methods, 0), "scope")

	// Find all calls in the scope
	result := callMethod(t, r, "calls", starlark.Tuple{starlark.String(scope)}, nil)
	if getListLen(t, result) < 2 {
		t.Fatalf("expected at least 2 calls, got %d", getListLen(t, result))
	}

	// Filter by name
	result = callMethod(t, r, "calls",
		starlark.Tuple{starlark.String(scope)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("NewBuiltin")}},
	)
	if getListLen(t, result) != 2 {
		t.Fatalf("expected 2 NewBuiltin calls, got %d", getListLen(t, result))
	}

	// Check first call's args
	call := getListItem(t, result, 0).(*starlarkstruct.Struct)
	argsAttr, _ := call.Attr("args")
	args := argsAttr.(*starlark.List)
	if args.Len() != 2 {
		t.Fatalf("expected 2 args, got %d", args.Len())
	}

	// First arg should be the string "plan.install"
	firstArg := args.Index(0).(*starlarkstruct.Struct)
	strVal, _ := firstArg.Attr("string_value")
	if str, _ := starlark.AsString(strVal); str != "plan.install" {
		t.Errorf("first arg string_value: got %q, want %q", str, "plan.install")
	}

	// Second arg should have ident_name "handleInstall" (from b.handleInstall)
	secondArg := args.Index(1).(*starlarkstruct.Struct)
	identVal, _ := secondArg.Attr("ident_name")
	if str, _ := starlark.AsString(identVal); str != "handleInstall" {
		t.Errorf("second arg ident_name: got %q, want %q", str, "handleInstall")
	}
}

func TestGoComposites(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"graph.go": `package example

type Node struct {
	Name       string
	Operations []string
}

func (b *Builder) buildGraph() {
	n := Node{
		Name:       "test",
		Operations: []string{"copy", "move"},
	}
	_ = n
}

type Builder struct{}
`,
	})

	r := NewGoReceiver()

	methods := callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("buildGraph")}},
	)
	scope := getStructAttr(t, getListItem(t, methods, 0), "scope")

	// All composites
	result := callMethod(t, r, "composites", starlark.Tuple{starlark.String(scope)}, nil)
	if getListLen(t, result) < 1 {
		t.Fatalf("expected at least 1 composite, got %d", getListLen(t, result))
	}

	// Filter by type
	result = callMethod(t, r, "composites",
		starlark.Tuple{starlark.String(scope)},
		[]starlark.Tuple{{starlark.String("type"), starlark.String("Node")}},
	)
	if getListLen(t, result) != 1 {
		t.Fatalf("expected 1 Node composite, got %d", getListLen(t, result))
	}

	comp := getListItem(t, result, 0).(*starlarkstruct.Struct)
	fieldsAttr, _ := comp.Attr("fields")
	fields := fieldsAttr.(*starlarkstruct.Struct)
	nameAttr, _ := fields.Attr("Name")
	if str, _ := starlark.AsString(nameAttr); str != "test" {
		t.Errorf("Name field: got %q, want %q", str, "test")
	}

	// Operations field should be a list of strings (nested composite)
	opsAttr, err := fields.Attr("Operations")
	if err != nil {
		t.Fatalf("Operations field not found: %v", err)
	}
	opsList, ok := opsAttr.(*starlark.List)
	if !ok {
		t.Fatalf("Operations: expected list, got %T", opsAttr)
	}
	if opsList.Len() != 2 {
		t.Errorf("Operations: expected 2 items, got %d", opsList.Len())
	}
	if str, _ := starlark.AsString(opsList.Index(0)); str != "copy" {
		t.Errorf("Operations[0]: got %q, want %q", str, "copy")
	}
}

func TestGoReturnString(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"ops.go": `package example

type CopyOp struct{}

func (c *CopyOp) Name() string {
	return "copy"
}

func (c *CopyOp) Execute() error {
	return nil
}
`,
	})

	r := NewGoReceiver()

	methods := callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{
			{starlark.String("name"), starlark.String("Name")},
			{starlark.String("receiver_type"), starlark.String("CopyOp")},
		},
	)
	if getListLen(t, methods) != 1 {
		t.Fatalf("expected 1 method, got %d", getListLen(t, methods))
	}
	scope := getStructAttr(t, getListItem(t, methods, 0), "scope")

	result := callMethod(t, r, "return_string", starlark.Tuple{starlark.String(scope)}, nil)
	if str, _ := starlark.AsString(result); str != "copy" {
		t.Errorf("return_string: got %q, want %q", str, "copy")
	}
}

func TestGoRawString(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"plan.go": `package example

func buildSystemPrompt() string {
	return ` + "`" + `You are a migration assistant.

Known platforms: linux, darwin, windows
` + "`" + `
}
`,
	})

	r := NewGoReceiver()

	funcs := callMethod(t, r, "funcs",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("buildSystemPrompt")}},
	)
	if getListLen(t, funcs) != 1 {
		t.Fatalf("expected 1 function, got %d", getListLen(t, funcs))
	}
	scope := getStructAttr(t, getListItem(t, funcs, 0), "scope")

	result := callMethod(t, r, "raw_string", starlark.Tuple{starlark.String(scope)}, nil)
	str, _ := starlark.AsString(result)
	if str == "" {
		t.Fatal("expected non-empty raw string")
	}
	if !contains(str, "migration assistant") {
		t.Errorf("raw_string should contain 'migration assistant', got %q", str)
	}
	if !contains(str, "Known platforms:") {
		t.Errorf("raw_string should contain 'Known platforms:', got %q", str)
	}
}

func TestGoReturnStrings(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"provider.go": `package example

type Provider struct{}

func (p *Provider) AttrNames() []string {
	return []string{"backup", "copy", "link", "mkdir"}
}
`,
	})

	r := NewGoReceiver()

	methods := callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{
			{starlark.String("name"), starlark.String("AttrNames")},
			{starlark.String("receiver_type"), starlark.String("Provider")},
		},
	)
	if getListLen(t, methods) != 1 {
		t.Fatalf("expected 1 method, got %d", getListLen(t, methods))
	}
	scope := getStructAttr(t, getListItem(t, methods, 0), "scope")

	result := callMethod(t, r, "return_strings", starlark.Tuple{starlark.String(scope)}, nil)
	list, ok := result.(*starlark.List)
	if !ok {
		t.Fatalf("expected list, got %T", result)
	}

	expected := []string{"backup", "copy", "link", "mkdir"}
	if list.Len() != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), list.Len())
	}
	for i, want := range expected {
		got, _ := starlark.AsString(list.Index(i))
		if got != want {
			t.Errorf("element[%d]: got %q, want %q", i, got, want)
		}
	}
}

func TestGoScope_InvalidScope(t *testing.T) {
	r := NewGoReceiver()

	// Test invalid scope format
	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("return_string")
	fn := attr.(*starlark.Builtin)
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String("invalid-no-separator")}, nil)
	if err == nil {
		t.Error("expected error for invalid scope")
	}
}

func TestGoScope_NotFound(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"empty.go": `package example
`,
	})

	r := NewGoReceiver()
	thread := &starlark.Thread{Name: "test"}
	attr, _ := r.Attr("return_string")
	fn := attr.(*starlark.Builtin)
	scope := filepath.Join(dir, "empty.go") + "::NonExistent"
	_, err := fn.CallInternal(thread, starlark.Tuple{starlark.String(scope)}, nil)
	if err == nil {
		t.Error("expected error for non-existent scope")
	}
}

func TestGoFileCache(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"cached.go": `package example

type Foo struct {
	Bar string
}

func Hello() string { return "hello" }
`,
	})

	r := NewGoReceiver()

	// First call parses the file
	callMethod(t, r, "structs", starlark.Tuple{starlark.String(dir)}, nil)

	// Second call should use cache
	callMethod(t, r, "funcs", starlark.Tuple{starlark.String(dir)}, nil)

	// Verify both return correct data
	structs := callMethod(t, r, "structs", starlark.Tuple{starlark.String(dir)}, nil)
	if getListLen(t, structs) != 1 {
		t.Errorf("expected 1 struct, got %d", getListLen(t, structs))
	}
	funcs := callMethod(t, r, "funcs", starlark.Tuple{starlark.String(dir)}, nil)
	if getListLen(t, funcs) != 1 {
		t.Errorf("expected 1 func, got %d", getListLen(t, funcs))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGoFuncsParams(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"params.go": `package example

type Node struct{}
type Result[T any] struct{}

func singleParam(x string) {}
func multiSameType(a, b int) {}
func multiDiffType(name string, count int, verbose bool) {}
func variadicFunc(prefix string, items ...string) {}
func pointerParam(node *Node) {}
func sliceParam(items []string) {}
func mapParam(data map[string]int) {}
func noParams() {}
func interfaceParam(v interface{}) {}
func funcParam(fn func(string) error) {}
func chanParam(ch chan string) {}
func genericParam(r Result[string]) {}
`,
	})

	r := NewGoReceiver()

	tests := []struct {
		name   string
		params []struct {
			name     string
			typ      string
			variadic bool
		}
	}{
		{
			name: "singleParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"x", "string", false},
			},
		},
		{
			name: "multiSameType",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"a", "int", false},
				{"b", "int", false},
			},
		},
		{
			name: "multiDiffType",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"name", "string", false},
				{"count", "int", false},
				{"verbose", "bool", false},
			},
		},
		{
			name: "variadicFunc",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"prefix", "string", false},
				{"items", "string", true},
			},
		},
		{
			name: "pointerParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"node", "*Node", false},
			},
		},
		{
			name: "sliceParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"items", "[]string", false},
			},
		},
		{
			name: "mapParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"data", "map[string]int", false},
			},
		},
		{
			name: "noParams",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{},
		},
		{
			name: "interfaceParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"v", "any", false},
			},
		},
		{
			name: "funcParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"fn", "func(string) error", false},
			},
		},
		{
			name: "chanParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"ch", "chan string", false},
			},
		},
		{
			name: "genericParam",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"r", "Result[string]", false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callMethod(t, r, "funcs",
				starlark.Tuple{starlark.String(dir)},
				[]starlark.Tuple{{starlark.String("name"), starlark.String(tc.name)}},
			)
			if getListLen(t, result) != 1 {
				t.Fatalf("expected 1 function %q, got %d", tc.name, getListLen(t, result))
			}
			fn := getListItem(t, result, 0)
			params := fn.(*starlarkstruct.Struct)
			paramsAttr, err := params.Attr("params")
			if err != nil {
				t.Fatalf("params attr: %v", err)
			}
			paramsList := paramsAttr.(*starlark.List)
			if paramsList.Len() != len(tc.params) {
				t.Fatalf("expected %d params, got %d", len(tc.params), paramsList.Len())
			}
			for i, expected := range tc.params {
				p := paramsList.Index(i)
				if got := getStructAttr(t, p, "name"); got != expected.name {
					t.Errorf("param[%d] name: got %q, want %q", i, got, expected.name)
				}
				if got := getStructAttr(t, p, "type"); got != expected.typ {
					t.Errorf("param[%d] type: got %q, want %q", i, got, expected.typ)
				}
				if got := getBoolAttr(t, p, "variadic"); got != expected.variadic {
					t.Errorf("param[%d] variadic: got %v, want %v", i, got, expected.variadic)
				}
			}
		})
	}
}

func TestGoMethodsParams(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"fileops.go": `package example

type FileOps struct{}

func (f *FileOps) Copy(source, dest string) error {
	return nil
}

func (f *FileOps) Install(packages ...string) error {
	return nil
}

func (f *FileOps) Check() bool {
	return true
}
`,
	})

	r := NewGoReceiver()

	tests := []struct {
		name   string
		params []struct {
			name     string
			typ      string
			variadic bool
		}
	}{
		{
			name: "Copy",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"source", "string", false},
				{"dest", "string", false},
			},
		},
		{
			name: "Install",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{
				{"packages", "string", true},
			},
		},
		{
			name: "Check",
			params: []struct {
				name     string
				typ      string
				variadic bool
			}{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callMethod(t, r, "methods",
				starlark.Tuple{starlark.String(dir)},
				[]starlark.Tuple{{starlark.String("name"), starlark.String(tc.name)}},
			)
			if getListLen(t, result) != 1 {
				t.Fatalf("expected 1 method %q, got %d", tc.name, getListLen(t, result))
			}
			method := getListItem(t, result, 0)
			s := method.(*starlarkstruct.Struct)
			paramsAttr, err := s.Attr("params")
			if err != nil {
				t.Fatalf("params attr: %v", err)
			}
			paramsList := paramsAttr.(*starlark.List)
			if paramsList.Len() != len(tc.params) {
				t.Fatalf("expected %d params, got %d", len(tc.params), paramsList.Len())
			}
			for i, expected := range tc.params {
				p := paramsList.Index(i)
				if got := getStructAttr(t, p, "name"); got != expected.name {
					t.Errorf("param[%d] name: got %q, want %q", i, got, expected.name)
				}
				if got := getStructAttr(t, p, "type"); got != expected.typ {
					t.Errorf("param[%d] type: got %q, want %q", i, got, expected.typ)
				}
				if got := getBoolAttr(t, p, "variadic"); got != expected.variadic {
					t.Errorf("param[%d] variadic: got %v, want %v", i, got, expected.variadic)
				}
			}
		})
	}
}

func TestGoTypeDoc(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"provider.go": `package example

// Provider provides file system actions.
//
// Compensable Forward methods return (map[string]any, error).
//
//devlore:plannable
type Provider struct{}
`,
		"ui.go": `package example

// UIHelper provides user-facing terminal messaging.
type UIHelper struct{}
`,
		"grouped.go": `package example

type (
	// GroupedType is inside a grouped declaration.
	GroupedType struct{}
)
`,
	})

	r := NewGoReceiver()

	// Default name="Provider" — should find the plannable doc
	result := callMethod(t, r, "type_doc", starlark.Tuple{starlark.String(dir)}, nil)
	doc, _ := starlark.AsString(result)
	if !contains(doc, "file system actions") {
		t.Errorf("expected doc to contain 'file system actions', got %q", doc)
	}
	if !contains(doc, "devlore:plannable") {
		t.Errorf("expected doc to contain 'devlore:plannable', got %q", doc)
	}

	// Explicit name — find UIHelper
	result = callMethod(t, r, "type_doc",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("UIHelper")}},
	)
	doc, _ = starlark.AsString(result)
	if !contains(doc, "terminal messaging") {
		t.Errorf("expected UIHelper doc to contain 'terminal messaging', got %q", doc)
	}
	if contains(doc, "devlore:plannable") {
		t.Errorf("UIHelper should not have plannable directive, got %q", doc)
	}

	// Grouped type — doc is on TypeSpec
	result = callMethod(t, r, "type_doc",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("GroupedType")}},
	)
	doc, _ = starlark.AsString(result)
	if !contains(doc, "grouped declaration") {
		t.Errorf("expected GroupedType doc to contain 'grouped declaration', got %q", doc)
	}

	// Non-existent type — returns empty string
	result = callMethod(t, r, "type_doc",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("NonExistent")}},
	)
	doc, _ = starlark.AsString(result)
	if doc != "" {
		t.Errorf("expected empty doc for non-existent type, got %q", doc)
	}
}

func TestParseParamDocs(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		wantDoc  string
		wantDocs map[string]string
	}{
		{
			name:     "empty",
			doc:      "",
			wantDoc:  "",
			wantDocs: map[string]string{},
		},
		{
			name:     "no Parameters section",
			doc:      "Link creates a symlink at path pointing to source.",
			wantDoc:  "Link creates a symlink at path pointing to source.",
			wantDocs: map[string]string{},
		},
		{
			name:     "with Parameters section",
			doc:      "Link creates a symlink.\n\nParameters:\n  - source: Absolute path to the symlink target\n  - path: Absolute path where the symlink will be created",
			wantDoc:  "Link creates a symlink.",
			wantDocs: map[string]string{"source": "Absolute path to the symlink target", "path": "Absolute path where the symlink will be created"},
		},
		{
			name:     "Parameters only",
			doc:      "Parameters:\n  - name: Package name",
			wantDoc:  "",
			wantDocs: map[string]string{"name": "Package name"},
		},
		{
			name:     "content after Parameters",
			doc:      "Copy writes content.\n\nParameters:\n  - path: Destination path\n  - mode: File permissions\n\nReturns: checksum",
			wantDoc:  "Copy writes content.\n\nReturns: checksum",
			wantDocs: map[string]string{"path": "Destination path", "mode": "File permissions"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotDoc, gotDocs := parseParamDocs(tc.doc)
			if gotDoc != tc.wantDoc {
				t.Errorf("cleanDoc:\n  got:  %q\n  want: %q", gotDoc, tc.wantDoc)
			}
			if len(gotDocs) != len(tc.wantDocs) {
				t.Errorf("paramDocs length: got %d, want %d", len(gotDocs), len(tc.wantDocs))
			}
			for k, wantV := range tc.wantDocs {
				if gotV, ok := gotDocs[k]; !ok {
					t.Errorf("missing param doc for %q", k)
				} else if gotV != wantV {
					t.Errorf("param doc %q: got %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

func TestGoMethodsParamDocs(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"provider.go": `package example

type Provider struct{}

// Link creates a symlink at path pointing to source.
//
// Parameters:
//   - source: Absolute path to the symlink target
//   - path: Absolute path where the symlink will be created
func (p *Provider) Link(source, path string) (string, error) {
	return path, nil
}

// NoDocs has no parameter documentation.
func (p *Provider) NoDocs(name string) (string, error) {
	return name, nil
}
`,
	})

	r := NewGoReceiver()

	// Link should have param docs stripped from method doc
	result := callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("Link")}},
	)
	if getListLen(t, result) != 1 {
		t.Fatalf("expected 1 Link method, got %d", getListLen(t, result))
	}
	link := getListItem(t, result, 0)
	doc := getStructAttr(t, link, "doc")
	if doc != "Link creates a symlink at path pointing to source." {
		t.Errorf("doc should be description only, got %q", doc)
	}

	// Check param docs are populated
	s := link.(*starlarkstruct.Struct)
	paramsAttr, _ := s.Attr("params")
	params := paramsAttr.(*starlark.List)
	if params.Len() != 2 {
		t.Fatalf("expected 2 params, got %d", params.Len())
	}
	sourceDoc := getStructAttr(t, params.Index(0), "doc")
	if sourceDoc != "Absolute path to the symlink target" {
		t.Errorf("source doc: got %q", sourceDoc)
	}
	pathDoc := getStructAttr(t, params.Index(1), "doc")
	if pathDoc != "Absolute path where the symlink will be created" {
		t.Errorf("path doc: got %q", pathDoc)
	}

	// NoDocs should have empty param docs
	result = callMethod(t, r, "methods",
		starlark.Tuple{starlark.String(dir)},
		[]starlark.Tuple{{starlark.String("name"), starlark.String("NoDocs")}},
	)
	noDocs := getListItem(t, result, 0)
	s2 := noDocs.(*starlarkstruct.Struct)
	paramsAttr2, _ := s2.Attr("params")
	params2 := paramsAttr2.(*starlark.List)
	nameDoc := getStructAttr(t, params2.Index(0), "doc")
	if nameDoc != "" {
		t.Errorf("expected empty doc for NoDocs.name, got %q", nameDoc)
	}
}

func TestTypeToStringExtended(t *testing.T) {
	dir := writeTempDir(t, map[string]string{
		"showcase.go": `package example

type Result[T any] struct{}

type TypeShowcase struct {
	AnyField     interface{}
	FuncField    func(string) error
	ChanField    chan string
	SendChan     chan<- int
	RecvChan     <-chan bool
	GenericField Result[string]
}
`,
	})

	r := NewGoReceiver()
	result := callMethod(t, r, "structs", starlark.Tuple{starlark.String(dir)}, nil)

	// Find TypeShowcase
	var showcase starlark.Value
	for i := 0; i < getListLen(t, result); i++ {
		item := getListItem(t, result, i)
		if getStructAttr(t, item, "name") == "TypeShowcase" {
			showcase = item
			break
		}
	}
	if showcase == nil {
		t.Fatal("TypeShowcase struct not found")
	}

	s := showcase.(*starlarkstruct.Struct)
	fieldsAttr, _ := s.Attr("fields")
	fields := fieldsAttr.(*starlark.List)

	expected := []struct {
		name string
		typ  string
	}{
		{"AnyField", "any"},
		{"FuncField", "func(string) error"},
		{"ChanField", "chan string"},
		{"SendChan", "chan<- int"},
		{"RecvChan", "<-chan bool"},
		{"GenericField", "Result[string]"},
	}

	if fields.Len() != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), fields.Len())
	}

	for i, exp := range expected {
		field := fields.Index(i)
		if got := getStructAttr(t, field, "name"); got != exp.name {
			t.Errorf("field[%d] name: got %q, want %q", i, got, exp.name)
		}
		if got := getStructAttr(t, field, "type"); got != exp.typ {
			t.Errorf("field[%d] type: got %q, want %q", i, got, exp.typ)
		}
	}
}
