// Package appcontext provides typed context.Context keys and accessors for request-scoped metadata.
//
// TS vs Go Syntax Note:
//   - In Express (`order-service/src/lib/types/express.d.ts`), middleware attaches properties directly
//     onto the mutable `req` object (`req.user = ...`, `req.correlationId = ...`).
//   - Go's `http.Request` is not mutated with custom fields. Instead, middleware derives a new immutable
//     `context.Context` via `context.WithValue(ctx, key, val)` and attaches it with `r.WithContext(ctx)`.
//   - Using an unexported `type ctxKey int` prevents key collisions across packages.
package appcontext

import "context"

type ctxKey int

const (
	claimsKey ctxKey = iota + 1
	correlationIDKey
)

// Claims represents the authenticated user's JWT payload (mirrors JWTPayload in order-service/src/lib/auth/jwt.ts).
type Claims struct {
	UserID         int64   `json:"userId"`
	Role           string  `json:"role"`
	Email          string  `json:"email"`
	RestaurantID   *int64  `json:"restaurantId,omitempty"`
	RestaurantRole *string `json:"restaurantRole,omitempty"`
	BranchIDs      []int64 `json:"branchIds,omitempty"`
}

// WithClaims attaches JWT claims to a context.Context.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFromContext retrieves JWT claims from ctx, returning nil if absent.
func ClaimsFromContext(ctx context.Context) *Claims {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// WithCorrelationID attaches a correlation ID string to a context.Context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from ctx, returning "" if absent.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(correlationIDKey).(string)
	return id
}

