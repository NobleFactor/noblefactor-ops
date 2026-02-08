// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

// WorkspacePlaceholder is expanded to the current working directory.
const WorkspacePlaceholder = "/workspace"

// sensitivePaths are system directories that extensions cannot access.
var sensitivePaths = []string{
	"/etc",
	"/var",
	"/usr",
	"/bin",
	"/sbin",
	"/lib",
	"/lib64",
	"/opt",
	"/root",
	"/System",
	"/Library",
	"/Applications",
}

// validHostCallNamespaces are the allowed host call namespaces.
var validHostCallNamespaces = map[string]bool{
	"shell": true,
	"http":  true,
	"fs":    true,
}

// ValidateCapabilities checks that the capabilities are valid.
// Returns an error if any capability is invalid.
func ValidateCapabilities(caps extension.Capabilities) error {
	// Validate filesystem paths
	for _, path := range caps.FS.Read {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("invalid read path %q: %w", path, err)
		}
	}
	for _, path := range caps.FS.Write {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("invalid write path %q: %w", path, err)
		}
	}

	// Validate host calls
	for _, call := range caps.HostCalls {
		if err := validateHostCall(call); err != nil {
			return fmt.Errorf("invalid host_call %q: %w", call, err)
		}
	}

	return nil
}

// validatePath checks that a path is valid for filesystem capabilities.
func validatePath(path string) error {
	// Must be absolute or use workspace placeholder
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, WorkspacePlaceholder) {
		return fmt.Errorf("path must be absolute or start with %s", WorkspacePlaceholder)
	}

	// Expand workspace placeholder for validation
	expanded := expandPath(path)

	// Block sensitive paths
	for _, sensitive := range sensitivePaths {
		if strings.HasPrefix(expanded, sensitive+"/") || expanded == sensitive {
			return fmt.Errorf("access to %s is not allowed", sensitive)
		}
	}

	return nil
}

// validateHostCall checks that a host call name is valid.
func validateHostCall(call string) error {
	// Must be namespace.method format
	parts := strings.SplitN(call, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("must be namespace.method format (e.g., shell.run)")
	}

	// Check namespace is known
	if !validHostCallNamespaces[parts[0]] {
		return fmt.Errorf("unknown namespace %q (valid: shell, http, fs)", parts[0])
	}

	return nil
}

// expandPath expands the workspace placeholder to the current working directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, WorkspacePlaceholder) {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		return strings.Replace(path, WorkspacePlaceholder, cwd, 1)
	}
	return path
}

// CapabilityChecker validates access against declared capabilities.
type CapabilityChecker struct {
	caps extension.Capabilities
}

// NewCapabilityChecker creates a new CapabilityChecker.
func NewCapabilityChecker(caps extension.Capabilities) *CapabilityChecker {
	return &CapabilityChecker{caps: caps}
}

// AllowsRead checks if reading the given path is allowed.
func (c *CapabilityChecker) AllowsRead(path string) bool {
	return c.allowsAccess(path, c.caps.FS.Read)
}

// AllowsWrite checks if writing to the given path is allowed.
func (c *CapabilityChecker) AllowsWrite(path string) bool {
	return c.allowsAccess(path, c.caps.FS.Write)
}

// AllowsHostCall checks if the given host call is allowed.
func (c *CapabilityChecker) AllowsHostCall(name string) bool {
	for _, allowed := range c.caps.HostCalls {
		if allowed == name {
			return true
		}
	}
	return false
}

// allowsAccess checks if the path is under any of the allowed directories.
func (c *CapabilityChecker) allowsAccess(path string, allowed []string) bool {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)

	for _, pattern := range allowed {
		// Expand workspace placeholder
		expanded := expandPath(pattern)

		// Get absolute path of the allowed directory
		allowedAbs, err := filepath.Abs(expanded)
		if err != nil {
			continue
		}
		allowedAbs = filepath.Clean(allowedAbs)

		// Check if path is under the allowed directory
		if pathIsUnder(absPath, allowedAbs) {
			return true
		}
	}

	return false
}

// pathIsUnder checks if child is under or equal to parent.
func pathIsUnder(child, parent string) bool {
	// Equal paths are allowed
	if child == parent {
		return true
	}

	// Child must start with parent + separator
	if strings.HasPrefix(child, parent+string(filepath.Separator)) {
		return true
	}

	return false
}

// CheckRead validates read access and returns an error if denied.
func (c *CapabilityChecker) CheckRead(path string) error {
	if !c.AllowsRead(path) {
		return CapabilityError("read", path)
	}
	return nil
}

// CheckWrite validates write access and returns an error if denied.
func (c *CapabilityChecker) CheckWrite(path string) error {
	if !c.AllowsWrite(path) {
		return CapabilityError("write", path)
	}
	return nil
}

// CheckHostCall validates host call access and returns an error if denied.
func (c *CapabilityChecker) CheckHostCall(name string) error {
	if !c.AllowsHostCall(name) {
		return CapabilityError("call", name)
	}
	return nil
}
