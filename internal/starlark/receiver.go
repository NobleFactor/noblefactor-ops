// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Receiver provides common implementations for Starlark binding namespaces.
// Embed this in concrete types to satisfy starlark.Value. Concrete types
// must implement starlark.HasAttrs (Attr and AttrNames) themselves.
//
// This is a local copy of the devlore-cli receiver base type. Hand-coded
// receivers will be migrated to framework providers, at which point this
// type can be deleted.
type Receiver struct {
	name string
}

// NewReceiver creates a new receiver with the given namespace name.
//
// Parameters:
//   - name: the Starlark namespace name.
//
// Returns:
//   - Receiver: the receiver base.
func NewReceiver(name string) Receiver {
	return Receiver{name: name}
}

// region EXPORTED METHODS

// region State management

// String implements starlark.Value. Returns the receiver namespace name.
//
// Returns:
//   - string: the namespace name.
func (r Receiver) String() string { return r.name }

// Truth implements starlark.Value. Receivers are always truthy.
//
// Returns:
//   - starlark.Bool: always true.
func (r Receiver) Truth() starlark.Bool { return true }

// Type implements starlark.Value. Returns the receiver namespace name as its type.
//
// Returns:
//   - string: the namespace name.
func (r Receiver) Type() string { return r.name }

// endregion

// region Behaviors

// Freeze implements starlark.Value. Receivers are immutable; this is a no-op.
func (r Receiver) Freeze() {}

// Hash implements starlark.Value. Receivers are not hashable.
//
// Returns:
//   - uint32: unused.
//   - error: always non-nil.
func (r Receiver) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", r.name)
}

// endregion

// endregion

// NoSuchAttrError returns an error for an unknown attribute.
//
// Parameters:
//   - receiver: the receiver name for the error message.
//   - attr: the attribute name that was not found.
//
// Returns:
//   - error: the formatted error.
func NoSuchAttrError(receiver, attr string) error {
	return fmt.Errorf("%s has no .%s attribute", receiver, attr)
}

// MakeAttr creates a starlark.Builtin from a name and handler function.
//
// Parameters:
//   - name: the qualified method name (e.g., "schema.validate").
//   - fn: the handler function.
//
// Returns:
//   - *starlark.Builtin: the builtin value.
func MakeAttr(name string, fn BuiltinFunc) *starlark.Builtin {
	return starlark.NewBuiltin(name, fn)
}

// BuiltinFunc is the signature for builtin function implementations.
type BuiltinFunc func(
	thread *starlark.Thread, fn *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple,
) (starlark.Value, error)
