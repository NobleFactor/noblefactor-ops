// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Discover scans a directory for extension specifications.
// It looks for files named "extension.yaml" or "extension.yml" in subdirectories.
// Returns all valid specs found; invalid specs are skipped with warnings logged.
func Discover(dir string) ([]*ExtensionSpec, error) {
	var specs []*ExtensionSpec

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process extension.yaml or extension.yml
		name := d.Name()
		if name != "extension.yaml" && name != "extension.yml" {
			return nil
		}

		spec, err := ParseSpec(path)
		if err != nil {
			// Log warning but continue scanning
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
			return nil
		}

		specs = append(specs, spec)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("discover extensions in %s: %w", dir, err)
	}

	return specs, nil
}

// DiscoverOne parses a single extension from a directory.
// The directory must contain an extension.yaml or extension.yml file.
func DiscoverOne(dir string) (*ExtensionSpec, error) {
	yamlPath := filepath.Join(dir, "extension.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		yamlPath = filepath.Join(dir, "extension.yml")
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("no extension.yaml or extension.yml in %s", dir)
		}
	}

	return ParseSpec(yamlPath)
}

// LoadAll discovers extensions from directories and registers them.
// Directories that don't exist are silently skipped.
// Returns the number of extensions loaded and any error.
func LoadAll(dirs ...string) (int, error) {
	var count int

	for _, dir := range dirs {
		// Skip non-existent directories
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return count, fmt.Errorf("stat %s: %w", dir, err)
		}
		if !info.IsDir() {
			continue
		}

		specs, err := Discover(dir)
		if err != nil {
			return count, err
		}

		for _, spec := range specs {
			if err := Register(spec); err != nil {
				// Skip duplicates silently
				if strings.Contains(err.Error(), "already registered") {
					continue
				}
				return count, fmt.Errorf("register %s: %w", spec.Extension, err)
			}
			count++
		}
	}

	return count, nil
}

// DefaultSearchPaths returns the standard directories to search for extensions.
// The paths are:
//   - ./extensions (project local)
//   - ~/.star/extensions (user extensions)
//   - /usr/local/share/star/extensions (system-wide)
func DefaultSearchPaths() []string {
	paths := []string{
		"extensions",
	}

	// User extensions directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".star", "extensions"))
	}

	// System-wide extensions (Unix-like systems)
	paths = append(paths, "/usr/local/share/star/extensions")

	return paths
}

// LoadDefaults loads extensions from all default search paths.
// Returns the number of extensions loaded.
func LoadDefaults() (int, error) {
	return LoadAll(DefaultSearchPaths()...)
}

// FindExtensionDir locates the directory containing an extension by name.
// Searches default paths and returns the path to the extension directory.
// Extension directories use the extension name directly (reverse domain format).
// Example: extension "com.noblefactor.star.CopyrightChecker" is in
// directory "com.noblefactor.star.CopyrightChecker/".
func FindExtensionDir(name string) (string, error) {
	for _, searchPath := range DefaultSearchPaths() {
		// Directory name is the extension name (reverse domain format)
		dir := filepath.Join(searchPath, name)
		yamlPath := filepath.Join(dir, "extension.yaml")

		if _, err := os.Stat(yamlPath); err == nil {
			return dir, nil
		}

		// Also try extension.yml
		ymlPath := filepath.Join(dir, "extension.yml")
		if _, err := os.Stat(ymlPath); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("extension %q not found in search paths", name)
}
