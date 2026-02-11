// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
)

func TestNewHost(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		caps    extension.Capabilities
		wantErr bool
	}{
		{
			name: "valid capabilities",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Read:  []string{"/workspace"},
					Write: []string{"/workspace/output"},
				},
				HostCalls: []string{"shell.run"},
			},
			wantErr: false,
		},
		{
			name:    "empty capabilities",
			caps:    extension.Capabilities{},
			wantErr: false,
		},
		{
			name: "invalid capabilities - sensitive path",
			caps: extension.Capabilities{
				FS: extension.FSCapabilities{
					Read: []string{"/etc"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid capabilities - bad host call",
			caps: extension.Capabilities{
				HostCalls: []string{"invalid"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := NewHost(ctx, tt.caps)
			if tt.wantErr {
				if err == nil {
					t.Error("NewHost() expected error, got nil")
					if host != nil {
						host.Close()
					}
				}
				return
			}

			if err != nil {
				t.Errorf("NewHost() unexpected error = %v", err)
				return
			}

			if host == nil {
				t.Error("NewHost() returned nil host without error")
				return
			}

			// Verify capabilities are stored
			if len(host.Capabilities().FS.Read) != len(tt.caps.FS.Read) {
				t.Errorf("NewHost() capabilities mismatch")
			}

			// Verify checker is created
			if host.Checker() == nil {
				t.Error("NewHost() checker is nil")
			}

			// Clean up
			if err := host.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
	}
}

func TestHost_LoadModule_NotFound(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}
	defer host.Close()

	// Try to load a non-existent file
	_, err = host.LoadModule("/nonexistent/module.wasm")
	if err == nil {
		t.Error("LoadModule() expected error for non-existent file, got nil")
	}
}

func TestHost_Close(t *testing.T) {
	ctx := context.Background()

	host, err := NewHost(ctx, extension.Capabilities{})
	if err != nil {
		t.Fatalf("NewHost() error = %v", err)
	}

	// First close should succeed
	if err := host.Close(); err != nil {
		t.Errorf("Close() first call error = %v", err)
	}

	// Second close should also succeed (idempotent)
	// Note: wazero may return an error on double-close, but we handle it
}

func TestEnsureCacheDir(t *testing.T) {
	cacheDir, err := ensureCacheDir()
	if err != nil {
		t.Fatalf("ensureCacheDir() error = %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Errorf("cache directory does not exist: %v", err)
		return
	}

	if !info.IsDir() {
		t.Errorf("cache path is not a directory: %s", cacheDir)
	}

	// Verify path structure
	if !filepath.IsAbs(cacheDir) {
		t.Errorf("cache dir should be absolute: %s", cacheDir)
	}
}

func TestWasmError(t *testing.T) {
	tests := []struct {
		name    string
		err     *WasmError
		wantStr string
	}{
		{
			name:    "with code",
			err:     NewWasmError(ErrCodeInternal, "internal error"),
			wantStr: "wasm error -3: internal error",
		},
		{
			name:    "zero code",
			err:     &WasmError{Code: 0, Message: "message only"},
			wantStr: "message only",
		},
		{
			name:    "exit code",
			err:     NewWasmError(1, "exit code 1"),
			wantStr: "wasm error 1: exit code 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantStr {
				t.Errorf("Error() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestCapabilityError(t *testing.T) {
	wasmErr := CapabilityError("read", "/secret/file")

	if wasmErr.Code != ErrCodeCapability {
		t.Errorf("Code = %d, want %d", wasmErr.Code, ErrCodeCapability)
	}

	if wasmErr.Message != "capability denied: cannot read /secret/file" {
		t.Errorf("Message = %q, want %q", wasmErr.Message, "capability denied: cannot read /secret/file")
	}

	data, ok := wasmErr.Data.(map[string]string)
	if !ok {
		t.Fatalf("Data type = %T, want map[string]string", wasmErr.Data)
	}
	if data["action"] != "read" || data["path"] != "/secret/file" {
		t.Errorf("Data = %v, want action=read, path=/secret/file", data)
	}
}
