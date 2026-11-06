// Package rbac provides the Require("permission") HTTP middleware.
//
// TS vs Go Syntax Note:
//   - Mirrors `rbac({ resource, action })` in `order-service/src/lib/auth/rbac.ts`.
//   - `system_admin` bypasses RBAC checks; `restaurant_user` roles (`owner`, `branch_manager`, `staff`)
//     are checked via the read-through `rbac.Cache`.
package rbac

import (
	"log/slog"
	"net/http"

	"quickbite/analytics-service/lib/appcontext"
	"quickbite/analytics-service/lib/auth"
	apperr "quickbite/analytics-service/lib/errors"
)

// Require returns an HTTP middleware that enforces the given permission (e.g. "analytics:read").
// system_admin bypasses RBAC checks; restaurant_user roles are checked via the read-through Cache.
func Require(cache *Cache, perm string, baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := appcontext.ClaimsFromContext(r.Context())
			if claims == nil {
				apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated)
				return
			}

			if claims.Role == auth.RoleSystemAdmin {
				next.ServeHTTP(w, r)
				return
			}

			if claims.Role == auth.RoleRestaurantUser && claims.RestaurantRole != nil {
				allowed, err := cache.HasPermission(r.Context(), *claims.RestaurantRole, perm)
				if err != nil {
					apperr.RenderError(w, r, baseLog, err)
					return
				}
				if !allowed {
					apperr.RenderError(w, r, baseLog, apperr.ErrForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			apperr.RenderError(w, r, baseLog, apperr.ErrForbidden)
		})
	}
}

