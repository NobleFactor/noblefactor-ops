// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package copyright

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChecker_CheckFile(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	checker, err := NewChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		license  string
		holder   string
		wantOK   bool
	}{
		{
			name:     "correct go header",
			filename: "main.go",
			content: `// SPDX-License-Identifier: MIT
// Copyright Test Corp. All rights reserved.

package main
`,
			license: "MIT",
			holder:  "Test Corp",
			wantOK:  true,
		},
		{
			name:     "missing header",
			filename: "missing.go",
			content:  "package main\n",
			license:  "MIT",
			holder:   "Test Corp",
			wantOK:   false,
		},
		{
			name:     "wrong license",
			filename: "wronglicense.go",
			content: `// SPDX-License-Identifier: Apache-2.0
// Copyright Test Corp. All rights reserved.

package main
`,
			license: "MIT",
			holder:  "Test Corp",
			wantOK:  false,
		},
		{
			name:     "wrong holder",
			filename: "wrongholder.go",
			content: `// SPDX-License-Identifier: MIT
// Copyright Other Corp. All rights reserved.

package main
`,
			license: "MIT",
			holder:  "Test Corp",
			wantOK:  false,
		},
		{
			name:     "correct shell script",
			filename: "script.sh",
			content: `#!/bin/bash
# SPDX-License-Identifier: MIT
# Copyright Test Corp. All rights reserved.

echo "hello"
`,
			license: "MIT",
			holder:  "Test Corp",
			wantOK:  true,
		},
		{
			name:     "correct python",
			filename: "main.py",
			content: `# SPDX-License-Identifier: MIT
# Copyright Test Corp. All rights reserved.

print("hello")
`,
			license: "MIT",
			holder:  "Test Corp",
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			expected := HeaderData{
				License: tt.license,
				Holder:  tt.holder,
			}

			result := checker.CheckFile(path, expected)

			if result.Valid != tt.wantOK {
				t.Errorf("CheckFile() valid = %v, want %v (result: %+v)", result.Valid, tt.wantOK, result)
			}
		})
	}
}

func TestChecker_FixFile(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	checker, err := NewChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		initial  string
		wantSPDX string
		wantCopy string
	}{
		{
			name:     "add header to go file",
			filename: "main.go",
			initial:  "package main\n\nfunc main() {}\n",
			wantSPDX: "// SPDX-License-Identifier: MIT",
			wantCopy: "// Copyright Test Corp. All rights reserved.",
		},
		{
			name:     "add header to python file",
			filename: "main.py",
			initial:  "print('hello')\n",
			wantSPDX: "# SPDX-License-Identifier: MIT",
			wantCopy: "# Copyright Test Corp. All rights reserved.",
		},
		{
			name:     "fix shell script preserving shebang",
			filename: "script.sh",
			initial:  "#!/bin/bash\necho hello\n",
			wantSPDX: "# SPDX-License-Identifier: MIT",
			wantCopy: "# Copyright Test Corp. All rights reserved.",
		},
		{
			name:     "replace wrong header",
			filename: "replace.go",
			initial: `// SPDX-License-Identifier: Apache-2.0
// Copyright Other Corp. All rights reserved.

package main
`,
			wantSPDX: "// SPDX-License-Identifier: MIT",
			wantCopy: "// Copyright Test Corp. All rights reserved.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.initial), 0644); err != nil {
				t.Fatal(err)
			}

			data := HeaderData{
				License: "MIT",
				Holder:  "Test Corp",
			}

			if err := checker.FixFile(path, data); err != nil {
				t.Fatalf("FixFile() error = %v", err)
			}

			// Read and verify
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			contentStr := string(content)

			if !contains(contentStr, tt.wantSPDX) {
				t.Errorf("fixed file missing SPDX line %q in:\n%s", tt.wantSPDX, contentStr)
			}
			if !contains(contentStr, tt.wantCopy) {
				t.Errorf("fixed file missing copyright line %q in:\n%s", tt.wantCopy, contentStr)
			}

			// Verify shebang is preserved
			if tt.filename == "script.sh" {
				if !hasPrefix(contentStr, "#!/bin/bash\n") {
					t.Error("shebang should be preserved at top of file")
				}
			}
		})
	}
}

func TestChecker_CheckDir(t *testing.T) {
	cfg, err := loadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	checker, err := NewChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	// Create some test files
	files := map[string]string{
		"main.go": `// SPDX-License-Identifier: MIT
// Copyright Test Corp. All rights reserved.

package main
`,
		"lib.go": "package lib\n", // Missing header
		"sub/util.go": `// SPDX-License-Identifier: MIT
// Copyright Test Corp. All rights reserved.

package sub
`,
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	expected := HeaderData{
		License: "MIT",
		Holder:  "Test Corp",
	}

	results, err := checker.CheckDir(tmpDir, expected)
	if err != nil {
		t.Fatalf("CheckDir() error = %v", err)
	}

	// Should find 3 Go files
	goFiles := 0
	invalidFiles := 0
	for _, r := range results {
		if r.Language == "go" {
			goFiles++
			if !r.Valid {
				invalidFiles++
			}
		}
	}

	if goFiles != 3 {
		t.Errorf("expected 3 Go files, got %d", goFiles)
	}
	if invalidFiles != 1 {
		t.Errorf("expected 1 invalid file, got %d", invalidFiles)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
