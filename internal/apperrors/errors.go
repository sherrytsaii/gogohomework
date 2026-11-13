// Package apperrors defines errors shared across application features.
// Use errors.Is for categories and errors.As for validation details.
package apperrors

import "errors"

var (
	// ErrInvalidInput identifies invalid application input.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound identifies a missing resource.
	ErrNotFound = errors.New("resource not found")
	// ErrIDExhausted indicates that no unique IDs remain.
	ErrIDExhausted = errors.New("identifier limit reached")
)

// ValidationError describes an invalid field and unwraps to ErrInvalidInput.
type ValidationError struct {
	// Field identifies the invalid input.
	Field string
	// Message is a client-facing explanation.
	Message string
}

// Error returns "field: message".
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// Unwrap returns ErrInvalidInput for errors.Is.
func (e *ValidationError) Unwrap() error {
	return ErrInvalidInput
}
