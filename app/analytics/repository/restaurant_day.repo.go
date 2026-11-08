// Package repository implements MongoDB data access for agg_restaurant_day.
//
// Divided into 2 cohesive files in `package repository`:
//   - restaurant_day.repo.go         : RestaurantDayRepo struct, constructor, and atomic write upserts
//   - restaurant_day_queries.repo.go : Read range query, active-restaurants aggregation pipeline, and BulkWrite
//
// TS vs Go Syntax Note:
//   - In `order-service` (`order.repo.ts`), repositories are exported functions taking `conn: Knex = db`.
//   - In Go, repositories are structs (`RestaurantDayRepo`) wrapping a `*mongo.Collection` handle
//     injected via `NewRestaurantDayRepo(db)`.
//   - Every method takes `ctx context.Context` as its FIRST argument (Go's standard convention for
//     propagating request cancellation and deadlines to DB/network calls, replacing Node's implicit
//     async context).
//   - Notice how MongoDB `$inc`, `$set`, and `$setOnInsert` with `SetUpsert(true)` replace Postgres
//     `INSERT ... ON CONFLICT DO UPDATE SET col = table.col + EXCLUDED.col`.
package repository

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"quickbite/analytics-service/app/analytics"
)

// RestaurantDayRepo manages the agg_restaurant_day collection.
type RestaurantDayRepo struct {
	coll *mongo.Collection
}

// NewRestaurantDayRepo constructs a RestaurantDayRepo.
func NewRestaurantDayRepo(db *mongo.Database) *RestaurantDayRepo {
	return &RestaurantDayRepo{
		coll: db.Collection(analytics.CollRestaurantDay),
	}
}

// UpsertOrderPlaced atomically increments orders_count and revenue_sum (and COD stats if COD).
// Safe under out-of-order arrival: if order.delivered created the row first, $inc merges cleanly.
func (r *RestaurantDayRepo) UpsertOrderPlaced(
	ctx context.Context,
	restaurantID int64,
	date string,
	currency string,
	totalMinor int64,
	paymentMethod string,
	now time.Time,
) error {
	filter := bson.M{
		"restaurant_id": restaurantID,
		"date":          date,
	}

	inc := bson.M{
		"orders_count": int64(1),
		"revenue_sum":  totalMinor,
	}
	if strings.EqualFold(paymentMethod, "cod") {
		inc["cod_payment_count"] = int64(1)
		inc["cod_payment_sum"] = totalMinor
	}

	setFields := bson.M{
		"updated_at": now,
	}
	if currency != "" {
		setFields["currency"] = currency
	}

	setOnInsert := bson.M{
		"restaurant_id":        restaurantID,
		"date":                 date,
		"delivered_count":      int64(0),
		"rejected_count":       int64(0),
		"cancelled_count":      int64(0),
		"online_payment_count": int64(0),
		"online_payment_sum":   int64(0),
		"delivery_ms_sum":      int64(0),
		"delivery_ms_count":    int64(0),
	}
	if !strings.EqualFold(paymentMethod, "cod") {
		setOnInsert["cod_payment_count"] = int64(0)
		setOnInsert["cod_payment_sum"] = int64(0)
	}

	update := bson.M{
		"$inc":         inc,
		"$set":         setFields,
		"$setOnInsert": setOnInsert,
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

// UpsertPaymentCompleted atomically increments online_payment_count and online_payment_sum.
func (r *RestaurantDayRepo) UpsertPaymentCompleted(
	ctx context.Context,
	restaurantID int64,
	date string,
	currency string,
	amountMinor int64,
	now time.Time,
) error {
	filter := bson.M{
		"restaurant_id": restaurantID,
		"date":          date,
	}
	setFields := bson.M{"updated_at": now}
	if currency != "" {
		setFields["currency"] = currency
	}
	update := bson.M{
		"$inc": bson.M{
			"online_payment_count": int64(1),
			"online_payment_sum":   amountMinor,
		},
		"$set": setFields,
		"$setOnInsert": bson.M{
			"restaurant_id":     restaurantID,
			"date":              date,
			"orders_count":      int64(0),
			"delivered_count":   int64(0),
			"rejected_count":    int64(0),
			"cancelled_count":   int64(0),
			"revenue_sum":       int64(0),
			"cod_payment_count": int64(0),
			"cod_payment_sum":   int64(0),
			"delivery_ms_sum":   int64(0),
			"delivery_ms_count": int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}
