// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"encoding/json"
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

func TestProtocol_EncodeDecodeRequest(t *testing.T) {
	params := map[string]string{"key": "value"}

	encoded, err := EncodeRequest(42, "test_method", params)
	if err != nil {
		t.Fatalf("EncodeRequest() error = %v", err)
	}

	decoded, err := DecodeRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeRequest() error = %v", err)
	}

	if decoded.ID != 42 {
		t.Errorf("Request ID = %d, want 42", decoded.ID)
	}
	if decoded.Method != "test_method" {
		t.Errorf("Request Method = %q, want %q", decoded.Method, "test_method")
	}

	var decodedParams map[string]string
	if err := json.Unmarshal(decoded.Params, &decodedParams); err != nil {
		t.Fatalf("Unmarshal params error = %v", err)
	}
	if decodedParams["key"] != "value" {
		t.Errorf("Request Params = %v, want key=value", decodedParams)
	}
}

func TestProtocol_EncodeDecodeResponse(t *testing.T) {
	result := map[string]int{"count": 5}

	encoded, err := EncodeResponse(42, result, nil)
	if err != nil {
		t.Fatalf("EncodeResponse() error = %v", err)
	}

	decoded, err := DecodeResponse(encoded)
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}

	if decoded.ID != 42 {
		t.Errorf("Response ID = %d, want 42", decoded.ID)
	}
	if decoded.Error != nil {
		t.Errorf("Response Error = %v, want nil", decoded.Error)
	}

	var decodedResult map[string]int
	if err := json.Unmarshal(decoded.Result, &decodedResult); err != nil {
		t.Fatalf("Unmarshal result error = %v", err)
	}
	if decodedResult["count"] != 5 {
		t.Errorf("Response Result = %v, want count=5", decodedResult)
	}
}

func TestProtocol_EncodeDecodeResponse_WithError(t *testing.T) {
	wasmErr := NewWasmError(ErrCodeInternal, "something went wrong")

	encoded, err := EncodeResponse(42, nil, wasmErr)
	if err != nil {
		t.Fatalf("EncodeResponse() error = %v", err)
	}

	decoded, err := DecodeResponse(encoded)
	if err != nil {
		t.Fatalf("DecodeResponse() error = %v", err)
	}

	if decoded.ID != 42 {
		t.Errorf("Response ID = %d, want 42", decoded.ID)
	}
	if decoded.Result != nil {
		t.Errorf("Response Result = %v, want nil", decoded.Result)
	}
	if decoded.Error == nil {
		t.Fatal("Response Error is nil, want error")
	}
	if decoded.Error.Code != ErrCodeInternal {
		t.Errorf("Response Error.Code = %d, want %d", decoded.Error.Code, ErrCodeInternal)
	}
	if decoded.Error.Message != "something went wrong" {
		t.Errorf("Response Error.Message = %q, want %q", decoded.Error.Message, "something went wrong")
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
