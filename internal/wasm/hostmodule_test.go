// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/NobleFactor/noblefactor-ops/internal/extension"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestInstantiateHostModule(t *testing.T) {
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	// WASI is required for our modules
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		t.Fatalf("failed to instantiate WASI: %v", err)
	}

	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
		HostCalls: []string{"shell.run", "http.get", "fs.read", "fs.write"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	hostModule, err := InstantiateHostModule(ctx, rt, callbacks)
	if err != nil {
		t.Fatalf("failed to instantiate host module: %v", err)
	}
	defer hostModule.Close(ctx)

	// Verify the module is named correctly
	if hostModule.Name() != HostModuleName {
		t.Errorf("module name = %q, want %q", hostModule.Name(), HostModuleName)
	}

	// Note: We cannot call ExportedFunction on host modules in wazero,
	// so we just verify the module was created successfully.
}

func TestHostModuleState(t *testing.T) {
	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
		HostCalls: []string{"shell.run"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	state := NewHostState(callbacks)

	// Test error handling
	state.setError("test error")
	got := state.getError()
	if got != "test error" {
		t.Errorf("getError() = %q, want %q", got, "test error")
	}

	// Error should be cleared after get
	got = state.getError()
	if got != "" {
		t.Errorf("getError() after clear = %q, want empty", got)
	}
}

func TestHostStateContext(t *testing.T) {
	caps := extension.Capabilities{
		FS:        extension.FSCapabilities{Read: []string{"/workspace"}},
		HostCalls: []string{"shell.run"},
	}
	checker := NewCapabilityChecker(caps)
	callbacks := NewDefaultCallbacks(checker)

	state := NewHostState(callbacks)
	ctx := context.Background()

	// Without state
	if getHostState(ctx) != nil {
		t.Error("expected nil state from empty context")
	}

	// With state
	ctxWithState := withHostState(ctx, state)
	got := getHostState(ctxWithState)
	if got != state {
		t.Errorf("getHostState() returned different state")
	}
}

func TestShellRunRequest_JSON(t *testing.T) {
	req := ShellRunRequest{
		Cmd:  "echo",
		Args: []string{"hello", "world"},
		Dir:  "/tmp",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ShellRunRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Cmd != req.Cmd {
		t.Errorf("Cmd = %q, want %q", decoded.Cmd, req.Cmd)
	}
	if len(decoded.Args) != len(req.Args) {
		t.Errorf("Args length = %d, want %d", len(decoded.Args), len(req.Args))
	}
	if decoded.Dir != req.Dir {
		t.Errorf("Dir = %q, want %q", decoded.Dir, req.Dir)
	}
}

func TestShellRunResponse_JSON(t *testing.T) {
	resp := ShellRunResponse{
		Stdout:   "output",
		Stderr:   "error",
		ExitCode: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ShellRunResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Stdout != resp.Stdout {
		t.Errorf("Stdout = %q, want %q", decoded.Stdout, resp.Stdout)
	}
	if decoded.Stderr != resp.Stderr {
		t.Errorf("Stderr = %q, want %q", decoded.Stderr, resp.Stderr)
	}
	if decoded.ExitCode != resp.ExitCode {
		t.Errorf("ExitCode = %d, want %d", decoded.ExitCode, resp.ExitCode)
	}
}

func TestHTTPGetRequest_JSON(t *testing.T) {
	req := HTTPGetRequest{
		URL:     "https://example.com",
		Headers: map[string]string{"Content-Type": "application/json"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded HTTPGetRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URL != req.URL {
		t.Errorf("URL = %q, want %q", decoded.URL, req.URL)
	}
	if decoded.Headers["Content-Type"] != req.Headers["Content-Type"] {
		t.Errorf("Headers = %v, want %v", decoded.Headers, req.Headers)
	}
}

func TestHTTPGetResponse_JSON(t *testing.T) {
	resp := HTTPGetResponse{
		Body:       []byte("response body"),
		StatusCode: 200,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded HTTPGetResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if string(decoded.Body) != string(resp.Body) {
		t.Errorf("Body = %q, want %q", string(decoded.Body), string(resp.Body))
	}
	if decoded.StatusCode != resp.StatusCode {
		t.Errorf("StatusCode = %d, want %d", decoded.StatusCode, resp.StatusCode)
	}
}

func TestFSReadRequest_JSON(t *testing.T) {
	req := FSReadRequest{
		Path: "/path/to/file.txt",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded FSReadRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Path != req.Path {
		t.Errorf("Path = %q, want %q", decoded.Path, req.Path)
	}
}

func TestFSReadResponse_JSON(t *testing.T) {
	resp := FSReadResponse{
		Data: []byte("file contents"),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded FSReadResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if string(decoded.Data) != string(resp.Data) {
		t.Errorf("Data = %q, want %q", string(decoded.Data), string(resp.Data))
	}
}

func TestFSWriteRequest_JSON(t *testing.T) {
	req := FSWriteRequest{
		Path: "/path/to/file.txt",
		Data: []byte("content to write"),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded FSWriteRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Path != req.Path {
		t.Errorf("Path = %q, want %q", decoded.Path, req.Path)
	}
	if string(decoded.Data) != string(req.Data) {
		t.Errorf("Data = %q, want %q", string(decoded.Data), string(req.Data))
	}
}
