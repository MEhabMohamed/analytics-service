// Package analytics defines shared module-level operational errors.
//
// TS vs Go Syntax Note:
//   - In TypeScript (`order-service/src/app/order/errors.ts`), errors are exported class instances:
//     `export const OrderNotFoundError = new AppError("Order not found", 404);` and thrown via `throw`.
//   - In Go, errors are values implementing the built-in `interface { Error() string }`.
//     By convention, package-level sentinel errors are prefixed with `Err` (`ErrInvalidDateRange`)
//     inside a `var ( ... )` block and returned as the last return value (`return nil, analytics.ErrInvalidDateRange`).
package analytics

import (
	"net/http"

	apperr "quickbite/analytics-service/lib/errors"
)

// Module-level operational errors with stable codes and messages.
var (
	ErrInvalidDateRange = apperr.New("ANALYTICS_INVALID_DATE_RANGE", http.StatusBadRequest, "'from' date must not be after 'to' date")
	ErrValidation       = apperr.New("VALIDATION_ERROR", http.StatusBadRequest, "Invalid request parameters")
	ErrUnauthenticated  = apperr.ErrUnauthenticated
	ErrForbidden        = apperr.ErrForbidden
)

