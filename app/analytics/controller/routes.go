// Package controller also defines route mounting for the analytics module.
//
// TS vs Go Syntax Note:
//   - In Express (`order-service/src/app/order/routes.ts`), middleware is passed inline per route:
//     `router.get("/restaurants/:restaurantId/orders", authenticate, rbac(...), requireRestaurantMember(), ctrl.list)`.
//   - In Chi (`github.com/go-chi/chi/v5`), URL parameters use `{restaurantId}` braces instead of `:restaurantId`,
//     and middleware chains are grouped cleanly with `r.Route(..., func(r chi.Router) { r.Use(...); r.Get(...) })`
//     or `r.With(mw).Get(...)`.
//   - Notice `apperr.Wrap(log, ctrl.GetRestaurantDays)`: since Go's `http.HandlerFunc` does not return an `error`
//     (unlike Express v5 which catches rejected promises automatically), `apperr.Wrap` bridges our error-returning
//     controller methods to standard `http.HandlerFunc`.
package controller

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/lib/auth"
	apperr "quickbite/analytics-service/lib/errors"
	"quickbite/analytics-service/lib/rbac"
)

// RegisterRoutes mounts all 8 analytics endpoints under /api/v1/analytics.
func RegisterRoutes(
	r chi.Router,
	ctrl *Controller,
	jwtSecret string,
	rbacCache *rbac.Cache,
	log *slog.Logger,
) {
	r.Route("/api/v1/analytics", func(r chi.Router) {
		// All analytics routes require a valid JWT
		r.Use(auth.Authenticate(jwtSecret, log))

		// Restaurant-scoped analytics (5 endpoints)
		// Guarded by RequireRestaurantMember("restaurantId") + RBAC "analytics:read"
		r.Route("/restaurants/{restaurantId}", func(r chi.Router) {
			r.Use(auth.RequireRestaurantMember("restaurantId", log))
			r.Use(rbac.Require(rbacCache, analytics.PermAnalyticsRead, log))

			r.Get("/days", apperr.Wrap(log, ctrl.GetRestaurantDays))
			r.Get("/failures", apperr.Wrap(log, ctrl.GetRestaurantFailures))
			r.Get("/delivery-avg", apperr.Wrap(log, ctrl.GetRestaurantDeliveryAvg))

			// Branch-granular and product-granular routes also enforce branchIds access for non-owners
			r.With(auth.RequireBranchAccess("branchId", log)).Get("/branches/days", apperr.Wrap(log, ctrl.GetBranchDays))
			r.With(auth.RequireBranchAccess("branchId", log)).Get("/products/top", apperr.Wrap(log, ctrl.GetTopProducts))
		})

		// Platform-wide admin analytics (3 endpoints)
		// Guarded by RequireAdmin (system_admin role)
		r.Route("/platform", func(r chi.Router) {
			r.Use(auth.RequireAdmin(log))

			r.Get("/days", apperr.Wrap(log, ctrl.GetPlatformDays))
			r.Get("/active-restaurants", apperr.Wrap(log, ctrl.GetActiveRestaurants))
			r.Get("/failures", apperr.Wrap(log, ctrl.GetPlatformFailures))
		})
	})
}

