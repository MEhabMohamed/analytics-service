package dto_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"quickbite/analytics-service/app/analytics/dto"
)

func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestParseRestaurantDateRange(t *testing.T) {
	t.Run("valid query with branchId and limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-10-01&to=2026-10-31&branchId=7&limit=25", nil)
		req = withChiParam(req, "restaurantId", "42")

		q, err := dto.ParseRestaurantDateRange(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.RestaurantID != 42 || q.BranchID != 7 || q.Limit != 25 || q.From != "2026-10-01" || q.To != "2026-10-31" {
			t.Fatalf("unexpected parsed query: %+v", q)
		}
	})

	t.Run("rejects from > to", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-11-10&to=2026-11-01", nil)
		req = withChiParam(req, "restaurantId", "42")

		if _, err := dto.ParseRestaurantDateRange(req); err == nil {
			t.Fatal("expected error when from > to, got nil")
		}
	})

	t.Run("rejects invalid restaurantId", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/abc/days?from=2026-11-01&to=2026-11-05", nil)
		req = withChiParam(req, "restaurantId", "abc")

		if _, err := dto.ParseRestaurantDateRange(req); err == nil {
			t.Fatal("expected error for non-numeric restaurantId")
		}
	})
}

func TestParsePlatformDateRange(t *testing.T) {
	t.Run("normalizes region to lowercase", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/platform/days?from=2026-11-01&to=2026-11-05&region=EG", nil)
		q, err := dto.ParsePlatformDateRange(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.Region != "eg" {
			t.Errorf("region = %q, want %q", q.Region, "eg")
		}
	})

	t.Run("rejects malformed date format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/platform/days?from=11-01-2026&to=2026-11-05", nil)
		if _, err := dto.ParsePlatformDateRange(req); err == nil {
			t.Fatal("expected error for malformed date")
		}
	})
}

