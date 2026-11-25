package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors. Wrap them with context (fmt.Errorf("%w: ...")) and the
// HTTP layer maps them to status codes with errors.Is.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// ValidationError describes bad input in a way that is safe to show to users.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

// Invalid is a shorthand for building a *ValidationError.
func Invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// NotFound wraps ErrNotFound with a readable message, e.g. "vehicle veh_x not found".
func NotFound(kind, id string) error {
	return fmt.Errorf("%s %s %w", kind, id, ErrNotFound)
}
