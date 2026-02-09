// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package extension_test

import (
	"path/filepath"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

func TestParseAllExtensions(t *testing.T) {
	matches, err := filepath.Glob("../../extensions/*/extension.yaml")
	if err != nil {
		t.Fatalf("Glob error: %v", err)
	}

	if len(matches) == 0 {
		t.Skip("No extensions found")
	}

	for _, path := range matches {
		t.Run(path, func(t *testing.T) {
			spec, err := extension.ParseSpec(path)
			if err != nil {
				t.Errorf("Failed to parse %s: %v", path, err)
				return
			}
			if spec.Extension == "" {
				t.Errorf("Extension name is empty in %s", path)
			}
			t.Logf("Parsed %s -> %s", path, spec.Extension)
		})
	}
}
