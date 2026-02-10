// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Receiver is the interface that all binding function receivers implement.
// It extends starlark.Value and starlark.HasAttrs.
type Receiver interface {
	starlark.Value
	starlark.HasAttrs
}

// BaseReceiver provides common implementations for receiver types.
// Embed this in concrete receiver types to satisfy starlark.Value.
type BaseReceiver struct {
	name string
}

// NewBaseReceiver creates a new BaseReceiver with the given name.
func NewBaseReceiver(name string) BaseReceiver {
	return BaseReceiver{name: name}
}

// String implements starlark.Value.
func (r BaseReceiver) String() string {
	return r.name
}

// Type implements starlark.Value.
func (r BaseReceiver) Type() string {
	return r.name
}

// Freeze implements starlark.Value.
func (r BaseReceiver) Freeze() {}

// Truth implements starlark.Value.
func (r BaseReceiver) Truth() starlark.Bool {
	return true
}

// Hash implements starlark.Value.
func (r BaseReceiver) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable type: %s", r.name)
}

// NoSuchAttrError returns an error for an unknown attribute.
func NoSuchAttrError(receiver, attr string) error {
	return fmt.Errorf("%s has no .%s attribute", receiver, attr)
}

// BuiltinFunc is the signature for builtin function implementations.
type BuiltinFunc func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

// MakeAttr creates a starlark.Builtin from a receiver method.
func MakeAttr(name string, fn BuiltinFunc) starlark.Value {
	return starlark.NewBuiltin(name, fn)
}
