// Package apperrors defines errors shared across application features.
package apperrors

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("resource not found")
	ErrIDExhausted  = errors.New("identifier limit reached")
)

// ValidationError describes an invalid field without prescribing its domain rules.
// It unwraps to ErrInvalidInput so callers can classify it with errors.Is.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalidInput
}
