// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

func TestValidateCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		caps    extension.Capabilities
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid absolute paths",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Read:  []string{"/home/user/project"},
					Write: []string{"/home/user/project/output"},
				},
				HostCalls: []string{"shell.run"},
			},
			wantErr: false,
		},
		{
			name: "valid workspace paths",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Read:  []string{"/workspace"},
					Write: []string{"/workspace/output"},
				},
			},
			wantErr: false,
		},
		{
			name: "relative path rejected",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Read: []string{"relative/path"},
				},
			},
			wantErr: true,
			errMsg:  "must be absolute or start with /workspace",
		},
		{
			name: "sensitive path rejected - etc",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Read: []string{"/etc/passwd"},
				},
			},
			wantErr: true,
			errMsg:  "access to /etc is not allowed",
		},
		{
			name: "sensitive path rejected - var",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Write: []string{"/var/log"},
				},
			},
			wantErr: true,
			errMsg:  "access to /var is not allowed",
		},
		{
			name: "invalid host call format - no dot",
			caps: extension.Capabilities{
				HostCalls: []string{"shellrun"},
			},
			wantErr: true,
			errMsg:  "must be namespace.method format",
		},
		{
			name: "invalid host call format - empty method",
			caps: extension.Capabilities{
				HostCalls: []string{"shell."},
			},
			wantErr: true,
			errMsg:  "must be namespace.method format",
		},
		{
			name: "unknown host call namespace",
			caps: extension.Capabilities{
				HostCalls: []string{"unknown.method"},
			},
			wantErr: true,
			errMsg:  "unknown namespace",
		},
		{
			name: "valid host calls",
			caps: extension.Capabilities{
				HostCalls: []string{"shell.run", "http.get", "fs.read"},
			},
			wantErr: false,
		},
		{
			name: "empty capabilities valid",
			caps: extension.Capabilities{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapabilities(tt.caps)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateCapabilities() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateCapabilities() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateCapabilities() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestCapabilityChecker_AllowsRead(t *testing.T) {
	// Get current working directory for workspace tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	checker := NewCapabilityChecker(extension.Capabilities{
		FS: extension.FSCapabilities{
			Read: []string{"/workspace", "/tmp/allowed"},
		},
	})

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "workspace root allowed",
			path: cwd,
			want: true,
		},
		{
			name: "workspace subdir allowed",
			path: filepath.Join(cwd, "subdir", "file.txt"),
			want: true,
		},
		{
			name: "tmp allowed",
			path: "/tmp/allowed/file.txt",
			want: true,
		},
		{
			name: "other path denied",
			path: "/home/user/secret",
			want: false,
		},
		{
			name: "tmp not allowed sibling",
			path: "/tmp/other/file.txt",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.AllowsRead(tt.path); got != tt.want {
				t.Errorf("AllowsRead(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCapabilityChecker_AllowsWrite(t *testing.T) {
	checker := NewCapabilityChecker(extension.Capabilities{
		FS: extension.FSCapabilities{
			Read:  []string{"/workspace"},
			Write: []string{"/tmp/output"},
		},
	})

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "write to output allowed",
			path: "/tmp/output/result.json",
			want: true,
		},
		{
			name: "write to workspace denied (read only)",
			path: "/workspace/file.txt",
			want: false,
		},
		{
			name: "write to other denied",
			path: "/home/user/file.txt",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.AllowsWrite(tt.path); got != tt.want {
				t.Errorf("AllowsWrite(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCapabilityChecker_AllowsHostCall(t *testing.T) {
	checker := NewCapabilityChecker(extension.Capabilities{
		HostCalls: []string{"shell.run", "http.get"},
	})

	tests := []struct {
		name string
		call string
		want bool
	}{
		{
			name: "shell.run allowed",
			call: "shell.run",
			want: true,
		},
		{
			name: "http.get allowed",
			call: "http.get",
			want: true,
		},
		{
			name: "fs.read denied",
			call: "fs.read",
			want: false,
		},
		{
			name: "shell.exec denied",
			call: "shell.exec",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.AllowsHostCall(tt.call); got != tt.want {
				t.Errorf("AllowsHostCall(%q) = %v, want %v", tt.call, got, tt.want)
			}
		})
	}
}

func TestCapabilityChecker_CheckRead(t *testing.T) {
	checker := NewCapabilityChecker(extension.Capabilities{
		FS: extension.FSCapabilities{
			Read: []string{"/tmp/allowed"},
		},
	})

	// Test allowed path
	if err := checker.CheckRead("/tmp/allowed/file.txt"); err != nil {
		t.Errorf("CheckRead() unexpected error for allowed path: %v", err)
	}

	// Test denied path
	err := checker.CheckRead("/tmp/denied/file.txt")
	if err == nil {
		t.Error("CheckRead() expected error for denied path, got nil")
	}
	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("CheckRead() error type = %T, want *WasmError", err)
	} else if wasmErr.Code != ErrCodeCapability {
		t.Errorf("CheckRead() error code = %d, want %d", wasmErr.Code, ErrCodeCapability)
	}
}

func TestExpandPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "workspace expanded",
			path: "/workspace",
			want: cwd,
		},
		{
			name: "workspace subdir expanded",
			path: "/workspace/subdir",
			want: filepath.Join(cwd, "subdir"),
		},
		{
			name: "absolute unchanged",
			path: "/tmp/file.txt",
			want: "/tmp/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandPath(tt.path); got != tt.want {
				t.Errorf("expandPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
