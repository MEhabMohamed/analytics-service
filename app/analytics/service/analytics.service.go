// Package service contains the AnalyticsService struct and its methods, split across cohesive files:
//   - analytics.service.go  : Service struct, constructor New(), and shared date/currency helpers
//   - events.service.go     : Inbound RabbitMQ event write projections (OnOrderPlaced, OnOrderDelivered, ...)
//   - restaurant.service.go : Restaurant, branch, and product HTTP read projections
//   - platform.service.go   : Platform-wide admin HTTP read projections
//
// TS vs Go Syntax Note (Splitting Large Files Across a Package):
//   - In TypeScript (`order-service`), splitting a class or module across multiple files requires
//     explicit relative `import { helper } from "./order.helpers"` statements or inheritance/mixins.
//   - In Go, the unit of compilation and visibility is the **package** (directory), not the single file!
//     All `.go` files inside `app/analytics/service/` declare `package service` and automatically share
//     `Service`, its private fields (`s.restaurantRepo`), and package-private functions (`extractUTCDate`)
//     without needing intra-package `import` statements. You can even split methods of the same `Service`
//     struct across multiple files (`events.service.go`, `restaurant.service.go`, `platform.service.go`).
package service

import (
	"strings"
	"time"

	"quickbite/analytics-service/app/analytics/repository"
)

// Service orchestrates analytics write rollups (from RabbitMQ events) and read projections (for HTTP endpoints).
type Service struct {
	restaurantRepo *repository.RestaurantDayRepo
	branchRepo     *repository.BranchDayRepo
	productRepo    *repository.ProductDayRepo
	platformRepo   *repository.PlatformDayRepo
}

// New constructs an AnalyticsService with explicit dependency injection.
func New(
	restaurantRepo *repository.RestaurantDayRepo,
	branchRepo *repository.BranchDayRepo,
	productRepo *repository.ProductDayRepo,
	platformRepo *repository.PlatformDayRepo,
) *Service {
	return &Service{
		restaurantRepo: restaurantRepo,
		branchRepo:     branchRepo,
		productRepo:    productRepo,
		platformRepo:   platformRepo,
	}
}

// ─── Package-Private Helpers (shared across service/*.go) ────────────────────

func extractUTCDate(iso string) string {
	trimmed := strings.TrimSpace(iso)
	if trimmed != "" {
		if t, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return t.UTC().Format("2006-01-02")
		}
		if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return t.UTC().Format("2006-01-02")
		}
		if len(trimmed) >= 10 {
			if _, err := time.Parse("2006-01-02", trimmed[:10]); err == nil {
				return trimmed[:10]
			}
		}
	}
	return time.Now().UTC().Format("2006-01-02")
}

func computeDeliveryMs(placedAtISO, deliveredAtISO string) int64 {
	if placedAtISO == "" || deliveredAtISO == "" {
		return 0
	}
	p, err1 := time.Parse(time.RFC3339Nano, placedAtISO)
	d, err2 := time.Parse(time.RFC3339Nano, deliveredAtISO)
	if err1 != nil || err2 != nil {
		return 0
	}
	diff := d.Sub(p).Milliseconds()
	if diff < 0 {
		return 0
	}
	return diff
}

func normalizeCurrency(c string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(c))
	if trimmed == "" {
		return "EGP"
	}
	return trimmed
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
