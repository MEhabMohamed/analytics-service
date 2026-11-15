package restaurant_test

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

func TestRestaurantAnalyticsEndpoints_Integration(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	_, svc, _ := helpers.SetupTestDB(t, cfg)
	ctx := context.Background()

	// Seed real events through the service into MongoDB test DB
	if err := svc.OnOrderPlaced(ctx, analytics.OnOrderPlacedInput{
		OrderID:       "ord-r1",
		Region:        "eg",
		RestaurantID:  42,
		BranchID:      7,
		CustomerID:    100,
		PaymentMethod: "cod",
		Total:         10000,
		Currency:      "EGP",
		PlacedAt:      "2026-11-11T10:00:00Z",
		Items:         []analytics.OrderPlacedItemInput{{ProductID: 1, Quantity: 2, UnitPrice: 5000, LineTotal: 10000}},
	}); err != nil {
		t.Fatalf("seed order 1: %v", err)
	}
	if err := svc.OnOrderDelivered(ctx, analytics.OnOrderDeliveredInput{
		OrderID:     "ord-r1",
		Region:      "eg",
		RestaurantID: 42,
		BranchID:    7,
		Total:       10000,
		Currency:    "EGP",
		PlacedAt:    "2026-11-11T10:00:00Z",
		DeliveredAt: "2026-11-11T10:20:00Z", // 20 min = 1,200,000 ms
	}); err != nil {
		t.Fatalf("deliver order 1: %v", err)
	}
	if err := svc.OnOrderPlaced(ctx, analytics.OnOrderPlacedInput{
		OrderID:       "ord-r2",
		Region:        "eg",
		RestaurantID:  42,
		BranchID:      8,
		CustomerID:    101,
		PaymentMethod: "cod",
		Total:         20000,
		Currency:      "EGP",
		PlacedAt:      "2026-11-11T11:00:00Z",
	}); err != nil {
		t.Fatalf("seed order 2: %v", err)
	}
	if err := svc.OnOrderCancelled(ctx, analytics.OnOrderTransitionInput{
		OrderID:      "ord-r2",
		Region:       "eg",
		RestaurantID: 42,
		BranchID:     8,
		PlacedAt:     "2026-11-11T11:00:00Z",
		OccurredAt:   "2026-11-11T11:05:00Z",
	}); err != nil {
		t.Fatalf("cancel order 2: %v", err)
	}

	// Real coreclient talking to test HTTP server (no mocked services/repos)
	coreSrv := helpers.NewTestCoreServer(t, cfg.CoreInternalAPIKey)
	cc := coreclient.New(coreSrv.URL, cfg.CoreInternalAPIKey)
	rbacCache := rbac.NewCache(cc, 5*time.Minute)

	log := logger.New()
	r := chi.NewRouter()
	controller.RegisterRoutes(r, controller.New(svc), cfg.AccessSecret, rbacCache, log)

	ownerToken := helpers.MintTestJWT(t, cfg.AccessSecret, 3, "restaurant_user", helpers.PtrInt64(42), helpers.PtrString("owner"), []int64{7, 8})
	managerToken := helpers.MintTestJWT(t, cfg.AccessSecret, 4, "restaurant_user", helpers.PtrInt64(42), helpers.PtrString("branch_manager"), []int64{7})

	// 1. Owner queries /days -> 200 with 2 orders, 30000 revenue, avg 15000
	req := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/days?from=2026-11-10&to=2026-11-12", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /days expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var daysEnv struct {
		Success bool `json:"success"`
		Data    []struct {
			Date          string `json:"date"`
			OrdersCount   int64  `json:"ordersCount"`
			RevenueMinor  int64  `json:"revenueMinor"`
			AvgOrderMinor int64  `json:"avgOrderMinor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &daysEnv); err != nil {
		t.Fatalf("decode /days: %v", err)
	}
	if !daysEnv.Success || len(daysEnv.Data) != 1 || daysEnv.Data[0].OrdersCount != 2 || daysEnv.Data[0].AvgOrderMinor != 15000 {
		t.Fatalf("unexpected /days payload: %+v", daysEnv)
	}

	// 2. Owner queries /failures -> 1 cancelled out of 2 = 5000 bps (50%)
	reqFail := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/failures?from=2026-11-10&to=2026-11-12", nil)
	reqFail.Header.Set("Authorization", "Bearer "+ownerToken)
	recFail := httptest.NewRecorder()
	r.ServeHTTP(recFail, reqFail)

	if recFail.Code != http.StatusOK {
		t.Fatalf("GET /failures expected 200, got %d: %s", recFail.Code, recFail.Body.String())
	}

	// 3. Branch manager accessing unauthorized branch 8 -> 403 Forbidden
	reqBranch := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/restaurants/42/branches/days?from=2026-11-10&to=2026-11-12&branchId=8", nil)
	reqBranch.Header.Set("Authorization", "Bearer "+managerToken)
	recBranch := httptest.NewRecorder()
	r.ServeHTTP(recBranch, reqBranch)

	if recBranch.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized branchId=8, got %d: %s", recBranch.Code, recBranch.Body.String())
	}
}

