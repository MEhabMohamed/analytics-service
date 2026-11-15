package events_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/tests/helpers"
)

func TestEventsIngestionAndIdempotency_Integration(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	db, svc, eventIDsRepo := helpers.SetupTestDB(t, cfg)
	ctx := context.Background()

	// 1. Ingest order.placed -> updates all 4 aggregate collections
	err := svc.OnOrderPlaced(ctx, analytics.OnOrderPlacedInput{
		OrderID:       "ord-1001",
		Region:        "eg",
		CountryCode:   "eg",
		RestaurantID:  42,
		BranchID:      7,
		CustomerID:    101,
		Status:        "placed",
		PaymentMethod: "online",
		Subtotal:      10000,
		DeliveryFee:   1500,
		ServiceFee:    500,
		Total:         12000,
		Currency:      "EGP",
		PlacedAt:      "2026-11-10T12:00:00Z",
		Items: []analytics.OrderPlacedItemInput{
			{ProductID: 501, Quantity: 2, UnitPrice: 5000, LineTotal: 10000},
		},
	})
	if err != nil {
		t.Fatalf("OnOrderPlaced failed: %v", err)
	}

	// 2. Ingest payment.completed and order.delivered
	err = svc.OnPaymentCompleted(ctx, analytics.OnPaymentCompletedInput{
		OrderID:      "ord-1001",
		Region:       "eg",
		RestaurantID: 42,
		BranchID:     7,
		Amount:       12000,
		Currency:     "EGP",
		CompletedAt:  "2026-11-10T12:01:00Z",
	})
	if err != nil {
		t.Fatalf("OnPaymentCompleted failed: %v", err)
	}

	err = svc.OnOrderDelivered(ctx, analytics.OnOrderDeliveredInput{
		OrderID:       "ord-1001",
		Region:        "eg",
		RestaurantID:  42,
		BranchID:      7,
		Total:         12000,
		Currency:      "EGP",
		PaymentMethod: "online",
		PlacedAt:      "2026-11-10T12:00:00Z",
		DeliveredAt:   "2026-11-10T12:30:00Z", // 30 minutes = 1,800,000 ms
	})
	if err != nil {
		t.Fatalf("OnOrderDelivered failed: %v", err)
	}

	// 3. Verify agg_restaurant_day document in MongoDB
	var restDoc bson.M
	if err := db.Collection("agg_restaurant_day").FindOne(ctx, bson.M{
		"restaurant_id": int64(42),
		"date":          "2026-11-10",
		"currency":      "EGP",
	}).Decode(&restDoc); err != nil {
		t.Fatalf("find agg_restaurant_day: %v", err)
	}
	if restDoc["orders_count"] != int64(1) || restDoc["delivered_count"] != int64(1) {
		t.Fatalf("unexpected restaurant counters: %+v", restDoc)
	}
	if restDoc["revenue_sum"] != int64(12000) || restDoc["delivery_ms_sum"] != int64(1800000) {
		t.Fatalf("unexpected restaurant revenue/delivery_ms: %+v", restDoc)
	}

	// 4. Verify agg_product_day document
	var prodDoc bson.M
	if err := db.Collection("agg_product_day").FindOne(ctx, bson.M{
		"restaurant_id": int64(42),
		"branch_id":     int64(7),
		"product_id":    int64(501),
		"date":          "2026-11-10",
	}).Decode(&prodDoc); err != nil {
		t.Fatalf("find agg_product_day: %v", err)
	}
	if prodDoc["quantity_sum"] != int64(2) || prodDoc["revenue_sum"] != int64(10000) {
		t.Fatalf("unexpected product doc: %+v", prodDoc)
	}

	// 5. Verify event_ids deduplication
	fresh, err := eventIDsRepo.MarkSeen(ctx, "evt-unique-1", "order.placed")
	if err != nil || !fresh {
		t.Fatalf("first MarkSeen expected true, got fresh=%v err=%v", fresh, err)
	}
	dup, err := eventIDsRepo.MarkSeen(ctx, "evt-unique-1", "order.placed")
	if err != nil || dup {
		t.Fatalf("duplicate MarkSeen expected false, got dup=%v err=%v", dup, err)
	}
}

