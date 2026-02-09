// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

func TestDefaultCallbacks_ShellRun(t *testing.T) {
	tests := []struct {
		name       string
		caps       extension.Capabilities
		cmd        string
		args       []string
		dir        string
		wantErr    bool
		errCode    int
		wantStdout string
		wantExit   int
	}{
		{
			name: "allowed command",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
				HostCalls: []string{"shell.run"},
			},
			cmd:        "echo",
			args:       []string{"hello"},
			wantStdout: "hello\n",
			wantExit:   0,
		},
		{
			name: "denied - no capability",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{Read: []string{"/workspace"}},
			},
			cmd:     "echo",
			args:    []string{"hello"},
			wantErr: true,
			errCode: ErrCodeCapability,
		},
		{
			name: "command not found",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
				HostCalls: []string{"shell.run"},
			},
			cmd:     "nonexistent-command-xyz",
			wantErr: true,
		},
		{
			name: "command with non-zero exit",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
				HostCalls: []string{"shell.run"},
			},
			cmd:      "sh",
			args:     []string{"-c", "exit 42"},
			wantExit: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCapabilityChecker(tt.caps)
			callbacks := NewDefaultCallbacks(checker)

			ctx := context.Background()
			stdout, stderr, exitCode, err := callbacks.ShellRun(ctx, tt.cmd, tt.args, tt.dir)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errCode != 0 {
					if wasmErr, ok := err.(*WasmError); ok {
						if wasmErr.Code != tt.errCode {
							t.Errorf("error code = %d, want %d", wasmErr.Code, tt.errCode)
						}
					} else {
						t.Errorf("expected WasmError, got %T", err)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantStdout != "" && stdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout, tt.wantStdout)
			}

			if exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", exitCode, tt.wantExit)
			}

			_ = stderr // Not checked in these tests
		})
	}
}

func TestDefaultCallbacks_ShellRun_ContextCanceled(t *testing.T) {
	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
		HostCalls: []string{"shell.run"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	// Use an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _, err := callbacks.ShellRun(ctx, "echo", []string{"hello"}, "")

	if err == nil {
		t.Error("expected canceled error, got nil")
		return
	}

	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("expected WasmError, got %T: %v", err, err)
		return
	}

	if wasmErr.Code != ErrCodeCanceled {
		t.Errorf("error code = %d, want %d (canceled)", wasmErr.Code, ErrCodeCanceled)
	}
}

func TestDefaultCallbacks_ShellRun_WorkingDir(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{tmpDir}},
		HostCalls: []string{"shell.run"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)
	callbacks.ShellTimeout = 5 * time.Second // Shorter timeout for CI

	ctx := context.Background()
	stdout, _, _, err := callbacks.ShellRun(ctx, "pwd", nil, tmpDir)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// stdout should contain the tmpDir path (resolve symlinks on output too)
	got := filepath.Clean(stdout[:len(stdout)-1]) // Remove trailing newline
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != "" {
		got = gotResolved
	}
	want := filepath.Clean(tmpDir)
	if got != want {
		t.Errorf("pwd output = %q, want %q", got, want)
	}
}

func TestDefaultCallbacks_ShellRun_WorkingDir_Denied(t *testing.T) {
	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
		HostCalls: []string{"shell.run"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	ctx := context.Background()
	_, _, _, err := callbacks.ShellRun(ctx, "pwd", nil, "/tmp/some-other-dir")

	if err == nil {
		t.Error("expected error for denied working directory, got nil")
		return
	}

	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("expected WasmError, got %T", err)
		return
	}

	if wasmErr.Code != ErrCodeCapability {
		t.Errorf("error code = %d, want %d (capability)", wasmErr.Code, ErrCodeCapability)
	}
}

func TestDefaultCallbacks_HTTPGet(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "test-value" {
			http.Error(w, "missing header", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello from server"))
	}))
	defer server.Close()

	tests := []struct {
		name       string
		caps       extension.Capabilities
		url        string
		headers    map[string]string
		wantErr    bool
		errCode    int
		wantStatus int
		wantBody   string
	}{
		{
			name: "allowed request",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
				HostCalls: []string{"http.get"},
			},
			url:        server.URL,
			headers:    map[string]string{"X-Custom": "test-value"},
			wantStatus: http.StatusOK,
			wantBody:   "hello from server",
		},
		{
			name: "denied - no capability",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{Read: []string{"/workspace"}},
			},
			url:     server.URL,
			wantErr: true,
			errCode: ErrCodeCapability,
		},
		{
			name: "invalid url",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
				HostCalls: []string{"http.get"},
			},
			url:     "://invalid",
			wantErr: true,
			errCode: ErrCodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCapabilityChecker(tt.caps)
			callbacks := NewDefaultCallbacks(checker)

			ctx := context.Background()
			body, statusCode, err := callbacks.HTTPGet(ctx, tt.url, tt.headers)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errCode != 0 {
					if wasmErr, ok := err.(*WasmError); ok {
						if wasmErr.Code != tt.errCode {
							t.Errorf("error code = %d, want %d", wasmErr.Code, tt.errCode)
						}
					} else {
						t.Errorf("expected WasmError, got %T", err)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if statusCode != tt.wantStatus {
				t.Errorf("status code = %d, want %d", statusCode, tt.wantStatus)
			}

			if tt.wantBody != "" && string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", string(body), tt.wantBody)
			}
		})
	}
}

