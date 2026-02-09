// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import "encoding/json"

// Request is the envelope for calling Wasm functions.
// It is written to the module's stdin as JSON.
type Request struct {
	// ID is a unique request identifier for correlation.
	ID uint64 `json:"id"`

	// Method is the function name to call.
	Method string `json:"method"`

	// Params contains the function arguments as JSON.
	// The structure depends on the specific function being called.
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is the envelope for Wasm function results.
// It is read from the module's stdout as JSON.
type Response struct {
	// ID matches the request ID for correlation.
	ID uint64 `json:"id"`

	// Result contains the function return value as JSON.
	// Present only on success (Error is nil).
	Result json.RawMessage `json:"result,omitempty"`

	// Error contains error information if the function failed.
	// Present only on failure (Result is nil).
	Error *WasmError `json:"error,omitempty"`
}

// EncodeRequest marshals a Request to JSON bytes.
func EncodeRequest(id uint64, method string, params any) ([]byte, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, err
		}
	}

	req := Request{
		ID:     id,
		Method: method,
		Params: paramsJSON,
	}
	return json.Marshal(req)
}

// DecodeResponse unmarshals a Response from JSON bytes.
func DecodeResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EncodeResponse marshals a Response to JSON bytes.
// This is used by Wasm modules to send results back to the host.
func EncodeResponse(id uint64, result any, err *WasmError) ([]byte, error) {
	var resultJSON json.RawMessage
	if result != nil && err == nil {
		var e error
		resultJSON, e = json.Marshal(result)
		if e != nil {
			return nil, e
		}
	}

	resp := Response{
		ID:     id,
		Result: resultJSON,
		Error:  err,
	}
	return json.Marshal(resp)
}

// DecodeRequest unmarshals a Request from JSON bytes.
// This is used by Wasm modules to parse incoming requests.
func DecodeRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}
