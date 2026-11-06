// Package analytics_test verifies the HTTP routing, JWT auth, RBAC read-through cache,
// validation rules, and Response DTO envelopes of analytics-service.
package analytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/app/analytics/controller"
	"quickbite/analytics-service/app/analytics/dto"
	"quickbite/analytics-service/app/analytics/service"
	"quickbite/analytics-service/lib/logger"
	"quickbite/analytics-service/lib/rbac"
)

type stubPermissionFetcher struct {
	perms map[string][]string
	calls int
}

func (f *stubPermissionFetcher) GetPermissionsByRole(_ context.Context, role string) ([]string, error) {
	f.calls++
	return f.perms[role], nil
}

func mintTestJWT(t *testing.T, secret string, userID int64, role string, restID int64, restRole string, branches []int64) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId":         userID,
		"role":           role,
		"email":          "test@quickbite.local",
		"restaurantId":   restID,
		"restaurantRole": restRole,
		"branchIds":      branches,
		"exp":            time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return s
}

func TestAuthRBACAndValidationEnvelopes(t *testing.T) {
	const secret = "test-secret"
	log := logger.New()

	fetcher := &stubPermissionFetcher{
		perms: map[string][]string{
			"owner":          {analytics.PermAnalyticsRead},
			"branch_manager": {analytics.PermAnalyticsRead},
			"staff":          {"orders:read"},
		},
	}
	rbacCache := rbac.NewCache(fetcher, 5*time.Minute)
	svc := service.New(nil, nil, nil, nil)
	ctrl := controller.New(svc)

	r := chi.NewRouter()
	controller.RegisterRoutes(r, ctrl, secret, rbacCache, log)

	ownerToken := mintTestJWT(t, secret, 1, "restaurant_user", 42, "owner", []int64{1, 2})
	staffToken := mintTestJWT(t, secret, 2, "restaurant_user", 42, "staff", []int64{1})

	// 1. Missing JWT -> 401 UNAUTHENTICATED
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-11-01&to=2026-11-06", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Garbage cookie -> 401 UNAUTHENTICATED
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-11-01&to=2026-11-06", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "garbage.jwt.token"})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for garbage cookie, got %d", rec.Code)
	}

	// 3. Staff without analytics:read -> 403 FORBIDDEN
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-11-01&to=2026-11-06", nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for staff lacking analytics:read, got %d", rec.Code)
	}

	// 4. Wrong restaurantId (restaurant 99 with token for 42) -> 403 FORBIDDEN
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/99/days?from=2026-11-01&to=2026-11-06", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong restaurantId, got %d", rec.Code)
	}

	// 5. Invalid path restaurantId ("abc") -> 400 VALIDATION_ERROR
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/abc/days?from=2026-11-01&to=2026-11-06", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid restaurantId, got %d", rec.Code)
	}

	// 6. Missing `from` query param -> 400 VALIDATION_ERROR
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?to=2026-11-06", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing from, got %d", rec.Code)
	}

	// 7. `from > to` -> 400 ANALYTICS_INVALID_DATE_RANGE
	req = httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-11-10&to=2026-11-01", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for from > to, got %d", rec.Code)
	}
	var errEnv struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &errEnv)
	if errEnv.Error.Code != "ANALYTICS_INVALID_DATE_RANGE" {
		t.Fatalf("expected error code ANALYTICS_INVALID_DATE_RANGE, got %q", errEnv.Error.Code)
	}

	// 8. Verify RBAC cache invalidation on rbac.permissions_changed
	callsBefore := fetcher.calls
	_, _ = rbacCache.HasPermission(context.Background(), "owner", analytics.PermAnalyticsRead)
	if fetcher.calls != callsBefore {
		t.Fatalf("expected RBAC cache hit without upstream fetch")
	}
	rbacCache.Invalidate("owner")
	_, _ = rbacCache.HasPermission(context.Background(), "owner", analytics.PermAnalyticsRead)
	if fetcher.calls != callsBefore+1 {
		t.Fatalf("expected upstream fetch after rbacCache.Invalidate('owner')")
	}

	// 9. Verify empty slice Response DTO serializes as `[]` (never `null`)
	emptyDTO := dto.FromRestaurantDayRows(nil)
	rawJSON, _ := json.Marshal(emptyDTO)
	if string(rawJSON) != "[]" {
		t.Fatalf("expected empty DTO slice to serialize as [], got %s", string(rawJSON))
	}
}

