// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const (
	// HostModuleName is the name of the host module that Wasm modules import.
	HostModuleName = "star_host"
)

// ShellRunRequest is the JSON request for shell.run callback.
type ShellRunRequest struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args,omitempty"`
	Dir  string   `json:"dir,omitempty"`
}

// ShellRunResponse is the JSON response for shell.run callback.
type ShellRunResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// HTTPGetRequest is the JSON request for http.get callback.
type HTTPGetRequest struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// HTTPGetResponse is the JSON response for http.get callback.
type HTTPGetResponse struct {
	Body       []byte `json:"body"`
	StatusCode int    `json:"status_code"`
}

// FSReadRequest is the JSON request for fs.read callback.
type FSReadRequest struct {
	Path string `json:"path"`
}

// FSReadResponse is the JSON response for fs.read callback.
type FSReadResponse struct {
	Data []byte `json:"data"`
}

// FSWriteRequest is the JSON request for fs.write callback.
type FSWriteRequest struct {
	Path string `json:"path"`
	Data []byte `json:"data"`
}

// hostModuleState holds per-instance state for host function calls.
// Each Wasm module instance gets its own state via context.
type hostModuleState struct {
	callbacks HostCallbacks
	lastError string
	mu        sync.Mutex
}

// hostStateKey is the context key for host module state.
type hostStateKey struct{}

// withHostState returns a context with the given host module state.
func withHostState(ctx context.Context, state *hostModuleState) context.Context {
	return context.WithValue(ctx, hostStateKey{}, state)
}

// getHostState retrieves the host module state from the context.
func getHostState(ctx context.Context) *hostModuleState {
	state, _ := ctx.Value(hostStateKey{}).(*hostModuleState)
	return state
}

// NewHostState creates a new host module state with the given callbacks.
func NewHostState(callbacks HostCallbacks) *hostModuleState {
	return &hostModuleState{
		callbacks: callbacks,
	}
}

// setError sets the last error message.
func (s *hostModuleState) setError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = err
}

// getError returns and clears the last error message.
func (s *hostModuleState) getError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.lastError
	s.lastError = ""
	return err
}

// InstantiateHostModule creates and instantiates the host module with callbacks.
// This must be called before instantiating user modules that import "star_host".
func InstantiateHostModule(ctx context.Context, runtime wazero.Runtime, callbacks HostCallbacks) (api.Module, error) {
	builder := runtime.NewHostModuleBuilder(HostModuleName)

	// shell_run(req_ptr, req_len, resp_ptr, resp_cap) -> resp_len
	// Returns negative value on error, positive value is response length
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(shellRunFn),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		WithParameterNames("req_ptr", "req_len", "resp_ptr", "resp_cap").
		WithResultNames("resp_len").
		Export("shell_run")

	// http_get(req_ptr, req_len, resp_ptr, resp_cap) -> resp_len
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(httpGetFn),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		WithParameterNames("req_ptr", "req_len", "resp_ptr", "resp_cap").
		WithResultNames("resp_len").
		Export("http_get")

	// fs_read(req_ptr, req_len, resp_ptr, resp_cap) -> resp_len
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(fsReadFn),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		WithParameterNames("req_ptr", "req_len", "resp_ptr", "resp_cap").
		WithResultNames("resp_len").
		Export("fs_read")

	// fs_write(req_ptr, req_len) -> error_code
	// Returns 0 on success, negative error code on failure
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(fsWriteFn),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		WithParameterNames("req_ptr", "req_len").
		WithResultNames("error_code").
		Export("fs_write")

	// get_last_error(error_ptr, error_cap) -> error_len
	// Returns the length of the error message, or 0 if no error
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(getLastErrorFn),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		WithParameterNames("error_ptr", "error_cap").
		WithResultNames("error_len").
		Export("get_last_error")

	return builder.Instantiate(ctx)
}

// readRequest reads a JSON request from Wasm memory.
func readRequest(mod api.Module, ptr, length uint32, v interface{}) error {
	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return NewWasmError(ErrCodeProtocol, "failed to read request from memory")
	}
	if err := json.Unmarshal(data, v); err != nil {
		return NewWasmErrorf(ErrCodeProtocol, "unmarshal request: %v", err)
	}
	return nil
}

