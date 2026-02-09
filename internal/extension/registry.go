// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds registered extensions.
type Registry struct {
	mu    sync.RWMutex
	specs map[string]*ExtensionSpec
}

// registry is the global extension registry.
var registry = &Registry{
	specs: make(map[string]*ExtensionSpec),
}

// Register adds an extension to the global registry.
// Returns an error if an extension with the same name is already registered.
func Register(spec *ExtensionSpec) error {
	return registry.Register(spec)
}

// Get returns an extension by name from the global registry.
// Returns nil if not found.
func Get(name string) *ExtensionSpec {
	return registry.Get(name)
}

// All returns a copy of all registered extensions from the global registry.
func All() map[string]*ExtensionSpec {
	return registry.All()
}

// Names returns a sorted list of registered extension names.
func Names() []string {
	return registry.Names()
}

// Clear removes all extensions from the global registry.
// Primarily useful for testing.
func Clear() {
	registry.Clear()
}

// Register adds an extension to the registry.
func (r *Registry) Register(spec *ExtensionSpec) error {
	if spec == nil {
		return fmt.Errorf("cannot register nil extension spec")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.specs[spec.Extension]; exists {
		return fmt.Errorf("extension %q already registered", spec.Extension)
	}

	r.specs[spec.Extension] = spec
	return nil
}

// Get returns an extension by name.
// Returns nil if not found.
func (r *Registry) Get(name string) *ExtensionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.specs[name]
}

// All returns a copy of all registered extensions.
func (r *Registry) All() map[string]*ExtensionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ExtensionSpec, len(r.specs))
	for k, v := range r.specs {
		result[k] = v
	}
	return result
}

// Names returns a sorted list of registered extension names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.specs))
	for name := range r.specs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Clear removes all extensions from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.specs = make(map[string]*ExtensionSpec)
}

// Count returns the number of registered extensions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.specs)
}

// Count returns the number of registered extensions in the global registry.
func Count() int {
	return registry.Count()
}
