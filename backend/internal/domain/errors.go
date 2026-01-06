package domain

import "errors"

var (
	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("the requested resource was not found")

	// ErrConflict indicates a conflict with the current state.
	ErrConflict = errors.New("the operation conflicts with the current state")

	// ErrInvalidInput indicates invalid input data.
	ErrInvalidInput = errors.New("the provided input is invalid or malformed")

	// ErrVersionMismatch indicates the specified answer version does not exist.
	ErrVersionMismatch = errors.New("the specified answer version does not exist or has been superseded")

	// ErrCompilationFailed indicates the LLM compilation failed.
	ErrCompilationFailed = errors.New("specification compilation failed")

	// ErrValidationFailed indicates JSON schema validation failed.
	ErrValidationFailed = errors.New("the compiled specification failed schema validation")
)
