package domain

import "errors"

var (
	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrConflict indicates a conflict with the current state.
	ErrConflict = errors.New("conflict")

	// ErrInvalidInput indicates invalid input data.
	ErrInvalidInput = errors.New("invalid input")

	// ErrVersionMismatch indicates the specified answer version does not exist.
	ErrVersionMismatch = errors.New("version mismatch")

	// ErrCompilationFailed indicates the LLM compilation failed.
	ErrCompilationFailed = errors.New("compilation failed")

	// ErrValidationFailed indicates JSON schema validation failed.
	ErrValidationFailed = errors.New("validation failed")
)
