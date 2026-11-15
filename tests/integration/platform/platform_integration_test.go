package platform_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/app/analytics/controller"
	"quickbite/analytics-service/lib/coreclient"
	"quickbite/analytics-service/lib/logger"
	"quickbite/analytics-service/lib/rbac"
	"quickbite/analytics-service/tests/helpers"
)

func TestPlatformAnalyticsEndpoints_Integration(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	_, svc, _ := helpers.SetupTestDB(t, cfg)
	ctx := context.Background()

	// Seed multi-region orders (eg & ksa)
	if err := svc.OnOrderPlaced(ctx, analytics.OnOrderPlacedInput{
		OrderID:       "ord-eg-1",
		Region:        "eg",
		RestaurantID:  10,
		BranchID:      1,
		PaymentMethod: "cod",
		Total:         15000,
		ServiceFee:    500,
		DeliveryFee:   1500,
		Currency:      "EGP",
		PlacedAt:      "2026-11-12T10:00:00Z",
	}); err != nil {
		t.Fatalf("seed eg order: %v", err)
	}

	if err := svc.OnOrderPlaced(ctx, analytics.OnOrderPlacedInput{
		OrderID:       "ord-ksa-1",
		Region:        "ksa",
		RestaurantID:  20,
		BranchID:      2,
		PaymentMethod: "online",
		Total:         8000,
		ServiceFee:    300,
		DeliveryFee:   800,
		Currency:      "SAR",
		PlacedAt:      "2026-11-12T11:00:00Z",
	}); err != nil {
		t.Fatalf("seed ksa order: %v", err)
	}

	coreSrv := helpers.NewTestCoreServer(t, cfg.CoreInternalAPIKey)
	cc := coreclient.New(coreSrv.URL, cfg.CoreInternalAPIKey)
	rbacCache := rbac.NewCache(cc, 5*time.Minute)

	log := logger.New()
	r := chi.NewRouter()
	controller.RegisterRoutes(r, controller.New(svc), cfg.AccessSecret, rbacCache, log)

	adminToken := helpers.MintTestJWT(t, cfg.AccessSecret, 1, "system_admin", nil, nil, nil)
	ownerToken := helpers.MintTestJWT(t, cfg.AccessSecret, 3, "restaurant_user", helpers.PtrInt64(10), helpers.PtrString("owner"), []int64{1})

	// 1. system_admin queries /platform/days?region=eg -> returns only eg row
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/platform/days?from=2026-11-10&to=2026-11-15&region=eg", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /platform/days expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var daysEnv struct {
		Success bool `json:"success"`
		Data    []struct {
			Region      string `json:"region"`
			Currency    string `json:"currency"`
			OrdersCount int64  `json:"ordersCount"`
			GmvMinor    int64  `json:"gmvMinor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &daysEnv); err != nil {
		t.Fatalf("decode /platform/days: %v", err)
	}
	if !daysEnv.Success || len(daysEnv.Data) != 1 || daysEnv.Data[0].Region != "eg" || daysEnv.Data[0].GmvMinor != 15000 {
		t.Fatalf("unexpected /platform/days response: %+v", daysEnv)
	}

	// 2. system_admin queries /platform/active-restaurants -> returns 2 active restaurants across eg + ksa
	reqActive := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/platform/active-restaurants?from=2026-11-10&to=2026-11-15", nil)
	reqActive.Header.Set("Authorization", "Bearer "+adminToken)
	recActive := httptest.NewRecorder()
	r.ServeHTTP(recActive, reqActive)

	if recActive.Code != http.StatusOK {
		t.Fatalf("GET /platform/active-restaurants expected 200, got %d: %s", recActive.Code, recActive.Body.String())
	}

	// 3. Non-admin (restaurant owner) calling /platform/days -> 403 Forbidden
	reqForbidden := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/platform/days?from=2026-11-10&to=2026-11-15", nil)
	reqForbidden.Header.Set("Authorization", "Bearer "+ownerToken)
	recForbidden := httptest.NewRecorder()
	r.ServeHTTP(recForbidden, reqForbidden)

	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin on /platform/days, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}
}

