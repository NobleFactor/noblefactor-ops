// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/NobleFactor/devlore-cli/pkg/op"
	"github.com/NobleFactor/noblefactor-ops/internal/cli"
	"github.com/NobleFactor/noblefactor-ops/internal/ignore"
)

// FileReceiver provides file system operations.
// Implements starlark.Value and starlark.HasAttrs.
type FileReceiver struct {
	op.Receiver
}

// NewFileReceiver creates a new FileReceiver.
func NewFileReceiver() *FileReceiver {
	return &FileReceiver{Receiver: op.NewReceiver("file")}
}

// Attr implements starlark.HasAttrs.
func (r *FileReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "read":
		return op.MakeAttr("file.read", r.read), nil
	case "write":
		return op.MakeAttr("file.write", r.write), nil
	case "exists":
		return op.MakeAttr("file.exists", r.exists), nil
	case "is_directory":
		return op.MakeAttr("file.is_directory", r.isDirectory), nil
	case "is_file":
		return op.MakeAttr("file.is_file", r.isFile), nil
	case "list":
		return op.MakeAttr("file.list", r.list), nil
	case "join":
		return op.MakeAttr("file.join", r.join), nil
	case "name":
		return op.MakeAttr("file.name", r.name), nil
	case "parent":
		return op.MakeAttr("file.parent", r.parent), nil
	case "glob":
		return op.MakeAttr("file.glob", r.glob), nil
	case "walk_tree":
		return op.MakeAttr("file.walk_tree", r.walkTree), nil
	case "mkdir":
		return op.MakeAttr("file.mkdir", r.mkdir), nil
	case "remove":
		return op.MakeAttr("file.remove", r.remove), nil
	case "remove_all":
		return op.MakeAttr("file.remove_all", r.removeAll), nil
	default:
		return nil, op.NoSuchAttrError("file", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *FileReceiver) AttrNames() []string {
	return []string{
		"exists",
		"glob",
		"is_directory",
		"is_file",
		"join",
		"list",
		"mkdir",
		"name",
		"parent",
		"read",
		"remove",
		"remove_all",
		"walk_tree",
		"write",
	}
}

// read reads the contents of a file.
func (r *FileReceiver) read(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.read", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("file.read: %w", err)
	}
	return starlark.String(data), nil
}

// write writes content to a file.
func (r *FileReceiver) write(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, content string
	if err := starlark.UnpackArgs("file.write", args, kwargs, "path", &path, "content", &content); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would write %d bytes to %s", len(content), path)
		return starlark.None, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}
	return starlark.None, nil
}

// exists checks if a path exists.
func (r *FileReceiver) exists(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.exists", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	_, err := os.Stat(path)
	return starlark.Bool(!os.IsNotExist(err)), nil
}

// isDirectory checks if a path is a directory.
func (r *FileReceiver) isDirectory(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.is_directory", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return starlark.Bool(false), nil
	}
	if err != nil {
		return nil, fmt.Errorf("file.is_directory: %w", err)
	}
	return starlark.Bool(info.IsDir()), nil
}

// isFile checks if a path is a regular file.
func (r *FileReceiver) isFile(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.is_file", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return starlark.Bool(false), nil
	}
	if err != nil {
		return nil, fmt.Errorf("file.is_file: %w", err)
	}
	return starlark.Bool(!info.IsDir()), nil
}

// list lists the contents of a directory.
// By default, respects .gitignore files. Use gitignore=False to disable.
func (r *FileReceiver) list(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	respectGitignore := true
	if err := starlark.UnpackArgs("file.list", args, kwargs,
		"path", &path,
		"gitignore?", &respectGitignore,
	); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("file.list: %w", err)
	}

	var tracker *ignore.Tracker
	if respectGitignore {
		tracker = newTrackerForPath(path)
	}

	var items []starlark.Value
	for _, entry := range entries {
		if tracker != nil {
			absPath, _ := filepath.Abs(path)
			entryRel, relErr := filepath.Rel(tracker.Root(), filepath.Join(absPath, entry.Name()))
			if relErr == nil {
				ignored, _ := tracker.IsIgnored(entryRel, entry.IsDir())
				if ignored {
					continue
				}
			}
		}

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

// join joins path components.
func (r *FileReceiver) join(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 0 {
		return starlark.String(""), nil
	}
	parts := make([]string, len(args))
	for i, arg := range args {
		s, ok := starlark.AsString(arg)
		if !ok {
			return nil, fmt.Errorf("file.join: argument %d is not a string", i)
		}
		parts[i] = s
	}
	return starlark.String(filepath.Join(parts...)), nil
}

// name returns the base name of a path.
func (r *FileReceiver) name(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.name", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	return starlark.String(filepath.Base(path)), nil
}

// parent returns the parent directory of a path.
func (r *FileReceiver) parent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.parent", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	return starlark.String(filepath.Dir(path)), nil
}

// glob finds files matching a pattern.
// By default, respects .gitignore files. Use gitignore=False to disable.
func (r *FileReceiver) glob(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var pattern string
	respectGitignore := true
	if err := starlark.UnpackArgs("file.glob", args, kwargs,
		"pattern", &pattern,
		"gitignore?", &respectGitignore,
	); err != nil {
		return nil, err
	}

	var matches []string
	var err error

	// Handle patterns with ** for recursive matching
	if strings.Contains(pattern, "**") {
		matches, err = fileRecursiveGlob(pattern)
	} else {
		matches, err = filepath.Glob(pattern)
	}

	if err != nil {
		return nil, fmt.Errorf("file.glob: %w", err)
	}

	// Filter by .gitignore if requested
	if respectGitignore && len(matches) > 0 {
		matches = filterByGitignore(matches)
	}

	var items []starlark.Value
	for _, m := range matches {
		items = append(items, starlark.String(m))
	}
	return starlark.NewList(items), nil
}

