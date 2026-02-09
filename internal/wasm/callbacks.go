// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package wasm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// HostCallbacks defines the interface for host callback implementations.
// All methods validate capabilities before executing.
type HostCallbacks interface {
	// ShellRun executes a shell command with the given arguments.
	// Requires "shell.run" capability.
	ShellRun(ctx context.Context, cmd string, args []string, dir string) (stdout, stderr string, exitCode int, err error)

	// HTTPGet performs an HTTP GET request.
	// Requires "http.get" capability.
	HTTPGet(ctx context.Context, url string, headers map[string]string) (body []byte, statusCode int, err error)

	// FSRead reads a file from the filesystem.
	// Requires "fs.read" capability and fs.read path permission.
	FSRead(ctx context.Context, path string) ([]byte, error)

	// FSWrite writes data to a file on the filesystem.
	// Requires "fs.write" capability and fs.write path permission.
	FSWrite(ctx context.Context, path string, data []byte) error
}

// DefaultCallbacks provides the default implementation of HostCallbacks.
type DefaultCallbacks struct {
	checker *CapabilityChecker

	// ShellTimeout is the maximum duration for shell commands.
	ShellTimeout time.Duration

	// HTTPTimeout is the maximum duration for HTTP requests.
	HTTPTimeout time.Duration

	// MaxFileSize is the maximum file size for read/write operations.
	MaxFileSize int64
}

// NewDefaultCallbacks creates a new DefaultCallbacks with the given checker.
func NewDefaultCallbacks(checker *CapabilityChecker) *DefaultCallbacks {
	return &DefaultCallbacks{
		checker:      checker,
		ShellTimeout: 60 * time.Second,
		HTTPTimeout:  30 * time.Second,
		MaxFileSize:  10 * 1024 * 1024, // 10MB
	}
}

// ShellRun executes a shell command.
func (c *DefaultCallbacks) ShellRun(ctx context.Context, cmd string, args []string, dir string) (stdout, stderr string, exitCode int, err error) {
	// Check capability
	if err := c.checker.CheckHostCall("shell.run"); err != nil {
		return "", "", -1, err
	}

	// Validate working directory if specified
	if dir != "" {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return "", "", -1, NewWasmErrorf(ErrCodeInternal, "resolve dir: %v", err)
		}
		// Working directory must be readable
		if !c.checker.AllowsRead(absDir) {
			return "", "", -1, CapabilityError("read", absDir)
		}
	}

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, c.ShellTimeout)
	defer cancel()

	execCmd := exec.CommandContext(cmdCtx, cmd, args...)
	if dir != "" {
		execCmd.Dir = dir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	execCmd.Stdout = &stdoutBuf
	execCmd.Stderr = &stderrBuf

	runErr := execCmd.Run()
	exitCode = 0

	// Check for timeout first - context error takes priority
	if cmdCtx.Err() == context.DeadlineExceeded {
		return "", "", -1, NewWasmError(ErrCodeTimeout, "shell command timeout")
	}
	if cmdCtx.Err() == context.Canceled {
		return "", "", -1, NewWasmError(ErrCodeCanceled, "shell command canceled")
	}

	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			runErr = nil // Non-zero exit is not an error
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, runErr
}

// HTTPGet performs an HTTP GET request.
func (c *DefaultCallbacks) HTTPGet(ctx context.Context, url string, headers map[string]string) (body []byte, statusCode int, err error) {
	// Check capability
	if err := c.checker.CheckHostCall("http.get"); err != nil {
		return nil, 0, err
	}

	// Create request with timeout
	reqCtx, cancel := context.WithTimeout(ctx, c.HTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, NewWasmErrorf(ErrCodeInternal, "create request: %v", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if reqCtx.Err() == context.DeadlineExceeded {
			return nil, 0, NewWasmError(ErrCodeTimeout, "http request timeout")
		}
		return nil, 0, NewWasmErrorf(ErrCodeInternal, "http request: %v", err)
	}
	defer resp.Body.Close()

	// Limit response body size
	limitReader := io.LimitReader(resp.Body, c.MaxFileSize)
	body, err = io.ReadAll(limitReader)
	if err != nil {
		return nil, 0, NewWasmErrorf(ErrCodeInternal, "read response: %v", err)
	}

	return body, resp.StatusCode, nil
}

// FSRead reads a file from the filesystem.
func (c *DefaultCallbacks) FSRead(ctx context.Context, path string) ([]byte, error) {
	// Check capability for fs.read host call
	if err := c.checker.CheckHostCall("fs.read"); err != nil {
		return nil, err
	}

	// Check path is readable
	if err := c.checker.CheckRead(path); err != nil {
		return nil, err
	}

	// Check file size before reading
	info, err := os.Stat(path)
	if err != nil {
		return nil, NewWasmErrorf(ErrCodeInternal, "stat file: %v", err)
	}
	if info.Size() > c.MaxFileSize {
		return nil, NewWasmErrorf(ErrCodeInternal, "file too large: %d > %d", info.Size(), c.MaxFileSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, NewWasmErrorf(ErrCodeInternal, "read file: %v", err)
	}

	return data, nil
}

// FSWrite writes data to a file.
func (c *DefaultCallbacks) FSWrite(ctx context.Context, path string, data []byte) error {
	// Check capability for fs.write host call
	if err := c.checker.CheckHostCall("fs.write"); err != nil {
		return err
	}

	// Check path is writable
	if err := c.checker.CheckWrite(path); err != nil {
		return err
	}

	// Check data size
	if int64(len(data)) > c.MaxFileSize {
		return NewWasmErrorf(ErrCodeInternal, "data too large: %d > %d", len(data), c.MaxFileSize)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewWasmErrorf(ErrCodeInternal, "create directory: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return NewWasmErrorf(ErrCodeInternal, "write file: %v", err)
	}

	return nil
}