func TestDefaultCallbacks_HTTPGet_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
		HostCalls: []string{"http.get"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	// Use an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := callbacks.HTTPGet(ctx, server.URL, nil)

	// Should get an error (either timeout or internal due to cancelled context)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDefaultCallbacks_FSRead(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		caps    extension.Capabilities
		path    string
		wantErr bool
		errCode int
		want    []byte
	}{
		{
			name: "allowed read",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{tmpDir}},
				HostCalls: []string{"fs.read"},
			},
			path: testFile,
			want: testContent,
		},
		{
			name: "denied - no host call capability",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{Read: []string{tmpDir}},
			},
			path:    testFile,
			wantErr: true,
			errCode: ErrCodeCapability,
		},
		{
			name: "denied - path not allowed",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{"/other-dir"}},
				HostCalls: []string{"fs.read"},
			},
			path:    testFile,
			wantErr: true,
			errCode: ErrCodeCapability,
		},
		{
			name: "file not found",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Read: []string{tmpDir}},
				HostCalls: []string{"fs.read"},
			},
			path:    filepath.Join(tmpDir, "nonexistent.txt"),
			wantErr: true,
			errCode: ErrCodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCapabilityChecker(tt.caps)
			callbacks := NewDefaultCallbacks(checker)

			ctx := context.Background()
			got, err := callbacks.FSRead(ctx, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errCode != 0 {
					if wasmErr, ok := err.(*WasmError); ok {
						if wasmErr.Code != tt.errCode {
							t.Errorf("error code = %d, want %d", wasmErr.Code, tt.errCode)
						}
					} else {
						t.Errorf("expected WasmError, got %T", err)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if string(got) != string(tt.want) {
				t.Errorf("content = %q, want %q", string(got), string(tt.want))
			}
		})
	}
}

func TestDefaultCallbacks_FSRead_FileTooLarge(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")
	// Create a file larger than MaxFileSize
	largeContent := make([]byte, 100)
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{tmpDir}},
		HostCalls: []string{"fs.read"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)
	callbacks.MaxFileSize = 50 // Set very small limit

	ctx := context.Background()
	_, err := callbacks.FSRead(ctx, testFile)

	if err == nil {
		t.Error("expected error for large file, got nil")
		return
	}

	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("expected WasmError, got %T", err)
		return
	}

	if wasmErr.Code != ErrCodeInternal {
		t.Errorf("error code = %d, want %d", wasmErr.Code, ErrCodeInternal)
	}
}

func TestDefaultCallbacks_FSWrite(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		caps    extension.Capabilities
		path    string
		data    []byte
		wantErr bool
		errCode int
	}{
		{
			name: "allowed write",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Write: []string{tmpDir}},
				HostCalls: []string{"fs.write"},
			},
			path: filepath.Join(tmpDir, "output.txt"),
			data: []byte("test content"),
		},
		{
			name: "allowed write - creates subdirectory",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Write: []string{tmpDir}},
				HostCalls: []string{"fs.write"},
			},
			path: filepath.Join(tmpDir, "subdir", "output.txt"),
			data: []byte("nested content"),
		},
		{
			name: "denied - no host call capability",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{Write: []string{tmpDir}},
			},
			path:    filepath.Join(tmpDir, "output.txt"),
			data:    []byte("test"),
			wantErr: true,
			errCode: ErrCodeCapability,
		},
		{
			name: "denied - path not allowed",
			caps: extension.Capabilities{
				FS:        extension.FSCapabilities{Write: []string{"/other-dir"}},
				HostCalls: []string{"fs.write"},
			},
			path:    filepath.Join(tmpDir, "output.txt"),
			data:    []byte("test"),
			wantErr: true,
			errCode: ErrCodeCapability,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCapabilityChecker(tt.caps)
			callbacks := NewDefaultCallbacks(checker)

			ctx := context.Background()
			err := callbacks.FSWrite(ctx, tt.path, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errCode != 0 {
					if wasmErr, ok := err.(*WasmError); ok {
						if wasmErr.Code != tt.errCode {
							t.Errorf("error code = %d, want %d", wasmErr.Code, tt.errCode)
						}
					} else {
						t.Errorf("expected WasmError, got %T", err)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify file was written
			got, err := os.ReadFile(tt.path)
			if err != nil {
				t.Errorf("failed to read written file: %v", err)
				return
			}

			if string(got) != string(tt.data) {
				t.Errorf("file content = %q, want %q", string(got), string(tt.data))
			}
		})
	}
}

func TestDefaultCallbacks_FSWrite_DataTooLarge(t *testing.T) {
	tmpDir := t.TempDir()

	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Write: []string{tmpDir}},
		HostCalls: []string{"fs.write"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)
	callbacks.MaxFileSize = 50 // Set very small limit

	largeData := make([]byte, 100)
	ctx := context.Background()
	err := callbacks.FSWrite(ctx, filepath.Join(tmpDir, "output.txt"), largeData)

	if err == nil {
		t.Error("expected error for large data, got nil")
		return
	}

	wasmErr, ok := err.(*WasmError)
	if !ok {
		t.Errorf("expected WasmError, got %T", err)
		return
	}

	if wasmErr.Code != ErrCodeInternal {
		t.Errorf("error code = %d, want %d", wasmErr.Code, ErrCodeInternal)
	}
}
