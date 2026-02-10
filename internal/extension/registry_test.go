// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

import (
	"fmt"
	"sync"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	spec := &ExtensionSpec{
		Extension:   "com.example.TestExtension",
		Description: "Test extension",
		Commands: []CommandSpec{
			{Name: "test.example", Help: "Test", Implementation: "commands/test.star"},
		},
	}

	if err := r.Register(spec); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := r.Get("com.example.TestExtension")
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Extension != "com.example.TestExtension" {
		t.Errorf("got.Extension = %q, want %q", got.Extension, "com.example.TestExtension")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	got := r.Get("nonexistent")
	if got != nil {
		t.Error("Get(nonexistent) should return nil")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "test", Help: "Test", Implementation: "commands/test.star"},
		},
	}

	if err := r.Register(spec); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err := r.Register(spec)
	if err == nil {
		t.Error("second Register should fail for duplicate")
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	err := r.Register(nil)
	if err == nil {
		t.Error("Register(nil) should fail")
	}
}

func TestRegistry_All(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	specs := []*ExtensionSpec{
		{Extension: "com.example.A", Commands: []CommandSpec{{Name: "a", Help: "A", Implementation: "commands/a.star"}}},
		{Extension: "com.example.B", Commands: []CommandSpec{{Name: "b", Help: "B", Implementation: "commands/b.star"}}},
		{Extension: "com.example.C", Commands: []CommandSpec{{Name: "c", Help: "C", Implementation: "commands/c.star"}}},
	}

	for _, spec := range specs {
		if err := r.Register(spec); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	all := r.All()
	if len(all) != 3 {
		t.Errorf("len(All()) = %d, want 3", len(all))
	}

	// Verify it's a copy by modifying the returned map
	delete(all, "com.example.A")
	if r.Get("com.example.A") == nil {
		t.Error("modifying All() result affected registry")
	}
}

func TestRegistry_Names(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	specs := []*ExtensionSpec{
		{Extension: "com.example.C", Commands: []CommandSpec{{Name: "c", Help: "C", Implementation: "commands/c.star"}}},
		{Extension: "com.example.A", Commands: []CommandSpec{{Name: "a", Help: "A", Implementation: "commands/a.star"}}},
		{Extension: "com.example.B", Commands: []CommandSpec{{Name: "b", Help: "B", Implementation: "commands/b.star"}}},
	}

	for _, spec := range specs {
		if err := r.Register(spec); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("len(Names()) = %d, want 3", len(names))
	}

	// Should be sorted
	expected := []string{"com.example.A", "com.example.B", "com.example.C"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestRegistry_Clear(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	spec := &ExtensionSpec{
		Extension: "com.example.Test",
		Commands: []CommandSpec{
			{Name: "test", Help: "Test", Implementation: "commands/test.star"},
		},
	}

	if err := r.Register(spec); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1", r.Count())
	}

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("Count() after Clear = %d, want 0", r.Count())
	}

	if r.Get("com.example.Test") != nil {
		t.Error("Get after Clear should return nil")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := &Registry{specs: make(map[string]*ExtensionSpec)}

	// Pre-register some extensions
	for i := 0; i < 10; i++ {
		spec := &ExtensionSpec{
			Extension: fmt.Sprintf("com.example.Ext%d", i),
			Commands: []CommandSpec{
				{Name: fmt.Sprintf("ext%d", i), Help: "Test", Implementation: "commands/test.star"},
			},
		}
		if err := r.Register(spec); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = r.Get(fmt.Sprintf("com.example.Ext%d", n%10))
			_ = r.All()
			_ = r.Names()
			_ = r.Count()
		}(i)
	}

	// Concurrent writes (new extensions)
	for i := 10; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			spec := &ExtensionSpec{
				Extension: fmt.Sprintf("com.example.Ext%d", n),
				Commands: []CommandSpec{
					{Name: fmt.Sprintf("ext%d", n), Help: "Test", Implementation: "commands/test.star"},
				},
			}
			if err := r.Register(spec); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent Register error: %v", err)
	}

	// Should have 20 extensions now
	if r.Count() != 20 {
		t.Errorf("Count() = %d, want 20", r.Count())
	}
}

// Test global registry functions
func TestGlobalRegistry(t *testing.T) {
	// Clear global registry first
	Clear()

	spec := &ExtensionSpec{
		Extension: "com.example.GlobalTest",
		Commands: []CommandSpec{
			{Name: "global.test", Help: "Test", Implementation: "commands/test.star"},
		},
	}

	if err := Register(spec); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if Get("com.example.GlobalTest") == nil {
		t.Error("Get returned nil")
	}

	if Count() != 1 {
		t.Errorf("Count() = %d, want 1", Count())
	}

	names := Names()
	if len(names) != 1 || names[0] != "com.example.GlobalTest" {
		t.Errorf("Names() = %v, want [com.example.GlobalTest]", names)
	}

	all := All()
	if len(all) != 1 {
		t.Errorf("len(All()) = %d, want 1", len(all))
	}

	Clear()
	if Count() != 0 {
		t.Error("Count after Clear should be 0")
	}
}
