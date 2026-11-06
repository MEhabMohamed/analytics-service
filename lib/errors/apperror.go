// Package errors defines operational AppErrors and the HTTP error-rendering wrapper.
//
// TS vs Go Syntax Note:
//   - In TypeScript (`order-service/src/lib/error/AppError.ts`), `class AppError extends Error`
//     inherits from the built-in `Error` prototype.
//   - Go has no class inheritance (`extends`). Instead, any struct that implements `Error() string`
//     satisfies the built-in `error` interface automatically. Implementing `Unwrap() error` allows
//     `errors.As(err, &appErr)` to inspect wrapped error chains (Go's equivalent of `err instanceof AppError`).
package errors

import "net/http"

// AppError represents a structured operational error with a stable code, HTTP status, and message.
// Equivalent to AppError in order-service/src/lib/error/AppError.ts.
type AppError struct {
	Code    string
	Status  int
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// New constructs a named AppError.
func New(code string, status int, message string) *AppError {
	return &AppError{
		Code:    code,
		Status:  status,
		Message: message,
	}
}

// WithMessage returns a shallow copy of the AppError with a custom detail message while keeping its Code and Status.
func (e *AppError) WithMessage(msg string) *AppError {
	return &AppError{
		Code:    e.Code,
		Status:  e.Status,
		Message: msg,
		Cause:   e.Cause,
	}
}

// Common cross-cutting errors used by middleware.
var (
	ErrUnauthenticated = New("UNAUTHENTICATED", http.StatusUnauthorized, "Authentication required")
	ErrForbidden       = New("FORBIDDEN", http.StatusForbidden, "Permission denied")
	ErrValidation      = New("VALIDATION_ERROR", http.StatusBadRequest, "Validation error")
	ErrInternal        = New("INTERNAL_ERROR", http.StatusInternalServerError, "Something went wrong")
)

