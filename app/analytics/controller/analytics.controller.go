// Package controller implements the HTTP transport layer for analytics-service.
//
// TS vs Go Syntax Note:
//   - In order-service (`order.controller.ts`), controller methods are arrow-function properties
//     (`getOrder = async (req: Request, res: Response) => { ... }`) so `this` stays bound when
//     passed to Express routes.
//   - In Go, bound method values like `ctrl.GetRestaurantDays` automatically capture the receiver
//     pointer `ctrl` (`*Controller`) — no arrow-function workaround is needed!
//   - Each controller method follows the exact 4-step rule from CLAUDE.md:
//     1. Parse & validate path/query input (`dto.Parse...`)
//     2. Call service (`c.svc.Get...`)
//     3. Map service rows to Response DTO (`dto.From...`)
//     4. Write envelope (`httplib.SendSuccess(w, http.StatusOK, resp)`)
package controller

import (
	"net/http"

	"quickbite/analytics-service/app/analytics/dto"
	"quickbite/analytics-service/app/analytics/service"
	httplib "quickbite/analytics-service/lib/http"
)

// Controller handles HTTP requests for `/api/v1/analytics/...`.
type Controller struct {
	svc *service.Service
}

// New constructs an analytics Controller.
func New(svc *service.Service) *Controller {
	return &Controller{svc: svc}
}

// GetRestaurantDays handles GET /api/v1/analytics/restaurants/{restaurantId}/days?from=&to=
func (c *Controller) GetRestaurantDays(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParseRestaurantDateRange(r)
	if err != nil {
		return err
	}
	rows, err := c.svc.GetRestaurantDays(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromRestaurantDayRows(rows))
	return nil
}

// GetRestaurantFailures handles GET /api/v1/analytics/restaurants/{restaurantId}/failures?from=&to=
func (c *Controller) GetRestaurantFailures(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParseRestaurantDateRange(r)
	if err != nil {
		return err
	}
	summary, err := c.svc.GetRestaurantFailures(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromRestaurantFailuresSummary(summary))
	return nil
}

// GetRestaurantDeliveryAvg handles GET /api/v1/analytics/restaurants/{restaurantId}/delivery-avg?from=&to=
func (c *Controller) GetRestaurantDeliveryAvg(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParseRestaurantDateRange(r)
	if err != nil {
		return err
	}
	summary, err := c.svc.GetRestaurantDeliveryAvg(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromRestaurantDeliveryAvgSummary(summary))
	return nil
}

// GetBranchDays handles GET /api/v1/analytics/restaurants/{restaurantId}/branches/days?from=&to=&branchId=
func (c *Controller) GetBranchDays(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParseRestaurantDateRange(r)
	if err != nil {
		return err
	}
	rows, err := c.svc.GetBranchDays(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromBranchDayRows(rows))
	return nil
}

// GetTopProducts handles GET /api/v1/analytics/restaurants/{restaurantId}/products/top?from=&to=&branchId=&limit=
func (c *Controller) GetTopProducts(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParseRestaurantDateRange(r)
	if err != nil {
		return err
	}
	rows, err := c.svc.GetTopProducts(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromTopProductRows(rows))
	return nil
}

// GetPlatformDays handles GET /api/v1/analytics/platform/days?from=&to=&region=
func (c *Controller) GetPlatformDays(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParsePlatformDateRange(r)
	if err != nil {
		return err
	}
	rows, err := c.svc.GetPlatformDays(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromPlatformDayRows(rows))
	return nil
}

// GetActiveRestaurants handles GET /api/v1/analytics/platform/active-restaurants?from=&to=
func (c *Controller) GetActiveRestaurants(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParsePlatformDateRange(r)
	if err != nil {
		return err
	}
	summary, err := c.svc.GetActiveRestaurants(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromActiveRestaurantsSummary(summary))
	return nil
}

// GetPlatformFailures handles GET /api/v1/analytics/platform/failures?from=&to=&region=
func (c *Controller) GetPlatformFailures(w http.ResponseWriter, r *http.Request) error {
	q, err := dto.ParsePlatformDateRange(r)
	if err != nil {
		return err
	}
	summary, err := c.svc.GetPlatformFailures(r.Context(), q)
	if err != nil {
		return err
	}
	httplib.SendSuccess(w, http.StatusOK, dto.FromPlatformFailuresSummary(summary))
	return nil
}

