// Package auth provides HTTP authentication and role/ownership authorization middlewares.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/auth/guard.ts` and `rbac.ts` (`authenticate`, `requireAdmin`,
//     `requireRestaurantMember`, `requireBranchAccess`).
//   - Checks the `access_token` cookie first (matching core-service / order-service browser sessions),
//     then falls back to `Authorization: Bearer <token>` for CLI/API callers.
package auth

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"quickbite/analytics-service/lib/appcontext"
	apperr "quickbite/analytics-service/lib/errors"
)

const (
	RoleSystemAdmin    = "system_admin"
	RoleRestaurantUser = "restaurant_user"
)

// Authenticate verifies the JWT from the access_token cookie first, falling back to Authorization: Bearer.
func Authenticate(secret string, baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			claims, err := VerifyAccessToken(token, secret)
			if err != nil {
				apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated)
				return
			}
			ctx := appcontext.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin gates a route to callers with the system_admin role.
func RequireAdmin(baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := appcontext.ClaimsFromContext(r.Context())
			if claims == nil {
				apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated)
				return
			}
			if claims.Role != RoleSystemAdmin {
				apperr.RenderError(w, r, baseLog, apperr.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRestaurantMember ensures the caller belongs to the restaurant named by paramName (system_admin bypasses).
// Also validates that paramName is a positive integer (> 0), returning 400 VALIDATION_ERROR if malformed.
func RequireRestaurantMember(paramName string, baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawID := chi.URLParam(r, paramName)
			restID, err := strconv.ParseInt(rawID, 10, 64)
			if err != nil || restID <= 0 {
				apperr.RenderError(w, r, baseLog, apperr.ErrValidation.WithMessage("Invalid restaurantId path parameter"))
				return
			}

			claims := appcontext.ClaimsFromContext(r.Context())
			if claims == nil {
				apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated)
				return
			}
			if claims.Role == RoleSystemAdmin {
				next.ServeHTTP(w, r)
				return
			}
			if claims.RestaurantID == nil || *claims.RestaurantID != restID {
				apperr.RenderError(w, r, baseLog, apperr.ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireBranchAccess ensures non-owner restaurant members can only query branches in their JWT branchIds list.
func RequireBranchAccess(paramOrQueryName string, baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := appcontext.ClaimsFromContext(r.Context())
			if claims == nil {
				apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated)
				return
			}
			if claims.Role == RoleSystemAdmin {
				next.ServeHTTP(w, r)
				return
			}
			if claims.RestaurantRole != nil && *claims.RestaurantRole == "owner" {
				next.ServeHTTP(w, r)
				return
			}

			rawBranch := chi.URLParam(r, paramOrQueryName)
			if rawBranch == "" {
				rawBranch = r.URL.Query().Get(paramOrQueryName)
			}
			if rawBranch == "" {
				next.ServeHTTP(w, r)
				return
			}

			branchID, err := strconv.ParseInt(rawBranch, 10, 64)
			if err != nil || branchID <= 0 {
				apperr.RenderError(w, r, baseLog, apperr.ErrValidation.WithMessage("Invalid branchId parameter"))
				return
			}

			for _, allowed := range claims.BranchIDs {
				if allowed == branchID {
					next.ServeHTTP(w, r)
					return
				}
			}
			apperr.RenderError(w, r, baseLog, apperr.ErrForbidden.WithMessage("You do not have access to this branch"))
		})
	}
}

func extractToken(r *http.Request) string {
	if c, err := r.Cookie("access_token"); err == nil && c.Value != "" {
		return c.Value
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}

