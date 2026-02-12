// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package ignore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// WalkFunc is the callback signature for WalkTree. It receives a path
// relative to the walk root and whether the entry is a directory.
type WalkFunc func(path string, isDir bool) error

// WalkOptions configures a WalkTree traversal.
type WalkOptions struct {
	Root     string   // Directory to walk (required)
	Tracker  *Tracker // Gitignore tracker (nil = no filtering)
	Callback WalkFunc // Called for each non-ignored entry
}

// ErrWalkStopped is returned by a callback to terminate the walk early.
// WalkTree treats this as a successful completion and returns nil.
var ErrWalkStopped = errors.New("walk stopped by callback")

// WalkTree performs a depth-first traversal of Root, respecting gitignore
// rules via the Tracker. Both files and directories are yielded to the
// callback. Ignored directories are skipped entirely (no descent).
//
// The callback receives relative paths from Root and can return:
//   - nil: continue walking
//   - filepath.SkipDir: skip the current directory's children
//   - ErrWalkStopped: terminate the walk (WalkTree returns nil)
//   - any other error: abort the walk (WalkTree returns the error)
func WalkTree(opts WalkOptions) error {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return err
	}

	// Verify root exists
	if _, err := os.Stat(absRoot); err != nil {
		return err
	}

	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}
		if relPath == "." {
			return nil // skip root itself
		}

		isDir := d.IsDir()

		// Always skip .git directories
		if isDir && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if opts.Tracker != nil {
			if isDir {
				// Push handles popping sibling entries automatically
				opts.Tracker.Push(relPath)
			}

			ignored, _ := opts.Tracker.IsIgnored(relPath, isDir)
			if ignored {
				if isDir {
					return filepath.SkipDir
				}
				return nil
			}
		}

		return opts.Callback(relPath, isDir)
	})

	if walkErr == ErrWalkStopped {
		return nil
	}
	return walkErr
}
