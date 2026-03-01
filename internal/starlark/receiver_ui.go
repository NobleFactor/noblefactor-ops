// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/pkg/op"
	"github.com/NobleFactor/devlore-cli/pkg/op/provider/ui"
)

// UiReceiver wraps a *ui.Provider as a Starlark HasAttrs value.
// Implements starlark.Value and starlark.HasAttrs.
type UiReceiver struct {
	op.Receiver
	provider *ui.Provider
}

// NewUiReceiver creates a new UiReceiver.
func NewUiReceiver(p *ui.Provider) *UiReceiver {
	return &UiReceiver{
		Receiver: op.NewReceiver("ui"),
		provider: p,
	}
}

// Attr implements starlark.HasAttrs.
func (r *UiReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "note":
		return op.MakeAttr("ui.note", r.note), nil
	case "warn":
		return op.MakeAttr("ui.warn", r.warn), nil
	case "error":
		return op.MakeAttr("ui.error", r.uiError), nil
	case "success":
		return op.MakeAttr("ui.success", r.success), nil
	case "fail":
		return op.MakeAttr("ui.fail", r.fail), nil
	default:
		return nil, op.NoSuchAttrError("ui", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *UiReceiver) AttrNames() []string {
	return []string{"error", "fail", "note", "success", "warn"}
}

func (r *UiReceiver) note(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	if err := starlark.UnpackArgs("ui.note", args, kwargs, "msg", &msg); err != nil {
		return nil, err
	}
	r.provider.Note(msg)
	return starlark.None, nil
}

func (r *UiReceiver) warn(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	if err := starlark.UnpackArgs("ui.warn", args, kwargs, "msg", &msg); err != nil {
		return nil, err
	}
	r.provider.Warn(msg)
	return starlark.None, nil
}

func (r *UiReceiver) uiError(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	if err := starlark.UnpackArgs("ui.error", args, kwargs, "msg", &msg); err != nil {
		return nil, err
	}
	r.provider.Error(msg)
	return starlark.None, nil
}

func (r *UiReceiver) success(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	if err := starlark.UnpackArgs("ui.success", args, kwargs, "msg", &msg); err != nil {
		return nil, err
	}
	r.provider.Success(msg)
	return starlark.None, nil
}

func (r *UiReceiver) fail(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var msg string
	if err := starlark.UnpackArgs("ui.fail", args, kwargs, "msg", &msg); err != nil {
		return nil, err
	}
	return nil, r.provider.Fail(msg)
}