// walkTree walks a directory tree, calling a Starlark callback for each entry.
// By default, respects .gitignore rules. The callback receives a struct with
// path (relative), name (base), and is_dir fields. Return "skip" to skip a
// directory's children, "stop" to terminate the walk.
func (r *FileReceiver) walkTree(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var root string
	var callback starlark.Callable
	respectGitignore := true

	if err := starlark.UnpackArgs("file.walk_tree", args, kwargs,
		"root", &root,
		"callback", &callback,
		"gitignore?", &respectGitignore,
	); err != nil {
		return nil, err
	}

	var tracker *ignore.Tracker
	if respectGitignore {
		var err error
		tracker, err = ignore.NewTracker(root)
		if err != nil {
			tracker = nil // walk without filtering on error
		}
	}

	err := ignore.WalkTree(ignore.WalkOptions{
		Root:    root,
		Tracker: tracker,
		Callback: func(path string, isDir bool) error {
			entry := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
				"path":   starlark.String(path),
				"name":   starlark.String(filepath.Base(path)),
				"is_dir": starlark.Bool(isDir),
			})

			res, callErr := starlark.Call(thread, callback, starlark.Tuple{entry}, nil)
			if callErr != nil {
				return callErr
			}

			if str, ok := res.(starlark.String); ok {
				switch str.GoString() {
				case "skip":
					if isDir {
						return filepath.SkipDir
					}
					return nil
				case "stop":
					return ignore.ErrWalkStopped
				}
			}

			return nil
		},
	})

	if err != nil {
		return nil, fmt.Errorf("file.walk_tree: %w", err)
	}

	return starlark.None, nil
}

// filterByGitignore filters out paths that match .gitignore patterns.
func filterByGitignore(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}

	// Find git root by walking up from first path's directory
	baseDir := filepath.Dir(paths[0])
	gitRoot := findGitRoot(baseDir)

	tracker, err := ignore.NewTracker(gitRoot)
	if err != nil {
		return paths
	}

	// Push intermediate directories for proper gitignore scoping
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		relPath, relErr := filepath.Rel(gitRoot, p)
		if relErr != nil {
			result = append(result, p)
			continue
		}

		// Push the directory containing this file to load its .gitignore
		dir := filepath.Dir(relPath)
		if dir != "." {
			pushPath(tracker, dir)
		}

		info, statErr := os.Stat(p)
		isDir := statErr == nil && info.IsDir()

		ignored, _ := tracker.IsIgnored(relPath, isDir)
		if !ignored {
			result = append(result, p)
		}
	}
	return result
}

// fileRecursiveGlob handles patterns with ** for recursive directory matching.
func fileRecursiveGlob(pattern string) ([]string, error) {
	parts := strings.SplitN(pattern, "**", 2)
	if len(parts) != 2 {
		return filepath.Glob(pattern)
	}

	base := parts[0]
	if base == "" {
		base = "."
	}
	base = strings.TrimSuffix(base, "/")

	filePart := strings.TrimPrefix(parts[1], "/")
	if filePart == "" {
		filePart = "*"
	}

	var matches []string
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible directories
		}
		if d.IsDir() {
			return nil
		}

		fileName := filepath.Base(path)
		matched, matchErr := filepath.Match(filePart, fileName)
		if matchErr != nil {
			return matchErr
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// mkdir creates a directory and any necessary parents.
func (r *FileReceiver) mkdir(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.mkdir", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would create directory %s", path)
		return starlark.None, nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("file.mkdir: %w", err)
	}
	return starlark.None, nil
}

// remove removes a file.
func (r *FileReceiver) remove(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.remove", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would remove %s", path)
		return starlark.None, nil
	}
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("file.remove: %w", err)
	}
	return starlark.None, nil
}

// removeAll removes a file or directory recursively.
func (r *FileReceiver) removeAll(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs("file.remove_all", args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	if DryRun {
		cli.Note("would remove %s (recursively)", path)
		return starlark.None, nil
	}
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("file.remove_all: %w", err)
	}
	return starlark.None, nil
}

// findGitRoot walks up from start to find the nearest .git directory.
func findGitRoot(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	dir := absStart
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return absStart
		}
		dir = parent
	}
}

// newTrackerForPath creates a tracker rooted at the git root above path,
// with intermediate directories pushed for proper gitignore scoping.
func newTrackerForPath(path string) *ignore.Tracker {
	absPath, _ := filepath.Abs(path)
	gitRoot := findGitRoot(absPath)

	tracker, err := ignore.NewTracker(gitRoot)
	if err != nil {
		return nil
	}

	// Push intermediate directories from gitRoot to path
	relPath, err := filepath.Rel(gitRoot, absPath)
	if err != nil || relPath == "." {
		return tracker
	}

	pushPath(tracker, relPath)
	return tracker
}

// pushPath pushes all intermediate directories onto the tracker.
func pushPath(tracker *ignore.Tracker, relPath string) {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for i := range parts {
		dir := strings.Join(parts[:i+1], "/")
		tracker.Push(dir)
	}
}
