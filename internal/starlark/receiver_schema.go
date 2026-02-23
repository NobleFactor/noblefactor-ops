// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package starlark

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.starlark.net/starlark"

	"github.com/NobleFactor/devlore-cli/pkg/op"
)

// SchemaReceiver provides JSON Schema validation operations.
// Implements starlark.Value and starlark.HasAttrs.
type SchemaReceiver struct {
	op.Receiver
}

// NewSchemaReceiver creates a new SchemaReceiver.
func NewSchemaReceiver() *SchemaReceiver {
	return &SchemaReceiver{Receiver: op.NewReceiver("schema")}
}

// Attr implements starlark.HasAttrs.
func (r *SchemaReceiver) Attr(name string) (starlark.Value, error) {
	switch name {
	case "validate":
		return op.MakeAttr("schema.validate", r.validate), nil
	default:
		return nil, op.NoSuchAttrError("schema", name)
	}
}

// AttrNames implements starlark.HasAttrs.
func (r *SchemaReceiver) AttrNames() []string {
	return []string{"validate"}
}

// validate validates data against a JSON Schema.
// Returns (valid: bool, errors: list[string]).
//
// Usage:
//
//	valid, errors = schema.validate(data, schema_json)
func (r *SchemaReceiver) validate(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var data starlark.Value
	var schemaJSON string
	if err := starlark.UnpackArgs("schema.validate", args, kwargs, "data", &data, "schema", &schemaJSON); err != nil {
		return nil, err
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaJSON)); err != nil {
		return nil, fmt.Errorf("schema.validate: invalid schema: %w", err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("schema.validate: compile schema: %w", err)
	}

	// Convert Starlark value to Go, then to JSON for validation
	goVal := starlarkToGo(data)
	jsonBytes, err := json.Marshal(goVal)
	if err != nil {
		return nil, fmt.Errorf("schema.validate: marshal data: %w", err)
	}

	// Unmarshal back to interface{} for the validator
	var jsonVal interface{}
	if err := json.Unmarshal(jsonBytes, &jsonVal); err != nil {
		return nil, fmt.Errorf("schema.validate: unmarshal data: %w", err)
	}

	// Validate
	validationErr := schema.Validate(jsonVal)
	if validationErr == nil {
		// Valid - return (true, [])
		return starlark.Tuple{starlark.Bool(true), starlark.NewList(nil)}, nil
	}

	// Invalid - extract error messages
	var errorList []starlark.Value
	var ve *jsonschema.ValidationError
	if errors.As(validationErr, &ve) {
		validationErrors := flattenValidationErrors(ve)
		for _, e := range validationErrors {
			errorList = append(errorList, starlark.String(e))
		}
	} else {
		errorList = append(errorList, starlark.String(validationErr.Error()))
	}

	return starlark.Tuple{starlark.Bool(false), starlark.NewList(errorList)}, nil
}

// flattenValidationErrors extracts all error messages from a ValidationError tree.
func flattenValidationErrors(ve *jsonschema.ValidationError) []string {
	var errMsgs []string

	// Build location string
	location := ve.InstanceLocation
	if location == "" {
		location = "(root)"
	}

	// Add this error's message if it has one
	if ve.Message != "" {
		errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", location, ve.Message))
	}

	// Recurse into causes
	for _, cause := range ve.Causes {
		errMsgs = append(errMsgs, flattenValidationErrors(cause)...)
	}

	return errMsgs
}