// writeResponse writes a JSON response to Wasm memory.
// Returns the number of bytes written, or a negative error code.
func writeResponse(mod api.Module, respPtr, respCap uint32, v interface{}, state *hostModuleState) int32 {
	data, err := json.Marshal(v)
	if err != nil {
		state.setError(err.Error())
		return int32(ErrCodeProtocol)
	}

	if uint32(len(data)) > respCap {
		state.setError("response buffer too small")
		return int32(ErrCodeProtocol)
	}

	if !mod.Memory().Write(respPtr, data) {
		state.setError("failed to write response to memory")
		return int32(ErrCodeProtocol)
	}

	return int32(len(data))
}

// handleError sets the error on state and returns the appropriate error code.
func handleError(err error, state *hostModuleState) int32 {
	state.setError(err.Error())
	if wasmErr, ok := err.(*WasmError); ok {
		return int32(wasmErr.Code)
	}
	return int32(ErrCodeInternal)
}

func shellRunFn(ctx context.Context, mod api.Module, stack []uint64) {
	reqPtr := api.DecodeU32(stack[0])
	reqLen := api.DecodeU32(stack[1])
	respPtr := api.DecodeU32(stack[2])
	respCap := api.DecodeU32(stack[3])

	state := getHostState(ctx)
	if state == nil {
		stack[0] = api.EncodeI32(int32(ErrCodeInternal))
		return
	}

	var req ShellRunRequest
	if err := readRequest(mod, reqPtr, reqLen, &req); err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	stdout, stderr, exitCode, err := state.callbacks.ShellRun(ctx, req.Cmd, req.Args, req.Dir)
	if err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	resp := ShellRunResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}

	result := writeResponse(mod, respPtr, respCap, resp, state)
	stack[0] = api.EncodeI32(result)
}

func httpGetFn(ctx context.Context, mod api.Module, stack []uint64) {
	reqPtr := api.DecodeU32(stack[0])
	reqLen := api.DecodeU32(stack[1])
	respPtr := api.DecodeU32(stack[2])
	respCap := api.DecodeU32(stack[3])

	state := getHostState(ctx)
	if state == nil {
		stack[0] = api.EncodeI32(int32(ErrCodeInternal))
		return
	}

	var req HTTPGetRequest
	if err := readRequest(mod, reqPtr, reqLen, &req); err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	body, statusCode, err := state.callbacks.HTTPGet(ctx, req.URL, req.Headers)
	if err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	resp := HTTPGetResponse{
		Body:       body,
		StatusCode: statusCode,
	}

	result := writeResponse(mod, respPtr, respCap, resp, state)
	stack[0] = api.EncodeI32(result)
}

func fsReadFn(ctx context.Context, mod api.Module, stack []uint64) {
	reqPtr := api.DecodeU32(stack[0])
	reqLen := api.DecodeU32(stack[1])
	respPtr := api.DecodeU32(stack[2])
	respCap := api.DecodeU32(stack[3])

	state := getHostState(ctx)
	if state == nil {
		stack[0] = api.EncodeI32(int32(ErrCodeInternal))
		return
	}

	var req FSReadRequest
	if err := readRequest(mod, reqPtr, reqLen, &req); err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	data, err := state.callbacks.FSRead(ctx, req.Path)
	if err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	resp := FSReadResponse{
		Data: data,
	}

	result := writeResponse(mod, respPtr, respCap, resp, state)
	stack[0] = api.EncodeI32(result)
}

func fsWriteFn(ctx context.Context, mod api.Module, stack []uint64) {
	reqPtr := api.DecodeU32(stack[0])
	reqLen := api.DecodeU32(stack[1])

	state := getHostState(ctx)
	if state == nil {
		stack[0] = api.EncodeI32(int32(ErrCodeInternal))
		return
	}

	var req FSWriteRequest
	if err := readRequest(mod, reqPtr, reqLen, &req); err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	err := state.callbacks.FSWrite(ctx, req.Path, req.Data)
	if err != nil {
		stack[0] = api.EncodeI32(handleError(err, state))
		return
	}

	stack[0] = 0 // Success
}

func getLastErrorFn(ctx context.Context, mod api.Module, stack []uint64) {
	errorPtr := api.DecodeU32(stack[0])
	errorCap := api.DecodeU32(stack[1])

	state := getHostState(ctx)
	if state == nil {
		stack[0] = 0
		return
	}

	lastError := state.getError()
	if lastError == "" {
		stack[0] = 0
		return
	}

	errBytes := []byte(lastError)
	if uint32(len(errBytes)) > errorCap {
		errBytes = errBytes[:errorCap]
	}

	if !mod.Memory().Write(errorPtr, errBytes) {
		stack[0] = 0
		return
	}

	stack[0] = api.EncodeU32(uint32(len(errBytes)))
}
