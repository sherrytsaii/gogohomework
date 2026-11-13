package apperrors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sherrytsaii/gogohomework/internal/apperrors"
)

func TestValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		field   string
		message string
		want    string
	}{
		{field: "name", message: "must not be blank", want: "name: must not be blank"},
		{field: "email", message: "must be a valid address", want: "email: must be a valid address"},
		{field: "status", message: "must be active or disabled", want: "status: must be active or disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			t.Parallel()
			original := &apperrors.ValidationError{Field: tt.field, Message: tt.message}
			if got := original.Error(); got != tt.want {
				t.Errorf("message = %q, want %q", got, tt.want)
			}

			wrapped := fmt.Errorf("save resource: %w", fmt.Errorf("validate input: %w", original))
			for _, err := range []error{original, wrapped} {
				if !errors.Is(err, apperrors.ErrInvalidInput) {
					t.Errorf("%v is not classified as invalid input", err)
				}
				if errors.Is(err, apperrors.ErrNotFound) || errors.Is(err, apperrors.ErrIDExhausted) {
					t.Errorf("%v incorrectly matches a storage error", err)
				}
				var details *apperrors.ValidationError
				if !errors.As(err, &details) {
					t.Fatalf("cannot retrieve validation details from %v", err)
				}
				if details != original {
					t.Errorf("validation details = %+v, want original error %+v", details, original)
				}
			}
		})
	}
}
