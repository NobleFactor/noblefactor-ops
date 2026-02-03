// SPDX-License-Identifier: MIT
// Copyright (c) 2025 Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"os"
	"path/filepath"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/noblefactor-ops/internal/cli"
)

// fsModule returns the fs module with file system operations.
func fsModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "fs",
		Members: starlark.StringDict{
			"read":       starlark.NewBuiltin("fs.read", fsRead),
			"write":      starlark.NewBuiltin("fs.write", fsWrite),
			"exists":     starlark.NewBuiltin("fs.exists", fsExists),
			"is_dir":     starlark.NewBuiltin("fs.is_dir", fsIsDir),
			"is_file":    starlark.NewBuiltin("fs.is_file", fsIsFile),
			"list_dir":   starlark.NewBuiltin("fs.list_dir", fsListDir),
			"join":       starlark.NewBuiltin("fs.join", fsJoin),
			"basename":   starlark.NewBuiltin("fs.basename", fsBasename),
			"dirname":    starlark.NewBuiltin("fs.dirname", fsDirname),
			"glob":       starlark.NewBuiltin("fs.glob", fsGlob),
			"mkdir":      starlark.NewBuiltin("fs.mkdir", fsMkdir),
			"remove":     starlark.NewBuiltin("fs.remove", fsRemove),
			"remove_all": starlark.NewBuiltin("fs.remove_all", fsRemoveAll),
		},
	}
}

func fsRead(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.read", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fs.read: %w", err)
	}
	return starlark.String(data), nil
}

func fsWrite(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, content string
	if err := starlark.UnpackArgs("fs.write", args, kwargs, "path", &path, "content", &content); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would write %d bytes to %s", len(content), path)
		return starlark.None, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("fs.write: %w", err)
	}
	return starlark.None, nil
}

func fsExists(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.exists", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	_, err := os.Stat(path)
	return starlark.Bool(!os.IsNotExist(err)), nil
}

func fsIsDir(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.is_dir", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return starlark.Bool(false), nil
	}
	if err != nil {
		return nil, fmt.Errorf("fs.is_dir: %w", err)
	}
	return starlark.Bool(info.IsDir()), nil
}

func fsIsFile(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.is_file", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return starlark.Bool(false), nil
	}
	if err != nil {
		return nil, fmt.Errorf("fs.is_file: %w", err)
	}
	return starlark.Bool(!info.IsDir()), nil
}

func fsListDir(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.list_dir", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("fs.list_dir: %w", err)
	}

	var items []starlark.Value
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		item := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"name":   starlark.String(entry.Name()),
			"path":   starlark.String(entryPath),
			"is_dir": starlark.Bool(entry.IsDir()),
		})
		items = append(items, item)
	}
	return starlark.NewList(items), nil
}

func fsJoin(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		return starlark.String(""), nil
	}
	parts := make([]string, len(args))
	for i, arg := range args {
		s, ok := starlark.AsString(arg)
		if !ok {
			return nil, fmt.Errorf("fs.join: argument %d is not a string", i)
		}
		parts[i] = s
	}
	return starlark.String(filepath.Join(parts...)), nil
}

func fsBasename(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.basename", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	return starlark.String(filepath.Base(path)), nil
}

func fsDirname(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.dirname", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	return starlark.String(filepath.Dir(path)), nil
}

func fsGlob(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern string
	if err := starlark.UnpackArgs("fs.glob", args, kwargs, "pattern", &pattern); err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("fs.glob: %w", err)
	}
	var items []starlark.Value
	for _, m := range matches {
		items = append(items, starlark.String(m))
	}
	return starlark.NewList(items), nil
}

func fsMkdir(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.mkdir", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would create directory %s", path)
		return starlark.None, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("fs.mkdir: %w", err)
	}
	return starlark.None, nil
}

func fsRemove(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.remove", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would remove %s", path)
		return starlark.None, nil
	}
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("fs.remove: %w", err)
	}
	return starlark.None, nil
}

func fsRemoveAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("fs.remove_all", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would remove %s (recursively)", path)
		return starlark.None, nil
	}
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("fs.remove_all: %w", err)
	}
	return starlark.None, nil
}
